// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      lidaogang <lidaogang@chandao.com>

package pipeline

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// When defines execution conditions.
// 当满足条件时执行
type When struct {
	Eval string `json:"eval,omitempty"`
}

// errWhenClause 是无法识别的 when 写法统一返回的错误。
var errWhenClause = errors.New("failed to unmarshal when clause")

// UnmarshalJSON implements the unmarshal interface.
// 解析JSON数据到When结构体
func (v *When) UnmarshalJSON(data []byte) error {
	// null或空白内容，视为无法识别。
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

	// 走到这里说明是无法识别的 when 写法返回错误让整份配置解析失败。
	return errWhenClause
}

// MarshalJSON implements the marshal interface.
// 将When结构体转换为JSON数据
func (v *When) MarshalJSON() ([]byte, error) {
	if v.Eval != "" {
		return json.Marshal(v.Eval)
	}
	return []byte("null"), nil
}
