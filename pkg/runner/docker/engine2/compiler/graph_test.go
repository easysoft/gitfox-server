// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package compiler

import (
	"context"
	"slices"
	"testing"

	harness "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
	normalize "github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse/normalize"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/engine"

	"github.com/drone/drone-go/drone"
	"github.com/drone/runner-go/environ/provider"
	"github.com/drone/runner-go/secret"
)

func runStep(id, name string, when *harness.When) *harness.Step {
	return &harness.Step{
		Id:   id,
		Name: name,
		Type: "run",
		When: when,
		Spec: &harness.StepRun{
			Container: &harness.Container{Image: "alpine"},
			Script:    harness.Stringorslice{"echo " + name},
		},
	}
}

func compileGraph(t *testing.T, steps []*harness.Step, buildID int64) *engine.Spec {
	t.Helper()
	return compileGraphWithBuild(t, steps, &drone.Build{ID: buildID})
}

func compileGraphWithBuild(t *testing.T, steps []*harness.Step, build *drone.Build) *engine.Spec {
	t.Helper()
	config := &harness.Config{
		Kind: "pipeline",
		Spec: &harness.Pipeline{
			Options: &harness.Default{
				Clone: &harness.Clone{Disabled: true},
			},
			Stages: []*harness.Stage{{
				Id:   "stage-1",
				Name: "build",
				Type: "ci",
				Spec: &harness.StageCI{Steps: steps},
			}},
		},
	}
	impl := &CompilerImpl{
		Environ: provider.Static(map[string]string{}),
		Secret:  secret.Static(nil),
	}
	spec, err := impl.Compile(context.Background(), Args{
		Config: config,
		Build:  build,
		Stage:  &drone.Stage{Name: "build"},
		Repo:   &drone.Repo{},
		System: &drone.System{},
		Netrc:  &drone.Netrc{},
	})
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}
	return spec
}

func graphStep(t *testing.T, spec *engine.Spec, id string) *engine.Step {
	t.Helper()
	for _, step := range spec.Steps {
		if step.ID == id {
			return step
		}
	}
	t.Fatalf("step with ID %q not found", id)
	return nil
}

func assertDeps(t *testing.T, step *engine.Step, want ...string) {
	t.Helper()
	if !slices.Equal(step.DependsOn, want) {
		t.Fatalf("step %q DependsOn = %v, want %v", step.ID, step.DependsOn, want)
	}
}

// 纯串行：A → B → clean，与旧的 configureSerial 结果一致。
func TestCompileSerialGraph(t *testing.T) {
	spec := compileGraph(t, []*harness.Step{
		runStep("a", "A", nil),
		runStep("b", "B", nil),
	}, 42)

	assertDeps(t, graphStep(t, spec, "a_42"))
	assertDeps(t, graphStep(t, spec, "b_42"), "a_42")
	assertDeps(t, findStep(t, spec, "clean"), "b_42")
}

// 中间 parallel：setup → [frontend, backend] → after → clean。
func TestCompileParallelGraph(t *testing.T) {
	spec := compileGraph(t, []*harness.Step{
		runStep("setup", "初始化", nil),
		{
			Id:   "build-parallel",
			Name: "并行构建",
			Type: "parallel",
			Spec: &harness.StepParallel{Steps: []*harness.Step{
				runStep("frontend", "前端构建", nil),
				runStep("backend", "后端构建", nil),
			}},
		},
		runStep("after", "汇总", nil),
	}, 42)

	assertDeps(t, graphStep(t, spec, "setup_42"))
	assertDeps(t, graphStep(t, spec, "frontend_42"), "setup_42")
	assertDeps(t, graphStep(t, spec, "backend_42"), "setup_42")
	assertDeps(t, graphStep(t, spec, "after_42"), "frontend_42", "backend_42")
	assertDeps(t, findStep(t, spec, "clean"), "after_42")
}

// 开头 parallel（clone disabled）：子 step 均无依赖，不会被串行化。
func TestCompileRootParallelGraph(t *testing.T) {
	spec := compileGraph(t, []*harness.Step{
		{
			Id:   "root-parallel",
			Name: "并行入口",
			Type: "parallel",
			Spec: &harness.StepParallel{Steps: []*harness.Step{
				runStep("a", "A", nil),
				runStep("b", "B", nil),
			}},
		},
	}, 42)

	assertDeps(t, graphStep(t, spec, "a_42"))
	assertDeps(t, graphStep(t, spec, "b_42"))
	assertDeps(t, findStep(t, spec, "clean"), "a_42", "b_42")
}

// parallel 内嵌 group：group 内部串行，出口是最后一个子 step。
func TestCompileNestedGroupGraph(t *testing.T) {
	spec := compileGraph(t, []*harness.Step{
		{
			Id:   "root-parallel",
			Name: "并行入口",
			Type: "parallel",
			Spec: &harness.StepParallel{Steps: []*harness.Step{
				runStep("a", "A", nil),
				{
					Id:   "group",
					Name: "串行组",
					Type: "group",
					Spec: &harness.StepGroup{Steps: []*harness.Step{
						runStep("b", "B", nil),
						runStep("c", "C", nil),
					}},
				},
			}},
		},
		runStep("after", "汇总", nil),
	}, 42)

	assertDeps(t, graphStep(t, spec, "a_42"))
	assertDeps(t, graphStep(t, spec, "b_42"))
	assertDeps(t, graphStep(t, spec, "c_42"), "b_42")
	assertDeps(t, graphStep(t, spec, "after_42"), "a_42", "c_42")
}

// 父级 when 会继承到未声明 when 的子 step；always 不能被用来放宽父级限制。
func TestCompileParallelWhenInheritance(t *testing.T) {
	spec := compileGraph(t, []*harness.Step{
		{
			Id:   "always-parallel",
			Name: "always",
			Type: "parallel",
			When: &harness.When{Eval: "always"},
			Spec: &harness.StepParallel{Steps: []*harness.Step{
				runStep("a", "A", nil),
			}},
		},
		{
			Id:   "success-parallel",
			Name: "onSuccess",
			Type: "parallel",
			When: &harness.When{Eval: "onSuccess"},
			Spec: &harness.StepParallel{Steps: []*harness.Step{
				runStep("b", "B", &harness.When{Eval: "onFailure"}),
				runStep("c", "C", &harness.When{Eval: "always"}),
			}},
		},
	}, 42)

	if got := graphStep(t, spec, "a_42").RunPolicy; got != engine.RunAlways {
		t.Fatalf("a_42 RunPolicy = %v, want RunAlways", got)
	}
	if got := graphStep(t, spec, "b_42").RunPolicy; got != engine.RunNever {
		t.Fatalf("b_42 RunPolicy = %v, want RunNever", got)
	}
	if got := graphStep(t, spec, "c_42").RunPolicy; got != engine.RunOnSuccess {
		t.Fatalf("c_42 RunPolicy = %v, want RunOnSuccess", got)
	}
}

// 复合表达式里的 always() 不能放宽祖先 group/parallel 的 when 约束（fail-open 回归）。
func TestCompileParallelCompositeAlwaysWhen(t *testing.T) {
	spec := compileGraphWithBuild(t, []*harness.Step{
		{
			Id:   "failure-parallel",
			Name: "onFailure",
			Type: "parallel",
			When: &harness.When{Eval: "onFailure"},
			Spec: &harness.StepParallel{Steps: []*harness.Step{
				runStep("a", "A", &harness.When{Eval: `${{ always() && build.event == 'push' }}`}),
			}},
		},
		{
			Id:   "always-parallel",
			Name: "always",
			Type: "parallel",
			When: &harness.When{Eval: `${{ always() && build.event == 'push' }}`},
			Spec: &harness.StepParallel{Steps: []*harness.Step{
				runStep("b", "B", &harness.When{Eval: "onSuccess"}),
			}},
		},
	}, &drone.Build{ID: 42, Event: "push"})

	if got := graphStep(t, spec, "a_42").RunPolicy; got != engine.RunOnFailure {
		t.Fatalf("a_42 RunPolicy = %v, want RunOnFailure", got)
	}
	if got := graphStep(t, spec, "b_42").RunPolicy; got != engine.RunOnSuccess {
		t.Fatalf("b_42 RunPolicy = %v, want RunOnSuccess", got)
	}
}

// 多层 group/parallel 的 when 逐层按 AND 合成。
func TestCompileNestedWhenInheritance(t *testing.T) {
	spec := compileGraph(t, []*harness.Step{
		{
			Id:   "outer",
			Name: "outer",
			Type: "parallel",
			When: &harness.When{Eval: "onSuccess"},
			Spec: &harness.StepParallel{Steps: []*harness.Step{
				{
					Id:   "inner",
					Name: "inner",
					Type: "group",
					When: &harness.When{Eval: "onFailure"},
					Spec: &harness.StepGroup{Steps: []*harness.Step{
						runStep("a", "A", nil),
					}},
				},
			}},
		},
	}, 42)

	if got := graphStep(t, spec, "a_42").RunPolicy; got != engine.RunNever {
		t.Fatalf("a_42 RunPolicy = %v, want RunNever", got)
	}
}

// 允许同名 step：执行身份是 ID，依赖接线不受显示名影响。
func TestCompileDuplicateDisplayName(t *testing.T) {
	spec := compileGraph(t, []*harness.Step{
		runStep("a", "构建", nil),
		runStep("b", "构建", nil),
	}, 42)

	if graphStep(t, spec, "a_42").Name != "构建" || graphStep(t, spec, "b_42").Name != "构建" {
		t.Fatal("同名 step 的 Name 应保持原样")
	}
	assertDeps(t, graphStep(t, spec, "b_42"), "a_42")
}

// 走一遍真实的 YAML → normalize → compile 路径，确认 parallel 能编译成目标图。
func TestCompileParallelFromYAML(t *testing.T) {
	doc := `
version: 1
kind: pipeline
spec:
  options:
    clone:
      disabled: true
  stages:
    - id: demo
      name: 阶段
      type: ci
      spec:
        platform:
          os: linux
          arch: amd64
        steps:
          - id: setup
            name: 初始化
            type: run
            spec:
              container:
                image: alpine:3.20
              script:
                - echo setup
          - id: build-parallel
            name: 并行构建
            type: parallel
            spec:
              steps:
                - id: frontend
                  name: 前端构建
                  type: run
                  spec:
                    container:
                      image: node:20
                    script:
                      - echo frontend
                - id: backend
                  name: 后端构建
                  type: run
                  spec:
                    container:
                      image: golang:1.22
                    script:
                      - echo backend
          - id: after
            name: 汇总
            type: run
            spec:
              container:
                image: alpine:3.20
              script:
                - echo after
`
	config, err := harness.ParseString(doc)
	if err != nil {
		t.Fatalf("ParseString() error: %v", err)
	}
	if err := normalize.Normalize(config); err != nil {
		t.Fatalf("Normalize() error: %v", err)
	}
	spec, err := compileConfig(config, "阶段")
	if err != nil {
		t.Fatalf("Compile() error: %v", err)
	}

	assertDeps(t, findStep(t, spec, "初始化"))
	assertDeps(t, findStep(t, spec, "前端构建"), "setup_0")
	assertDeps(t, findStep(t, spec, "后端构建"), "setup_0")
	assertDeps(t, findStep(t, spec, "汇总"), "frontend_0", "backend_0")
	assertDeps(t, findStep(t, spec, "clean"), "after_0")
}
