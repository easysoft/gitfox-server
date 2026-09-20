package sqladapter

// DMAdapter 达梦数据库适配器，保留字（trigger/commit/ref）需用双引号包裹标识符
type DMAdapter struct{}

// ListByPr 返回达梦方言的按 PR 查询流水线执行 SQL。
// 表名、别名与列名统一使用双引号，且别名引用保持一致，避免达梦将未加引号的标识符大写化。
func (DMAdapter) ListByPr(commit, ref string) (string, []any) {
	sql := `SELECT "e"."id", "e"."PipelineID", "p"."name", "e"."trigger", "e"."commit", "e"."ref", "e"."status" ` +
		`FROM "ops_pipeline_executions" "e" ` +
		`JOIN "ops_pipeline" "p" ON "e"."PipelineID" = "p"."id" ` +
		`WHERE "p"."deleted" = 0`

	var args []any
	if commit != "" {
		sql += ` AND "e"."commit" = ?`
		args = append(args, commit)
	}
	if ref != "" {
		sql += ` AND "e"."ref" = ?`
		args = append(args, ref)
	}
	return sql, args
}

// QuoteIdentifier 使用双引号引用标识符
func (DMAdapter) QuoteIdentifier(ident string) string {
	return quoteIdentifier(ident, `"`)
}

func init() {
	Register("dm", DMAdapter{})
}
