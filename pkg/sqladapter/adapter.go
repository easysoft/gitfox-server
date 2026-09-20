package sqladapter

import (
	"strings"

	"gorm.io/gorm"
)

// SQLAdapter 数据库SQL适配接口，定义所有差异化SQL的生成方法
type SQLAdapter interface {
	// ListByPr 返回按 PR 查询流水线执行的方言 SQL 与绑定参数。
	// commit/ref 为空表示该条件不参与过滤，绑定参数顺序与 SQL 中占位符一致。
	ListByPr(commit, ref string) (string, []any)

	// QuoteIdentifier 按方言引用标识符（如 "g.desc"），用于保留字列/表。
	// 支持点号限定的多段标识符，逐段引用；"*" 保持原样。
	QuoteIdentifier(ident string) string
}

// quoteIdentifier 按给定引号逐段引用标识符。
func quoteIdentifier(ident, quote string) string {
	if ident == "" {
		return ident
	}
	parts := strings.Split(ident, ".")
	for i, part := range parts {
		if part == "*" {
			continue
		}
		parts[i] = quote + part + quote
	}
	return strings.Join(parts, ".")
}

// AdapterManager 适配器管理器，根据驱动名获取适配器
var AdapterManager = make(map[string]SQLAdapter)

// Register 注册适配器（在各数据库适配器初始化时调用）
func Register(driverName string, adapter SQLAdapter) {
	AdapterManager[driverName] = adapter
}

// GetAdapter 根据驱动名获取适配器，无匹配则返回通用适配器
func GetAdapter(driverName string) SQLAdapter {
	if adapter, ok := AdapterManager[driverName]; ok {
		return adapter
	}
	return GenericAdapter{}
}

// GetAdapterByDB 根据 gorm.DB 的方言名获取适配器
func GetAdapterByDB(db *gorm.DB) SQLAdapter {
	return GetAdapter(db.Dialector.Name())
}
