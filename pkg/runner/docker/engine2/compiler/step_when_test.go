package compiler

import (
	"context"
	"testing"

	harness "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/engine"
)

// step 级 when 条件需要换算成容器运行策略：
// (false, false) 表示跳过该 step（RunNever），表达式解析失败时保持默认策略。
func TestApplyStepRunPolicy(t *testing.T) {
	// when 求值变量在生产里由 inputs.Build(build) 生成；这里的用例都不读
	// build.* 字段，给一个空的 build 变量即可。
	inputs := map[string]interface{}{"build": map[string]interface{}{}}

	tests := []struct {
		name    string
		when    *harness.When
		want    engine.RunPolicy
		wantErr bool
	}{
		{
			name: "没有 when 条件时保持默认策略",
			when: nil,
			want: engine.RunOnSuccess,
		},
		{
			name: "always 表达式",
			when: &harness.When{Eval: `${{ always() }}`},
			want: engine.RunAlways,
		},
		{
			name: "failure 表达式",
			when: &harness.When{Eval: `${{ failure() }}`},
			want: engine.RunOnFailure,
		},
		{
			name: "直接写运行策略 always",
			when: &harness.When{Eval: "always"},
			want: engine.RunAlways,
		},
		{
			name: "直接写运行策略 onFailure",
			when: &harness.When{Eval: "onFailure"},
			want: engine.RunOnFailure,
		},
		{
			name:    "表达式无法求值时返回错误",
			when:    &harness.When{Eval: `${{ unknown.var == 1 }}`},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			step := &engine.Step{}
			err := applyStepRunPolicy(context.Background(), step, test.when, inputs)
			if test.wantErr {
				if err == nil {
					t.Fatalf("期望返回错误，实际 nil（RunPolicy = %v）", step.RunPolicy)
				}
				return
			}
			if err != nil {
				t.Fatalf("applyStepRunPolicy: %v", err)
			}
			if step.RunPolicy != test.want {
				t.Errorf("RunPolicy = %v, want %v", step.RunPolicy, test.want)
			}
		})
	}
}
