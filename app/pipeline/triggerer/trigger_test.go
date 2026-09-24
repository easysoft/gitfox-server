package triggerer

import (
	"context"
	"fmt"
	"testing"

	"github.com/easysoft/gitfox-community/app/constant"
	executionModel "github.com/easysoft/gitfox-community/app/model/execution"
	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
	stepModel "github.com/easysoft/gitfox-community/app/model/step"
	stepSvc "github.com/easysoft/gitfox-community/app/service/step"
	v1yaml "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
)

// whenSkipDoc 是 when 条件跳过的样例流水线：
// prepare 条件不成立、deploy 条件成立、notify 用 always。
const whenSkipDoc = `
kind: pipeline
spec:
  stages:
    - id: prepare
      name: prepare
      type: ci
      when: ${{ build.ref == 'refs/heads/main' }}
      spec:
        steps:
          - id: run
            name: run
            type: run
            spec:
              script:
                - echo prepare
    - id: build
      name: build
      type: ci
      spec:
        steps:
          - id: run
            name: run
            type: run
            spec:
              script:
                - echo build
    - id: deploy
      name: deploy
      type: ci
      when: ${{ build.ref == 'refs/heads/release' }}
      spec:
        steps:
          - id: run
            name: run
            type: run
            spec:
              script:
                - echo deploy
    - id: notify
      name: notify
      type: ci
      when: always
      spec:
        steps:
          - id: run
            name: run
            type: run
            spec:
              script:
                - echo notify
`

// when 条件在成功、失败两种状态下都不成立时，阶段应当真跳过：
// 落库状态为 skipped，且不再作为下游阶段的依赖。
func TestParseV1StagesWhenSkip(t *testing.T) {
	execution := &executionModel.Execution{
		ID:      1,
		Trigger: string(constant.TriggerActionBranchUpdated),
		Ref:     "refs/heads/release",
	}
	hook := &Hook{
		Action: constant.TriggerActionBranchUpdated,
		Ref:    "refs/heads/release",
	}

	stages, err := parseV1Stages(context.Background(), []byte(whenSkipDoc), execution, hook, nil)
	if err != nil {
		t.Fatalf("parseV1Stages: %v", err)
	}
	if len(stages) != 4 {
		t.Fatalf("stage 数量 = %d, want 4", len(stages))
	}

	want := []struct {
		code      string
		status    constant.CIStatus
		runPolicy string
		depends   []string
	}{
		// when 条件对成功/失败两种状态都不成立 -> 真跳过
		{code: "prepare", status: constant.CIStatusSkipped, runPolicy: string(constant.RunPolicyOnSuccess)},
		// 跳过的阶段不再是下游依赖，因此 build 直接被调度
		{code: "build", status: constant.CIStatusPending, runPolicy: string(constant.RunPolicyOnSuccess)},
		// 条件只依赖 build.ref（与流水线状态无关）且当前成立 -> 成功后执行，依赖 build
		{code: "deploy", status: constant.CIStatusWaitingOnDeps, runPolicy: string(constant.RunPolicyOnSuccess), depends: []string{"build"}},
		// always 策略
		{code: "notify", status: constant.CIStatusWaitingOnDeps, runPolicy: string(constant.RunPolicyAlways), depends: []string{"deploy"}},
	}

	for i, w := range want {
		got := stages[i]
		if got.Code != w.code {
			t.Errorf("stages[%d].Code = %q, want %q", i, got.Code, w.code)
		}
		if got.Status != w.status {
			t.Errorf("stage %s status = %q, want %q", w.code, got.Status, w.status)
		}
		if got.RunPolicy != w.runPolicy {
			t.Errorf("stage %s runPolicy = %q, want %q", w.code, got.RunPolicy, w.runPolicy)
		}
		if len(got.Depends) != len(w.depends) {
			t.Errorf("stage %s depends = %v, want %v", w.code, got.Depends, w.depends)
			continue
		}
		for j := range w.depends {
			if got.Depends[j] != w.depends[j] {
				t.Errorf("stage %s depends = %v, want %v", w.code, got.Depends, w.depends)
			}
		}
	}

	if stages[0].Started == nil || stages[0].Finished == nil {
		t.Error("跳过的阶段应当写上 started/finished 时间，避免前端展示为空")
	}

	// 参考错误信息的方式：跳过的阶段状态仍为 skipped，跳过原因写入 errorLog 返回给前端。
	wantSkipLog := fmt.Sprintf(v1yaml.WhenSkippedStageTmpl, `${{ build.ref == 'refs/heads/main' }}`)
	if stages[0].ErrorLog != wantSkipLog {
		t.Errorf("跳过的阶段 errorLog = %q, want %q", stages[0].ErrorLog, wantSkipLog)
	}
	for _, s := range stages[1:] {
		if s.ErrorLog != "" {
			t.Errorf("阶段 %s 未被跳过，errorLog 应为空，实际 %q", s.Code, s.ErrorLog)
		}
	}
}

func TestAllStagesSkipped(t *testing.T) {
	if allStagesSkipped(nil) {
		t.Error("空 stage 列表不算全部跳过")
	}
	if !allStagesSkipped([]*stageModel.Stage{{Status: constant.CIStatusSkipped}}) {
		t.Error("全部 skipped 时应当返回 true")
	}
	if allStagesSkipped([]*stageModel.Stage{{Status: constant.CIStatusSkipped}, {Status: constant.CIStatusPending}}) {
		t.Error("存在待执行阶段时不应返回 true")
	}
}

// fakeStepSvc 只记录创建的任务记录，用于断言被跳过阶段的任务是否落库。
type fakeStepSvc struct {
	stepSvc.Interface
	created []*stepModel.Step
}

func (f *fakeStepSvc) Create(_ context.Context, s *stepModel.Step) error {
	s.ID = uint(100 + len(f.created))
	f.created = append(f.created, s)
	return nil
}

// 触发时阶段被 when 跳过：阶段内的任务也要以 skipped 状态落库（但不写跳过原因，原因只挂在阶段上）。
func TestCreateSkippedStageSteps(t *testing.T) {
	stepSvc_ := &fakeStepSvc{}
	tr := &triggerer{stepService: stepSvc_}

	execution := &executionModel.Execution{ID: 5, PipelineID: 3}
	stages := []*stageModel.Stage{
		{ID: 11, Code: "prepare", Status: constant.CIStatusSkipped, ErrorLog: fmt.Sprintf(v1yaml.WhenSkippedStageTmpl, "xx")},
		{ID: 12, Code: "build", Status: constant.CIStatusPending},
		{ID: 13, Code: "notify", Status: constant.CIStatusWaitingOnDeps},
	}

	stepsByStage, err := tr.skippedStageSteps(context.Background(), []byte(whenSkipDoc), stages)
	if err != nil {
		t.Fatalf("skippedStageSteps: %v", err)
	}
	if err := tr.createSkippedStageSteps(context.Background(), stepsByStage, execution, stages); err != nil {
		t.Fatalf("createSkippedStageSteps: %v", err)
	}

	// 只有跳过的 prepare 阶段需要落库，且状态为 skipped
	if len(stepSvc_.created) != 1 {
		t.Fatalf("任务记录数 = %d, want 1", len(stepSvc_.created))
	}
	created := stepSvc_.created[0]
	if created.StageID != 11 {
		t.Errorf("task.StageID = %d, want 11", created.StageID)
	}
	if created.Code != "run_5" {
		t.Errorf("task.Code = %q, want %q", created.Code, "run_5")
	}
	if created.Status != constant.CIStatusSkipped {
		t.Errorf("task.Status = %q, want %q", created.Status, constant.CIStatusSkipped)
	}
	// 阶段跳过时只在阶段上写原因，阶段内的任务不写跳过原因（不产生日志）。
	if created.ErrorLog != "" {
		t.Errorf("task.ErrorLog = %q, want 空", created.ErrorLog)
	}
}

// 没有阶段被跳过时不创建任何任务记录。
func TestCreateSkippedStageStepsWithoutSkip(t *testing.T) {
	stepSvc_ := &fakeStepSvc{}
	tr := &triggerer{stepService: stepSvc_}

	execution := &executionModel.Execution{ID: 5, PipelineID: 3}
	stages := []*stageModel.Stage{{ID: 12, Code: "build", Status: constant.CIStatusPending}}

	stepsByStage, err := tr.skippedStageSteps(context.Background(), []byte(whenSkipDoc), stages)
	if err != nil {
		t.Fatalf("skippedStageSteps: %v", err)
	}
	if err := tr.createSkippedStageSteps(context.Background(), stepsByStage, execution, stages); err != nil {
		t.Fatalf("createSkippedStageSteps: %v", err)
	}
	if len(stepSvc_.created) != 0 {
		t.Errorf("任务记录数 = %d, want 0", len(stepSvc_.created))
	}
}
