package manager

import (
	"context"
	"fmt"
	"testing"

	"github.com/easysoft/gitfox-community/app/constant"
	executionModel "github.com/easysoft/gitfox-community/app/model/execution"
	pipelineModel "github.com/easysoft/gitfox-community/app/model/pipeline"
	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
	stepModel "github.com/easysoft/gitfox-community/app/model/step"
	"github.com/easysoft/gitfox-community/app/service/pipeline"
	"github.com/easysoft/gitfox-community/app/service/step"
	v1yaml "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
)

const skipStepsDoc = `
kind: pipeline
spec:
  stages:
    - id: prepare
      name: prepare
      type: ci
      spec:
        steps:
          - id: clone
            name: git克隆
            type: run
            spec:
              script:
                - git clone
          - id: grp
            name: 分组任务
            type: group
            spec:
              steps:
                - id: child
                  name: 子任务
                  type: run
                  spec:
                    script:
                      - echo child
    - id: build
      name: build
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

func unexpectedLookup(name, kind, typ, version string) (*v1yaml.Config, error) {
	return nil, fmt.Errorf("unexpected plugin lookup: %s", name)
}

func TestStageStepsFromYAML(t *testing.T) {
	steps, err := StageStepsFromYAML([]byte(skipStepsDoc), unexpectedLookup)
	if err != nil {
		t.Fatalf("StageStepsFromYAML: %v", err)
	}
	if len(steps["prepare"]) != 2 {
		t.Errorf("prepare 任务数 = %d, want 2", len(steps["prepare"]))
	}
	if len(steps["build"]) != 1 {
		t.Errorf("build 任务数 = %d, want 1", len(steps["build"]))
	}
}

// 阶段被跳过后，阶段内的任务需要落库为 skipped；分组任务只落库其中的子任务。
func TestSkippedStepRecords(t *testing.T) {
	steps, err := StageStepsFromYAML([]byte(skipStepsDoc), unexpectedLookup)
	if err != nil {
		t.Fatalf("StageStepsFromYAML: %v", err)
	}

	records := SkippedStepRecords(steps["prepare"], 11, 7)
	if len(records) != 2 {
		t.Fatalf("任务记录数 = %d, want 2", len(records))
	}

	want := []struct {
		code string
		name string
	}{
		{code: "clone_7", name: "git克隆"},
		{code: "child_7", name: "子任务"},
	}
	for i, w := range want {
		record := records[i]
		if record.Code != w.code || record.Name != w.name {
			t.Errorf("records[%d] = %q/%q, want %q/%q", i, record.Code, record.Name, w.code, w.name)
		}
		if record.StageID != 11 {
			t.Errorf("records[%d].StageID = %d, want 11", i, record.StageID)
		}
		if record.SN != i+1 {
			t.Errorf("records[%d].SN = %d, want %d", i, record.SN, i+1)
		}
		if record.Status != constant.CIStatusSkipped {
			t.Errorf("records[%d].Status = %q, want %q", i, record.Status, constant.CIStatusSkipped)
		}
		if record.Started == nil || record.Finished == nil {
			t.Errorf("records[%d] 缺少 started/finished 时间", i)
		}
	}
}

// fakeContentSvc 只提供流水线内容。
type fakeContentSvc struct {
	pipeline.Interface
	file *pipelineModel.PipelineFile
}

func (f *fakeContentSvc) GetPipelineContent(_ context.Context, _ int64) (*pipelineModel.PipelineFile, error) {
	return f.file, nil
}

// fakeSkipStepSvc 记录创建的任务记录。
type fakeSkipStepSvc struct {
	step.Interface
	created []*stepModel.Step
}

func (f *fakeSkipStepSvc) Create(_ context.Context, s *stepModel.Step) error {
	f.created = append(f.created, s)
	return nil
}

// 阶段在 teardown 因运行策略不匹配被跳过时，阶段内的任务同样要落库为 skipped。
func TestTeardownCreateSkippedSteps(t *testing.T) {
	stepSvc := &fakeSkipStepSvc{}
	td := &teardown{
		PipelineSvc: &fakeContentSvc{file: &pipelineModel.PipelineFile{Data: []byte(skipStepsDoc)}},
		StepSvc:     stepSvc,
	}

	execution := &executionModel.Execution{ID: 7, PipelineID: 3}
	stage := &stageModel.Stage{ID: 11, Code: "prepare"}

	if err := td.createSkippedSteps(context.Background(), execution, stage); err != nil {
		t.Fatalf("createSkippedSteps: %v", err)
	}

	// prepare 阶段声明了 2 个任务（clone + 分组内的 child）
	if len(stepSvc.created) != 2 {
		t.Fatalf("任务记录数 = %d, want 2", len(stepSvc.created))
	}
	for _, s := range stepSvc.created {
		if s.StageID != 11 {
			t.Errorf("task.StageID = %d, want 11", s.StageID)
		}
		if s.Status != constant.CIStatusSkipped {
			t.Errorf("task.Status = %q, want %q", s.Status, constant.CIStatusSkipped)
		}
		if s.ErrorLog != "" {
			t.Errorf("被跳过阶段的任务不写跳过原因（errorLog），got %q", s.ErrorLog)
		}
	}
}
