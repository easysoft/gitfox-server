package script

import "testing"

// global 命名空间与 parameters 是两个独立命名空间，求值时互不影响。
func TestExpandGlobalNamespace(t *testing.T) {
	inputs := map[string]interface{}{
		"parameters": map[string]interface{}{"registry": "from-parameters"},
		"global":     map[string]interface{}{"registry": "from-global"},
	}

	if got, want := Expand("${{global.registry}}", inputs), "from-global"; got != want {
		t.Errorf("global.registry expanded to %q, want %q", got, want)
	}
	if got, want := Expand("${{parameters.registry}}", inputs), "from-parameters"; got != want {
		t.Errorf("parameters.registry expanded to %q, want %q", got, want)
	}
}
