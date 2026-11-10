// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)

package runtime

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/engine"

	"github.com/drone/drone-go/drone"
	"github.com/drone/runner-go/pipeline"
)

// fakeEngine 记录 step 并发数，并可通过 release 通道让 step 阻塞在容器启动阶段。
type fakeEngine struct {
	mu         sync.Mutex
	running    int
	maxRunning int
	finished   map[string]bool
	exitCode   map[string]int
	started    chan string
	release    <-chan struct{}
}

func (e *fakeEngine) Setup(context.Context, *engine.Spec) error   { return nil }
func (e *fakeEngine) Destroy(context.Context, *engine.Spec) error { return nil }

func (e *fakeEngine) Run(ctx context.Context, _ *engine.Spec, step *engine.Step, _ io.Writer) (*engine.State, error) {
	e.mu.Lock()
	// join 步骤必须先等到所有依赖完成，否则说明 DAG 接线有误。
	if step.ID == "c_1" && (!e.finished["a_1"] || !e.finished["b_1"]) {
		e.mu.Unlock()
		return nil, fmt.Errorf("join step %s started before dependencies finished", step.ID)
	}
	e.running++
	if e.running > e.maxRunning {
		e.maxRunning = e.running
	}
	e.mu.Unlock()

	if e.started != nil {
		e.started <- step.ID
	}
	if e.release != nil {
		select {
		case <-e.release:
		case <-ctx.Done():
		}
	}

	e.mu.Lock()
	e.running--
	e.finished[step.ID] = true
	e.mu.Unlock()

	return &engine.State{ExitCode: e.exitCode[step.ID]}, nil
}

func (e *fakeEngine) maxConcurrency() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.maxRunning
}

func graphSpec() *engine.Spec {
	return &engine.Spec{Steps: []*engine.Step{
		{ID: "a_1", Name: "A"},
		{ID: "b_1", Name: "B"},
		{ID: "c_1", Name: "C", DependsOn: []string{"a_1", "b_1"}},
	}}
}

func graphState() *pipeline.State {
	return &pipeline.State{
		Build: &drone.Build{},
		Stage: &drone.Stage{Steps: []*drone.Step{
			{ID: 1, Number: 1, Name: "a_1::A", Status: drone.StatusPending},
			{ID: 2, Number: 2, Name: "b_1::B", Status: drone.StatusPending},
			{ID: 3, Number: 3, Name: "c_1::C", Status: drone.StatusPending},
		}},
		Repo:   &drone.Repo{},
		System: &drone.System{},
	}
}

// 无依赖的 a/b 必须同时启动，join 步骤 c 在两者都完成后才启动。
func TestExecParallelBranchesAndJoin(t *testing.T) {
	started := make(chan string, 3)
	release := make(chan struct{})
	eng := &fakeEngine{
		finished: map[string]bool{},
		exitCode: map[string]int{},
		started:  started,
		release:  release,
	}
	exec := NewExecer(pipeline.NopReporter(), pipeline.NopStreamer(), nil, eng, 0)
	state := graphState()

	errCh := make(chan error, 1)
	go func() {
		errCh <- exec.Exec(context.Background(), graphSpec(), state)
	}()

	first := waitStep(t, started)
	second := waitStep(t, started)
	if (first == "a_1" && second == "b_1") || (first == "b_1" && second == "a_1") {
		// a/b 都已进入运行，说明是并发启动。
	} else {
		t.Fatalf("first two steps = %q/%q, want a_1/b_1", first, second)
	}

	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("Exec() error: %v", err)
	}
	if got := eng.maxConcurrency(); got != 2 {
		t.Fatalf("max concurrency = %d, want 2", got)
	}
	if got := state.Find("c_1::C").Status; got != drone.StatusPassing {
		t.Fatalf("join step status = %q, want %q", got, drone.StatusPassing)
	}
}

// MaxParallelSteps=1 时并行分支被信号量串行化。
func TestExecParallelSemaphore(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	eng := &fakeEngine{
		finished: map[string]bool{},
		exitCode: map[string]int{},
		started:  started,
		release:  release,
	}
	exec := NewExecer(pipeline.NopReporter(), pipeline.NopStreamer(), nil, eng, 1)
	state := graphState()

	errCh := make(chan error, 1)
	go func() {
		errCh <- exec.Exec(context.Background(), graphSpec(), state)
	}()

	waitStep(t, started)
	select {
	case got := <-started:
		t.Fatalf("第二个 step %q 不应在第一个完成前启动", got)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("Exec() error: %v", err)
	}
	if got := eng.maxConcurrency(); got != 1 {
		t.Fatalf("max concurrency = %d, want 1", got)
	}
}

// 一个分支失败时，其他分支继续；RunOnSuccess 的 join 步骤被跳过。
func TestExecParallelFailureSkipsJoin(t *testing.T) {
	eng := &fakeEngine{
		finished: map[string]bool{},
		exitCode: map[string]int{"a_1": 1},
	}
	exec := NewExecer(pipeline.NopReporter(), pipeline.NopStreamer(), nil, eng, 0)
	state := graphState()

	// b 用 RunAlways，避免与 a 的失败发生时序竞争。
	spec := graphSpec()
	spec.Steps[1].RunPolicy = engine.RunAlways

	if err := exec.Exec(context.Background(), spec, state); err != nil {
		t.Fatalf("Exec() error: %v", err)
	}

	if got := state.Find("a_1::A").Status; got != drone.StatusFailing {
		t.Errorf("a status = %q, want %q", got, drone.StatusFailing)
	}
	if got := state.Find("b_1::B").Status; got != drone.StatusPassing {
		t.Errorf("b status = %q, want %q", got, drone.StatusPassing)
	}
	if got := state.Find("c_1::C").Status; got != drone.StatusSkipped {
		t.Errorf("c status = %q, want %q", got, drone.StatusSkipped)
	}
}

func waitStep(t *testing.T, started <-chan string) string {
	t.Helper()
	select {
	case id := <-started:
		return id
	case <-time.After(2 * time.Second):
		t.Fatal("等待 step 启动超时")
		return ""
	}
}
