/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package manager

import (
	"context"

	"github.com/easysoft/gitfox-community/app/constant"
	events "github.com/easysoft/gitfox-community/app/events/pipeline"
	eventModel "github.com/easysoft/gitfox-community/app/model/event"
	executionModel "github.com/easysoft/gitfox-community/app/model/execution"
	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
	"github.com/easysoft/gitfox-community/app/service/pipeline"
	"github.com/easysoft/gitfox-community/app/service/sse"
	"github.com/easysoft/gitfox-community/pkg/logger"
	"github.com/easysoft/gitfox-community/pkg/utils/datetime"
)

type ExecutionUpdater interface {
	Update(ctx context.Context, execution *executionModel.Execution) error
}

type StageLister interface {
	ListWithSteps(ctx context.Context, executionID uint) ([]*stageModel.StageWithSteps, error)
}

type ExecutionFinalizer struct {
	ExecutionUpdater ExecutionUpdater
	StageLister      StageLister
	PipelineSvc      pipeline.Interface
	SSEStreamer      sse.Streamer
	Reporter         events.Reporter
}

// Finalize 数据库侧收尾：
// 写入最终状态与时间，StartedDate 为空时回退为 FinishedDate；
// 同步流水线的最近执行结果（LastExec / LastResult）。
func (f *ExecutionFinalizer) Finalize(ctx context.Context, execution *executionModel.Execution, status constant.CIStatus) error {
	log := logger.GetLogger(ctx).With(
		"execution.id", execution.ID,
		"execution.status", string(status),
	)

	execution.Status = status
	execution.FinishedDate = datetime.NowLocalTime()
	// 全跳过时不会经过 setup，StartedDate 可能为 nil，这里回退为 FinishedDate，避免执行详情里的时长为空。
	if execution.StartedDate == nil {
		execution.StartedDate = execution.FinishedDate
	}
	if err := f.ExecutionUpdater.Update(ctx, execution); err != nil {
		log.Warnw("finalize: cannot update the execution", "error", err)
		return err
	}

	pl, err := f.PipelineSvc.GetPipelineByID(ctx, int64(execution.PipelineID))
	if err != nil {
		log.Warnw("finalize: cannot find pipeline", "error", err)
		return err
	}
	pl.LastExec = execution.FinishedDate
	pl.LastResult = string(execution.Status)
	if err := f.PipelineSvc.UpdatePipeline(ctx, pl); err != nil {
		log.Warnw("finalize: cannot update the pipeline", "error", err)
	}

	return nil
}

// Publish 对外通知：SSE 完成事件 + 事件订阅方（通知、插件等）。
func (f *ExecutionFinalizer) Publish(ctx context.Context, execution *executionModel.Execution) {
	log := logger.GetLogger(ctx).With("execution.id", execution.ID)

	// 取不到流水线就整体跳过对外通知
	pl, err := f.PipelineSvc.GetPipelineByID(ctx, int64(execution.PipelineID))
	if err != nil {
		log.Warnw("finalize: cannot find pipeline, skip execution completed event and notification", "error", err)
		return
	}

	stages, err := f.StageLister.ListWithSteps(ctx, execution.ID)
	if err != nil {
		log.Warnw("finalize: cannot get stages", "error", err)
		stages = nil
	}

	executionDetail := executionModel.ExecutionWithStages{
		Execution: *execution,
		Stages:    stages,
	}
	if err := f.SSEStreamer.Publish(ctx, pl.SpaceID, eventModel.SSETypeExecutionCompleted, executionDetail); err != nil {
		log.Warnw("finalize: could not publish execution completed event", "error", err)
	}

	f.Reporter.Executed(ctx, &events.ExecutedPayload{
		PipelineID:   execution.PipelineID,
		ExecutionNum: execution.ID,
	})
}
