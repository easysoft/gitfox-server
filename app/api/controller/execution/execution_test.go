// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/easysoft/gitfox-community/app/api/utils/req"
	"github.com/easysoft/gitfox-community/app/constant"
	authModel "github.com/easysoft/gitfox-community/app/model/auth"
	executionModel "github.com/easysoft/gitfox-community/app/model/execution"
	pipelineModel "github.com/easysoft/gitfox-community/app/model/pipeline"
	repoModel "github.com/easysoft/gitfox-community/app/model/repo"
	principal "github.com/easysoft/gitfox-community/app/model/user"
	"github.com/easysoft/gitfox-community/app/pipeline/triggerer"
	gitMock "github.com/easysoft/gitfox-community/app/service/branch/mock"
	pipelineService "github.com/easysoft/gitfox-community/app/service/pipeline"
	repoSvcMock "github.com/easysoft/gitfox-community/app/service/repo/mock"
	"github.com/easysoft/gitfox-community/pkg/errors"
	"github.com/easysoft/gitfox-community/pkg/git"
	"github.com/easysoft/gitfox-community/pkg/git/sha"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testPipelineID = uint(7)
	testRepoID     = int64(100)
	testRepoURL    = "https://git.example.com/foo/bar.git"
	testBranchSHA  = "1111111111111111111111111111111111111111"
	testAccount    = "tester"
)

// fakePipelineService 只实现 Create 用到的两个方法，其余方法由嵌入的接口兜底
type fakePipelineService struct {
	pipelineService.Interface
	pipeline  *pipelineModel.Pipeline
	variables map[string]string
	getErr    error
	varsErr   error
}

func (f *fakePipelineService) GetPipelineByID(_ context.Context, _ int64) (*pipelineModel.Pipeline, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.pipeline, nil
}

// GetPipelineVariables 返回一份拷贝，与真实实现一致（每次都是新 map，且非 nil）
func (f *fakePipelineService) GetPipelineVariables(_ context.Context, _ int64) (map[string]string, error) {
	if f.varsErr != nil {
		return nil, f.varsErr
	}
	out := make(map[string]string, len(f.variables))
	for k, v := range f.variables {
		out[k] = v
	}
	return out, nil
}

// fakeTriggerer 捕获交给 triggerer 的 hook，供断言使用
type fakeTriggerer struct {
	calls int
	hook  *triggerer.Hook
}

func (f *fakeTriggerer) Trigger(_ context.Context, _ uint, hook *triggerer.Hook) (*executionModel.Execution, error) {
	f.calls++
	f.hook = hook
	return &executionModel.Execution{ID: 1, PipelineID: testPipelineID}, nil
}

// ====================== Create 方法测试 ======================
// CreateTestCase 表格驱动用例结构体
type CreateTestCase struct {
	name string

	// 前置条件
	pipeline  *pipelineModel.Pipeline
	variables map[string]string
	getErr    error
	varsErr   error

	// 请求体中的 params，nil 表示整个字段都不传
	params map[string]string

	// 期望的依赖调用
	expectRepoLookup   bool
	expectBranchLookup bool

	// 期望结果
	expectStatus    int
	expectTriggered bool
	expectRef       string
	expectCommit    string
	expectAction    constant.TriggerAction
	expectParams    map[string]string
}

func TestCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []CreateTestCase{
		{
			// 回归用例：整个 params 字段不传时，hook.Params 一度是 nil map，
			// 往里面写 gitURL 会 panic（assignment to entry in nil map）
			name:             "不传 params 时使用流水线变量且不 panic",
			pipeline:         &pipelineModel.Pipeline{ID: 7, RepoID: testRepoID, SpaceID: 5},
			variables:        map[string]string{"FOO": "bar", "PLUGIN_WORKSPACE": "demo"},
			params:           nil,
			expectRepoLookup: true,
			expectStatus:     http.StatusOK,
			expectTriggered:  true,
			expectParams:     map[string]string{"FOO": "bar", "PLUGIN_WORKSPACE": "demo", "gitURL": testRepoURL, "gitRef": "", "GITFOX_PIPELINE_TRIGGER": testAccount},
		},
		{
			name:             "流水线没有变量且不传 params 时只注入 gitURL",
			pipeline:         &pipelineModel.Pipeline{ID: 7, RepoID: testRepoID, SpaceID: 5},
			variables:        map[string]string{},
			params:           nil,
			expectRepoLookup: true,
			expectStatus:     http.StatusOK,
			expectTriggered:  true,
			expectParams:     map[string]string{"gitURL": testRepoURL, "gitRef": "", "GITFOX_PIPELINE_TRIGGER": testAccount},
		},
		{
			name:             "请求体参数覆盖同名流水线变量",
			pipeline:         &pipelineModel.Pipeline{ID: 7, RepoID: testRepoID, SpaceID: 5},
			variables:        map[string]string{"FOO": "default", "KEEP": "yes"},
			params:           map[string]string{"FOO": "override"},
			expectRepoLookup: true,
			expectStatus:     http.StatusOK,
			expectTriggered:  true,
			expectParams:     map[string]string{"FOO": "override", "KEEP": "yes", "gitURL": testRepoURL, "gitRef": "", "GITFOX_PIPELINE_TRIGGER": testAccount},
		},
		{
			name:               "流水线变量里的 gitRef 决定分支和 commit",
			pipeline:           &pipelineModel.Pipeline{ID: 7, RepoID: testRepoID, SpaceID: 5},
			variables:          map[string]string{"gitRef": "release/1.0"},
			params:             nil,
			expectRepoLookup:   true,
			expectBranchLookup: true,
			expectStatus:       http.StatusOK,
			expectTriggered:    true,
			expectRef:          "release/1.0",
			expectCommit:       testBranchSHA,
			expectParams:       map[string]string{"gitRef": "release/1.0", "gitURL": testRepoURL, "GITFOX_PIPELINE_TRIGGER": testAccount},
		},
		{
			name:               "请求体 gitRef 覆盖流水线变量里的 gitRef",
			pipeline:           &pipelineModel.Pipeline{ID: 7, RepoID: testRepoID, SpaceID: 5},
			variables:          map[string]string{"gitRef": "release/1.0"},
			params:             map[string]string{"gitRef": "main"},
			expectRepoLookup:   true,
			expectBranchLookup: true,
			expectStatus:       http.StatusOK,
			expectTriggered:    true,
			expectRef:          "main",
			expectCommit:       testBranchSHA,
			expectParams:       map[string]string{"gitRef": "main", "gitURL": testRepoURL, "GITFOX_PIPELINE_TRIGGER": testAccount},
		},
		{
			// gitURL 属于系统信息，只认请求体和仓库地址，不允许被流水线变量改写
			name:             "流水线变量里的 gitURL 不覆盖仓库地址",
			pipeline:         &pipelineModel.Pipeline{ID: 7, RepoID: testRepoID, SpaceID: 5},
			variables:        map[string]string{"gitURL": "https://evil.example.com/x.git"},
			params:           nil,
			expectRepoLookup: true,
			expectStatus:     http.StatusOK,
			expectTriggered:  true,
			expectParams:     map[string]string{"gitURL": testRepoURL, "gitRef": "", "GITFOX_PIPELINE_TRIGGER": testAccount},
		},
		{
			// auto 是接口的控制标志，不是运行参数，只有请求体能决定
			name:             "流水线变量里的 auto 不改变触发动作",
			pipeline:         &pipelineModel.Pipeline{ID: 7, RepoID: testRepoID, SpaceID: 5},
			variables:        map[string]string{"auto": "true"},
			params:           nil,
			expectRepoLookup: true,
			expectStatus:     http.StatusOK,
			expectTriggered:  true,
			expectAction:     "",
			expectParams:     map[string]string{"auto": "true", "gitURL": testRepoURL, "gitRef": "", "GITFOX_PIPELINE_TRIGGER": testAccount},
		},
		{
			name:             "请求体 auto 为 true 时触发动作为 auto",
			pipeline:         &pipelineModel.Pipeline{ID: 7, RepoID: testRepoID, SpaceID: 5},
			variables:        map[string]string{},
			params:           map[string]string{"auto": "true"},
			expectRepoLookup: true,
			expectStatus:     http.StatusOK,
			expectTriggered:  true,
			expectAction:     constant.TriggerActionPipelineAuto,
			expectParams:     map[string]string{"auto": "true", "gitURL": testRepoURL, "gitRef": "", "GITFOX_PIPELINE_TRIGGER": testAccount},
		},
		{
			// 触发者来自登录态，不允许被流水线变量伪造
			name:             "流水线变量伪造 GITFOX_PIPELINE_TRIGGER 不生效",
			pipeline:         &pipelineModel.Pipeline{ID: 7, RepoID: testRepoID, SpaceID: 5},
			variables:        map[string]string{"GITFOX_PIPELINE_TRIGGER": "someone-else"},
			params:           nil,
			expectRepoLookup: true,
			expectStatus:     http.StatusOK,
			expectTriggered:  true,
			expectParams:     map[string]string{"gitURL": testRepoURL, "gitRef": "", "GITFOX_PIPELINE_TRIGGER": testAccount},
		},
		{
			// 触发者来自登录态，不允许被请求体伪造
			name:             "请求体伪造 GITFOX_PIPELINE_TRIGGER 不生效",
			pipeline:         &pipelineModel.Pipeline{ID: 7, RepoID: testRepoID, SpaceID: 5},
			variables:        map[string]string{},
			params:           map[string]string{"GITFOX_PIPELINE_TRIGGER": "someone-else"},
			expectRepoLookup: true,
			expectStatus:     http.StatusOK,
			expectTriggered:  true,
			expectParams:     map[string]string{"gitURL": testRepoURL, "gitRef": "", "GITFOX_PIPELINE_TRIGGER": testAccount},
		},
		{
			name:            "获取流水线失败返回 500",
			pipeline:        nil,
			getErr:          errors.New("pipeline not found"),
			expectStatus:    http.StatusInternalServerError,
			expectTriggered: false,
		},
		{
			name:            "获取流水线变量失败返回 500",
			pipeline:        &pipelineModel.Pipeline{ID: 7, RepoID: testRepoID, SpaceID: 5},
			varsErr:         errors.New("invalid variables json"),
			expectStatus:    http.StatusInternalServerError,
			expectTriggered: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			repoSvc := repoSvcMock.NewMockInterface(ctrl)
			gitSvc := gitMock.NewMockInterface(ctrl)

			if tc.expectRepoLookup {
				repoSvc.EXPECT().
					GetByID(gomock.Any(), gomock.Any(), uint(testRepoID)).
					Return(&repoModel.Repo{ID: uint(testRepoID), GitURL: testRepoURL, GitUID: "uid-1"}, nil)
			}
			if tc.expectBranchLookup {
				gitSvc.EXPECT().
					GetBranch(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, params *git.GetBranchParams) (*git.GetBranchOutput, error) {
						assert.Equal(t, tc.expectRef, params.BranchName, "应按期望的分支查询 commit")
						return &git.GetBranchOutput{
							Branch: git.Branch{Name: params.BranchName, SHA: sha.Must(testBranchSHA)},
						}, nil
					})
			}

			triggererFake := &fakeTriggerer{}
			pipelineSvc := &fakePipelineService{
				pipeline:  tc.pipeline,
				variables: tc.variables,
				getErr:    tc.getErr,
				varsErr:   tc.varsErr,
			}
			// Create 只用到 triggerer/pipelineService/repoService/gitPkg，canceler 与 executionService 传 nil
			controller := NewController(triggererFake, nil, nil, pipelineSvc, repoSvc, gitSvc)

			ctx, w := newCreateContext(t, tc.params)

			controller.Create(ctx)

			assert.Equal(t, tc.expectStatus, w.Code)
			require.Equal(t, tc.expectTriggered, triggererFake.calls > 0, "triggerer 调用次数不符合预期")
			if !tc.expectTriggered {
				return
			}

			require.NotNil(t, triggererFake.hook)
			assert.Equal(t, tc.expectRef, triggererFake.hook.Ref)
			assert.Equal(t, tc.expectCommit, triggererFake.hook.Commit)
			assert.Equal(t, tc.expectAction, triggererFake.hook.Action)
			assert.Equal(t, tc.expectParams, triggererFake.hook.Params)
		})
	}
}

// newCreateContext 构造一个带登录态、params 可整体缺省的 Create 请求上下文
func newCreateContext(t *testing.T, params map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	body := map[string]interface{}{"pipelineID": testPipelineID}
	if params != nil {
		body["params"] = params
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v2/pipeline/executions", bytes.NewReader(raw))
	ctx.Request.Header.Set("Content-Type", "application/json")

	session := &authModel.Session{
		Principal: &principal.Principal{App: "gitfox", DisplayName: "tester"},
		User:      &principal.User{ID: 42, Account: testAccount},
	}
	ctx.Request = ctx.Request.WithContext(req.SetAuthSession(ctx.Request.Context(), session))

	return ctx, w
}
