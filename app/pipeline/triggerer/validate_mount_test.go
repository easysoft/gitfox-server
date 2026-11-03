package triggerer

import (
	"errors"
	"strings"
	"testing"

	v1yaml "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
)

// 挂载校验失败时要带上 stage 上下文，并保留原始错误原因，
// 避免多阶段/同名步骤的流水线难以定位出错阶段。
func TestValidateV1PipelineSpecReportsStageContext(t *testing.T) {
	pipeline := &v1yaml.Pipeline{
		Stages: []*v1yaml.Stage{
			{
				Id: "stage-a",
				Spec: &v1yaml.StageCI{Steps: []*v1yaml.Step{{
					Id:   "step-a",
					Name: "build",
					Type: "run",
					Spec: &v1yaml.StepRun{},
				}}},
			},
			{
				Id: "stage-b",
				Spec: &v1yaml.StageCI{Steps: []*v1yaml.Step{{
					Id:   "step-b",
					Name: "build",
					Type: "run",
					Spec: &v1yaml.StepRun{
						Mount: []*v1yaml.Mount{{Name: "tmp", Path: "/tmp"}},
					},
				}}},
			},
		},
	}

	err := validateV1PipelineSpec(pipeline)
	if err == nil {
		t.Fatal("expected mount validation error, got nil")
	}
	if !strings.Contains(err.Error(), `stage "stage-b"`) {
		t.Fatalf("error = %q, want stage context", err)
	}
	if !strings.Contains(err.Error(), `mount volume "tmp" is not declared`) {
		t.Fatalf("error = %q, want underlying mount error", err)
	}
	if errors.Unwrap(err) == nil {
		t.Fatalf("error = %q, want wrapped underlying error", err)
	}
}
