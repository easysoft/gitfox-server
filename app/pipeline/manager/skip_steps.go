package manager

import (
	"context"
	"fmt"

	"github.com/easysoft/gitfox-community/app/constant"
	executionModel "github.com/easysoft/gitfox-community/app/model/execution"
	stepModel "github.com/easysoft/gitfox-community/app/model/step"
	"github.com/easysoft/gitfox-community/app/service/pipeline"
	"github.com/easysoft/gitfox-community/app/service/step"
	"github.com/easysoft/gitfox-community/pkg/logger"
	v1yaml "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
	"github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse"
	normalize "github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse/normalize"
	specresolver "github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse/resolver"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/inputs"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/script"
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
// 解析顺序与 runner 对齐：normalize → resolve → 展开 ${{ }} → normalize，
// 保证被跳过阶段的任务名/脚本与 runner 编译结果一致。任务 Id 仍由展开前的名称生成，
// 这样 map key（stage.Id）与 parseV1Stages 生成 stage.Code 时使用的值保持一致。
func StageStepsFromYAML(data []byte, lookup specresolver.LookupFunc, execution *executionModel.Execution) (map[string][]*v1yaml.Step, error) {
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

	// 与 runner 一致地展开 ${{ }}；否则含表达式的任务名会和实际编译结果不一致。
	script.ExpandConfig(config, stageStepsExprInputs(execution, pipeline_))

	// 解析/展开后再次 normalize，补全解析期注入的任务 Id。
	if err := normalize.Normalize(config); err != nil {
		return nil, fmt.Errorf("could not normalize pipeline yaml: %w", err)
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

// stageStepsExprInputs 构造与 runner 编译期尽可能一致的表达式上下文。
// 服务端拿不到明文字典（secrets 返回空串）和完整的 stage/repo 环境变量，
// 但 build/parameters/inputs/global/env 足以覆盖任务 Id/Name 的常见写法。
func stageStepsExprInputs(execution *executionModel.Execution, pipeline_ *v1yaml.Pipeline) map[string]interface{} {
	if execution == nil {
		execution = &executionModel.Execution{}
	}

	plain, global, _ := inputs.SplitGlobal(execution.Params)
	envs := make(map[string]string, len(plain))
	for k, v := range plain {
		envs[k] = v
	}

	return map[string]interface{}{
		"build":      inputs.Build(ConvertToDroneBuild(execution)),
		"matrix":     map[string]interface{}{},
		"parameters": inputs.Inputs(pipeline_.Inputs, plain),
		"inputs":     inputs.Inputs(pipeline_.Inputs, plain),
		"global":     global,
		"env":        envs,
		"secrets": map[string]interface{}{
			"get": func(string) string { return "" },
		},
	}
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
