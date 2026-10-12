/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package variable

import (
	"context"

	variableModel "github.com/easysoft/gitfox-community/app/model/variable"
	"github.com/easysoft/gitfox-community/pkg/database"
	"gorm.io/gorm"
)

var _ Interface = (*Store)(nil)

type Store struct {
	db *gorm.DB
}

func NewVariableStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// ListByKeys 根据键批量查询指定作用域下未删除的变量，返回 key -> 变量 的映射。
// 查询始终按作用域过滤，避免跨作用域取到同名变量。
func (s *Store) ListByKeys(ctx context.Context, keys []string, scope string) (map[string]*variableModel.Variable, error) {
	result := make(map[string]*variableModel.Variable, len(keys))
	if len(keys) == 0 {
		return result, nil
	}

	var rows []*Variable
	err := s.db.WithContext(ctx).Where("`key` IN ? AND scope = ? AND deleted = 0", keys, scope).Find(&rows).Error
	if err != nil {
		return nil, database.ProcessSQLErrorf(ctx, err)
	}

	for _, row := range rows {
		result[row.Key] = mapToModel(*row)
	}
	return result, nil
}

// mapToModel 将存储层 Variable 转换为模型层 Variable
func mapToModel(data Variable) *variableModel.Variable {
	value := ""
	if data.Value != nil {
		value = *data.Value
	}

	return &variableModel.Variable{
		ID:          data.ID,
		SpaceID:     data.SpaceID,
		RepoID:      data.RepoID,
		Name:        data.Name,
		Key:         data.Key,
		Value:       value,
		Scope:       data.Scope,
		Type:        data.Type,
		Desc:        data.Desc,
		CreatedBy:   data.CreatedBy,
		CreatedDate: data.CreatedDate,
		EditedBy:    data.EditedBy,
		EditedDate:  data.EditedDate,
	}
}
