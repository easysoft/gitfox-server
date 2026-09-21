package manager

import (
	"context"
	"fmt"

	"github.com/easysoft/gitfox-community/app/constant"
	stepModel "github.com/easysoft/gitfox-community/app/model/step"
	"github.com/easysoft/gitfox-community/app/service/pipeline"
	"github.com/easysoft/gitfox-community/app/service/step"
	"github.com/easysoft/gitfox-community/pkg/logger"
	v1yaml "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
	"github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse"
	normalize "github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse/normalize"
	specresolver "github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse/resolver"
	"github.com/easysoft/gitfox-community/pkg/utils/datetime"
)

// PipelineLookupFunc 返回展开插件/模板阶段所需的查询函数。
func PipelineLookupFunc(ctx context.Context, pipelineSvc pipeline.Interface) specresolver.LookupFunc {
	return func(name, kind, typ, version string) (*v1yaml.Config, error) {
		yamlConfig, err := pipelineSvc.Resolve(ctx, kind, typ, name)
		if err != nil {
			return nil, err
		}
		return parse.ToConfig(yamlConfig)
	}
}

// StageStepsFromYAML 解析流水线 YAML，用于阶段被 when 条件跳过后，把阶段内的任务一并落库为 skipped。
func StageStepsFromYAML(data []byte, lookup specresolver.LookupFunc) (map[string][]*v1yaml.Step, error) {
	config, err := v1yaml.ParseBytes(data)
	if err != nil {
		return nil, fmt.Errorf("could not parse pipeline yaml: %w", err)
	}
	if err := normalize.Normalize(config); err != nil {
		return nil, fmt.Errorf("could not normalize pipeline yaml: %w", err)
	}
	if err := specresolver.Resolve(config, lookup); err != nil {
		return nil, fmt.Errorf("could not resolve yaml plugins/templates: %w", err)
	}

	pipeline_, ok := config.Spec.(*v1yaml.Pipeline)
	if !ok {
		return nil, fmt.Errorf("unknown yaml: not a pipeline")
	}

	steps := map[string][]*v1yaml.Step{}
	for _, stage := range pipeline_.Stages {
		stageCI, ok := stage.Spec.(*v1yaml.StageCI)
		if !ok {
			continue
		}
		steps[stage.Id] = stageCI.Steps
	}
	return steps, nil
}

// SkippedStepRecords 把任务转换成状态为 skipped 的任务记录。
func SkippedStepRecords(steps []*v1yaml.Step, stageID, executionID uint) []*stepModel.Step {
	records := []*stepModel.Step{}

	var walk func(list []*v1yaml.Step)
	walk = func(list []*v1yaml.Step) {
		for _, item := range list {
			switch spec := item.Spec.(type) {
			case *v1yaml.StepGroup:
				walk(spec.Steps)
				continue
			case *v1yaml.StepParallel:
				walk(spec.Steps)
				continue
			}

			now := datetime.NowLocalTime()
			records = append(records, &stepModel.Step{
				StageID:  stageID,
				Code:     fmt.Sprintf("%s_%d", item.Id, executionID),
				Name:     item.Name,
				SN:       len(records) + 1,
				Status:   constant.CIStatusSkipped,
				Started:  now,
				Finished: now,
			})
		}
	}
	walk(steps)

	return records
}

// CreateSkippedSteps 把 SkippedStepRecords 生成的任务记录逐条落库。
func CreateSkippedSteps(ctx context.Context, stepSvc step.Interface, records []*stepModel.Step) error {
	log := logger.GetLogger(ctx)

	for _, record := range records {
		if err := stepSvc.Create(ctx, record); err != nil {
			log.Errorw("manager: cannot create skipped step", "stage.id", record.StageID, "step.code", record.Code, "error", err)
			return err
		}
	}
	return nil
}
