package triggerer

import (
	"context"
	"testing"

	"github.com/easysoft/gitfox-community/app/constant"
	executionModel "github.com/easysoft/gitfox-community/app/model/execution"
	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
)

// when 条件在成功、失败两种状态下都不成立时，阶段应当真跳过：
// 落库状态为 skipped，且不再作为下游阶段的依赖。
func TestParseV1StagesWhenSkip(t *testing.T) {
	const doc = `
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
	execution := &executionModel.Execution{
		ID:      1,
		Trigger: string(constant.TriggerActionBranchUpdated),
		Ref:     "refs/heads/release",
	}
	hook := &Hook{
		Action: constant.TriggerActionBranchUpdated,
		Ref:    "refs/heads/release",
	}

	stages, err := parseV1Stages(context.Background(), []byte(doc), execution, hook, nil)
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
