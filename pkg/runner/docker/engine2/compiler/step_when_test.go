/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package compiler

import (
	"context"
	"testing"

	harness "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/engine"
)

// step 级 when 条件（含祖先 group/parallel 的 when）需要换算成容器运行策略：
// (false, false) 表示跳过该 step（RunNever）；表达式无法求值时把该步骤标记为失败
// （RunNever + WhenError），由运行时落到该步骤上。
func TestApplyStepRunPolicy(t *testing.T) {
	// when 求值变量在生产里由 inputs.Build(build) 生成。
	inputs := map[string]interface{}{"build": map[string]interface{}{"event": "push"}}

	tests := []struct {
		name       string
		whens      []*harness.When
		want       engine.RunPolicy
		wantFailed bool // when 求值失败：置为 RunNever 并写入 WhenError
	}{
		{
			name:  "没有 when 条件时保持默认策略",
			whens: nil,
			want:  engine.RunOnSuccess,
		},
		{
			name:  "always 表达式",
			whens: []*harness.When{{Eval: `${{ always() }}`}},
			want:  engine.RunAlways,
		},
		{
			name:  "failure 表达式",
			whens: []*harness.When{{Eval: `${{ failure() }}`}},
			want:  engine.RunOnFailure,
		},
		{
			name:  "直接写运行策略 always",
			whens: []*harness.When{{Eval: "always"}},
			want:  engine.RunAlways,
		},
		{
			name:  "直接写运行策略 onFailure",
			whens: []*harness.When{{Eval: "onFailure"}},
			want:  engine.RunOnFailure,
		},
		{
			// 修复前合并成 "failure() && (always() && ...)" 后会被 always() 短路成 RunAlways。
			name: "组 onFailure 与叶子复合 always 按 AND 合成",
			whens: []*harness.When{
				{Eval: "onFailure"},
				{Eval: `${{ always() && build.event == 'push' }}`},
			},
			want: engine.RunOnFailure,
		},
		{
			// 修复前合并成 "(always() && ...) && success()" 后会被 always() 短路成 RunAlways。
			name: "组复合 always 与叶子 onSuccess 按 AND 合成",
			whens: []*harness.When{
				{Eval: `${{ always() && build.event == 'push' }}`},
				{Eval: "onSuccess"},
			},
			want: engine.RunOnSuccess,
		},
		{
			name: "组表达式与叶子 onFailure 不可同时成立时跳过",
			whens: []*harness.When{
				{Eval: `${{ build.event == 'push' }}`},
				{Eval: "onFailure"},
			},
			want: engine.RunNever,
		},
		{
			name: "叶子表达式不成立时跳过",
			whens: []*harness.When{
				{Eval: "onSuccess"},
				{Eval: `${{ build.event == 'tag' }}`},
			},
			want: engine.RunNever,
		},
		{
			name:       "表达式无法求值时把该步骤标记为失败",
			whens:      []*harness.When{{Eval: `${{ unknown.var == 1 }}`}},
			want:       engine.RunNever,
			wantFailed: true,
		},
		{
			name: "父级 when 求值失败同样标记步骤失败",
			whens: []*harness.When{
				{Eval: "onFailure"},
				{Eval: `${{ unknown.var == 1 }}`},
			},
			want:       engine.RunNever,
			wantFailed: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			step := &engine.Step{}
			applyStepRunPolicy(context.Background(), step, test.whens, inputs)
			if test.wantFailed {
				if step.WhenError == "" {
					t.Fatalf("期望写入 WhenError，实际为空（RunPolicy = %v）", step.RunPolicy)
				}
			} else if step.WhenError != "" {
				t.Errorf("不应写入 WhenError: %q", step.WhenError)
			}
			if step.RunPolicy != test.want {
				t.Errorf("RunPolicy = %v, want %v", step.RunPolicy, test.want)
			}
		})
	}
}

// WhenCondition 仅用于运行期日志展示，应保留用户书写的各条 when 条件。
func TestApplyStepRunPolicyWhenCondition(t *testing.T) {
	step := &engine.Step{}
	applyStepRunPolicy(context.Background(), step, []*harness.When{
		{Eval: "onFailure"},
		{Eval: `${{ always() && build.event == 'push' }}`},
	}, map[string]interface{}{"build": map[string]interface{}{"event": "push"}})

	want := "onFailure && always() && build.event == 'push'"
	if step.WhenCondition != want {
		t.Fatalf("WhenCondition = %q, want %q", step.WhenCondition, want)
	}
}
