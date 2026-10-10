/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package variable

import (
	"context"
	"fmt"
	"strings"
	"testing"

	variableModel "github.com/easysoft/gitfox-community/app/model/variable"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&Variable{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	return NewVariableStore(db)
}

func strPtr(v string) *string { return &v }

// 作用域筛选：只返回指定作用域、未删除的变量。
func TestStoreListByKeysScope(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	rows := []*Variable{
		{Key: "registry", Name: "镜像仓库地址", Value: strPtr("registry.gitfox.cn/prod"), Scope: variableModel.ScopeSystem},
		{Key: "npm_token", Name: "npm token", Value: strPtr("s3cr3t"), Scope: variableModel.ScopeSystem, Type: variableModel.TypeCipher},
		// 同名的仓库级变量不应命中 system 作用域的查询。
		{Key: "registry", Name: "仓库级镜像地址", Value: strPtr("repo.example.com"), Scope: "repo", RepoID: 1},
		// 已删除的系统级变量不应命中。
		{Key: "removed", Name: "已删除", Value: strPtr("x"), Scope: variableModel.ScopeSystem, Deleted: 1},
	}
	if err := store.db.WithContext(ctx).Create(&rows).Error; err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	got, err := store.ListByKeys(ctx, []string{"registry", "npm_token", "removed"}, variableModel.ScopeSystem)
	if err != nil {
		t.Fatalf("ListByKeys returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2 (%v)", len(got), got)
	}
	if got["registry"].Value != "registry.gitfox.cn/prod" {
		t.Errorf("registry = %q, want system 作用域的值", got["registry"].Value)
	}
	if got["npm_token"] == nil || got["npm_token"].Type != variableModel.TypeCipher {
		t.Errorf("npm_token = %+v, want cipher 变量", got["npm_token"])
	}
	if _, ok := got["removed"]; ok {
		t.Error("已删除的变量不应被查询到")
	}
}

// 没有键时不查询数据库，直接返回空结果。
func TestStoreListByKeysEmpty(t *testing.T) {
	store := newTestStore(t)

	got, err := store.ListByKeys(context.Background(), nil, variableModel.ScopeSystem)
	if err != nil {
		t.Fatalf("ListByKeys returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("rows = %d, want 0", len(got))
	}
}
