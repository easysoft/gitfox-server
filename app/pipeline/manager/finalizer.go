package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/easysoft/gitfox-community/app/constant"
	events "github.com/easysoft/gitfox-community/app/events/pipeline"
	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
	"github.com/easysoft/gitfox-community/app/service/execution"
	"github.com/easysoft/gitfox-community/app/service/pipeline"
	"github.com/easysoft/gitfox-community/app/service/sse"
	"github.com/easysoft/gitfox-community/app/service/stage"
	"github.com/easysoft/gitfox-community/app/service/step"
	"github.com/easysoft/gitfox-community/pkg/livelog"
	"github.com/easysoft/gitfox-community/pkg/logger"
	"github.com/easysoft/gitfox-community/pkg/utils/datetime"
)

const (
	// finalizeAttempts bounds the retry of the teardown tail. Once the stage is
	// persisted as terminal it drops out of ListIncomplete, so nothing will
	// call back into the approval gate again — this retry is the only chance to
	// finish the rollup.
	finalizeAttempts = 3
	// finalizeRetryDelay is multiplied by the attempt number (0, 100ms, 200ms).
	finalizeRetryDelay = 100 * time.Millisecond
)

// Finalizer finalizes stages that will never be dispatched: it marks them as
// failed and drives the execution teardown on their behalf.
//
// Unlike Manager it does not depend on the scheduler — the wire graph
// constructs the approval gate upstream of the scheduler, so depending on it
// would close a cycle. The teardown it builds therefore carries no Scheduler:
// downstream stages are flipped to pending and picked up by the next queue
// signal instead of being woken immediately.
type Finalizer struct {
	PipelineSvc  pipeline.Interface
	ExecutionSvc execution.Interface
	StageSvc     stage.Interface
	StepSvc      step.Interface
	SSEStreamer  sse.Streamer
	Reporter     executionReporter
	Logz         livelog.LogStream
}

// NewFinalizer creates the stage finalizer.
func NewFinalizer(
	pipelineSvc pipeline.Interface,
	executionSvc execution.Interface,
	stageSvc stage.Interface,
	stepSvc step.Interface,
	sseStreamer sse.Streamer,
	reporter events.Reporter,
	logz livelog.LogStream,
) *Finalizer {
	return &Finalizer{
		PipelineSvc:  pipelineSvc,
		ExecutionSvc: executionSvc,
		StageSvc:     stageSvc,
		StepSvc:      stepSvc,
		SSEStreamer:  sseStreamer,
		Reporter:     &reporter,
		Logz:         logz,
	}
}

// FinalizeFailedStage marks a stage that is never going to be dispatched as
// failed and finalizes its execution: downstream stages are skipped or
// scheduled according to their run policy, the execution is rolled up and the
// result is published over SSE and reported as an event.
//
// The reason is supplied by the caller — the finalizer is not aware of
// approvals or of anything else that keeps a stage from running.
//
// It is idempotent and safe to call concurrently from several instances: the
// build queue lock is in-memory only, so a repeated call must not corrupt
// state. A stage that is already terminal is left untouched.
func (f *Finalizer) FinalizeFailedStage(ctx context.Context, stageID uint, errorType, errorLog string) error {
	log := logger.GetLogger(ctx).With("stage.id", stageID)

	stage, err := f.StageSvc.GetByIDWithSteps(ctx, stageID)
	if err != nil {
		return fmt.Errorf("failed to get stage %d: %w", stageID, err)
	}

	execution, err := f.ExecutionSvc.GetByID(ctx, stage.ExecutionID)
	if err != nil {
		return fmt.Errorf("failed to get execution %d: %w", stage.ExecutionID, err)
	}

	if execution.Status == constant.CIStatusKilled {
		// The execution was cancelled. Finalization belongs to the canceler,
		// which owns the killed state — running complete here would overwrite
		// its rollup with a failure. Only settle this stage, which the canceler
		// may not have reached yet.
		return f.settleSkipped(stage)
	}

	if stage.Status.IsDone() {
		// The stage is already terminal: a sibling's teardown skipped it, the
		// canceler settled it, or a previous attempt got as far as persisting
		// it. Whoever wrote that state owns the stage — overwriting it here
		// would flip a skipped stage to failed.
		log.Debug("manager: stage already settled, nothing to do")
		return nil
	}

	now := datetime.NowLocalTime()
	stage.Status = constant.CIStatusFailure
	stage.ErrorType = errorType
	// The reason is caller-supplied — cap it like the other write points
	// (setup/teardown/updater) so a long message cannot overflow the column.
	stage.ErrorLog = errorLog
	if len(stage.ErrorLog) > 500 {
		stage.ErrorLog = stage.ErrorLog[:500]
	}
	stage.Finished = now
	// Started stays nil: the stage never ran, and stamping it would render as
	// a zero-second execution in the UI.
	for _, step := range stage.Steps {
		if step.Status.IsDone() {
			continue
		}
		step.Status = constant.CIStatusSkipped
	}

	t := &teardown{
		PipelineSvc:  f.PipelineSvc,
		ExecutionSvc: f.ExecutionSvc,
		StageSvc:     f.StageSvc,
		StepSvc:      f.StepSvc,
		SSEStreamer:  f.SSEStreamer,
		Reporter:     f.Reporter,
		Logz:         f.Logz,
	}

	for attempt := 1; attempt <= finalizeAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt-1) * finalizeRetryDelay)
		}
		err = t.complete(ctx, stage, execution)
		if err == nil {
			return nil
		}
		log.Warnw("manager: cannot finalize failed stage",
			"attempt", attempt,
			"error", err,
		)
	}

	return fmt.Errorf("failed to finalize stage %d: %w", stageID, err)
}

// settleSkipped marks a stage and its unfinished steps as skipped without
// touching the execution. It is used when the execution was cancelled: the
// canceler already wrote the killed state and owns the rollup.
func (f *Finalizer) settleSkipped(stage *stageModel.StageWithSteps) error {
	if stage.Status.IsDone() {
		return nil
	}

	stage.Status = constant.CIStatusSkipped
	for _, step := range stage.Steps {
		if !step.Status.IsDone() {
			step.Status = constant.CIStatusSkipped
		}
		if err := f.StepSvc.Update(noContext, step); err != nil {
			return fmt.Errorf("failed to settle step %d of cancelled execution: %w", step.ID, err)
		}
	}
	if err := f.StageSvc.Update(noContext, &stage.Stage); err != nil {
		return fmt.Errorf("failed to settle stage %d of cancelled execution: %w", stage.ID, err)
	}
	return nil
}
