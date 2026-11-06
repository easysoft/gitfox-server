package runtime

import (
	"testing"

	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/engine"

	"github.com/drone/drone-go/drone"
	"github.com/drone/runner-go/pipeline"
)

func TestStepKey(t *testing.T) {
	tests := []struct {
		name string
		step *engine.Step
		want string
	}{
		{
			name: "普通 step",
			step: &engine.Step{ID: "test1_42", Name: "测试"},
			want: "test1_42::测试",
		},
		{
			name: "空显示名",
			step: &engine.Step{ID: "test1_42", Name: ""},
			want: "test1_42::",
		},
		{
			name: "background 的 Name 是 code",
			step: &engine.Step{ID: "drone-abc_42", Name: "dind-build"},
			want: "drone-abc_42::dind-build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stepKey(tt.step); got != tt.want {
				t.Fatalf("stepKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStepDisplayName(t *testing.T) {
	tests := []struct {
		name string
		step *engine.Step
		want string
	}{
		{
			name: "优先使用 Display",
			step: &engine.Step{ID: "drone-abc", Name: "dind-build", Display: "构建 - daemon"},
			want: "构建 - daemon",
		},
		{
			name: "Display 为空时回退 Name",
			step: &engine.Step{ID: "drone-abc", Name: "clean"},
			want: "clean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stepDisplayName(tt.step); got != tt.want {
				t.Fatalf("stepDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// 两个同名 step 在 state 里必须是两个不同的 key，否则第二个会被误判为已结束。
func TestStateLookupWithDuplicateDisplayNames(t *testing.T) {
	state := &pipeline.State{
		Build: &drone.Build{},
		Stage: &drone.Stage{
			Steps: []*drone.Step{
				{ID: 1, Number: 1, Name: "test1_42::测试", Status: drone.StatusPending},
				{ID: 2, Number: 2, Name: "test2_42::测试", Status: drone.StatusPending},
			},
		},
	}

	first := &engine.Step{ID: "test1_42", Name: "测试"}
	second := &engine.Step{ID: "test2_42", Name: "测试"}

	if stepKey(first) == stepKey(second) {
		t.Fatalf("同名 step 的复合键不能相同: %q", stepKey(first))
	}

	state.Start(stepKey(first))
	if got := state.Find(stepKey(first)); got.ID != 1 || got.Status != drone.StatusRunning {
		t.Fatalf("第一个 step 状态错误: %+v", got)
	}
	if got := state.Find(stepKey(second)); got.ID != 2 || got.Status != drone.StatusPending {
		t.Fatalf("第二个 step 不应被第一个影响: %+v", got)
	}
}
