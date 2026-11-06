package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"

	runnerModel "github.com/easysoft/gitfox-community/app/model/runner"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateUsesDefaultRunnerCode(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&Runner{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	store := NewStore(db, nil)
	runner := &runnerModel.Runner{
		Name:   "default-runner",
		Token:  "internal.default.1",
		Online: "online",
		Status: "enable",
	}
	if err := store.Create(context.Background(), runner); err != nil {
		t.Fatalf("create runner failed: %v", err)
	}

	var code string
	if err := db.Raw("SELECT code FROM ops_runner WHERE id = ?", runner.ID).Scan(&code).Error; err != nil {
		t.Fatalf("query runner code failed: %v", err)
	}
	if code != "gitfox" {
		t.Fatalf("code = %q, want database default gitfox", code)
	}
}
