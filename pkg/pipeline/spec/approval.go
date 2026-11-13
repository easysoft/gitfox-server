package yaml

import (
	"encoding/json"
	"errors"
	"strings"
)

// Approval stage 级审批配置。整体省略 = 无需审批。
type Approval struct {
	Approvers CommaSeparated `json:"approvers,omitempty"` // 逗号分隔的审批人账号，如 "zhangsan,lisi"
	Mode      string         `json:"mode,omitempty"`      // and=会签(默认) / or=或签
	Timeout   string         `json:"timeout,omitempty"`   // Go duration，范围 1h-72h，如 1h / 24h / 72h
	// Percent 会签通过阈值，1-100；省略（nil）= 100（全票通过）。
	// 用指针区分「未配置」与「显式配置为 0」，后者在校验时报错。
	Percent *int `json:"percent,omitempty"`
}

// CommaSeparated 逗号分隔的账号列表。
// 注意：不要复用 Stringorslice —— 它对 "a,b" 会解析成 ["a,b"] 一整项。
type CommaSeparated []string

// UnmarshalJSON implements the json.Unmarshaler interface.
func (s *CommaSeparated) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return errors.New(`approvers must be a comma separated string, e.g. "zhangsan,lisi"`)
	}

	out := make([]string, 0, 4)
	for _, part := range strings.Split(str, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	*s = out
	return nil
}
