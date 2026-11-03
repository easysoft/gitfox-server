// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package compiler

import (
	"context"
	"strings"
	"testing"

	harness "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
	"github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse"
	normalize "github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse/normalize"
	"github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse/resolver"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/engine"

	"github.com/drone/drone-go/drone"
	"github.com/drone/runner-go/environ/provider"
	"github.com/drone/runner-go/secret"
)

func compileConfig(config *harness.Config, stageName string) (*engine.Spec, error) {
	impl := &CompilerImpl{
		Environ: provider.Static(map[string]string{}),
		Secret:  secret.Static(nil),
	}
	return impl.Compile(context.Background(), Args{
		Config: config,
		Build:  &drone.Build{},
		Stage:  &drone.Stage{Name: stageName},
		Repo:   &drone.Repo{},
		System: &drone.System{},
		Netrc:  &drone.Netrc{},
	})
}

func compileStage(stageSpec *harness.StageCI) (*engine.Spec, error) {
	config := &harness.Config{
		Kind: "pipeline",
		Spec: &harness.Pipeline{
			Stages: []*harness.Stage{{
				Id:   "stage-1",
				Name: "build",
				Type: "ci",
				Spec: stageSpec,
			}},
		},
	}
	return compileConfig(config, "build")
}

func findStep(t *testing.T, spec *engine.Spec, name string) *engine.Step {
	t.Helper()
	for _, step := range spec.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("step %q not found in compiled spec", name)
	return nil
}

func hasMount(step *engine.Step, name, path string) bool {
	for _, m := range step.Volumes {
		if m.Name == name && m.Path == path {
			return true
		}
	}
	return false
}

// 步骤上声明的挂载会编译成容器卷挂载。
func TestCompileRunStepMountsStageVolume(t *testing.T) {
	spec, err := compileStage(&harness.StageCI{
		Volumes: []*harness.Volume{
			{Name: "tmp", Type: "temp", Spec: &harness.VolumeTemp{}},
		},
		Steps: []*harness.Step{{
			Id:   "step-1",
			Name: "run",
			Type: "run",
			Spec: &harness.StepRun{
				Script: harness.Stringorslice{"echo hi"},
				Mount:  []*harness.Mount{{Name: "tmp", Path: "/tmp"}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	step := findStep(t, spec, "run")
	if !hasMount(step, "tmp", "/tmp") {
		t.Fatalf("step volumes = %v, want mount tmp:/tmp", step.Volumes)
	}
	if !hasMount(step, "_workspace", "/gitfox") {
		t.Fatalf("step volumes = %v, want workspace mount", step.Volumes)
	}
}

// 用户把卷挂到工作区路径 /gitfox 时，内置工作区挂载被跳过。
func TestCompileStepMountOverridesWorkspace(t *testing.T) {
	spec, err := compileStage(&harness.StageCI{
		Volumes: []*harness.Volume{
			{Name: "tmp", Type: "temp", Spec: &harness.VolumeTemp{}},
		},
		Steps: []*harness.Step{{
			Id:   "step-1",
			Name: "clone-to-tmp",
			Type: "run",
			Spec: &harness.StepRun{
				Script: harness.Stringorslice{"echo hi"},
				Mount:  []*harness.Mount{{Name: "tmp", Path: "/gitfox"}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	step := findStep(t, spec, "clone-to-tmp")
	if hasMount(step, "_workspace", "/gitfox") {
		t.Fatalf("step volumes = %v, workspace mount should be skipped", step.Volumes)
	}
	if !hasMount(step, "tmp", "/gitfox") {
		t.Fatalf("step volumes = %v, want mount tmp:/gitfox", step.Volumes)
	}

	// 克隆和清理步骤仍然使用内置工作区卷。
	cloneStep := findStep(t, spec, cloneStepName)
	if !hasMount(cloneStep, "_workspace", "/gitfox") {
		t.Fatalf("clone volumes = %v, want workspace mount", cloneStep.Volumes)
	}
}

// 等价写法（末尾斜杠/..）归一化后同样视为挂到工作区路径，内置工作区挂载被跳过。
func TestCompileStepMountOverridesWorkspaceWithEquivalentPath(t *testing.T) {
	spec, err := compileStage(&harness.StageCI{
		Volumes: []*harness.Volume{
			{Name: "tmp", Type: "temp", Spec: &harness.VolumeTemp{}},
		},
		Steps: []*harness.Step{{
			Id:   "step-1",
			Name: "clone-to-tmp",
			Type: "run",
			Spec: &harness.StepRun{
				Script: harness.Stringorslice{"echo hi"},
				Mount:  []*harness.Mount{{Name: "tmp", Path: "/gitfox/"}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	step := findStep(t, spec, "clone-to-tmp")
	if hasMount(step, "_workspace", "/gitfox") {
		t.Fatalf("step volumes = %v, workspace mount should be skipped", step.Volumes)
	}
	if !hasMount(step, "tmp", "/gitfox") {
		t.Fatalf("step volumes = %v, want normalized mount tmp:/gitfox", step.Volumes)
	}
}

// appendWorkspaceMount 自身也按归一化路径比较，防止绕过校验时重复挂载。
// Docker 解析挂载目标时会先做 filepath.Clean，因此以下写法都指向 /gitfox。
func TestAppendWorkspaceMountComparesCleanedPaths(t *testing.T) {
	for _, path := range []string{"/gitfox/", "//gitfox", "/gitfox/.", "/gitfox/src/.."} {
		t.Run(path, func(t *testing.T) {
			step := &engine.Step{
				Name:    "test",
				Volumes: []*engine.VolumeMount{{Name: "tmp", Path: path}},
			}
			appendWorkspaceMount(context.Background(), step, &engine.VolumeMount{Name: "_workspace", Path: "/gitfox"})

			if len(step.Volumes) != 1 || step.Volumes[0].Name != "tmp" {
				t.Fatalf("step volumes = %v, want only the user mount", step.Volumes)
			}
		})
	}
}

// 挂载引用未声明的卷时编译直接失败。
func TestCompileRejectsUndeclaredMountVolume(t *testing.T) {
	_, err := compileStage(&harness.StageCI{
		Steps: []*harness.Step{{
			Id:   "step-1",
			Name: "run",
			Type: "run",
			Spec: &harness.StepRun{
				Script: harness.Stringorslice{"echo hi"},
				Mount:  []*harness.Mount{{Name: "data", Path: "/data"}},
			},
		}},
	})
	if err == nil {
		t.Fatal("Compile: expected error, got nil")
	}
	if !strings.Contains(err.Error(), `mount volume "data" is not declared`) {
		t.Fatalf("Compile error = %q, want undeclared volume error", err)
	}
}

// 设计稿 §6.2 的原始场景：plugin 步骤把 temp 卷挂到 /gitfox，
// 后续 run 步骤把同一卷挂到 /tmp，编译结果里两个步骤都应带上挂载。
func TestCompilePluginStepMountsFromYAML(t *testing.T) {
	data := `kind: pipeline
spec:
  stages:
    - name: build
      type: ci
      spec:
        volumes:
          - name: tmp
            type: temp
            spec: {}
        steps:
          - name: git克隆
            type: plugin
            spec:
              name: scm_git_clone
              mount:
                - name: tmp
                  path: /gitfox
          - name: 自定义脚本
            type: run
            spec:
              mount:
                - name: tmp
                  path: /tmp
              script:
                - ls -l /tmp
`
	config, err := parse.ParseString(data)
	if err != nil {
		t.Fatalf("ParseString: %v", err)
	}

	lookup := func(name, kind, typ, version string) (*harness.Config, error) {
		return &harness.Config{
			Kind: "plugin",
			Type: "step",
			Spec: &harness.PluginStep{
				Step: &harness.Step{
					Type: "script",
					Spec: &harness.StepScript{Image: "alpine", Run: "clone"},
				},
			},
		}, nil
	}
	if err := resolver.Resolve(config, lookup); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if err := normalize.Normalize(config); err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	spec, err := compileConfig(config, "build")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	pluginStep := findStep(t, spec, "git克隆")
	if !hasMount(pluginStep, "tmp", "/gitfox") {
		t.Fatalf("plugin step volumes = %v, want mount tmp:/gitfox", pluginStep.Volumes)
	}
	if hasMount(pluginStep, "_workspace", "/gitfox") {
		t.Fatalf("plugin step volumes = %v, workspace mount should be skipped", pluginStep.Volumes)
	}

	runStep := findStep(t, spec, "自定义脚本")
	if !hasMount(runStep, "tmp", "/tmp") {
		t.Fatalf("run step volumes = %v, want mount tmp:/tmp", runStep.Volumes)
	}
}

// 编译器不支持声明中的卷类型时编译直接失败。
func TestCompileRejectsUnsupportedVolumeType(t *testing.T) {
	_, err := compileStage(&harness.StageCI{
		Volumes: []*harness.Volume{
			{Name: "cache", Type: "claim", Spec: &harness.VolumeClaim{}},
		},
		Steps: []*harness.Step{{
			Id:   "step-1",
			Name: "run",
			Type: "run",
			Spec: &harness.StepRun{Script: harness.Stringorslice{"echo hi"}},
		}},
	})
	if err == nil {
		t.Fatal("Compile: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "is not supported") {
		t.Fatalf("Compile error = %q, want unsupported volume type error", err)
	}
}
