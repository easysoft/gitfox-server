// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      lidaogang <lidaogang@chandao.com>

package yaml

import (
	"encoding/json"
	"fmt"
)

// Strategy defines an optional execution strategy.
type Strategy struct {
	Type string      `json:"type,omitempty"`
	Spec interface{} `json:"spec,omitempty"`
}

// UnmarshalJSON implement the json.Unmarshaler interface.
func (v *Strategy) UnmarshalJSON(data []byte) error {
	type S Strategy
	type T struct {
		*S
		Spec json.RawMessage `json:"spec"`
	}

	obj := &T{S: (*S)(v)}
	if err := json.Unmarshal(data, obj); err != nil {
		return err
	}

	switch v.Type {
	case "for":
		v.Spec = new(For)
	case "matrix":
		// Matrix expansion is intentionally not supported: declare the
		// parallel branches explicitly with a step of type: parallel.
		return fmt.Errorf("matrix strategy is not supported; declare the branches explicitly with a step of type: parallel")
	case "while":
		v.Spec = new(While)
	default:
		return fmt.Errorf("unknown type %s", v.Type)
	}

	return json.Unmarshal(obj.Spec, v.Spec)
}
