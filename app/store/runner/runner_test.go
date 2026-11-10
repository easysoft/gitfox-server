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

func TestCreateWithoutHostIDSupportsLegacyRunnerTable(t *testing.T) {
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE ops_runner (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			code TEXT NOT NULL DEFAULT 'gitfox',
			"desc" TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL DEFAULT '',
			hostname TEXT NOT NULL DEFAULT '',
			ip TEXT NOT NULL DEFAULT '',
			os TEXT NOT NULL DEFAULT '',
			arch TEXT NOT NULL DEFAULT '',
			runtime TEXT NOT NULL DEFAULT '',
			labels TEXT NOT NULL DEFAULT '',
			token TEXT NOT NULL,
			heartBeat INTEGER,
			online TEXT NOT NULL DEFAULT 'offline',
			status TEXT NOT NULL DEFAULT 'disable',
			isDefault INTEGER NOT NULL DEFAULT 0,
			createdBy TEXT,
			createdDate DATETIME,
			editedBy TEXT,
			editedDate DATETIME,
			deleted INTEGER NOT NULL DEFAULT 0
		)
	`).Error; err != nil {
		t.Fatalf("create legacy runner table failed: %v", err)
	}

	store := NewStore(db, nil)
	runner := &runnerModel.Runner{
		Name:   "legacy-runner",
		Code:   "gitfox",
		Token:  "legacy-token",
		Online: "online",
		Status: "enable",
	}
	if err := store.Create(context.Background(), runner); err != nil {
		t.Fatalf("create runner without hostID column failed: %v", err)
	}
	if runner.ID == 0 {
		t.Fatal("runner ID should be set after create")
	}
}
