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

// step 级 when 条件需要换算成容器运行策略：
// (false, false) 表示跳过该 step（RunNever）；表达式无法求值时把该步骤标记为失败
// （RunNever + WhenError），由运行时落到该步骤上。
func TestApplyStepRunPolicy(t *testing.T) {
	// when 求值变量在生产里由 inputs.Build(build) 生成；
	// 这里的用例都不读build.* 字段，给一个空的 build 变量即可。
	inputs := map[string]interface{}{"build": map[string]interface{}{}}

	tests := []struct {
		name       string
		when       *harness.When
		want       engine.RunPolicy
		wantFailed bool // when 求值失败：置为 RunNever 并写入 WhenError
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
			name:       "表达式无法求值时把该步骤标记为失败",
			when:       &harness.When{Eval: `${{ unknown.var == 1 }}`},
			want:       engine.RunNever,
			wantFailed: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			step := &engine.Step{}
			applyStepRunPolicy(context.Background(), step, test.when, inputs)
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
