// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      OpenAI Codex

package tag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	apiReq "github.com/easysoft/gitfox-community/app/api/utils/req"
	authModel "github.com/easysoft/gitfox-community/app/model/auth"
	repoModel "github.com/easysoft/gitfox-community/app/model/repo"
	principal "github.com/easysoft/gitfox-community/app/model/user"
	repoSvc "github.com/easysoft/gitfox-community/app/service/repo"
	tagSvc "github.com/easysoft/gitfox-community/app/service/tag"
	"github.com/easysoft/gitfox-community/pkg/i18n"
	"github.com/gin-gonic/gin"
)

type deleteTagRepoService struct {
	repoSvc.Interface
	getByIDCalled bool
}

func (s *deleteTagRepoService) GetByID(context.Context, *authModel.Session, uint) (*repoModel.Repo, error) {
	s.getByIDCalled = true
	return &repoModel.Repo{GitUID: "repo-uid"}, nil
}

type deleteTagService struct {
	tagSvc.Interface
	deleteCalled bool
}

func (s *deleteTagService) Delete(context.Context, string, *authModel.Session, *repoModel.Repo) error {
	s.deleteCalled = true
	return nil
}

func TestDeleteWithoutEmailReturnsEmailRequiredWithoutCallingDependencies(t *testing.T) {
	const emailRequiredMessage = "user email required"
	i18n.GetGlobalManager().AddTranslation("zh-CN", string(i18n.RepoEmailRequired), emailRequiredMessage)

	repoService := &deleteTagRepoService{}
	tagService := &deleteTagService{}
	controller := NewController(tagService, repoService)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/repos/:id/tags/:name", controller.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/repos/1/tags/v1.0.0", nil)
	req = req.WithContext(apiReq.SetAuthSession(req.Context(), &authModel.Session{
		User: &principal.User{Account: "tester"},
	}))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	var response struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Message != emailRequiredMessage {
		t.Fatalf("expected message %q, got %q", emailRequiredMessage, response.Message)
	}
	if repoService.getByIDCalled {
		t.Fatal("expected repository lookup not to be called when email is empty")
	}
	if tagService.deleteCalled {
		t.Fatal("expected tag deletion not to be called when email is empty")
	}
}
