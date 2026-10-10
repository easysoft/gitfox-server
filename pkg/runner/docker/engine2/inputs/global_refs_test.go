/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package inputs

import (
	"reflect"
	"testing"
)

func TestGlobalRefs(t *testing.T) {
	config := []byte(`
stages:
  - name: build
    when: ${{ global.env == 'prod' }}
    spec:
      steps:
        - name: clone
          spec:
            inputs:
              registry: ${{global.registry}}
              token: ${{ global.npm_token }}
              repository: ${{ parameters.gitURL }}
        - name: other
          script:
            - echo ${{global.registry}}
`)

	got := GlobalRefs(config)
	want := []string{"env", "registry", "npm_token"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GlobalRefs = %v, want %v", got, want)
	}
}

func TestGlobalRefsNone(t *testing.T) {
	for _, config := range [][]byte{
		[]byte("inputs:\n  a: ${{ parameters.x }}\n"),
		[]byte("inputs:\n  a: ${{ inputs.registry }}\n"),
		nil,
	} {
		if got := GlobalRefs(config); len(got) != 0 {
			t.Errorf("GlobalRefs(%q) = %v, want empty", config, got)
		}
	}
}
