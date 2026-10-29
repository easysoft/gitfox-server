// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package stage

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/easysoft/gitfox-community/app/constant"
	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
	stepModel "github.com/easysoft/gitfox-community/app/model/step"
	stepStore "github.com/easysoft/gitfox-community/app/store/step"
	"github.com/easysoft/gitfox-community/pkg/database"
	"github.com/easysoft/gitfox-community/pkg/database/dbtx"
)

// Create creates a new stage
func (s *Store) Create(ctx context.Context, stage *stageModel.Stage) error {
	db := dbtx.GetOrmAccessor(ctx, s.db)

	// Convert maps/slices to JSON strings
	var labelsJSON, dependsJSON string
	if stage.Labels != nil {
		bytes, err := json.Marshal(stage.Labels)
		if err != nil {
			return err
		}
		labelsJSON = string(bytes)
	}
	if stage.Depends != nil {
		bytes, err := json.Marshal(stage.Depends)
		if err != nil {
			return err
		}
		dependsJSON = string(bytes)
	}

	approvalConfigJSON, err := marshalApprovalConfig(stage.ApprovalConfig)
	if err != nil {
		return err
	}

	dbStage := &Stage{
		RepoID:         stage.RepoID,
		ExecutionID:    stage.ExecutionID,
		Parent:         stage.Parent,
		Name:           stage.Name,
		Code:           stage.Code,
		SN:             stage.SN,
		Type:           stage.Type,
		Host:           stage.Host,
		OS:             stage.OS,
		Arch:           stage.Arch,
		Labels:         labelsJSON,
		Depends:        dependsJSON,
		RunPolicy:      stage.RunPolicy,
		CustomPolicy:   stage.CustomPolicy,
		FailPolicy:     stage.FailPolicy,
		Started:        stage.Started,
		Finished:       stage.Finished,
		Duration:       stage.Duration,
		Status:         string(stage.Status),
		ApprovalConfig: approvalConfigJSON,
		ErrorType:      stage.ErrorType,
		ErrorCode:      stage.ErrorCode,
		ErrorLog:       database.NullIfEmpty(stage.ErrorLog),
	}

	if err := db.Create(dbStage).Error; err != nil {
		return database.ProcessSQLErrorf(ctx, err)
	}

	stage.ID = dbStage.ID
	return nil
}

// Update updates a stage
func (s *Store) Update(ctx context.Context, stage *stageModel.Stage) error {
	db := dbtx.GetOrmAccessor(ctx, s.db)
	populateDuration(stage)

	var labelsJSON, dependsJSON string
	if stage.Labels != nil {
		bytes, err := json.Marshal(stage.Labels)
		if err != nil {
			return err
		}
		labelsJSON = string(bytes)
	}
	if stage.Depends != nil {
		bytes, err := json.Marshal(stage.Depends)
		if err != nil {
			return err
		}
		dependsJSON = string(bytes)
	}

	dbStage := &Stage{
		RepoID:       stage.RepoID,
		ExecutionID:  stage.ExecutionID,
		Parent:       stage.Parent,
		Name:         stage.Name,
		Code:         stage.Code,
		SN:           stage.SN,
		Type:         stage.Type,
		Host:         stage.Host,
		OS:           stage.OS,
		Arch:         stage.Arch,
		Labels:       labelsJSON,
		Depends:      dependsJSON,
		RunPolicy:    stage.RunPolicy,
		CustomPolicy: stage.CustomPolicy,
		FailPolicy:   stage.FailPolicy,
		Started:      stage.Started,
		Finished:     stage.Finished,
		Duration:     stage.Duration,
		Status:       string(stage.Status),
		ErrorType:    stage.ErrorType,
		ErrorCode:    stage.ErrorCode,
		ErrorLog:     database.NullIfEmpty(stage.ErrorLog),
	}

	// Omit scheduling metadata fields that are set once at creation time.
	// Runners must not overwrite these via UpdateStage/UpdateStep callbacks.
	// 注意：这里的名字必须写成 gorm column tag 的值（驼峰），写成 snake_case 会被
	// GORM 静默忽略、字段照常被覆写（runPolicy/customPolicy/failPolicy 曾因此漏网）。
	// 每条 Omit 都有 TestStageUpdateKeepsSchedulingColumns / ...KeepsApprovalColumns 兜底。
	// approvalConfig 创建后不可变；approvalStatus 由禅道 DevOps 侧维护，GitFox 只读。
	if err := db.Model(&Stage{}).Where("id = ?", stage.ID).
		Omit("code", "type", "labels", "depends", "runPolicy", "customPolicy", "failPolicy",
			"approvalConfig", "approvalStatus").
		Updates(dbStage).Error; err != nil {
		return database.ProcessSQLErrorf(ctx, err)
	}

	return nil
}

// GetByID gets stage by ID
func (s *Store) GetByID(ctx context.Context, id uint) (*stageModel.Stage, error) {
	db := dbtx.GetOrmAccessor(ctx, s.db)

	var dbStage Stage
	if err := db.Where("id = ?", id).First(&dbStage).Error; err != nil {
		return nil, database.ProcessSQLErrorf(ctx, err)
	}

	return mapToModel(&dbStage), nil
}

// GetByIDWithSteps gets stage by ID
func (s *Store) GetByIDWithSteps(ctx context.Context, id uint) (*stageModel.StageWithSteps, error) {
	db := dbtx.GetOrmAccessor(ctx, s.db)

	var dbStage Stage
	if err := db.Where("id = ?", id).First(&dbStage).Error; err != nil {
		return nil, database.ProcessSQLErrorf(ctx, err)
	}
	stage := mapToModel(&dbStage)

	var dbSteps []stepStore.Step
	if err := db.Where("stageID = ?", id).Order("sn ASC").Find(&dbSteps).Error; err != nil {
		return nil, database.ProcessSQLErrorf(ctx, err)
	}

	steps := make([]*stepModel.Step, len(dbSteps))
	for _, dbStep := range dbSteps {
		var depends []string
		if dbStep.Depends != "" {
			_ = json.Unmarshal([]byte(dbStep.Depends), &depends)
		}
		step := &stepModel.Step{
			ID:         dbStep.ID,
			StageID:    dbStep.StageID,
			Name:       dbStep.Name,
			Code:       dbStep.Code,
			SN:         dbStep.SN,
			Image:      dbStep.Image,
			Depends:    depends,
			GroupID:    dbStep.GroupID,
			FailPolicy: dbStep.FailPolicy,
			Started:    dbStep.Started,
			Finished:   dbStep.Finished,
			Duration:   dbStep.Duration,
			Status:     constant.CIStatus(dbStep.Status),
			ErrorCode:  dbStep.ErrorCode,
			ErrorLog:   dbStep.ErrorLog,
		}
		steps = append(steps, step)
	}

	result := stageModel.StageWithSteps{
		Stage: *stage,
		Steps: steps,
	}
	return &result, nil
}

// FindByNumber returns a stage by execution ID and stage number (sn)
func (s *Store) FindByNumber(ctx context.Context, executionID int64, sn int) (*stageModel.Stage, error) {
	db := dbtx.GetOrmAccessor(ctx, s.db)

	var dbStage Stage
	if err := db.Where("executionID = ? AND sn = ?", executionID, sn).First(&dbStage).Error; err != nil {
		return nil, database.ProcessSQLErrorf(ctx, err)
	}

	return mapToModel(&dbStage), nil
}

func mapToModel(s *Stage) *stageModel.Stage {
	var labels map[string]string
	var depends []string

	if s.Labels != "" {
		_ = json.Unmarshal([]byte(s.Labels), &labels)
	}
	if s.Depends != "" {
		_ = json.Unmarshal([]byte(s.Depends), &depends)
	}

	return &stageModel.Stage{
		ID:             s.ID,
		RepoID:         s.RepoID,
		ExecutionID:    s.ExecutionID,
		Parent:         s.Parent,
		Name:           s.Name,
		Code:           s.Code,
		SN:             s.SN,
		Type:           s.Type,
		Host:           s.Host,
		OS:             s.OS,
		Arch:           s.Arch,
		Labels:         labels,
		Depends:        depends,
		RunPolicy:      s.RunPolicy,
		CustomPolicy:   s.CustomPolicy,
		FailPolicy:     s.FailPolicy,
		Started:        s.Started,
		Finished:       s.Finished,
		Duration:       s.Duration,
		Status:         constant.CIStatus(s.Status),
		ApprovalConfig: unmarshalApprovalConfig(s.ApprovalConfig),
		ApprovalStatus: constant.StageApprovalStatus(s.ApprovalStatus),
		ErrorType:      s.ErrorType,
		ErrorCode:      s.ErrorCode,
		ErrorLog:       s.ErrorLog.String,
	}
}

// marshalApprovalConfig 把审批配置序列化为 JSON，空配置写空串（列 NOT NULL DEFAULT ”）。
func marshalApprovalConfig(a *stageModel.StageApproval) (string, error) {
	if a == nil {
		return "", nil
	}
	bytes, err := json.Marshal(a)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// unmarshalApprovalConfig 解析审批配置 JSON，空串/脏数据都返回 nil（不阻断调度）。
func unmarshalApprovalConfig(raw string) *stageModel.StageApproval {
	if raw == "" {
		return nil
	}
	var out stageModel.StageApproval
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return &out
}

// ListIncomplete returns a stage list where the stage is incomplete (pending or running)
func (s *Store) ListIncomplete(ctx context.Context) ([]*stageModel.Stage, error) {
	db := dbtx.GetOrmAccessor(ctx, s.db)

	var dbStages []Stage
	if err := db.Where("status IN ?", []string{"pending", "running"}).Order("sn ASC").Find(&dbStages).Error; err != nil {
		return nil, database.ProcessSQLErrorf(ctx, err)
	}

	stages := make([]*stageModel.Stage, len(dbStages))
	for i := range dbStages {
		stages[i] = mapToModel(&dbStages[i])
	}
	return stages, nil
}

// List returns a list of stages corresponding to an execution ID
func (s *Store) List(ctx context.Context, executionID uint) ([]*stageModel.Stage, error) {
	db := dbtx.GetOrmAccessor(ctx, s.db)

	var dbStages []Stage
	if err := db.Where("executionID = ?", executionID).Order("sn ASC").Find(&dbStages).Error; err != nil {
		return nil, database.ProcessSQLErrorf(ctx, err)
	}

	stages := make([]*stageModel.Stage, len(dbStages))
	for i := range dbStages {
		stages[i] = mapToModel(&dbStages[i])
	}
	return stages, nil
}

// ListWithSteps returns a stage list with steps for an execution
func (s *Store) ListWithSteps(ctx context.Context, executionID uint) ([]*stageModel.StageWithSteps, error) {
	db := dbtx.GetOrmAccessor(ctx, s.db)

	// Get all stages for the execution
	var dbStages []Stage
	if err := db.Where("executionID = ?", executionID).Order("sn ASC").Find(&dbStages).Error; err != nil {
		return nil, database.ProcessSQLErrorf(ctx, err)
	}

	if len(dbStages) == 0 {
		return []*stageModel.StageWithSteps{}, nil
	}

	// Collect stage IDs
	stageIDs := make([]uint, len(dbStages))
	for i, stage := range dbStages {
		stageIDs[i] = stage.ID
	}

	// Get all steps for these stages
	var dbSteps []stepStore.Step
	if err := db.Where("stageID IN ?", stageIDs).Order("sn ASC").Find(&dbSteps).Error; err != nil {
		return nil, database.ProcessSQLErrorf(ctx, err)
	}

	// Group steps by stageID
	stepsByStageID := make(map[uint][]*stepModel.Step)
	for _, dbStep := range dbSteps {
		var depends []string
		if dbStep.Depends != "" {
			_ = json.Unmarshal([]byte(dbStep.Depends), &depends)
		}
		step := &stepModel.Step{
			ID:         dbStep.ID,
			StageID:    dbStep.StageID,
			Name:       dbStep.Name,
			Code:       dbStep.Code,
			SN:         dbStep.SN,
			Image:      dbStep.Image,
			Depends:    depends,
			GroupID:    dbStep.GroupID,
			FailPolicy: dbStep.FailPolicy,
			Started:    dbStep.Started,
			Finished:   dbStep.Finished,
			Duration:   dbStep.Duration,
			Status:     constant.CIStatus(dbStep.Status),
			ErrorCode:  dbStep.ErrorCode,
			ErrorLog:   dbStep.ErrorLog,
		}
		stepsByStageID[dbStep.StageID] = append(stepsByStageID[dbStep.StageID], step)
	}

	// Build result
	result := make([]*stageModel.StageWithSteps, len(dbStages))
	for i := range dbStages {
		stage := mapToModel(&dbStages[i])
		steps := stepsByStageID[dbStages[i].ID]
		if steps == nil {
			steps = []*stepModel.Step{}
		}
		result[i] = &stageModel.StageWithSteps{
			Stage: *stage,
			Steps: steps,
		}
	}

	return result, nil
}

func populateDuration(stage *stageModel.Stage) {
	if stage == nil || stage.Started == nil || stage.Finished == nil {
		return
	}
	duration := time.Time(*stage.Finished).Sub(time.Time(*stage.Started))
	stage.Duration = uint(math.Ceil(duration.Seconds()))
}
