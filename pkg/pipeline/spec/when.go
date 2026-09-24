package yaml

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

type (
	// When defines execution conditions.
	When struct {
		Eval    string
		Cond    []map[string]*Expr
		Changes []string
	}

	// Expr defines an execution expression.
	Expr struct {
		Eq  string   `json:"eq,omitempty"`
		In  []string `json:"in,omitempty"`
		Not *Expr    `json:"not,omitempty"`
	}
)

// when 条件求值结果对应的日志文案模板。
const (
	WhenSkippedStageTmpl      = "执行条件：%s，运算结果：false，跳过当前阶段。"
	WhenSkippedStepTmpl       = "执行条件：%s，运算结果：false，跳过当前步骤。"
	WhenExecutedStepTmpl      = "执行条件：%s，运算结果：true，执行当前步骤。"
	RunPolicySkippedStageTmpl = "运行策略 %s 与流水线当前状态不匹配，跳过当前阶段。"
	WhenEvalFailedStepTmpl    = "执行条件 %s 无法求值：%v"
)

// errWhenClause 是无法识别的 when 写法统一返回的错误。
var errWhenClause = errors.New("failed to unmarshal when clause")

// UnmarshalJSON implements the unmarshal interface.
func (v *When) UnmarshalJSON(data []byte) error {
	// null / 空白内容不包含任何条件信息，视为无法识别。
	if trimmed := bytes.TrimSpace(data); len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return errWhenClause
	}

	// 解析表达式或运行策略字符串。
	if err := json.Unmarshal(data, &v.Eval); err == nil {
		if strings.TrimSpace(v.Eval) == "" {
			return errWhenClause
		}
		return nil
	}

	// 解析对象形式：只识别 eval / changes。带 cond 键的对象不在支持范围内
	type whenTemp struct {
		Eval    string   `json:"eval,omitempty"`
		Changes []string `json:"changes,omitempty"`
	}
	var tmp whenTemp
	if err := json.Unmarshal(data, &tmp); err == nil {
		v.Eval = tmp.Eval
		v.Changes = tmp.Changes
		if strings.TrimSpace(v.Eval) != "" || len(v.Changes) > 0 {
			return nil
		}
	}

	// 解析声明式 when 子句数组。
	var conds []map[string]*Expr
	if err := json.Unmarshal(data, &conds); err == nil && len(conds) > 0 {
		v.Cond = conds
		return nil
	}

	// 解析声明式 when 子句。
	cond := map[string]*Expr{}
	if err := json.Unmarshal(data, &cond); err == nil && len(cond) > 0 {
		v.Cond = append(v.Cond, cond)
		return nil
	}

	// 走到这里说明是无法识别的 when 写法返回错误让整份配置解析失败
	return errWhenClause
}

// MarshalJSON implements the marshal interface.
func (v *When) MarshalJSON() ([]byte, error) {
	if len(v.Changes) > 0 {
		type whenTemp struct {
			Eval    string             `json:"eval,omitempty"`
			Cond    []map[string]*Expr `json:"cond,omitempty"`
			Changes []string           `json:"changes,omitempty"`
		}
		return json.Marshal(whenTemp{
			Eval:    v.Eval,
			Cond:    v.Cond,
			Changes: v.Changes,
		})
	}

	if v.Eval != "" {
		return json.Marshal(v.Eval)
	}
	if len(v.Cond) > 0 {
		return json.Marshal(v.Cond)
	}
	return []byte("null"), nil
}
