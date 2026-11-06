package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runnerModel "github.com/easysoft/gitfox-community/app/model/runner"
	runnerSvc "github.com/easysoft/gitfox-community/app/service/runner"
	runnerStore "github.com/easysoft/gitfox-community/app/store/runner"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRegisterTestController(t *testing.T) (*Controller, *runnerStore.Store, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&runnerStore.Runner{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	cache := runnerStore.NewTokenCache(0)
	t.Cleanup(cache.Stop)
	cache.Set("temp-token", "admin")

	store := runnerStore.NewStore(db, cache)
	service := runnerSvc.NewService(store)
	controller := &Controller{
		runnerSvc: service,
	}
	return controller, store, db
}

func performRegisterRequest(controller *Controller, token, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/runner-client/register", strings.NewReader(body))
	request.Header.Set("X-Runner-Token", token)
	request.RemoteAddr = "10.0.0.227:12345"
	ctx.Request = request

	controller.Register(ctx)
	return recorder
}

func TestRegisterPersistsHostID(t *testing.T) {
	controller, _, db := newRegisterTestController(t)
	recorder := performRegisterRequest(controller, "temp-token", `{"name":"runner","code":"gitfox","hostID":3}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	var hostID uint
	if err := db.Table("ops_runner").Select("hostID").Scan(&hostID).Error; err != nil {
		t.Fatalf("query hostID failed: %v", err)
	}
	if hostID != 3 {
		t.Fatalf("hostID = %d, want 3", hostID)
	}
}

func TestRegisterExistingRunnerDoesNotBindHost(t *testing.T) {
	controller, store, db := newRegisterTestController(t)

	existing := &runnerModel.Runner{
		Name:   "existing",
		Code:   "gitfox",
		Token:  "temp-token",
		Online: "online",
		Status: "enable",
	}
	if err := store.Create(context.Background(), existing); err != nil {
		t.Fatalf("create existing runner failed: %v", err)
	}

	recorder := performRegisterRequest(controller, "temp-token", `{"name":"runner","code":"gitfox","hostID":3}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var hostID uint
	if err := db.Table("ops_runner").Select("hostID").Where("id = ?", existing.ID).Scan(&hostID).Error; err != nil {
		t.Fatalf("query hostID failed: %v", err)
	}
	if hostID != 0 {
		t.Fatalf("hostID = %d, want 0 for an existing runner", hostID)
	}
}
