package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	pipelineModel "gitfox.zcorp.cc/git/Pangu/gitfox-community/app/model/pipeline"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sqlCapture struct {
	mu    sync.Mutex
	lines []string
}

func (c *sqlCapture) Printf(format string, args ...interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, fmt.Sprintf(format, args...))
}

func (c *sqlCapture) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = nil
}

// updateSQL returns the most recent UPDATE statement log line.
func (c *sqlCapture) updateSQL() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.lines) - 1; i >= 0; i-- {
		if strings.Contains(c.lines[i], "UPDATE ") {
			return c.lines[i]
		}
	}
	return ""
}

func newTestStore(t *testing.T) (*Store, *sqlCapture) {
	t.Helper()

	cap := &sqlCapture{}
	db, err := gorm.Open(sqlite.Open("file:pipeline_store_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.New(cap, logger.Config{LogLevel: logger.Info, Colorful: false}),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Pipeline{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return NewPipelineStore(db), cap
}

func TestUpdatePipelineDoesNotWriteIdentityColumn(t *testing.T) {
	store, cap := newTestStore(t)
	ctx := context.Background()

	row := Pipeline{
		Name:          "pipeline",
		Engine:        "gitfox",
		Scope:         "repo",
		SpaceID:       1,
		RepoID:        1,
		Status:        "active",
		DefaultBranch: "main",
	}
	if err := store.db.WithContext(ctx).Create(&row).Error; err != nil {
		t.Fatalf("seed pipeline: %v", err)
	}
	if row.ID == 0 {
		t.Fatal("expected auto-increment id to be assigned")
	}
	cap.reset()

	model := &pipelineModel.Pipeline{
		ID:            row.ID,
		Name:          "pipeline-renamed",
		Engine:        "gitfox",
		Scope:         "repo",
		SpaceID:       1,
		RepoID:        1,
		Status:        "active",
		DefaultBranch: "main",
	}
	if err := store.UpdatePipeline(ctx, model); err != nil {
		t.Fatalf("update pipeline: %v", err)
	}

	sql := cap.updateSQL()
	if sql == "" {
		t.Fatal("no UPDATE statement captured")
	}
	if strings.Contains(sql, "`id`") {
		t.Fatalf("UPDATE must not write the auto-increment column, got: %s", sql)
	}
}
