// Copyright 2019 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package compiler

import (
	"strings"

	"github.com/easysoft/gitfox-community/app/constant"
	harness "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine/resource"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/engine"

	"github.com/drone/drone-go/drone"
	"github.com/drone/runner-go/manifest"
)

// helper function returns true if the step is configured to
// always run regardless of status.
func isRunAlways(step *resource.Step) bool {
	if len(step.When.Status.Include) == 0 &&
		len(step.When.Status.Exclude) == 0 {
		return false
	}
	return step.When.Status.Match(drone.StatusFailing) &&
		step.When.Status.Match(drone.StatusPassing)
}

// helper function returns true if the step is configured to
// only run on failure.
func isRunOnFailure(step *resource.Step) bool {
	if len(step.When.Status.Include) == 0 &&
		len(step.When.Status.Exclude) == 0 {
		return false
	}
	return step.When.Status.Match(drone.StatusFailing)
}

// whenExpr returns the normalized expression of a when clause. Direct policy
// values (always/onSuccess/onFailure) are returned as-is.
func whenExpr(w *harness.When) string {
	if w == nil {
		return ""
	}
	s := strings.TrimSpace(w.Eval)
	if strings.HasPrefix(s, "${{") && strings.HasSuffix(s, "}}") {
		s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "${{"), "}}"))
	}
	return s
}

// runPolicyPair 表示一个 when 条件在流水线“成功/失败”两种状态下的求值结果：
// onSuccess / onFailure 为 true 表示对应状态下步骤需要执行。
type runPolicyPair struct {
	onSuccess bool
	onFailure bool
}

// runPolicyAlways 不限制步骤的执行状态（always 或没有 when 条件），是 AND 合成中的中性元。
var runPolicyAlways = runPolicyPair{onSuccess: true, onFailure: true}

// and 按 AND 语义合成两个 when 条件：只有两种状态下都成立时步骤才执行。
func (p runPolicyPair) and(q runPolicyPair) runPolicyPair {
	return runPolicyPair{
		onSuccess: p.onSuccess && q.onSuccess,
		onFailure: p.onFailure && q.onFailure,
	}
}

// enginePolicy 把 when 求值结果转换为引擎的运行策略。
func (p runPolicyPair) enginePolicy() engine.RunPolicy {
	switch {
	case p.onSuccess && p.onFailure:
		return engine.RunAlways
	case p.onSuccess:
		return engine.RunOnSuccess
	case p.onFailure:
		return engine.RunOnFailure
	default:
		return engine.RunNever
	}
}

// directRunPolicy maps the supported direct policy spellings to a policy.
// pipelineSuccess, pipelineFailure, and always are the canonical condition names.
func directRunPolicy(s string) (runPolicyPair, bool) {
	switch s {
	case string(constant.RunPolicyAlways), "always()":
		return runPolicyAlways, true
	case string(constant.RunPolicyOnSuccess), "success", "success()",
		harness.WhenPipelineSuccess, harness.WhenPipelineSuccess + "()":
		return runPolicyPair{onSuccess: true}, true
	case string(constant.RunPolicyOnFailure), "failure", "failure()",
		harness.WhenPipelineFailure, harness.WhenPipelineFailure + "()":
		return runPolicyPair{onFailure: true}, true
	}
	return runPolicyPair{}, false
}

// evalWhen 对单条 when 条件求值。直接策略值不是合法表达式，先按字面量识别；
// 空条件在 AND 合成中是中性元。
func evalWhen(w *harness.When, inputs map[string]interface{}) (runPolicyPair, error) {
	code := whenExpr(w)
	if code == "" {
		return runPolicyAlways, nil
	}
	if policy, ok := directRunPolicy(code); ok {
		return policy, nil
	}
	onSuccess, onFailure, err := evalif(code, inputs)
	if err != nil {
		return runPolicyPair{}, err
	}
	return runPolicyPair{onSuccess: onSuccess, onFailure: onFailure}, nil
}

// appendWhen 返回追加了 w 的 when 链。分配新切片，避免并行分支之间共用底层数组。
func appendWhen(parents []*harness.When, w *harness.When) []*harness.When {
	if whenExpr(w) == "" {
		return parents
	}
	out := make([]*harness.When, len(parents)+1)
	copy(out, parents)
	out[len(parents)] = w
	return out
}

// helper function converts the environment variables to a map,
// returning only inline environment variables not derived from
// a secret.
func convertStaticEnv(src map[string]*manifest.Variable) map[string]string {
	dst := map[string]string{}
	for k, v := range src {
		if v == nil {
			continue
		}
		if strings.TrimSpace(v.Secret) == "" {
			dst[k] = v.Value
		}
	}
	return dst
}

// helper function converts the environment variables to a map,
// returning only inline environment variables not derived from
// a secret.
func convertSecretEnv(src map[string]*manifest.Variable) []*engine.Secret {
	dst := []*engine.Secret{}
	for k, v := range src {
		if v == nil {
			continue
		}
		if strings.TrimSpace(v.Secret) != "" {
			dst = append(dst, &engine.Secret{
				Name: v.Secret,
				Mask: true,
				Env:  k,
			})
		}
	}
	return dst
}

// helper function modifies the pipeline dependency graph to
// account for the clone step.
func convertPullPolicy(s string) engine.PullPolicy {
	switch strings.ToLower(s) {
	case "always":
		return engine.PullAlways
	case "if-not-exists":
		return engine.PullIfNotExists
	case "never":
		return engine.PullNever
	default:
		return engine.PullDefault
	}
}

// helper function returns true if the environment variable
// is restricted for internal-use only.
func isRestrictedVariable(env map[string]string) bool {
	for _, name := range restrictedVars {
		if _, ok := env[name]; ok {
			return true
		}
	}
	return false
}

// list of restricted variables
var restrictedVars = []string{
	"XDG_RUNTIME_DIR",
	"DOCKER_OPTS",
	"DOCKER_HOST",
	"PATH",
	"HOME",
}
