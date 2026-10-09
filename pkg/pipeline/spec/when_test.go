package yaml

import (
	"encoding/json"
	"testing"
)

func TestWhenUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		eval string
	}{
		{
			name: "表达式字符串",
			in:   `"${{ build.ref == \"refs/heads/main\" }}"`,
			eval: `${{ build.ref == "refs/heads/main" }}`,
		},
		{
			name: "运行策略字符串",
			in:   `"always"`,
			eval: "always",
		},
		{
			name: "eval 对象",
			in:   `{"eval": "${{ always() }}"}`,
			eval: "${{ always() }}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var when When
			if err := json.Unmarshal([]byte(test.in), &when); err != nil {
				t.Fatalf("unmarshal %s: %v", test.in, err)
			}
			if when.Eval != test.eval {
				t.Errorf("Eval = %q, want %q", when.Eval, test.eval)
			}
		})
	}
}

// 无法识别的 when 写法（drone 简写、声明式 cond、changes、null、空对象、空数组、空字符串、数字等）
// 必须返回错误、让整份配置解析失败：静默忽略条件会让执行行为与预期不符。
func TestWhenUnmarshalUnrecognized(t *testing.T) {
	for _, in := range []string{
		`{"event": "push", "branch": "main"}`,
		`[{"status": ["success", "failure"]}]`,
		`{"cond": [{"branch": {"eq": "main"}}]}`,
		`{"changes": ["src/**"]}`,
		`true`,
		`false`,
		`{}`,
		`null`,
		`[]`,
		`""`,
		`0`,
	} {
		t.Run(in, func(t *testing.T) {
			var when When
			if err := json.Unmarshal([]byte(in), &when); err == nil {
				t.Fatalf("unmarshal %s: 期望返回错误，实际解析成功 %+v", in, when)
			}
		})
	}
}

// YAML 中各种 when 写法都要能解析成功。
func TestWhenParseYAML(t *testing.T) {
	const doc = `
kind: pipeline
spec:
  stages:
    - id: expression
      name: expression
      type: ci
      when: ${{ build.ref == 'refs/heads/main' }}
      spec:
        steps:
          - id: run
            name: run
            type: run
            spec:
              script:
                - echo hi
    - id: object
      name: object
      type: ci
      when:
        eval: ${{ always() }}
      spec:
        steps:
          - id: run
            name: run
            type: run
            spec:
              script:
                - echo hi
`
	config, err := ParseString(doc)
	if err != nil {
		t.Fatalf("解析 yaml 失败: %v", err)
	}
	pipeline, ok := config.Spec.(*Pipeline)
	if !ok {
		t.Fatalf("期望 Pipeline，实际 %T", config.Spec)
	}
	if len(pipeline.Stages) != 2 {
		t.Fatalf("stage 数量 = %d, want 2", len(pipeline.Stages))
	}
	if got, want := pipeline.Stages[0].When.Eval, `${{ build.ref == 'refs/heads/main' }}`; got != want {
		t.Errorf("表达式形式 Eval = %q, want %q", got, want)
	}
	if got, want := pipeline.Stages[1].When.Eval, "${{ always() }}"; got != want {
		t.Errorf("对象形式 Eval = %q, want %q", got, want)
	}
}

// drone 简写（when: {status: [success, failure]}）当前不支持，必须让整份配置解析失败。
func TestWhenParseYAMLDroneShorthandFails(t *testing.T) {
	const doc = `
kind: pipeline
spec:
  stages:
    - id: drone-shorthand
      name: drone-shorthand
      type: ci
      when:
        status: [success, failure]
      spec:
        steps:
          - id: run
            name: run
            type: run
            spec:
              script:
                - echo hi
`
	if _, err := ParseString(doc); err == nil {
		t.Fatal("drone 简写 when 应当让整份配置解析失败")
	}
}
