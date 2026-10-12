package scheduler

import (
	"context"

	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
)

// IStageApproval stage 审批闸门（付费扩展点）。
// 返回 blocked=true 表示本次 signal 不派发该 stage（等待审批）。
// 社区版绑定 NoopStageApproval（永不阻塞），付费版绑定 app/pro/approval.Service。
type IStageApproval interface {
	BeforeStageDispatch(ctx context.Context, stage *stageModel.Stage) (blocked bool, err error)
}

// NoopStageApproval 社区版实现：不做任何审批判断，行为与现状一致。
type NoopStageApproval struct{}

var _ IStageApproval = (*NoopStageApproval)(nil)

// NewNoopStageApproval creates the community-edition IStageApproval implementation.
func NewNoopStageApproval() *NoopStageApproval {
	return &NoopStageApproval{}
}

// BeforeStageDispatch always allows the stage to be dispatched.
func (s *NoopStageApproval) BeforeStageDispatch(_ context.Context, _ *stageModel.Stage) (bool, error) {
	return false, nil
}
