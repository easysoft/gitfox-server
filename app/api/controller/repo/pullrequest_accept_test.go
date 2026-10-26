// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)

package repo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/easysoft/gitfox-community/app/api/utils/req"
	"github.com/easysoft/gitfox-community/app/api/utils/resp"
	"github.com/easysoft/gitfox-community/app/constant"
	"github.com/easysoft/gitfox-community/app/model/auth"
	repoModel "github.com/easysoft/gitfox-community/app/model/repo"
	principal "github.com/easysoft/gitfox-community/app/model/user"
	pullrequestSvc "github.com/easysoft/gitfox-community/app/service/pullrequest"
	repoSvc "github.com/easysoft/gitfox-community/app/service/repo"
	pkgErrors "github.com/easysoft/gitfox-community/pkg/errors"
	"github.com/easysoft/gitfox-community/pkg/i18n"
	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	i18n.InitGlobal(&i18n.ManagerConfig{
		DefaultLocale:   "zh-CN",
		FallbackLocale:  "en-US",
		AutoLoad:        true,
		TranslationsDir: "../../../../resources/locales",
	})
	os.Exit(m.Run())
}

type acceptPullRequestService struct {
	pullrequestSvc.Interface
	err    error
	called bool
}

func (s *acceptPullRequestService) AcceptAIIssue(
	context.Context,
	*auth.Session,
	uint,
	uint,
	*repoModel.AIReviewAcceptReq,
) (*repoModel.AIReviewAcceptResponse, error) {
	s.called = true
	return nil, s.err
}

type acceptRepoService struct {
	repoSvc.Interface

	repo           *repoModel.Repo
	err            error
	called         bool
	repoID         uint
	gotPermissions []constant.Permission
}

func (s *acceptRepoService) GetRepoCheckAccessByID(
	_ context.Context,
	_ *auth.Session,
	repoID uint,
	permissions []constant.Permission,
) (*repoModel.Repo, error) {
	s.called = true
	s.repoID = repoID
	s.gotPermissions = append([]constant.Permission(nil), permissions...)
	return s.repo, s.err
}

func newAcceptRequestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/repos/1/pullreq/2/accept", strings.NewReader(`{"issueIDs":[1]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request = c.Request.WithContext(req.SetAuthSession(c.Request.Context(), &auth.Session{
		User: &principal.User{Email: "tester@example.com"},
	}))
	c.Params = gin.Params{
		{Key: "id", Value: "1"},
		{Key: req.PullrequestID, Value: "2"},
	}
	c.Set(i18n.XLocaleHeader, "zh-CN")
	return c, recorder
}

func TestAcceptAIIssueChecksTargetRepoPushPermission(t *testing.T) {
	permissionKey := i18n.RepoPushPermissionError
	repoService := &acceptRepoService{
		err: pkgErrors.WithI18nKey(errors.New("push denied"), permissionKey),
	}
	pullrequestService := &acceptPullRequestService{}
	ctl := &Controller{
		repoSvc:        repoService,
		pullrequestSvc: pullrequestService,
	}
	c, recorder := newAcceptRequestContext(t)

	ctl.AcceptAIIssue(c)

	if !repoService.called {
		t.Fatal("target repo permission was not checked")
	}
	if repoService.repoID != 1 {
		t.Fatalf("want target repo 1 to be checked, got %d", repoService.repoID)
	}
	if len(repoService.gotPermissions) != 1 || repoService.gotPermissions[0] != constant.PermissionRepoPush {
		t.Fatalf("want repo_push permission, got %v", repoService.gotPermissions)
	}
	if pullrequestService.called {
		t.Fatal("pull request service must not run without repo push permission")
	}

	var body resp.ObjectResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := body.Message, i18n.TWithLocale("zh-CN", string(permissionKey)); got != want {
		t.Fatalf("want permission error %q, got %q; response=%s", want, got, recorder.Body.String())
	}
}

func TestAcceptAIIssuePreservesServiceI18nKey(t *testing.T) {
	specificKey := i18n.CodeReviewAcceptPullRequestQueryError
	repoService := &acceptRepoService{repo: &repoModel.Repo{ID: 1}}
	pullrequestService := &acceptPullRequestService{
		err: pkgErrors.WithI18nKey(pkgErrors.New("pull request query failed"), specificKey),
	}
	ctl := &Controller{
		repoSvc:        repoService,
		pullrequestSvc: pullrequestService,
	}
	c, recorder := newAcceptRequestContext(t)

	ctl.AcceptAIIssue(c)

	if !repoService.called {
		t.Fatal("target repo permission was not checked")
	}
	if !pullrequestService.called {
		t.Fatal("pull request service was not called after permission passed")
	}

	var body resp.ObjectResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got, want := body.Message, i18n.TWithLocale("zh-CN", string(specificKey)); got != want {
		t.Fatalf("want specific error %q, got %q; response=%s", want, got, recorder.Body.String())
	}
}
