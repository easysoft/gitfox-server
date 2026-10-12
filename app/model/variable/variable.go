/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package variable

import (
	"github.com/easysoft/gitfox-community/pkg/utils/datetime"
)

const (
	ScopeSystem = "system" // 系统级变量
)

const (
	TypePlain  = "plain"  // 明文存储
	TypeCipher = "cipher" // 密文存储
)

// Variable 变量模型
type Variable struct {
	ID          uint            `json:"id"`
	SpaceID     uint            `json:"spaceID"`
	RepoID      uint            `json:"repoID"`
	Name        string          `json:"name"`
	Key         string          `json:"key"`
	Value       string          `json:"value"`
	Scope       string          `json:"scope"`
	Type        string          `json:"type"`
	Desc        string          `json:"desc"`
	CreatedBy   string          `json:"createdBy"`
	CreatedDate *datetime.Local `json:"createdDate"`
	EditedBy    string          `json:"editedBy"`
	EditedDate  *datetime.Local `json:"editedDate"`
}
