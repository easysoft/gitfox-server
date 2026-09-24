package step

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	stepModel "gitfox.zcorp.cc/git/Pangu/gitfox-community/app/model/step"
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
	db, err := gorm.Open(sqlite.Open("file:step_store_test?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.New(cap, logger.Config{LogLevel: logger.Info, Colorful: false}),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Step{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return NewStepStore(db), cap
}

func TestUpdateDoesNotWriteIdentityColumn(t *testing.T) {
	store, cap := newTestStore(t)
	ctx := context.Background()

	row := Step{StageID: 10, Name: "build", Code: "build", SN: 1, Status: "pending"}
	if err := store.db.WithContext(ctx).Create(&row).Error; err != nil {
		t.Fatalf("seed step: %v", err)
	}
	if row.ID == 0 {
		t.Fatal("expected auto-increment id to be assigned")
	}
	cap.reset()

	model := &stepModel.Step{
		ID:      row.ID,
		StageID: 10,
		Name:    "build",
		SN:      1,
		Status:  "running",
	}
	if err := store.Update(ctx, model); err != nil {
		t.Fatalf("update step: %v", err)
	}

	sql := cap.updateSQL()
	if sql == "" {
		t.Fatal("no UPDATE statement captured")
	}
	if strings.Contains(sql, "`id`") {
		t.Fatalf("UPDATE must not write the auto-increment column, got: %s", sql)
	}
}
