package variable

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=$GOFILE -destination=mock/variable_store.go -package=mock
import (
	"context"

	variableModel "github.com/easysoft/gitfox-community/app/model/variable"
)

type Interface interface {
	// ListByKeys 根据变量键批量查询未删除的变量，返回 key -> 变量 的映射
	ListByKeys(ctx context.Context, keys []string) (map[string]*variableModel.Variable, error)
}
