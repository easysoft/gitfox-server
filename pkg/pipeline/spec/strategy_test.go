// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package yaml

import (
	"strings"
	"testing"
)

func TestParseRejectsMatrixStrategy(t *testing.T) {
	tests := map[string]string{
		"step level": `
version: 1
kind: pipeline
spec:
  stages:
    - name: ci
      type: ci
      spec:
        steps:
          - name: test
            type: run
            strategy:
              type: matrix
              spec:
                axis:
                  go: ["1.21", "1.22"]
            spec:
              container:
                image: golang:1.22
              script:
                - go test ./...
`,
		"stage level": `
version: 1
kind: pipeline
spec:
  stages:
    - name: ci
      type: ci
      strategy:
        type: matrix
        spec:
          axis:
            go: ["1.21", "1.22"]
      spec:
        steps:
          - name: build
            type: run
            spec:
              container:
                image: alpine
              script:
                - echo build
`,
	}

	for name, doc := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseString(doc); err == nil {
				t.Fatal("expected matrix strategy to be rejected")
			} else if !strings.Contains(err.Error(), "matrix strategy is not supported") {
				t.Fatalf("unexpected error: %v", err)
			} else if !strings.Contains(err.Error(), "step of type: parallel") {
				t.Fatalf("error should point to step type: parallel, got: %v", err)
			}
		})
	}
}

func TestParseAllowsExplicitParallelAndOtherStrategies(t *testing.T) {
	tests := map[string]string{
		"explicit parallel": `
version: 1
kind: pipeline
spec:
  stages:
    - name: ci
      type: ci
      spec:
        steps:
          - name: p
            type: parallel
            spec:
              steps:
                - name: a
                  type: run
                  spec:
                    container:
                      image: alpine
                    script:
                      - echo a
`,
		"for strategy": `
version: 1
kind: pipeline
spec:
  stages:
    - name: ci
      type: ci
      spec:
        steps:
          - name: loop
            type: run
            strategy:
              type: for
              spec:
                iterations: 2
            spec:
              container:
                image: alpine
              script:
                - echo loop
`,
	}

	for name, doc := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseString(doc); err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
		})
	}
}
