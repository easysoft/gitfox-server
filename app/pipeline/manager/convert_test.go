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
		text string
		want string
	}{
		{name: "正常拼接", code: "test1_42", text: "测试", want: "test1_42::测试"},
		{name: "legacy 无 code 退化", code: "", text: "测试", want: "测试"},
		{name: "空显示名", code: "test1_42", text: "", want: "test1_42::"},
		{name: "显示名含分隔符", code: "test1_42", text: "a::b", want: "test1_42::a::b"},
		{name: "legacy 显示名含分隔符", code: "", text: "a::b", want: "a::b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runnerStepName(tt.code, tt.text); got != tt.want {
				t.Fatalf("runnerStepName(%q, %q) = %q, want %q", tt.code, tt.text, got, tt.want)
			}
		})
	}
}

func TestSplitRunnerStepName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		buildID  int64
		wantCode string
		wantName string
	}{
		{name: "engine2 普通 step", input: "test1_42::测试", buildID: 42, wantCode: "test1_42", wantName: "测试"},
		{name: "legacy 无 code", input: "测试", buildID: 42, wantCode: "", wantName: "测试"},
		{name: "legacy 显示名含分隔符", input: "a::b", buildID: 42, wantCode: "", wantName: "a::b"},
		{name: "legacy 前缀不像 code", input: "build::frontend", buildID: 42, wantCode: "", wantName: "build::frontend"},
		{name: "engine2 显示名含分隔符", input: "test1_42::a::b", buildID: 42, wantCode: "test1_42", wantName: "a::b"},
		{name: "engine2 空显示名", input: "test1_42::", buildID: 42, wantCode: "test1_42", wantName: ""},
		{name: "内置 clone/clean", input: "drone-abcdefghijklmnopqrst::clean", buildID: 42, wantCode: "drone-abcdefghijklmnopqrst", wantName: "clean"},
		{name: "空字符串", input: "", buildID: 42, wantCode: "", wantName: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, name := splitRunnerStepName(tt.input, tt.buildID)
			if code != tt.wantCode || name != tt.wantName {
				t.Fatalf("splitRunnerStepName(%q, %d) = (%q, %q), want (%q, %q)", tt.input, tt.buildID, code, name, tt.wantCode, tt.wantName)
			}
		})
	}
}

// 空 code + 显示名含 "::" 必须能原样往返，不能被误当成 code::name。
func TestRunnerStepNameLegacySeparatorRoundTrip(t *testing.T) {
	wire := runnerStepName("", "a::b")
	if wire != "a::b" {
		t.Fatalf("runnerStepName = %q, want %q", wire, "a::b")
	}
	code, display := splitRunnerStepName(wire, 42)
	if code != "" || display != "a::b" {
		t.Fatalf("splitRunnerStepName = (%q, %q), want (%q, %q)", code, display, "", "a::b")
	}
}

// 单数路径（UpdateStep）不解码 wire name，避免 legacy 显示名被截断。
func TestConvertFromDroneStepPreservesWireName(t *testing.T) {
	for _, name := range []string{"build::frontend", "test1_42::测试", "a::b::c"} {
		got := ConvertFromDroneStep(&drone.Step{Name: name})
		if got.Code != "" || got.Name != name {
			t.Fatalf("ConvertFromDroneStep(%q) = (%q, %q), want (%q, %q)", name, got.Code, got.Name, "", name)
		}
	}
}

func TestConvertStepIdentityRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		text    string
		buildID int64
	}{
		{name: "engine2 普通 step", code: "test1_42", text: "测试", buildID: 42},
		{name: "legacy 无 code", code: "", text: "构建", buildID: 42},
		{name: "engine2 显示名含分隔符", code: "frontend_42", text: "构建::前端", buildID: 42},
		{name: "engine2 空显示名", code: "deploy_42", text: "", buildID: 42},
		{name: "legacy 显示名含分隔符", code: "", text: "a::b", buildID: 42},
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

			wire := ConvertToDroneStep(model)
			if wire.Name != runnerStepName(tt.code, tt.text) {
				t.Fatalf("ConvertToDroneStep name = %q, want %q", wire.Name, runnerStepName(tt.code, tt.text))
			}

			// 复数：UpdateStage / teardown 走这条
			backSteps := ConvertFromDroneSteps([]*drone.Step{wire}, tt.buildID)
			if len(backSteps) != 1 || backSteps[0].Code != tt.code || backSteps[0].Name != tt.text {
				t.Fatalf("plural round-trip = %+v, want code=%q name=%q", backSteps, tt.code, tt.text)
			}
		})
	}
}
