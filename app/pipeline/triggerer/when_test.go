package triggerer

import (
	"testing"

	"github.com/easysoft/gitfox-community/app/constant"
	v1yaml "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
)

func TestDirectStageRunPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		condition string
		want      string
		ok        bool
	}{
		{condition: "onSuccess", want: string(constant.RunPolicyOnSuccess), ok: true},
		{condition: "success", want: string(constant.RunPolicyOnSuccess), ok: true},
		{condition: "success()", want: string(constant.RunPolicyOnSuccess), ok: true},
		{condition: v1yaml.WhenPipelineSuccess, want: string(constant.RunPolicyOnSuccess), ok: true},
		{condition: v1yaml.WhenPipelineSuccess + "()", want: string(constant.RunPolicyOnSuccess), ok: true},
		{condition: "onFailure", want: string(constant.RunPolicyOnFailure), ok: true},
		{condition: "failure", want: string(constant.RunPolicyOnFailure), ok: true},
		{condition: "failure()", want: string(constant.RunPolicyOnFailure), ok: true},
		{condition: v1yaml.WhenPipelineFailure, want: string(constant.RunPolicyOnFailure), ok: true},
		{condition: v1yaml.WhenPipelineFailure + "()", want: string(constant.RunPolicyOnFailure), ok: true},
		{condition: v1yaml.WhenAlways, want: string(constant.RunPolicyAlways), ok: true},
		{condition: "always()", want: string(constant.RunPolicyAlways), ok: true},
		{condition: `branch == "main"`},
	}

	for _, test := range tests {
		t.Run(test.condition, func(t *testing.T) {
			t.Parallel()

			got, ok := directStageRunPolicy(test.condition)
			if ok != test.ok {
				t.Fatalf("directStageRunPolicy(%q) ok = %v, want %v", test.condition, ok, test.ok)
			}
			if got != test.want {
				t.Fatalf("directStageRunPolicy(%q) = %q, want %q", test.condition, got, test.want)
			}
		})
	}
}
