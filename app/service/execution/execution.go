package execution

import (
	"context"

	executionModel "github.com/easysoft/gitfox-community/app/model/execution"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/inputs"
)

// Create creates a new execution
func (s *Service) Create(ctx context.Context, execution *executionModel.Execution) error {
	return s.executionStore.Create(ctx, execution)
}

// Update updates an execution
func (s *Service) Update(ctx context.Context, execution *executionModel.Execution) error {
	return s.executionStore.Update(ctx, execution)
}

// GetByID gets execution by ID
func (s *Service) GetByID(ctx context.Context, id uint) (*executionModel.Execution, error) {
	return s.executionStore.GetByID(ctx, id)
}

// GetWithStages gets execution with all stages and steps
func (s *Service) GetWithStages(ctx context.Context, id uint) (*executionModel.ExecutionWithStages, error) {
	// Get execution
	exec, err := s.executionStore.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get stages with steps
	stages, err := s.stageStore.ListWithSteps(ctx, id)
	if err != nil {
		return nil, err
	}

	// 对外展示时脱敏：全局参数中的密文值替换成 ******。
	display := *exec
	display.Params = inputs.RedactGlobalParams(exec.Params)

	return &executionModel.ExecutionWithStages{
		Execution: display,
		Stages:    stages,
	}, nil
}

// ListByPr gets executions by PR with pipeline name
func (s *Service) ListByPr(ctx context.Context, filter executionModel.ExecutionInfoReqFilter) ([]*executionModel.ExecutionByPr, error) {
	return s.executionStore.ListByPr(ctx, filter)
}
