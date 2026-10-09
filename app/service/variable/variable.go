/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package variable

import (
	"context"
	"strings"

	variableModel "github.com/easysoft/gitfox-community/app/model/variable"
	variableStore "github.com/easysoft/gitfox-community/app/store/variable"
)

type Service struct {
	store variableStore.Interface
}

func NewService(store variableStore.Interface) *Service {
	return &Service{store: store}
}

// ResolveByKeys 按参数名（即 YAML 中 ${{global.<name>}} 的 name）查询全局参数表。
// type=cipher 的行会额外出现在 masked 中，供日志掩码使用。
func (s *Service) ResolveByKeys(ctx context.Context, keys []string) (map[string]string, []string, error) {
	resolved := map[string]string{}
	if s == nil || s.store == nil {
		return resolved, nil, nil
	}

	names := normalizeKeys(keys)
	if len(names) == 0 {
		return resolved, nil, nil
	}

	rows, err := s.store.ListByKeys(ctx, names)
	if err != nil {
		return nil, nil, err
	}

	var masked []string
	for _, row := range rows {
		if row == nil {
			continue
		}
		resolved[row.Key] = row.Value
		if row.Type == variableModel.TypeCipher {
			masked = append(masked, row.Key)
		}
	}

	return resolved, masked, nil
}

// normalizeKeys 去掉空白与重复项，避免无意义的查询与重复命中。
func normalizeKeys(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	keys := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		keys = append(keys, v)
	}
	return keys
}
