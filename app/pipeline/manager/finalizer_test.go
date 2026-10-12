package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/easysoft/gitfox-community/app/constant"
	events "github.com/easysoft/gitfox-community/app/events/pipeline"
	eventModel "github.com/easysoft/gitfox-community/app/model/event"
	executionModel "github.com/easysoft/gitfox-community/app/model/execution"
	pipelineModel "github.com/easysoft/gitfox-community/app/model/pipeline"
	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
	stepModel "github.com/easysoft/gitfox-community/app/model/step"
	"github.com/easysoft/gitfox-community/app/service/execution"
	"github.com/easysoft/gitfox-community/app/service/pipeline"
	"github.com/easysoft/gitfox-community/app/service/sse"
	"github.com/easysoft/gitfox-community/app/service/stage"
	"github.com/easysoft/gitfox-community/app/service/step"
	"github.com/easysoft/gitfox-community/pkg/livelog"
)

type stubPipeline struct {
	pipeline.Interface
	pipeline    *pipelineModel.Pipeline
	updateCalls int
}

func (f *stubPipeline) GetPipelineByID(context.Context, int64) (*pipelineModel.Pipeline, error) {
	return f.pipeline, nil
}

func (f *stubPipeline) UpdatePipeline(context.Context, *pipelineModel.Pipeline) error {
	f.updateCalls++
	return nil
}

type stubExecution struct {
	execution.Interface
	execution   *executionModel.Execution
	updateCalls int
	updateErr   error
}

func (f *stubExecution) GetByID(context.Context, uint) (*executionModel.Execution, error) {
	return f.execution, nil
}

func (f *stubExecution) Update(context.Context, *executionModel.Execution) error {
	f.updateCalls++
	return f.updateErr
}

type stubStage struct {
	stage.Interface
	stage       *stageModel.StageWithSteps
	siblings    []*stageModel.StageWithSteps
	updateCalls int
	updateErr   error
}

func (f *stubStage) GetByIDWithSteps(context.Context, uint) (*stageModel.StageWithSteps, error) {
	return f.stage, nil
}

func (f *stubStage) ListWithSteps(context.Context, uint) ([]*stageModel.StageWithSteps, error) {
	return f.siblings, nil
}

func (f *stubStage) Update(context.Context, *stageModel.Stage) error {
	f.updateCalls++
	return f.updateErr
}

type stubStep struct {
	step.Interface
	updateCalls int
}

func (f *stubStep) Update(context.Context, *stepModel.Step) error {
	f.updateCalls++
	return nil
}

type stubStreamer struct {
	sse.Streamer
	publishCalls int
}

func (f *stubStreamer) Publish(context.Context, int64, eventModel.SSEType, any) error {
	f.publishCalls++
	return nil
}

type stubReporter struct {
	calls int
}

func (f *stubReporter) Executed(context.Context, *events.ExecutedPayload) {
	f.calls++
}

type stubLogz struct {
	livelog.LogStream
	deleteCalls int
}

func (f *stubLogz) Delete(context.Context, int64) error {
	f.deleteCalls++
	return livelog.ErrStreamNotFound
}

type finalizerFixture struct {
	pipelineSvc  *stubPipeline
	executionSvc *stubExecution
	stageSvc     *stubStage
	stepSvc      *stubStep
	streamer     *stubStreamer
	reporter     *stubReporter
	logz         *stubLogz
}

func newFinalizerFixture() *finalizerFixture {
	return &finalizerFixture{
		pipelineSvc:  &stubPipeline{pipeline: &pipelineModel.Pipeline{ID: 7, SpaceID: 3}},
		executionSvc: &stubExecution{},
		stageSvc:     &stubStage{},
		stepSvc:      &stubStep{},
		streamer:     &stubStreamer{},
		reporter:     &stubReporter{},
		logz:         &stubLogz{},
	}
}

func (f *finalizerFixture) finalizer() *Finalizer {
	return &Finalizer{
		PipelineSvc:  f.pipelineSvc,
		ExecutionSvc: f.executionSvc,
		StageSvc:     f.stageSvc,
		StepSvc:      f.stepSvc,
		SSEStreamer:  f.streamer,
		Reporter:     f.reporter,
		Logz:         f.logz,
	}
}

// blockedStage returns a stage that was gated by an approval and never
// dispatched: pending, with one pending step and no start timestamp.
func blockedStage() *stageModel.StageWithSteps {
	return &stageModel.StageWithSteps{
		Stage: stageModel.Stage{
			ID:          11,
			ExecutionID: 42,
			Code:        "build",
			Name:        "build",
			Status:      constant.CIStatusPending,
			RunPolicy:   string(constant.RunPolicyOnSuccess),
		},
		Steps: []*stepModel.Step{
			{ID: 21, StageID: 11, Name: "build", Status: constant.CIStatusPending},
		},
	}
}

func TestFinalizeFailedStageMarksStageFailedAndRollsUpExecution(t *testing.T) {
	t.Parallel()

	f := newFinalizerFixture()
	stage := blockedStage()
	f.stageSvc.stage = stage
	f.stageSvc.siblings = []*stageModel.StageWithSteps{stage}
	f.executionSvc.execution = &executionModel.Execution{
		ID:         42,
		PipelineID: 7,
		Status:     constant.CIStatusRunning,
	}

	if err := f.finalizer().FinalizeFailedStage(context.Background(), stage.ID, "approvalRejected", "rejected"); err != nil {
		t.Fatalf("FinalizeFailedStage returned error: %v", err)
	}

	if stage.Status != constant.CIStatusFailure {
		t.Errorf("stage status = %q, want %q", stage.Status, constant.CIStatusFailure)
	}
	if stage.ErrorType != "approvalRejected" {
		t.Errorf("stage errorType = %q, want approvalRejected", stage.ErrorType)
	}
	if stage.ErrorLog != "rejected" {
		t.Errorf("stage errorLog = %q, want rejected", stage.ErrorLog)
	}
	if stage.Finished == nil {
		t.Error("stage finished is nil, want a timestamp")
	}
	if stage.Started != nil {
		t.Errorf("stage started = %v, want nil (the stage never ran)", stage.Started)
	}
	if got := stage.Steps[0].Status; got != constant.CIStatusSkipped {
		t.Errorf("step status = %q, want %q", got, constant.CIStatusSkipped)
	}
	if f.executionSvc.execution.Status != constant.CIStatusFailure {
		t.Errorf("execution status = %q, want %q", f.executionSvc.execution.Status, constant.CIStatusFailure)
	}
	if f.executionSvc.execution.FinishedDate == nil {
		t.Error("execution finished date is nil, want a timestamp")
	}
	if f.executionSvc.updateCalls != 1 {
		t.Errorf("execution update calls = %d, want 1", f.executionSvc.updateCalls)
	}
	if f.streamer.publishCalls != 1 {
		t.Errorf("SSE publish calls = %d, want 1", f.streamer.publishCalls)
	}
	if f.reporter.calls != 1 {
		t.Errorf("execution reported %d times, want 1", f.reporter.calls)
	}
}

func TestFinalizeFailedStageSkipsDownstreamOnSuccessPolicy(t *testing.T) {
	t.Parallel()

	f := newFinalizerFixture()
	stage := blockedStage()
	downstream := &stageModel.StageWithSteps{
		Stage: stageModel.Stage{
			ID:          12,
			Code:        "deploy",
			Status:      constant.CIStatusWaitingOnDeps,
			Depends:     []string{"build"},
			RunPolicy:   string(constant.RunPolicyOnSuccess),
			ExecutionID: 42,
		},
	}
	f.stageSvc.stage = stage
	f.stageSvc.siblings = []*stageModel.StageWithSteps{stage, downstream}
	f.executionSvc.execution = &executionModel.Execution{ID: 42, PipelineID: 7, Status: constant.CIStatusRunning}

	if err := f.finalizer().FinalizeFailedStage(context.Background(), stage.ID, "approvalRejected", "rejected"); err != nil {
		t.Fatalf("FinalizeFailedStage returned error: %v", err)
	}

	if downstream.Status != constant.CIStatusSkipped {
		t.Errorf("downstream status = %q, want %q", downstream.Status, constant.CIStatusSkipped)
	}
	if f.executionSvc.execution.Status != constant.CIStatusFailure {
		t.Errorf("execution status = %q, want %q", f.executionSvc.execution.Status, constant.CIStatusFailure)
	}
}

func TestFinalizeFailedStageSchedulesDownstreamOnFailurePolicy(t *testing.T) {
	t.Parallel()

	f := newFinalizerFixture()
	stage := blockedStage()
	// The finalizer carries no scheduler, so the downstream stage must be
	// flipped to pending and left for the next queue signal.
	downstream := &stageModel.StageWithSteps{
		Stage: stageModel.Stage{
			ID:          12,
			Code:        "cleanup",
			Status:      constant.CIStatusWaitingOnDeps,
			Depends:     []string{"build"},
			RunPolicy:   string(constant.RunPolicyOnFailure),
			ExecutionID: 42,
		},
	}
	f.stageSvc.stage = stage
	f.stageSvc.siblings = []*stageModel.StageWithSteps{stage, downstream}
	f.executionSvc.execution = &executionModel.Execution{ID: 42, PipelineID: 7, Status: constant.CIStatusRunning}

	if err := f.finalizer().FinalizeFailedStage(context.Background(), stage.ID, "approvalRejected", "rejected"); err != nil {
		t.Fatalf("FinalizeFailedStage returned error: %v", err)
	}

	if downstream.Status != constant.CIStatusPending {
		t.Errorf("downstream status = %q, want %q", downstream.Status, constant.CIStatusPending)
	}
	// A pending downstream stage keeps the execution open for the next round.
	if f.executionSvc.execution.Status != constant.CIStatusRunning {
		t.Errorf("execution status = %q, want it untouched (running)", f.executionSvc.execution.Status)
	}
}

func TestFinalizeFailedStageIsIdempotent(t *testing.T) {
	t.Parallel()

	f := newFinalizerFixture()
	stage := blockedStage()
	stage.Status = constant.CIStatusFailure
	f.stageSvc.stage = stage
	f.stageSvc.siblings = []*stageModel.StageWithSteps{stage}
	f.executionSvc.execution = &executionModel.Execution{ID: 42, PipelineID: 7, Status: constant.CIStatusFailure}

	if err := f.finalizer().FinalizeFailedStage(context.Background(), stage.ID, "approvalRejected", "rejected"); err != nil {
		t.Fatalf("FinalizeFailedStage returned error: %v", err)
	}

	if f.stageSvc.updateCalls != 0 {
		t.Errorf("stage update calls = %d, want 0", f.stageSvc.updateCalls)
	}
	if f.executionSvc.updateCalls != 0 {
		t.Errorf("execution update calls = %d, want 0", f.executionSvc.updateCalls)
	}
	if f.streamer.publishCalls != 0 {
		t.Errorf("SSE publish calls = %d, want 0", f.streamer.publishCalls)
	}
}

func TestFinalizeFailedStageLeavesAlreadySettledStageAlone(t *testing.T) {
	t.Parallel()

	f := newFinalizerFixture()
	// A sibling's teardown already skipped this stage because of its run
	// policy; the execution itself is still running.
	stage := blockedStage()
	stage.Status = constant.CIStatusSkipped
	stage.Steps[0].Status = constant.CIStatusSkipped
	f.stageSvc.stage = stage
	f.stageSvc.siblings = []*stageModel.StageWithSteps{stage}
	f.executionSvc.execution = &executionModel.Execution{ID: 42, PipelineID: 7, Status: constant.CIStatusRunning}

	if err := f.finalizer().FinalizeFailedStage(context.Background(), stage.ID, "approvalRejected", "rejected"); err != nil {
		t.Fatalf("FinalizeFailedStage returned error: %v", err)
	}

	if stage.Status != constant.CIStatusSkipped {
		t.Errorf("stage status = %q, want it left as %q", stage.Status, constant.CIStatusSkipped)
	}
	if f.stageSvc.updateCalls != 0 {
		t.Errorf("stage update calls = %d, want 0", f.stageSvc.updateCalls)
	}
	if stage.ErrorType != "" {
		t.Errorf("stage errorType = %q, want empty", stage.ErrorType)
	}
}

func TestFinalizeFailedStageOnCancelledExecutionDoesNotOverwriteRollup(t *testing.T) {
	t.Parallel()

	f := newFinalizerFixture()
	stage := blockedStage()
	f.stageSvc.stage = stage
	f.executionSvc.execution = &executionModel.Execution{ID: 42, PipelineID: 7, Status: constant.CIStatusKilled}

	if err := f.finalizer().FinalizeFailedStage(context.Background(), stage.ID, "approvalRejected", "rejected"); err != nil {
		t.Fatalf("FinalizeFailedStage returned error: %v", err)
	}

	if stage.Status != constant.CIStatusSkipped {
		t.Errorf("stage status = %q, want %q", stage.Status, constant.CIStatusSkipped)
	}
	if stage.Steps[0].Status != constant.CIStatusSkipped {
		t.Errorf("step status = %q, want %q", stage.Steps[0].Status, constant.CIStatusSkipped)
	}
	if f.executionSvc.execution.Status != constant.CIStatusKilled {
		t.Errorf("execution status = %q, want it left as %q", f.executionSvc.execution.Status, constant.CIStatusKilled)
	}
	if f.executionSvc.updateCalls != 0 {
		t.Errorf("execution update calls = %d, want 0", f.executionSvc.updateCalls)
	}
}

func TestFinalizeFailedStageRetriesTheTeardownTail(t *testing.T) {
	t.Parallel()

	f := newFinalizerFixture()
	stage := blockedStage()
	f.stageSvc.stage = stage
	f.stageSvc.siblings = []*stageModel.StageWithSteps{stage}
	f.executionSvc.execution = &executionModel.Execution{ID: 42, PipelineID: 7, Status: constant.CIStatusRunning}

	// The stage write fails on every attempt, so the rollup never runs and the
	// error must surface after the bounded retries.
	f.stageSvc.updateErr = errors.New("db is down")

	err := f.finalizer().FinalizeFailedStage(context.Background(), stage.ID, "approvalRejected", "rejected")
	if err == nil {
		t.Fatal("FinalizeFailedStage returned nil, want an error")
	}
	if f.stageSvc.updateCalls != finalizeAttempts {
		t.Errorf("stage update calls = %d, want %d", f.stageSvc.updateCalls, finalizeAttempts)
	}
}
