/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package variable

import (
	"github.com/easysoft/gitfox-community/pkg/utils/datetime"
)

// Variable 对应数据库表 ops_variable（凭证表）。
type Variable struct {
	ID          uint            `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement;comment:'ID'" json:"id"`
	SpaceID     uint            `gorm:"column:spaceID;type:int unsigned;not null;default:0;comment:'空间ID'" json:"spaceID"`
	RepoID      uint            `gorm:"column:repoID;type:int unsigned;not null;default:0;comment:'仓库ID'" json:"repoID"`
	Name        string          `gorm:"column:name;type:varchar(50);not null;default:'';comment:'名称'" json:"name"`
	Key         string          `gorm:"column:key;type:varchar(50);not null;default:'';comment:'变量键'" json:"key"`
	Value       *string         `gorm:"column:value;type:text;comment:'变量值'" json:"value"` // 允许为NULL，指针类型
	Scope       string          `gorm:"column:scope;type:varchar(30);not null;default:'system';comment:'变量作用域'" json:"scope"`
	Type        string          `gorm:"column:type;type:varchar(30);not null;default:'plain';comment:'plain（明文）|cipher(密文)'" json:"type"`
	Desc        string          `gorm:"column:desc;type:varchar(500);not null;default:'';comment:'详情'" json:"desc"`
	CreatedBy   string          `gorm:"column:createdBy;type:varchar(30);not null;default:'';comment:'由谁创建'" json:"createdBy"`
	CreatedDate *datetime.Local `gorm:"column:createdDate;type:datetime;comment:'创建时间'" json:"createdDate,omitempty"` // 允许为NULL，指针类型
	EditedBy    string          `gorm:"column:editedBy;type:varchar(30);not null;default:'';comment:'由谁编辑'" json:"editedBy"`
	EditedDate  *datetime.Local `gorm:"column:editedDate;type:datetime;comment:'编辑时间'" json:"editedDate,omitempty"` // 允许为NULL，指针类型
	Deleted     uint8           `gorm:"column:deleted;type:tinyint unsigned;not null;default:0;comment:'是否删除'" json:"deleted"`
}

// TableName 显式指定结构体对应的数据表名为 ops_variable
func (Variable) TableName() string {
	return "ops_variable"
}
