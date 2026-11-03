package sqladapter

// GenericAdapter 通用适配器，适用于 MySQL、SQLite 等接受反引号标识符的数据库
type GenericAdapter struct{}

// ListByPr 返回通用方言的按 PR 查询流水线执行 SQL。
// trigger/commit/ref 为保留字，需用反引号包裹。
func (GenericAdapter) ListByPr(commit, ref string) (string, []any) {
	sql := "SELECT `e`.`id`, `e`.`PipelineID`, `p`.`name`, `e`.`trigger`, `e`.`commit`, `e`.`ref`, `e`.`status` " +
		"FROM `ops_pipeline_executions` `e` " +
		"JOIN `ops_pipeline` `p` ON `e`.`PipelineID` = `p`.`id` " +
		"WHERE `p`.`deleted` = 0"

	var args []any
	if commit != "" {
		sql += " AND `e`.`commit` = ?"
		args = append(args, commit)
	}
	if ref != "" {
		sql += " AND `e`.`ref` = ?"
		args = append(args, ref)
	}
	return sql, args
}

// QuoteIdentifier 使用反引号引用标识符
func (GenericAdapter) QuoteIdentifier(ident string) string {
	return quoteIdentifier(ident, "`")
}

func init() {
	Register("mysql", GenericAdapter{})
	Register("sqlite", GenericAdapter{})
}
