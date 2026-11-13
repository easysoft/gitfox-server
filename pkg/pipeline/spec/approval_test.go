package yaml

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestCommaSeparatedUnmarshalJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{name: "comma separated string", input: `"zhangsan,lisi"`, want: []string{"zhangsan", "lisi"}},
		{name: "spaces trimmed", input: `" zhangsan , lisi "`, want: []string{"zhangsan", "lisi"}},
		{name: "single account", input: `"zhangsan"`, want: []string{"zhangsan"}},
		{name: "empty string", input: `""`, want: []string{}},
		{name: "blank items dropped", input: `"zhangsan,,"`, want: []string{"zhangsan"}},
		{name: "array is rejected", input: `["zhangsan","lisi"]`, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			var got CommaSeparated
			err := json.Unmarshal([]byte(c.input), &got)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", []string(got))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal([]string(got), c.want) {
				t.Fatalf("expected %v, got %v", c.want, []string(got))
			}
		})
	}
}

func TestParsePipelineApproval(t *testing.T) {
	t.Parallel()

	const pipelineYAML = `
version: 1
kind: pipeline
spec:
  stages:
    - id: pkg_install
      name: stageWithApproval
      type: ci
      approval:
        approvers: zhangsan,lisi
        mode: or
        timeout: 24h
        percent: 60
      spec:
        steps:
          - id: run
            name: run
            type: run
            spec:
              script:
                - echo 1
    - id: plain
      name: stageWithoutApproval
      type: ci
      spec:
        steps:
          - id: run
            name: run
            type: run
            spec:
              script:
                - echo 2
`

	config, err := ParseString(pipelineYAML)
	if err != nil {
		t.Fatalf("parse pipeline failed: %v", err)
	}

	pipeline, ok := config.Spec.(*Pipeline)
	if !ok {
		t.Fatalf("unexpected spec type %T", config.Spec)
	}
	if len(pipeline.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(pipeline.Stages))
	}

	withApproval := pipeline.Stages[0].Approval
	if withApproval == nil {
		t.Fatal("expected approval config on first stage")
	}
	if want := []string{"zhangsan", "lisi"}; !slices.Equal([]string(withApproval.Approvers), want) {
		t.Fatalf("expected approvers %v, got %v", want, []string(withApproval.Approvers))
	}
	if withApproval.Mode != "or" {
		t.Fatalf("expected mode or, got %q", withApproval.Mode)
	}
	if withApproval.Timeout != "24h" {
		t.Fatalf("expected timeout 24h, got %q", withApproval.Timeout)
	}
	if withApproval.Percent == nil || *withApproval.Percent != 60 {
		t.Fatalf("expected percent 60, got %v", withApproval.Percent)
	}

	if pipeline.Stages[1].Approval != nil {
		t.Fatalf("expected nil approval on second stage, got %+v", pipeline.Stages[1].Approval)
	}
}

func TestParsePipelineApprovalPercentOmitted(t *testing.T) {
	t.Parallel()

	const pipelineYAML = `
version: 1
kind: pipeline
spec:
  stages:
    - id: pkg_install
      name: stageWithApproval
      type: ci
      approval:
        approvers: zhangsan,lisi
      spec:
        steps:
          - id: run
            name: run
            type: run
            spec:
              script:
                - echo 1
`

	config, err := ParseString(pipelineYAML)
	if err != nil {
		t.Fatalf("parse pipeline failed: %v", err)
	}

	pipeline, ok := config.Spec.(*Pipeline)
	if !ok {
		t.Fatalf("unexpected spec type %T", config.Spec)
	}
	if got := pipeline.Stages[0].Approval.Percent; got != nil {
		t.Fatalf("expected percent to be nil when omitted, got %d", *got)
	}
}

// TestParsePipelineApprovalPercentZero 确认显式写 percent: 0 时解析结果是
// 指向 0 的非 nil 指针，便于触发期区分「未配置」和「配置为 0」并拒绝后者。
func TestParsePipelineApprovalPercentZero(t *testing.T) {
	t.Parallel()

	const pipelineYAML = `
version: 1
kind: pipeline
spec:
  stages:
    - id: pkg_install
      name: stageWithApproval
      type: ci
      approval:
        approvers: zhangsan,lisi
        percent: 0
      spec:
        steps:
          - id: run
            name: run
            type: run
            spec:
              script:
                - echo 1
`

	config, err := ParseString(pipelineYAML)
	if err != nil {
		t.Fatalf("parse pipeline failed: %v", err)
	}

	pipeline, ok := config.Spec.(*Pipeline)
	if !ok {
		t.Fatalf("unexpected spec type %T", config.Spec)
	}
	got := pipeline.Stages[0].Approval.Percent
	if got == nil || *got != 0 {
		t.Fatalf("expected percent pointer to 0, got %v", got)
	}
}
