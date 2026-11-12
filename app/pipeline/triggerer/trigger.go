// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package triggerer

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/easysoft/gitfox-community/app/constant"
	events "github.com/easysoft/gitfox-community/app/events/pipeline"
	executionModel "github.com/easysoft/gitfox-community/app/model/execution"
	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
	"github.com/easysoft/gitfox-community/app/pipeline/manager"
	"github.com/easysoft/gitfox-community/app/pipeline/scheduler"

	pipelineSvc "github.com/easysoft/gitfox-community/app/service/pipeline"
	"github.com/easysoft/gitfox-community/app/service/sse"
	stepSvc "github.com/easysoft/gitfox-community/app/service/step"
	executionStore "github.com/easysoft/gitfox-community/app/store/execution"
	stageStore "github.com/easysoft/gitfox-community/app/store/stage"
	"github.com/easysoft/gitfox-community/pkg/config"
	"github.com/easysoft/gitfox-community/pkg/database/dbtx"
	"github.com/easysoft/gitfox-community/pkg/errors"
	"github.com/easysoft/gitfox-community/pkg/i18n"
	"github.com/easysoft/gitfox-community/pkg/logger"
	v1yaml "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
	normalize "github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse/normalize"
	specresolver "github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse/resolver"
	"github.com/easysoft/gitfox-community/pkg/pipeline/spec/parse/validate"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/inputs"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/script"
	"github.com/easysoft/gitfox-community/pkg/utils/datetime"
)

var _ Triggerer = (*triggerer)(nil)

// Hook represents the payload of a post-commit hook.
type Hook struct {
	Parent       int64                  `json:"parent"`
	Trigger      string                 `json:"trigger"`
	TriggeredBy  int64                  `json:"triggered_by"`
	Link         string                 `json:"link"`
	Timestamp    int64                  `json:"timestamp"`
	Action       constant.TriggerAction `json:"action"`
	Ref          string                 `json:"ref"`
	Commit       string                 `json:"commit"`
	CommitBefore string                 `json:"commitBefore"`
	Cron         string                 `json:"cron"`
	Sender       string                 `json:"sender"`
	Params       map[string]string      `json:"params"`
	// 用于匹配 scan trigger 的字段，与 Ref 可能不同
	// 比如对于 branch 事件，Ref 是 refs/heads/main，而 Match 是 main；对于 tag 事件，Ref 是 refs/tags/v1.0，而 Match 是 v1.0
	// 比如对于 pull request 事件，Ref 是 refs/pullreq/123/head，而 Match 根据 action 的不同而不同
	Match       string `json:"match"`
	CommitMatch string `json:"commitMatch"`
	// 重试相关字段
	RetrySourceID     *int64 `json:"retry_source_id,omitempty"`
	RetryCount        int    `json:"retry_count,omitempty"`
	RetrySourceNumber int64  `json:"retry_source_number,omitempty"`
}

// Triggerer is responsible for triggering a Execution from an
// incoming hook (could be manual or webhook). If an execution is skipped a nil value is
// returned.
type Triggerer interface {
	Trigger(ctx context.Context, pipelineID uint, hook *Hook) (*executionModel.Execution, error)
}

type triggerer struct {
	pipelineService  pipelineSvc.Interface
	executionStore   executionStore.Interface
	stageStore       stageStore.Interface
	stepService      stepSvc.Interface
	scheduler        scheduler.Scheduler
	tx               dbtx.TxRunner
	config           *config.Config
	sseStreamer      sse.Streamer
	reporter         events.Reporter
	variableResolver GlobalParamResolver
}

func New(
	pipelineService pipelineSvc.Interface,
	executionStore executionStore.Interface,
	stageStore stageStore.Interface,
	stepService stepSvc.Interface,
	scheduler scheduler.Scheduler,
	tx dbtx.TxRunner,
	cfg *config.Config,
	sseStreamer sse.Streamer,
	reporter events.Reporter,
	variableResolver GlobalParamResolver,
) Triggerer {
	return &triggerer{
		pipelineService:  pipelineService,
		executionStore:   executionStore,
		stageStore:       stageStore,
		stepService:      stepService,
		scheduler:        scheduler,
		tx:               tx,
		config:           cfg,
		sseStreamer:      sseStreamer,
		reporter:         reporter,
		variableResolver: variableResolver,
	}
}

// GlobalParamResolver 按参数名查询全局参数表。
// 返回值：命中的全局参数（name -> value）、需要日志掩码的参数名。
type GlobalParamResolver interface {
	ResolveByKeys(ctx context.Context, keys []string) (map[string]string, []string, error)
}

//nolint:gocognit,gocyclo,cyclop //TODO: Refactor @Vistaar
func (t *triggerer) Trigger(
	ctx context.Context,
	pipelineID uint,
	base *Hook,
) (*executionModel.Execution, error) {
	log := logger.GetLogger(ctx).With(
		"pipeline.id", pipelineID,
		"trigger.ref", base.Ref,
	)

	log.Info("[trigger]: received")
	defer func() {
		// taking the paranoid approach to recover from
		// a panic that should absolutely never happen.
		if r := recover(); r != nil {
			log.Errorf("[trigger]: unexpected panic: %s", r)
			debug.PrintStack()
		}
	}()

	// 从数据库查询 pipeline yaml 内容
	file, err := t.pipelineService.GetPipelineContent(ctx, int64(pipelineID))
	if err != nil {
		log.Errorf("[trigger]: could not find pipeline content: %v", err)
		return nil, err
	}

	// 将 gitfox 服务的 Host/Port 注入到 base.Params，供 YAML 中作为环境变量使用
	if base.Params == nil {
		base.Params = make(map[string]string)
	}
	base.Params["PLUGIN_GITFOX_SERVER_HOST"] = t.config.Server.ServerHost
	base.Params["PLUGIN_GITFOX_SERVER_PORT"] = fmt.Sprintf("%d", t.config.Server.Port)
	base.Params["PLUGIN_GITFOX_SERVER_PROTO"] = t.config.Server.Proto
	// 注入插件调用 gitfox API 所需的环境变量（与 scan pipeline 注入保持一致）
	base.Params["PLUGIN_TOKEN"] = t.config.PipelineAuth.Password
	base.Params["GITFOX_API_ENDPOINT"] = t.config.URL.Base + "/api/v2"

	if globals, masked, err := t.resolveGlobalParams(ctx, file.Data); err != nil {
		log.Warnf("[trigger]: resolve global params failed: %v", err)
	} else {
		for name, value := range globals {
			key := inputs.GlobalKey(name)
			// 若用户传入的变量和全局变量同名，优先使用用户传入的变量
			if _, exists := base.Params[key]; !exists {
				base.Params[key] = value
			}
		}
		if len(masked) > 0 {
			base.Params[inputs.GlobalMaskKey] = strings.Join(masked, ",")
		}
		if len(globals) > 0 {
			log.Infof("[trigger]: injected %d global params, masked=%v", len(globals), masked)
		} else if refs := inputs.GlobalRefs(file.Data); len(refs) > 0 {
			// 流水线引用了全局参数但一条都没命中
			log.Warnf("[trigger]: global params referenced but not found in table: %v", refs)
		}
	}

	if globals, masked, err := t.resolveGlobalParams(ctx, file.Data); err != nil {
		log.Warnf("[trigger]: resolve global params failed: %v", err)
	} else {
		for name, value := range globals {
			key := inputs.GlobalKey(name)
			// 若用户传入的变量和全局变量同名，优先使用用户传入的变量
			if _, exists := base.Params[key]; !exists {
				base.Params[key] = value
			}
		}
		if len(masked) > 0 {
			base.Params[inputs.GlobalMaskKey] = strings.Join(masked, ",")
		}
		if len(globals) > 0 {
			log.Infof("[trigger]: injected %d global params, masked=%v", len(globals), masked)
		} else if refs := inputs.GlobalRefs(file.Data); len(refs) > 0 {
			// 流水线引用了全局参数但一条都没命中
			log.Warnf("[trigger]: global params referenced but not found in table: %v", refs)
		}
	}

	trigger := string(base.Action)
	createdBy := base.Trigger
	if trigger == "" {
		trigger = "manual"
	}
	if trigger == "auto" {
		createdBy = "system"
	}
	execution := &executionModel.Execution{
		Commit:       base.Commit,
		CommitBefore: base.CommitBefore,
		PipelineID:   pipelineID,
		Trigger:      trigger, // 使用 action 取代原先的 event
		CreatedBy:    createdBy,
		Parent:       uint(base.Parent),
		Status:       constant.CIStatusPending,
		Ref:          base.Ref,
		Params:       base.Params,
	}

	// For V1 YAML - for now we can just parse the stages and create them sequentially.
	stages, err := parseV1Stages(ctx, file.Data, execution, base, t.pipelineService)
	if err != nil {
		// 具体错误信息通过 i18n 模板的 %s 透传给用户（如缺少 stages/steps 关键字的校验错误）
		return nil, errors.WithI18nKey(err, i18n.PipelineTriggerV1ParseError)
	}

	// 被 when 条件跳过的阶段，阶段内的任务也要落库为 skipped。
	skippedSteps, err := t.skippedStageSteps(ctx, file.Data, stages, execution)
	if err != nil {
		return nil, errors.WithI18nKey(
			fmt.Errorf("could not parse skipped steps: %w", err),
			i18n.ExecutionTriggerError,
		)
	}

	// 全部阶段被跳过的 execution 不会经过 setup/teardown。
	allSkipped := allStagesSkipped(stages)

	// 事务：execution + 阶段 + 被跳过阶段内的任务 + 全跳过时的收尾。
	if err := t.createExecutionWithStagesWithTx(ctx, execution, stages, skippedSteps, allSkipped); err != nil {
		return nil, errors.WithI18nKey(
			fmt.Errorf("could not create execution: %w", err),
			i18n.ExecutionTriggerError,
		)
	}

	if allSkipped {
		finalizer := t.finalizer()
		finalizer.Publish(ctx, execution)
		log.Info("[trigger]: all stages are skipped, execution finished as skipped")
		return execution, nil
	}

	for _, stage := range stages {
		if stage.Status != constant.CIStatusPending {
			continue
		}
		log.Infof("[trigger] scheduling stage code=%s name=%s", stage.Code, stage.Name)
		err = t.scheduler.Schedule()
		if err != nil {
			log.Errorf("[trigger]: cannot enqueue execution: %s", err)
			return nil, err
		}
	}

	return execution, nil
}

// allStagesSkipped 判断是否所有阶段都被 when 条件跳过。
func allStagesSkipped(stages []*stageModel.Stage) bool {
	for _, stage := range stages {
		if stage.Status != constant.CIStatusSkipped {
			return false
		}
	}
	return len(stages) > 0
}

// skippedStageSteps 预解析流水线 YAML，取出各阶段的任务定义（键为阶段 code）。
func (t *triggerer) skippedStageSteps(ctx context.Context, data []byte, stages []*stageModel.Stage, execution *executionModel.Execution) (map[string][]*v1yaml.Step, error) {
	hasSkipped := false
	for _, stage := range stages {
		if stage.Status == constant.CIStatusSkipped {
			hasSkipped = true
			break
		}
	}
	if !hasSkipped {
		return nil, nil
	}
	return manager.StageStepsFromYAML(data, manager.PipelineLookupFunc(ctx, t.pipelineService), execution)
}

// createSkippedStageSteps 把被 when 条件跳过的阶段内的任务也标记为 skipped。
func (t *triggerer) createSkippedStageSteps(ctx context.Context, stepsByStage map[string][]*v1yaml.Step, execution *executionModel.Execution, stages []*stageModel.Stage) error {
	for _, stage := range stages {
		if stage.Status != constant.CIStatusSkipped {
			continue
		}
		records := manager.SkippedStepRecords(stepsByStage[stage.Code], stage.ID, execution.ID)
		if err := manager.CreateSkippedSteps(ctx, t.stepService, records); err != nil {
			return err
		}
	}
	return nil
}

// directStageRunPolicy maps stage when literals to the persisted run policy.
// pipeline* names are canonical; the original names remain compatibility aliases.
func directStageRunPolicy(condition string) (string, bool) {
	switch condition {
	case string(constant.RunPolicyOnSuccess), "success", "success()",
		v1yaml.WhenPipelineSuccess, v1yaml.WhenPipelineSuccess + "()":
		return string(constant.RunPolicyOnSuccess), true
	case string(constant.RunPolicyOnFailure), "failure", "failure()",
		v1yaml.WhenPipelineFailure, v1yaml.WhenPipelineFailure + "()":
		return string(constant.RunPolicyOnFailure), true
	case string(constant.RunPolicyAlways), "always()":
		return string(constant.RunPolicyAlways), true
	default:
		return "", false
	}
}

// parseV1Stages tries to parse the yaml into a list of stages and returns an error
// if we are unable to do so or the yaml contains something unexpected.
// Currently, all the stages will be executed one after the other on completion.
// Once we have depends on in v1, this will be changed to use the DAG.
//
//nolint:gocognit // refactor if needed.
func parseV1Stages(
	ctx context.Context,
	data []byte,
	execution *executionModel.Execution,
	base *Hook,
	pipelineService pipelineSvc.Interface,
) ([]*stageModel.Stage, error) {
	stages := []*stageModel.Stage{}
	// For V1 YAML, just go through the YAML and create stages serially for now
	config, err := v1yaml.ParseBytes(data)
	if err != nil {
		return nil, fmt.Errorf("could not parse v1 yaml: %w", err)
	}

	// stage/step 的 id、name 不支持表达式，必须在 normalize 之前校验：
	// normalize 会对 id 做 slug 化，把 "${{ ... }}" 的语法抹掉，之后就识别不出来了。
	if pipeline, ok := config.Spec.(*v1yaml.Pipeline); ok {
		if err := validateV1StaticNames(pipeline); err != nil {
			return nil, err
		}
	}

	// Normalize the config to make sure stage names and step names are unique
	err = normalize.Normalize(config)
	if err != nil {
		return nil, fmt.Errorf("could not normalize v1 yaml: %w", err)
	}

	if config.Kind != "pipeline" {
		return nil, fmt.Errorf("cannot support non-pipeline kinds in v1 at the moment: %w", err)
	}

	// parameters 只保留普通参数，全局参数单独放进 global 命名空间。
	plainParams, globalParams, _ := inputs.SplitGlobal(base.Params)

	inputParams := map[string]interface{}{}
	inputParams["build"] = inputs.Build(manager.ConvertToDroneBuild(execution))
	inputParams["parameters"] = plainParams
	inputParams["global"] = globalParams

	var prevStage string

	// expand stage level plugins
	lookupFunc := manager.PipelineLookupFunc(ctx, pipelineService)

	if err := specresolver.Resolve(config, lookupFunc); err != nil {
		return nil, fmt.Errorf("could not resolve yaml plugins/templates: %w", err)
	}

	switch v := config.Spec.(type) {
	case *v1yaml.Pipeline:
		// 校验 yaml 关键字完整性与卷挂载声明，避免关键字拼写错误导致静默生成
		// 空流水线，或因卷名/路径/卷类型有误导致挂载在运行时被静默丢弃
		if err := validateV1PipelineSpec(v); err != nil {
			return nil, err
		}

		// Expand expressions in strings and matrices
		script.ExpandConfig(config, inputParams)

		for idx, stage := range v.Stages {
			// Only parse CI stages for now
			switch stage.Spec.(type) {
			case *v1yaml.StageCI:
				runPolicy := string(constant.RunPolicyOnSuccess)
				// when 条件在成功、失败状态下都不成立时，该阶段被跳过。
				skipStage := false
				// 条件不成立时把原因写入 errorLog，随阶段详情返回给前端。
				whenErrorLog := ""
				if stage.When != nil {
					if when := stage.When.Eval; when != "" {
						trimmedWhen := strings.TrimSpace(when)
						if policy, ok := directStageRunPolicy(trimmedWhen); ok {
							// Direct policy value, use it as-is
							runPolicy = policy
						} else {
							// Evaluate as expression
							onSuccess, onFailure, err := script.EvalWhen(trimmedWhen, inputParams)
							if err != nil {
								return nil, fmt.Errorf("could not resolve when condition for stage: %w", err)
							}
							switch {
							case onSuccess && onFailure:
								runPolicy = string(constant.RunPolicyAlways)
							case onSuccess:
								runPolicy = string(constant.RunPolicyOnSuccess)
							case onFailure:
								runPolicy = string(constant.RunPolicyOnFailure)
							default:
								// (false, false)：条件不成立，整个阶段跳过
								skipStage = true
							}
						}

						// when 条件不成立时把跳过原因写入 errorLog
						if skipStage {
							whenErrorLog = fmt.Sprintf(v1yaml.WhenSkippedStageTmpl, trimmedWhen)
						}
					}
				}

				dependsOn := []string{}
				if prevStage != "" {
					dependsOn = append(dependsOn, prevStage)
				}
				status := constant.CIStatusWaitingOnDeps
				// If the stage has no dependencies, it can be picked up for execution.
				if len(dependsOn) == 0 {
					status = constant.CIStatusPending
				}
				if skipStage {
					// 阶段级跳过：以 skipped 状态入库，不参与调度，也不再成为下游阶段的依赖。
					status = constant.CIStatusSkipped
				}
				temp := &stageModel.Stage{
					ExecutionID: execution.ID,
					Code:        stage.Id,
					Name:        stage.Name,
					SN:          idx + 1,
					Depends:     dependsOn,
					Status:      status,
					RunPolicy:   runPolicy,
					ErrorLog:    whenErrorLog,
				}

				if stage.Approval != nil {
					temp.ApprovalConfig = &stageModel.StageApproval{
						Approvers: []string(stage.Approval.Approvers),
						Mode:      stage.Approval.Mode,
						Timeout:   stage.Approval.Timeout,
						Percent:   stageApprovalPercent(stage.Approval.Percent),
					}
				}

				// Inline fixStageDetail logic
				stageCI := stage.Spec.(*v1yaml.StageCI)
				if stageCI.Runtime != nil {
					if stageCI.Runtime.Type == "agent" {
						temp.Type = "agent"
						if stageCI.Runtime.Spec != nil {
							runtimeSpec := stageCI.Runtime.Spec.(*v1yaml.RuntimeAgent)
							if len(runtimeSpec.Selector) > 0 {
								temp.Labels = runtimeSpec.Selector
							}
						}
					}
				}
				if stageCI.Platform != nil {
					if stageCI.Platform.Os != "" {
						temp.OS = stageCI.Platform.Os
					}
					if stageCI.Platform.Arch != "" {
						temp.Arch = stageCI.Platform.Arch
					}
				}

				if skipStage {
					now := datetime.NowLocalTime()
					temp.Started = now
					temp.Finished = now
				} else {
					prevStage = temp.Code
				}
				stages = append(stages, temp)
			default:
				return nil, fmt.Errorf("only CI and template stages are supported in v1 at the moment")
			}
		}
	default:
		return nil, fmt.Errorf("unknown yaml: %w", err)
	}
	return stages, nil
}

// validateV1PipelineSpec 校验 v1 yaml 解析结果的完整性，
// 避免 stages/steps 等关键字拼写错误导致静默生成空流水线；
// 同时校验各阶段 step 上的卷挂载声明（需在插件/模板展开之后调用，
// 才能覆盖展开后步骤继承的挂载）。
func validateV1PipelineSpec(pipeline *v1yaml.Pipeline) error {
	if len(pipeline.Stages) == 0 {
		return fmt.Errorf("check 'stages' key in yaml")
	}
	// 插件/模板展开后可能注入新的 step，这里再校验一次；
	// 用户自己编写的 id 此时已被 normalize slug 化，表达式由 parse 阶段的前置校验拦截。
	if err := validateV1StaticNames(pipeline); err != nil {
		return err
	}
	for _, stage := range pipeline.Stages {
		if err := validateV1Stage(stage); err != nil {
			return err
		}
		if err := validateV1StageMounts(stage); err != nil {
			return err
		}
	}
	return nil
}

// validateV1StaticNames 校验 stage/step 的 id、name 不允许使用 ${{ }} 表达式。
//
// 服务端与 runner 编译期的表达式上下文不一致：服务端拿不到明文 secrets，
// env 也只覆盖普通参数。一旦 id/name 里出现表达式，被跳过阶段落库的任务名/Code
// 就会和 runner 实际编译结果不一致；stage name 还可能因为两边展开结果不同导致
// runner 匹配不到阶段。
//
// 注意：对用户 YAML 必须在 normalize 之前调用，normalize 会把 id slug 化，
// 表达式语法会被抹掉；插件/模板展开后再调用一次，用于覆盖注入的 step。
func validateV1StaticNames(pipeline *v1yaml.Pipeline) error {
	for _, stage := range pipeline.Stages {
		if strings.Contains(stage.Id, "${{") {
			return fmt.Errorf("stage id %q does not support expressions", stage.Id)
		}
		if strings.Contains(stage.Name, "${{") {
			return fmt.Errorf("stage name %q does not support expressions", stage.Name)
		}

		stageCI, ok := stage.Spec.(*v1yaml.StageCI)
		if !ok {
			continue
		}
		for _, step := range stageCI.Steps {
			if err := validateV1StepStaticNames(step); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateV1StepStaticNames 递归校验 step 及其 group/parallel 子任务的 id、name。
func validateV1StepStaticNames(step *v1yaml.Step) error {
	if strings.Contains(step.Id, "${{") {
		return fmt.Errorf("step id %q does not support expressions", step.Id)
	}
	if strings.Contains(step.Name, "${{") {
		return fmt.Errorf("step name %q does not support expressions", step.Name)
	}

	switch spec := step.Spec.(type) {
	case *v1yaml.StepGroup:
		for _, child := range spec.Steps {
			if err := validateV1StepStaticNames(child); err != nil {
				return err
			}
		}
	case *v1yaml.StepParallel:
		for _, child := range spec.Steps {
			if err := validateV1StepStaticNames(child); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateV1StageMounts 校验单个 stage 内 step 上的卷挂载声明，
// 错误补充 stage 上下文并保留原始原因（%w），方便定位到具体阶段。
func validateV1StageMounts(stage *v1yaml.Stage) error {
	if err := validate.ValidateStageMounts(stage); err != nil {
		return fmt.Errorf("stage %q: %w", stage.Id, err)
	}
	return nil
}

// validateV1Stage 校验 stage 的 spec 与 steps 存在
func validateV1Stage(stage *v1yaml.Stage) error {
	if err := validateV1StageApproval(stage); err != nil {
		return err
	}

	stageCI, ok := stage.Spec.(*v1yaml.StageCI)
	if !ok {
		// 非 CI 类型 stage 会在后续解析时报错，这里不重复校验
		return nil
	}
	if len(stageCI.Steps) == 0 {
		return fmt.Errorf("check 'steps' key in stage %q", stage.Id)
	}
	for _, step := range stageCI.Steps {
		if err := validateV1Step(step); err != nil {
			return err
		}
	}
	return nil
}

// validateV1StageApproval 校验 stage 的 approval 配置（整体省略时不校验）。
func validateV1StageApproval(stage *v1yaml.Stage) error {
	a := stage.Approval
	if a == nil {
		return nil
	}

	if len(a.Approvers) == 0 {
		return fmt.Errorf("check 'approvers' key in approval of stage %q", stage.Id)
	}
	if len(a.Approvers) > constant.MaxStageApprovalApprovers {
		return fmt.Errorf("too many approvers (%d > %d) in approval of stage %q",
			len(a.Approvers), constant.MaxStageApprovalApprovers, stage.Id)
	}
	switch a.Mode {
	case "", string(constant.StageApprovalModeAnd), string(constant.StageApprovalModeOr):
	default:
		return fmt.Errorf("invalid approval mode %q in stage %q", a.Mode, stage.Id)
	}
	if a.Percent != 0 && (a.Percent < constant.MinStageApprovalPercent || a.Percent > constant.MaxStageApprovalPercent) {
		return fmt.Errorf("invalid approval percent %d in stage %q: must be between %d and %d",
			a.Percent, stage.Id, constant.MinStageApprovalPercent, constant.MaxStageApprovalPercent)
	}
	if a.Timeout != "" {
		if d, err := time.ParseDuration(a.Timeout); err != nil || d <= 0 {
			return fmt.Errorf("invalid approval timeout %q in stage %q", a.Timeout, stage.Id)
		}
	}

	// approvalConfig 列宽为 constant.MaxApprovalConfigLen，超长会被 DB 拒绝或静默截断，
	// 这里前置校验，保证用户看到明确错误。
	raw, err := json.Marshal(&stageModel.StageApproval{
		Approvers: []string(a.Approvers),
		Mode:      a.Mode,
		Timeout:   a.Timeout,
		Percent:   stageApprovalPercent(a.Percent),
	})
	if err != nil {
		return fmt.Errorf("marshal approval config of stage %q: %w", stage.Id, err)
	}
	if len(raw) > constant.MaxApprovalConfigLen {
		return fmt.Errorf("approval config of stage %q is too long (%d > %d), reduce approvers",
			stage.Id, len(raw), constant.MaxApprovalConfigLen)
	}
	return nil
}

// stageApprovalPercent 归一化 approval 的 percent：0（省略）按默认值 100 落库，
// 让下游（付费版、前端展示）不必再区分「未配置」和「配置为全票」。
func stageApprovalPercent(percent int) int {
	if percent == 0 {
		return constant.DefaultStageApprovalPercent
	}
	return percent
}

// validateV1Step 校验 step 的 spec 完整性
func validateV1Step(step *v1yaml.Step) error {
	switch spec := step.Spec.(type) {
	case *v1yaml.StepGroup:
		if len(spec.Steps) == 0 {
			return fmt.Errorf("check 'steps' key in step %q", step.Id)
		}
		for _, s := range spec.Steps {
			if err := validateV1Step(s); err != nil {
				return err
			}
		}
	case *v1yaml.StepParallel:
		if len(spec.Steps) == 0 {
			return fmt.Errorf("check 'steps' key in step %q", step.Id)
		}
		for _, s := range spec.Steps {
			if err := validateV1Step(s); err != nil {
				return err
			}
		}
	}
	return nil
}

// createExecutionWithStages writes an execution along with its stages in a single transaction.
func (t *triggerer) createExecutionWithStages(
	ctx context.Context,
	execution *executionModel.Execution,
	stages []*stageModel.Stage,
) error {
	err := t.executionStore.Create(ctx, execution)
	if err != nil {
		return err
	}

	for _, stage := range stages {
		stage.ExecutionID = execution.ID
		err := t.stageStore.Create(ctx, stage)
		if err != nil {
			return err
		}
	}
	return nil
}

// createExecutionWithStagesWithTx 在同一个事务里创建 execution、阶段、被跳过阶段内的任务，
// 并在全跳过时一并完成收尾（状态/时间/流水线汇总），保证不会留下中间态。
func (t *triggerer) createExecutionWithStagesWithTx(
	ctx context.Context,
	execution *executionModel.Execution,
	stages []*stageModel.Stage,
	skippedSteps map[string][]*v1yaml.Step,
	finalizeSkipped bool,
) error {
	return t.tx.WithTx(ctx, func(txCtx context.Context) error {
		if err := t.createExecutionWithStages(txCtx, execution, stages); err != nil {
			return fmt.Errorf("cannot create execution: %w", err)
		}
		if err := t.createSkippedStageSteps(txCtx, skippedSteps, execution, stages); err != nil {
			return fmt.Errorf("cannot create skipped steps: %w", err)
		}
		if finalizeSkipped {
			finalizer := t.finalizer()
			if err := finalizer.Finalize(txCtx, execution, constant.CIStatusSkipped); err != nil {
				return fmt.Errorf("cannot finish skipped execution: %w", err)
			}
		}
		return nil
	})
}

// finalizer 组装统一的执行收尾器（与 teardown 共用）。
func (t *triggerer) finalizer() manager.ExecutionFinalizer {
	return manager.ExecutionFinalizer{
		ExecutionUpdater: t.executionStore,
		StageLister:      t.stageStore,
		PipelineSvc:      t.pipelineService,
		SSEStreamer:      t.sseStreamer,
		Reporter:         &t.reporter,
	}
}

// resolveGlobalParams 扫描流水线 YAML 中引用的全局参数名（${{global.<name>}}），
// 按名字去全局参数表取值。未接入全局参数表或没有引用时返回空结果。
func (t *triggerer) resolveGlobalParams(ctx context.Context, config []byte) (map[string]string, []string, error) {
	if t.variableResolver == nil {
		return nil, nil, nil
	}
	names := inputs.GlobalRefs(config)
	if len(names) == 0 {
		return nil, nil, nil
	}
	return t.variableResolver.ResolveByKeys(ctx, names)
}
