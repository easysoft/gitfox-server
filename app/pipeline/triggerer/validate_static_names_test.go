package triggerer

import (
	"context"
	"strings"
	"testing"

	executionModel "github.com/easysoft/gitfox-community/app/model/execution"
	v1yaml "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
)

func runStepForNameTest(id, name string) *v1yaml.Step {
	return &v1yaml.Step{
		Id:   id,
		Name: name,
		Type: "run",
		Spec: &v1yaml.StepRun{},
	}
}

// stage/step 的 id、name 不支持表达式：服务端与 runner 的表达式上下文不同，
// 名称一旦依赖 secrets/env 就会产生落库名称不一致或 stage 匹配失败。
func TestValidateV1StaticNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pipeline *v1yaml.Pipeline
		wantErr  string
	}{
		{
			name: "静态名称合法",
			pipeline: &v1yaml.Pipeline{Stages: []*v1yaml.Stage{{
				Id:   "build",
				Name: "构建",
				Spec: &v1yaml.StageCI{Steps: []*v1yaml.Step{
					runStepForNameTest("compile", "编译"),
					{
						Id:   "group",
						Name: "分组",
						Type: "group",
						Spec: &v1yaml.StepGroup{Steps: []*v1yaml.Step{
							runStepForNameTest("child", "子任务"),
						}},
					},
					{
						Id:   "parallel",
						Name: "并行",
						Type: "parallel",
						Spec: &v1yaml.StepParallel{Steps: []*v1yaml.Step{
							runStepForNameTest("p1", "并行任务"),
						}},
					},
				}},
			}}},
		},
		{
			name: "stage id 含表达式",
			pipeline: &v1yaml.Pipeline{Stages: []*v1yaml.Stage{{
				Id:   "build-${{ parameters.target }}",
				Name: "构建",
				Spec: &v1yaml.StageCI{Steps: []*v1yaml.Step{runStepForNameTest("compile", "编译")}},
			}}},
			wantErr: "stage id",
		},
		{
			name: "stage name 含表达式",
			pipeline: &v1yaml.Pipeline{Stages: []*v1yaml.Stage{{
				Id:   "build",
				Name: "构建-${{ build.branch }}",
				Spec: &v1yaml.StageCI{Steps: []*v1yaml.Step{runStepForNameTest("compile", "编译")}},
			}}},
			wantErr: "stage name",
		},
		{
			name: "step id 含表达式",
			pipeline: &v1yaml.Pipeline{Stages: []*v1yaml.Stage{{
				Id:   "build",
				Name: "构建",
				Spec: &v1yaml.StageCI{Steps: []*v1yaml.Step{
					runStepForNameTest("compile-${{ parameters.target }}", "编译"),
				}},
			}}},
			wantErr: "step id",
		},
		{
			name: "step name 含表达式",
			pipeline: &v1yaml.Pipeline{Stages: []*v1yaml.Stage{{
				Id:   "build",
				Name: "构建",
				Spec: &v1yaml.StageCI{Steps: []*v1yaml.Step{
					runStepForNameTest("compile", "编译-${{ secrets.get(\"X\") }}"),
				}},
			}}},
			wantErr: "step name",
		},
		{
			name: "group 子任务 name 含表达式",
			pipeline: &v1yaml.Pipeline{Stages: []*v1yaml.Stage{{
				Id:   "build",
				Name: "构建",
				Spec: &v1yaml.StageCI{Steps: []*v1yaml.Step{{
					Id:   "group",
					Name: "分组",
					Type: "group",
					Spec: &v1yaml.StepGroup{Steps: []*v1yaml.Step{
						runStepForNameTest("child", "子任务-${{ env.CI_COMMIT_SHA }}"),
					}},
				}}},
			}}},
			wantErr: "step name",
		},
		{
			name: "parallel 子任务 id 含表达式",
			pipeline: &v1yaml.Pipeline{Stages: []*v1yaml.Stage{{
				Id:   "build",
				Name: "构建",
				Spec: &v1yaml.StageCI{Steps: []*v1yaml.Step{{
					Id:   "parallel",
					Name: "并行",
					Type: "parallel",
					Spec: &v1yaml.StepParallel{Steps: []*v1yaml.Step{
						runStepForNameTest("p1-${{ build.branch }}", "并行任务"),
					}},
				}}},
			}}},
			wantErr: "step id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateV1StaticNames(test.pipeline)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", test.wantErr)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %q, want containing %q", err.Error(), test.wantErr)
			}
		})
	}
}

const staticNamesPipelineHeader = `
kind: pipeline
spec:
  stages:
`

// 校验必须发生在 normalize 之前：normalize 会把显式 id 中的表达式 slug 化，
// 之后再校验就识别不出来了。
func TestParseV1StagesRejectsExpressionsInNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		doc     string
		wantErr string
	}{
		{
			name: "stage id 含表达式",
			doc: staticNamesPipelineHeader + `
    - id: build-${{ parameters.target }}
      name: 构建
      type: ci
      spec:
        steps:
          - id: compile
            name: 编译
            type: run
            spec:
              script:
                - make
`,
			wantErr: "stage id",
		},
		{
			name: "stage name 含表达式",
			doc: staticNamesPipelineHeader + `
    - id: build
      name: 构建-${{ build.branch }}
      type: ci
      spec:
        steps:
          - id: compile
            name: 编译
            type: run
            spec:
              script:
                - make
`,
			wantErr: "stage name",
		},
		{
			name: "step id 含表达式",
			doc: staticNamesPipelineHeader + `
    - id: build
      name: 构建
      type: ci
      spec:
        steps:
          - id: compile-${{ parameters.target }}
            name: 编译
            type: run
            spec:
              script:
                - make
`,
			wantErr: "step id",
		},
		{
			name: "step name 含表达式",
			doc: staticNamesPipelineHeader + `
    - id: build
      name: 构建
      type: ci
      spec:
        steps:
          - id: compile
            name: 编译-${{ secrets.get("X") }}
            type: run
            spec:
              script:
                - make
`,
			wantErr: "step name",
		},
		{
			name: "group 子任务 name 含表达式",
			doc: staticNamesPipelineHeader + `
    - id: build
      name: 构建
      type: ci
      spec:
        steps:
          - id: group
            name: 分组
            type: group
            spec:
              steps:
                - id: child
                  name: 子任务-${{ env.CI_COMMIT_SHA }}
                  type: run
                  spec:
                    script:
                      - echo child
`,
			wantErr: "step name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseV1Stages(
				context.Background(),
				[]byte(test.doc),
				&executionModel.Execution{},
				&Hook{},
				nil,
			)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", test.wantErr)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %q, want containing %q", err.Error(), test.wantErr)
			}
		})
	}
}

// 静态名称不受影响，仍可正常解析出阶段。
func TestParseV1StagesAllowsStaticNames(t *testing.T) {
	t.Parallel()

	doc := staticNamesPipelineHeader + `
    - id: build
      name: 构建
      type: ci
      spec:
        steps:
          - id: compile
            name: 编译
            type: run
            spec:
              script:
                - make
`
	stages, err := parseV1Stages(
		context.Background(),
		[]byte(doc),
		&executionModel.Execution{},
		&Hook{},
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stages) != 1 || stages[0].Code != "build" || stages[0].Name != "构建" {
		t.Fatalf("stages = %+v, want one stage build/构建", stages)
	}
}
