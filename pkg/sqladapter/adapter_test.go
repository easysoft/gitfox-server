package sqladapter

import (
	"reflect"
	"testing"
)

func TestGenericAdapterListByPr(t *testing.T) {
	tests := []struct {
		name   string
		commit string
		ref    string
		sql    string
		args   []any
	}{
		{
			name: "no filters",
			sql: "SELECT `e`.`id`, `e`.`PipelineID`, `p`.`name`, `e`.`trigger`, `e`.`commit`, `e`.`ref`, `e`.`status` " +
				"FROM `ops_pipeline_executions` `e` " +
				"JOIN `ops_pipeline` `p` ON `e`.`PipelineID` = `p`.`id` " +
				"WHERE `p`.`deleted` = 0",
			args: nil,
		},
		{
			name:   "commit only",
			commit: "abc",
			sql: "SELECT `e`.`id`, `e`.`PipelineID`, `p`.`name`, `e`.`trigger`, `e`.`commit`, `e`.`ref`, `e`.`status` " +
				"FROM `ops_pipeline_executions` `e` " +
				"JOIN `ops_pipeline` `p` ON `e`.`PipelineID` = `p`.`id` " +
				"WHERE `p`.`deleted` = 0 AND `e`.`commit` = ?",
			args: []any{"abc"},
		},
		{
			name: "ref only",
			ref:  "refs/pullreq/1/head",
			sql: "SELECT `e`.`id`, `e`.`PipelineID`, `p`.`name`, `e`.`trigger`, `e`.`commit`, `e`.`ref`, `e`.`status` " +
				"FROM `ops_pipeline_executions` `e` " +
				"JOIN `ops_pipeline` `p` ON `e`.`PipelineID` = `p`.`id` " +
				"WHERE `p`.`deleted` = 0 AND `e`.`ref` = ?",
			args: []any{"refs/pullreq/1/head"},
		},
		{
			name:   "commit and ref",
			commit: "abc",
			ref:    "refs/pullreq/1/head",
			sql: "SELECT `e`.`id`, `e`.`PipelineID`, `p`.`name`, `e`.`trigger`, `e`.`commit`, `e`.`ref`, `e`.`status` " +
				"FROM `ops_pipeline_executions` `e` " +
				"JOIN `ops_pipeline` `p` ON `e`.`PipelineID` = `p`.`id` " +
				"WHERE `p`.`deleted` = 0 AND `e`.`commit` = ? AND `e`.`ref` = ?",
			args: []any{"abc", "refs/pullreq/1/head"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := GenericAdapter{}.ListByPr(tt.commit, tt.ref)
			if sql != tt.sql {
				t.Errorf("sql mismatch:\n got: %s\nwant: %s", sql, tt.sql)
			}
			if !reflect.DeepEqual(args, tt.args) {
				t.Errorf("args mismatch: got %v, want %v", args, tt.args)
			}
		})
	}
}

func TestDMAdapterListByPr(t *testing.T) {
	tests := []struct {
		name   string
		commit string
		ref    string
		sql    string
		args   []any
	}{
		{
			name: "no filters",
			sql: `SELECT "e"."id", "e"."PipelineID", "p"."name", "e"."trigger", "e"."commit", "e"."ref", "e"."status" ` +
				`FROM "ops_pipeline_executions" "e" ` +
				`JOIN "ops_pipeline" "p" ON "e"."PipelineID" = "p"."id" ` +
				`WHERE "p"."deleted" = 0`,
			args: nil,
		},
		{
			name:   "commit and ref",
			commit: "abc",
			ref:    "refs/pullreq/1/head",
			sql: `SELECT "e"."id", "e"."PipelineID", "p"."name", "e"."trigger", "e"."commit", "e"."ref", "e"."status" ` +
				`FROM "ops_pipeline_executions" "e" ` +
				`JOIN "ops_pipeline" "p" ON "e"."PipelineID" = "p"."id" ` +
				`WHERE "p"."deleted" = 0 AND "e"."commit" = ? AND "e"."ref" = ?`,
			args: []any{"abc", "refs/pullreq/1/head"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := DMAdapter{}.ListByPr(tt.commit, tt.ref)
			if sql != tt.sql {
				t.Errorf("sql mismatch:\n got: %s\nwant: %s", sql, tt.sql)
			}
			if !reflect.DeepEqual(args, tt.args) {
				t.Errorf("args mismatch: got %v, want %v", args, tt.args)
			}
		})
	}
}

func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		adapter SQLAdapter
		ident   string
		want    string
	}{
		{"generic simple", GenericAdapter{}, "desc", "`desc`"},
		{"generic qualified", GenericAdapter{}, "g.desc", "`g`.`desc`"},
		{"generic star", GenericAdapter{}, "g.*", "`g`.*"},
		{"generic empty", GenericAdapter{}, "", ""},
		{"dm simple", DMAdapter{}, "desc", `"desc"`},
		{"dm qualified", DMAdapter{}, "g.desc", `"g"."desc"`},
		{"dm star", DMAdapter{}, "g.*", `"g".*`},
		{"dm empty", DMAdapter{}, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.adapter.QuoteIdentifier(tt.ident); got != tt.want {
				t.Errorf("QuoteIdentifier(%q) = %q, want %q", tt.ident, got, tt.want)
			}
		})
	}
}

func TestGetAdapter(t *testing.T) {
	if _, ok := GetAdapter("dm").(DMAdapter); !ok {
		t.Errorf("expected DMAdapter for driver dm, got %T", GetAdapter("dm"))
	}
	if _, ok := GetAdapter("mysql").(GenericAdapter); !ok {
		t.Errorf("expected GenericAdapter for driver mysql, got %T", GetAdapter("mysql"))
	}
	if _, ok := GetAdapter("unknown").(GenericAdapter); !ok {
		t.Errorf("expected GenericAdapter fallback for unknown driver, got %T", GetAdapter("unknown"))
	}
}
