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

// directRunPolicy maps the supported direct policy spellings to a policy.
// success()/failure()/always() are included so a standalone expression can
// participate in the same AND merge as their direct forms.
func directRunPolicy(s string) (constant.RunPolicy, bool) {
	switch s {
	case string(constant.RunPolicyAlways), "always()":
		return constant.RunPolicyAlways, true
	case string(constant.RunPolicyOnSuccess), "success()":
		return constant.RunPolicyOnSuccess, true
	case string(constant.RunPolicyOnFailure), "failure()":
		return constant.RunPolicyOnFailure, true
	}
	return "", false
}

// mergeRunPolicy implements AND semantics for direct policies. always is
// neutral; onSuccess + onFailure is never. ok=false means Never.
func mergeRunPolicy(a, b constant.RunPolicy) (constant.RunPolicy, bool) {
	if a == constant.RunPolicyAlways {
		return b, true
	}
	if b == constant.RunPolicyAlways {
		return a, true
	}
	if a == b {
		return a, true
	}
	return "", false
}

// combineWhen merges a parent group/parallel when with a child when.
// always is neutral and cannot loosen a stricter parent policy.
func combineWhen(parent, child *harness.When) *harness.When {
	p := whenExpr(parent)
	c := whenExpr(child)
	switch {
	case p == "":
		return child
	case c == "":
		return parent
	case p == c:
		return &harness.When{Eval: p}
	}

	pp, pok := directRunPolicy(p)
	cp, cok := directRunPolicy(c)
	switch {
	case pok && cok:
		if merged, ok := mergeRunPolicy(pp, cp); ok {
			return &harness.When{Eval: string(merged)}
		}
		return &harness.When{Eval: "false"}
	case pok:
		switch pp {
		case constant.RunPolicyAlways:
			return &harness.When{Eval: c}
		case constant.RunPolicyOnSuccess:
			return &harness.When{Eval: "success() && (" + c + ")"}
		case constant.RunPolicyOnFailure:
			return &harness.When{Eval: "failure() && (" + c + ")"}
		}
	case cok:
		switch cp {
		case constant.RunPolicyAlways:
			return &harness.When{Eval: p}
		case constant.RunPolicyOnSuccess:
			return &harness.When{Eval: "(" + p + ") && success()"}
		case constant.RunPolicyOnFailure:
			return &harness.When{Eval: "(" + p + ") && failure()"}
		}
	default:
		return &harness.When{Eval: "(" + p + ") && (" + c + ")"}
	}
	return child
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
