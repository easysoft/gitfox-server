// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)

package repo

import (
	"context"
	"testing"

	"github.com/easysoft/gitfox-community/app/constant"
	"github.com/easysoft/gitfox-community/app/model/auth"
	repoModel "github.com/easysoft/gitfox-community/app/model/repo"
	principal "github.com/easysoft/gitfox-community/app/model/user"
	pkgErrors "github.com/easysoft/gitfox-community/pkg/errors"
	"github.com/easysoft/gitfox-community/pkg/i18n"
)

type denyAuthorizer struct{}

func (denyAuthorizer) Check(
	context.Context,
	*auth.Session,
	constant.Scope,
	constant.Permission,
) (bool, error) {
	return false, nil
}

func (denyAuthorizer) CheckAll(
	context.Context,
	*auth.Session,
	...constant.PermissionCheck,
) (bool, error) {
	return false, nil
}

func TestCheckRepoPermissionsPushDeniedUsesI18nKey(t *testing.T) {
	service := &Service{authorizer: denyAuthorizer{}}
	err := service.checkRepoPermissions(
		context.Background(),
		&auth.Session{User: &principal.User{Account: "reader"}},
		&repoModel.Repo{ID: 7, Identifier: "demo"},
		[]constant.Permission{constant.PermissionRepoPush},
	)
	if err == nil {
		t.Fatal("want permission error, got nil")
	}
	if got := pkgErrors.GetI18nKey(err); got != i18n.RepoPushPermissionError {
		t.Fatalf("want i18n key %q, got %q (error: %v)", i18n.RepoPushPermissionError, got, err)
	}
}
