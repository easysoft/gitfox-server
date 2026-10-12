/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package variable

import (
	"context"
	"reflect"
	"testing"

	variableModel "github.com/easysoft/gitfox-community/app/model/variable"
	variableStoreMock "github.com/easysoft/gitfox-community/app/store/variable/mock"

	"go.uber.org/mock/gomock"
)

func TestResolveByKeys(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := variableStoreMock.NewMockInterface(ctrl)
	// 按 YAML 引用到的名字查询，重复项与空白会被去掉。
	store.EXPECT().ListByKeys(gomock.Any(), []string{"registry", "npm_token"}, variableModel.ScopeSystem).Return(map[string]*variableModel.Variable{
		"registry":  {Key: "registry", Name: "镜像仓库地址", Value: "registry.gitfox.cn/prod", Scope: variableModel.ScopeSystem},
		"npm_token": {Key: "npm_token", Value: "s3cr3t", Scope: variableModel.ScopeSystem, Type: variableModel.TypeCipher},
	}, nil)

	values, masked, err := NewService(store).ResolveByKeys(
		context.Background(),
		[]string{"registry", " npm_token ", "registry", ""},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := values["registry"]; got != "registry.gitfox.cn/prod" {
		t.Errorf("registry = %q, want registry.gitfox.cn/prod", got)
	}
	if got := values["npm_token"]; got != "s3cr3t" {
		t.Errorf("npm_token = %q, want s3cr3t", got)
	}

	if want := []string{"npm_token"}; !reflect.DeepEqual(masked, want) {
		t.Errorf("masked = %v, want %v", masked, want)
	}
}

func TestResolveByKeysNoNames(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 没有引用任何全局参数时不查询存储。
	store := variableStoreMock.NewMockInterface(ctrl)

	values, masked, err := NewService(store).ResolveByKeys(context.Background(), []string{"", "  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 0 || len(masked) != 0 {
		t.Errorf("expected empty results, got %v %v", values, masked)
	}
}
