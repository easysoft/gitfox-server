/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package yaml

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// When defines execution conditions.
type When struct {
	Eval string
}

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

	// 解析对象形式：只识别 eval。
	type whenTemp struct {
		Eval string `json:"eval,omitempty"`
	}
	var tmp whenTemp
	if err := json.Unmarshal(data, &tmp); err == nil && strings.TrimSpace(tmp.Eval) != "" {
		v.Eval = tmp.Eval
		return nil
	}

	// 走到这里说明是无法识别的 when 写法，返回错误让整份配置解析失败。
	return errWhenClause
}

// MarshalJSON implements the marshal interface.
func (v *When) MarshalJSON() ([]byte, error) {
	if v.Eval != "" {
		return json.Marshal(v.Eval)
	}
	return []byte("null"), nil
}
