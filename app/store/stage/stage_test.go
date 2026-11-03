package stage

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/easysoft/gitfox-community/app/constant"
	stageModel "github.com/easysoft/gitfox-community/app/model/stage"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&Stage{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return &Store{db: db}
}

func TestStageApprovalConfigRoundTrip(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	t.Run("with approval config", func(t *testing.T) {
		err := store.Create(ctx, &stageModel.Stage{
			ExecutionID: 1,
			Name:        "stageWithApproval",
			Code:        "stage-1",
			SN:          1,
			Status:      constant.CIStatusPending,
			ApprovalConfig: &stageModel.StageApproval{
				Approvers: []string{"zhangsan", "lisi"},
				Mode:      string(constant.StageApprovalModeAnd),
				Timeout:   "24h",
			},
		})
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}

		// 审批状态初始为空，由禅道侧写入
		var raw Stage
		if err := store.db.Where("code = ?", "stage-1").First(&raw).Error; err != nil {
			t.Fatalf("query raw stage failed: %v", err)
		}
		if raw.ApprovalStatus != "" {
			t.Fatalf("expected empty approvalStatus, got %q", raw.ApprovalStatus)
		}
		if !json.Valid([]byte(raw.ApprovalConfig)) {
			t.Fatalf("expected valid JSON approvalConfig, got %q", raw.ApprovalConfig)
		}

		got, err := store.GetByID(ctx, raw.ID)
		if err != nil {
			t.Fatalf("get by id failed: %v", err)
		}
		if got.ApprovalConfig == nil {
			t.Fatal("expected approval config, got nil")
		}
		if want := []string{"zhangsan", "lisi"}; !slices.Equal(got.ApprovalConfig.Approvers, want) {
			t.Fatalf("expected approvers %v, got %v", want, got.ApprovalConfig.Approvers)
		}
		if got.ApprovalConfig.Mode != string(constant.StageApprovalModeAnd) {
			t.Fatalf("expected mode and, got %q", got.ApprovalConfig.Mode)
		}
		if got.ApprovalConfig.Timeout != "24h" {
			t.Fatalf("expected timeout 24h, got %q", got.ApprovalConfig.Timeout)
		}
		if got.ApprovalStatus != constant.StageApprovalStatusNone {
			t.Fatalf("expected empty approval status, got %q", got.ApprovalStatus)
		}
	})

	t.Run("without approval config", func(t *testing.T) {
		err := store.Create(ctx, &stageModel.Stage{
			ExecutionID: 1,
			Name:        "plainStage",
			Code:        "stage-2",
			SN:          2,
			Status:      constant.CIStatusPending,
		})
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}

		var raw Stage
		if err := store.db.Where("code = ?", "stage-2").First(&raw).Error; err != nil {
			t.Fatalf("query raw stage failed: %v", err)
		}
		if raw.ApprovalConfig != "" {
			t.Fatalf("expected empty approvalConfig, got %q", raw.ApprovalConfig)
		}

		got, err := store.GetByID(ctx, raw.ID)
		if err != nil {
			t.Fatalf("get by id failed: %v", err)
		}
		if got.ApprovalConfig != nil {
			t.Fatalf("expected nil approval config, got %+v", got.ApprovalConfig)
		}
	})

	t.Run("invalid json is ignored", func(t *testing.T) {
		got := unmarshalApprovalConfig("{not-json")
		if got != nil {
			t.Fatalf("expected nil for invalid json, got %+v", got)
		}
	})
}

func TestStageUpdateKeepsApprovalColumns(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	stage := &stageModel.Stage{
		ExecutionID: 2,
		Name:        "stageWithApproval",
		Code:        "stage-3",
		SN:          1,
		Status:      constant.CIStatusPending,
		ApprovalConfig: &stageModel.StageApproval{
			Approvers: []string{"zhangsan"},
		},
	}
	if err := store.Create(ctx, stage); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// 模拟禅道侧写入审批状态
	if err := store.db.Model(&Stage{}).Where("id = ?", stage.ID).
		Update("approvalStatus", string(constant.StageApprovalStatusInProgress)).Error; err != nil {
		t.Fatalf("update approval status failed: %v", err)
	}

	// runner 回调风格的更新：重新读出来改状态再写回
	loaded, err := store.GetByID(ctx, stage.ID)
	if err != nil {
		t.Fatalf("get by id failed: %v", err)
	}
	loaded.Status = constant.CIStatusRunning
	loaded.ApprovalConfig = &stageModel.StageApproval{Approvers: []string{"someone-else"}}
	loaded.ApprovalStatus = constant.StageApprovalStatusApproved
	if err := store.Update(ctx, loaded); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err := store.GetByID(ctx, stage.ID)
	if err != nil {
		t.Fatalf("get by id failed: %v", err)
	}
	if got.Status != constant.CIStatusRunning {
		t.Fatalf("expected status running, got %q", got.Status)
	}
	// approvalConfig 创建后不可变，approvalStatus 由禅道侧维护，Update 都不应写入
	if got.ApprovalConfig == nil || !slices.Equal(got.ApprovalConfig.Approvers, []string{"zhangsan"}) {
		t.Fatalf("expected approval config untouched, got %+v", got.ApprovalConfig)
	}
	if got.ApprovalStatus != constant.StageApprovalStatusInProgress {
		t.Fatalf("expected approval status inProgress, got %q", got.ApprovalStatus)
	}
}

func TestStageUpdateKeepsSchedulingColumns(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	// These fields are written once at creation time and must survive a
	// runner-style update (read the stage, change its status, write it back)
	// even when the payload drops them. The Omit list has to spell them the
	// way the gorm column tags do — snake_case entries are silently ignored
	// and the columns get overwritten.
	stage := &stageModel.Stage{
		ExecutionID:  2,
		Name:         "stageWithPolicy",
		Code:         "stage-4",
		Type:         "docker",
		SN:           1,
		Labels:       map[string]string{"os": "linux"},
		Depends:      []string{"stage-1"},
		RunPolicy:    string(constant.RunPolicyOnSuccess),
		CustomPolicy: "custom-original",
		FailPolicy:   "fail-original",
		Status:       constant.CIStatusPending,
	}
	if err := store.Create(ctx, stage); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	loaded, err := store.GetByID(ctx, stage.ID)
	if err != nil {
		t.Fatalf("get by id failed: %v", err)
	}
	loaded.Code = "stage-overwritten"
	loaded.Type = "overwritten"
	loaded.Labels = map[string]string{"os": "windows"}
	loaded.Depends = []string{"stage-2"}
	loaded.RunPolicy = string(constant.RunPolicyOnFailure)
	loaded.CustomPolicy = "custom-overwritten"
	loaded.FailPolicy = "fail-overwritten"
	loaded.Status = constant.CIStatusRunning
	if err := store.Update(ctx, loaded); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	got, err := store.GetByID(ctx, stage.ID)
	if err != nil {
		t.Fatalf("get by id failed: %v", err)
	}
	if got.Status != constant.CIStatusRunning {
		t.Fatalf("expected status running, got %q", got.Status)
	}
	if got.RunPolicy != string(constant.RunPolicyOnSuccess) {
		t.Errorf("expected runPolicy untouched, got %q", got.RunPolicy)
	}
	if got.CustomPolicy != "custom-original" {
		t.Errorf("expected customPolicy untouched, got %q", got.CustomPolicy)
	}
	if got.FailPolicy != "fail-original" {
		t.Errorf("expected failPolicy untouched, got %q", got.FailPolicy)
	}
	if got.Code != "stage-4" {
		t.Errorf("expected code untouched, got %q", got.Code)
	}
	if got.Type != "docker" {
		t.Errorf("expected type untouched, got %q", got.Type)
	}
	if !slices.Equal(got.Depends, []string{"stage-1"}) {
		t.Errorf("expected depends untouched, got %v", got.Depends)
	}
	if got.Labels["os"] != "linux" {
		t.Errorf("expected labels untouched, got %v", got.Labels)
	}
}
