package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/easysoft/gitfox-community/app/constant"
	runnerModel "github.com/easysoft/gitfox-community/app/model/runner"
	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
	runnerStore "github.com/easysoft/gitfox-community/app/store/runner"
	stageStore "github.com/easysoft/gitfox-community/app/store/stage"
	"github.com/easysoft/gitfox-community/pkg/database"
	"github.com/easysoft/gitfox-community/pkg/lock"
)

type fakeStageStore struct {
	stageStore.Interface
	stages []*stageModel.Stage
}

func (f *fakeStageStore) ListIncomplete(context.Context) ([]*stageModel.Stage, error) {
	return f.stages, nil
}

type fakeRunnerStore struct {
	runnerStore.Interface
}

func (f *fakeRunnerStore) List(context.Context, *runnerModel.ReqFilter) (*database.PageResult[*runnerModel.Runner], error) {
	return &database.PageResult[*runnerModel.Runner]{}, nil
}

type fakeApproval struct {
	blocked bool
	err     error
	calls   int
}

func (f *fakeApproval) BeforeStageDispatch(_ context.Context, _ *stageModel.Stage) (bool, error) {
	f.calls++
	return f.blocked, f.err
}

func TestQueueSignalApprovalGate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		stage          *stageModel.Stage
		approval       *fakeApproval
		wantDispatched bool
		wantGateCalls  int
	}{
		{
			name:           "no approval service blocking",
			stage:          &stageModel.Stage{ID: 1, Name: "s1", Type: "inner", Status: constant.CIStatusPending},
			approval:       &fakeApproval{},
			wantDispatched: true,
			wantGateCalls:  1,
		},
		{
			name:           "blocked by approval",
			stage:          &stageModel.Stage{ID: 2, Name: "s2", Type: "inner", Status: constant.CIStatusPending},
			approval:       &fakeApproval{blocked: true},
			wantDispatched: false,
			wantGateCalls:  1,
		},
		{
			name:           "approval error keeps stage pending",
			stage:          &stageModel.Stage{ID: 3, Name: "s3", Type: "inner", Status: constant.CIStatusPending},
			approval:       &fakeApproval{err: errors.New("devops unavailable")},
			wantDispatched: false,
			wantGateCalls:  1,
		},
		{
			name:           "already running stage skips the gate",
			stage:          &stageModel.Stage{ID: 4, Name: "s4", Type: "inner", Status: constant.CIStatusRunning},
			approval:       &fakeApproval{blocked: true},
			wantDispatched: false,
			wantGateCalls:  0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			mx, err := lock.NewInMemory(lock.Config{}).NewMutex("test_queue")
			if err != nil {
				t.Fatalf("new mutex failed: %v", err)
			}

			q := &queue{
				store:       &fakeStageStore{stages: []*stageModel.Stage{c.stage}},
				runnerStore: &fakeRunnerStore{},
				approvalSvc: c.approval,
				globMx:      mx,
				ready:       make(chan struct{}, 1),
				workers:     map[*worker]struct{}{},
				ctx:         context.Background(),
			}

			w := &worker{
				token:   "runner-token",
				typ:     "inner",
				channel: make(chan *stageModel.Stage, 1),
				done:    ctx.Done(),
			}
			q.workers[w] = struct{}{}

			if err := q.signal(ctx); err != nil {
				t.Fatalf("signal failed: %v", err)
			}

			select {
			case got := <-w.channel:
				if !c.wantDispatched {
					t.Fatalf("expected stage to be blocked, but it was dispatched: %+v", got)
				}
				if got.ID != c.stage.ID {
					t.Fatalf("expected stage %d, got %d", c.stage.ID, got.ID)
				}
			default:
				if c.wantDispatched {
					t.Fatal("expected stage to be dispatched, but nothing was received")
				}
			}

			if c.approval.calls != c.wantGateCalls {
				t.Fatalf("expected %d approval gate calls, got %d", c.wantGateCalls, c.approval.calls)
			}
		})
	}
}
