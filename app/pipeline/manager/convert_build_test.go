package manager

import (
	"testing"

	"github.com/easysoft/gitfox-community/app/constant"
	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
)

// always 策略的阶段在 drone.Stage 上需要同时满足 on_success 和 on_failure，
// 否则 runner或服务端会把它当成只跑成功或只跑失败。
func TestConvertToDroneStageRunPolicy(t *testing.T) {
	tests := []struct {
		policy    string
		onSuccess bool
		onFailure bool
	}{
		{policy: string(constant.RunPolicyOnSuccess), onSuccess: true, onFailure: false},
		{policy: string(constant.RunPolicyOnFailure), onSuccess: false, onFailure: true},
		{policy: string(constant.RunPolicyAlways), onSuccess: true, onFailure: true},
		{policy: "", onSuccess: true, onFailure: false},
	}

	for _, test := range tests {
		stage := &stageModel.StageWithSteps{Stage: stageModel.Stage{ID: 1, ExecutionID: 2, RunPolicy: test.policy}}
		out := ConvertToDroneStage(stage)
		if out.OnSuccess != test.onSuccess || out.OnFailure != test.onFailure {
			t.Errorf("policy %q: OnSuccess = %v, OnFailure = %v, want %v/%v",
				test.policy, out.OnSuccess, out.OnFailure, test.onSuccess, test.onFailure)
		}
	}
}
