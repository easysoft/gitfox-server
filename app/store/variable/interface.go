/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package variable

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=$GOFILE -destination=mock/variable_store.go -package=mock
import (
	"context"

	variableModel "github.com/easysoft/gitfox-community/app/model/variable"
)

type Interface interface {
	// ListByKeys 根据变量键批量查询指定作用域下未删除的变量，返回 key -> 变量 的映射。
	ListByKeys(ctx context.Context, keys []string, scope string) (map[string]*variableModel.Variable, error)
}
