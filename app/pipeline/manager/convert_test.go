// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package manager

import (
	"testing"
	"time"

	"github.com/drone/drone-go/drone"
	stepModel "github.com/easysoft/gitfox-community/app/model/step"
)

func TestResolveLineTimestamp(t *testing.T) {
	base := time.Unix(1789004677, 0)
	tests := []struct {
		name string
		ts   int64
		base time.Time
		want int64
	}{
		{
			name: "相对秒数按 step 开始时间换算",
			ts:   8,
			base: base,
			want: 1789004685,
		},
		{
			name: "首行的相对时间 0 即 step 开始时间",
			ts:   0,
			base: base,
			want: 1789004677,
		},
		{
			name: "已是绝对时间的原样保留",
			ts:   1789004685,
			base: base,
			want: 1789004685,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveLineTimestamp(tt.ts, tt.base); got != tt.want {
				t.Fatalf("ResolveLineTimestamp(%d, %v) = %d, want %d", tt.ts, tt.base, got, tt.want)
			}
		})
	}
}

// 拿不到 step 开始时间时退回当前时间，避免把相对秒数当成绝对时间写进日志文件。
func TestResolveLineTimestampWithoutBase(t *testing.T) {
	start := time.Now().Unix()
	got := ResolveLineTimestamp(3, time.Time{})
	if end := time.Now().Unix(); got < start || got > end {
		t.Fatalf("ResolveLineTimestamp(3, 零值时间) = %d, 期望落在 [%d, %d] 内", got, start, end)
	}
}

func TestConvertFromDroneLine(t *testing.T) {
	base := time.Unix(1789004677, 0)
	got := ConvertFromDroneLine(&drone.Line{Number: 7, Message: "hello\n", Timestamp: 3}, base)

	if got.Number != 7 || got.Message != "hello\n" {
		t.Fatalf("日志行内容转换错误: %+v", got)
	}
	if want := int64(1789004680); got.Timestamp != want {
		t.Fatalf("Timestamp = %d, want %d", got.Timestamp, want)
	}
}

func TestRunnerStepName(t *testing.T) {
	tests := []struct {
		name string
		code string
		want string
	}{
		{name: "正常拼接", code: "test1_42", want: "test1_42::测试"},
		{name: "legacy 无 code 退化", code: "", want: "测试"},
		{name: "空显示名", code: "test1_42", want: "test1_42::"},
		{name: "显示名含分隔符", code: "test1_42", want: "test1_42::a::b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display := "测试"
			switch tt.name {
			case "空显示名":
				display = ""
			case "显示名含分隔符":
				display = "a::b"
			}
			if got := runnerStepName(tt.code, display); got != tt.want {
				t.Fatalf("runnerStepName(%q, %q) = %q, want %q", tt.code, display, got, tt.want)
			}
		})
	}
}

func TestSplitRunnerStepName(t *testing.T) {
	tests := []struct {
		input    string
		wantCode string
		wantName string
	}{
		{input: "test1_42::测试", wantCode: "test1_42", wantName: "测试"},
		{input: "测试", wantCode: "", wantName: "测试"},
		{input: "a::b::c", wantCode: "a", wantName: "b::c"},
		{input: "test1_42::", wantCode: "test1_42", wantName: ""},
		{input: "", wantCode: "", wantName: ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			code, name := splitRunnerStepName(tt.input)
			if code != tt.wantCode || name != tt.wantName {
				t.Fatalf("splitRunnerStepName(%q) = (%q, %q), want (%q, %q)", tt.input, code, name, tt.wantCode, tt.wantName)
			}
		})
	}
}

func TestConvertStepIdentityRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		code string
		text string
	}{
		{name: "普通 step", code: "test1_42", text: "测试"},
		{name: "legacy 无 code", code: "", text: "构建"},
		{name: "显示名含分隔符", code: "frontend_42", text: "构建::前端"},
		{name: "空显示名", code: "deploy_42", text: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &stepModel.Step{
				ID:      1,
				StageID: 2,
				SN:      3,
				Code:    tt.code,
				Name:    tt.text,
			}

			// 单数：UpdateStep 走这条
			wire := ConvertToDroneStep(model)
			back := ConvertFromDroneStep(wire)
			if back.Code != tt.code || back.Name != tt.text {
				t.Fatalf("single round-trip = (%q, %q), want (%q, %q)", back.Code, back.Name, tt.code, tt.text)
			}

			// 复数：UpdateStage 走这条
			backSteps := ConvertFromDroneSteps(ConvertToDroneSteps([]*stepModel.Step{model}))
			if len(backSteps) != 1 || backSteps[0].Code != tt.code || backSteps[0].Name != tt.text {
				t.Fatalf("plural round-trip = %+v, want code=%q name=%q", backSteps, tt.code, tt.text)
			}
		})
	}
}
