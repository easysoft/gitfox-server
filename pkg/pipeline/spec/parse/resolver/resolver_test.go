// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package resolver

import (
	"strings"
	"testing"

	schema "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
)

func pluginPipeline(stepName string, mounts []*schema.Mount) *schema.Config {
	return &schema.Config{
		Kind: "pipeline",
		Spec: &schema.Pipeline{
			Stages: []*schema.Stage{{
				Type: "ci",
				Spec: &schema.StageCI{
					Steps: []*schema.Step{{
						Name: stepName,
						Type: "plugin",
						Spec: &schema.StepPlugin{
							Name:  "scm_git_clone",
							Mount: mounts,
						},
					}},
				},
			}},
		},
	}
}

func stepPluginLookup(step *schema.Step) LookupFunc {
	return func(name, kind, typ, version string) (*schema.Config, error) {
		return &schema.Config{
			Kind: "plugin",
			Type: "step",
			Spec: &schema.PluginStep{Step: step},
		}, nil
	}
}

func resolvedStep(t *testing.T, config *schema.Config) *schema.Step {
	t.Helper()
	pipeline, ok := config.Spec.(*schema.Pipeline)
	if !ok {
		t.Fatalf("config spec = %T, want *schema.Pipeline", config.Spec)
	}
	stageCI, ok := pipeline.Stages[0].Spec.(*schema.StageCI)
	if !ok {
		t.Fatalf("stage spec = %T, want *schema.StageCI", pipeline.Stages[0].Spec)
	}
	return stageCI.Steps[0]
}

func assertMounts(t *testing.T, got, want []*schema.Mount) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("mounts = %v, want %v", describeMounts(got), describeMounts(want))
	}
	for i := range want {
		if got[i].Name != want[i].Name || got[i].Path != want[i].Path {
			t.Fatalf("mounts = %v, want %v", describeMounts(got), describeMounts(want))
		}
	}
}

func describeMounts(mounts []*schema.Mount) []string {
	out := make([]string, 0, len(mounts))
	for _, m := range mounts {
		out = append(out, m.Name+":"+m.Path)
	}
	return out
}

// plugin 步骤上声明的 mount 需要合并到展开后的 StepScript 上。
func TestResolvePluginStepInheritsMounts(t *testing.T) {
	config := pluginPipeline("git克隆", []*schema.Mount{{Name: "tmp", Path: "/gitfox"}})
	lookup := stepPluginLookup(&schema.Step{
		Type: "script",
		Spec: &schema.StepScript{Image: "alpine", Run: "echo hi"},
	})

	if err := Resolve(config, lookup); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	step := resolvedStep(t, config)
	spec, ok := step.Spec.(*schema.StepScript)
	if !ok {
		t.Fatalf("step spec = %T, want *schema.StepScript", step.Spec)
	}
	assertMounts(t, spec.Mount, []*schema.Mount{{Name: "tmp", Path: "/gitfox"}})
}

// 用户没有声明 mount 时，插件定义里的 mount 原样保留。
func TestResolvePluginStepKeepsDefinitionMounts(t *testing.T) {
	config := pluginPipeline("build", nil)
	lookup := stepPluginLookup(&schema.Step{
		Type: "script",
		Spec: &schema.StepScript{
			Image: "alpine",
			Mount: []*schema.Mount{{Name: "cache", Path: "/cache"}},
		},
	})

	if err := Resolve(config, lookup); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	step := resolvedStep(t, config)
	spec := step.Spec.(*schema.StepScript)
	assertMounts(t, spec.Mount, []*schema.Mount{{Name: "cache", Path: "/cache"}})
}

// 同一 path 的挂载由用户声明覆盖插件定义，其余 path 保留。
func TestResolvePluginStepUserMountOverridesSamePath(t *testing.T) {
	config := pluginPipeline("build", []*schema.Mount{{Name: "tmp", Path: "/cache"}})
	lookup := stepPluginLookup(&schema.Step{
		Type: "script",
		Spec: &schema.StepScript{
			Image: "alpine",
			Mount: []*schema.Mount{
				{Name: "cache", Path: "/cache"},
				{Name: "logs", Path: "/logs"},
			},
		},
	})

	if err := Resolve(config, lookup); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	step := resolvedStep(t, config)
	spec := step.Spec.(*schema.StepScript)
	assertMounts(t, spec.Mount, []*schema.Mount{
		{Name: "logs", Path: "/logs"},
		{Name: "tmp", Path: "/cache"},
	})
}

// 插件展开后的类型不支持挂载时返回明确错误，而不是静默丢弃。
func TestResolvePluginStepMountUnsupported(t *testing.T) {
	config := pluginPipeline("group", []*schema.Mount{{Name: "tmp", Path: "/tmp"}})
	lookup := stepPluginLookup(&schema.Step{
		Type: "group",
		Spec: &schema.StepGroup{},
	})

	err := Resolve(config, lookup)
	if err == nil {
		t.Fatal("Resolve: expected error, got nil")
	}
	if !strings.Contains(err.Error(), `does not support mount`) {
		t.Fatalf("Resolve error = %q, want it to mention unsupported mount", err)
	}
}

// 同一来源内重复 path 不做合并，交给校验器报错。
func TestMergeMountsKeepsDuplicateOverridePaths(t *testing.T) {
	base := []*schema.Mount{{Name: "cache", Path: "/data"}}
	override := []*schema.Mount{
		{Name: "a", Path: "/data"},
		{Name: "b", Path: "/data"},
	}

	got := mergeMounts(base, override)
	assertMounts(t, got, override)
}
