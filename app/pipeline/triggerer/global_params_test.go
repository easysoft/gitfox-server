package triggerer

import (
	"context"
	"testing"
)

type fakeGlobalResolver struct {
	keys []string
}

func (f *fakeGlobalResolver) ResolveByKeys(_ context.Context, keys []string) (map[string]string, []string, error) {
	f.keys = keys
	return map[string]string{"registry": "registry.gitfox.cn/prod"}, []string{"npm_token"}, nil
}

// 流水线 YAML 中引用的 ${{global.<name>}} 会被收集出来，按名字去全局参数表取值。
func TestResolveGlobalParams(t *testing.T) {
	resolver := &fakeGlobalResolver{}
	tr := &triggerer{variableResolver: resolver}

	config := []byte(`
stages:
  - name: build
    spec:
      steps:
        - name: clone
          spec:
            inputs:
              registry: ${{global.registry}}
              token: ${{ global.npm_token }}
              repository: ${{ parameters.gitURL }}
`)

	globals, masked, err := tr.resolveGlobalParams(context.Background(), config)
	if err != nil {
		t.Fatalf("resolveGlobalParams: %v", err)
	}

	if len(resolver.keys) != 2 || resolver.keys[0] != "registry" || resolver.keys[1] != "npm_token" {
		t.Errorf("resolver keys = %v, want [registry npm_token]", resolver.keys)
	}
	if globals["registry"] != "registry.gitfox.cn/prod" {
		t.Errorf("globals = %v", globals)
	}
	if len(masked) != 1 || masked[0] != "npm_token" {
		t.Errorf("masked = %v, want [npm_token]", masked)
	}
}

// 没接入全局参数表时不报错、不查表。
func TestResolveGlobalParamsWithoutResolver(t *testing.T) {
	tr := &triggerer{}

	globals, masked, err := tr.resolveGlobalParams(context.Background(), []byte("a: ${{global.x}}"))
	if err != nil || len(globals) != 0 || len(masked) != 0 {
		t.Errorf("unexpected result: globals=%v masked=%v err=%v", globals, masked, err)
	}
}
