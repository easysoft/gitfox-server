// Copyright 2019 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package shell

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestIsMultiline(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"echo hello", false},
		{"echo hello\n", false},
		{"echo hello\n\n", false},
		{"echo hello\necho world", true},
		{"echo hello\necho world\n", true},
		{"echo hello\r\necho world\r\n", true},
	}
	for _, test := range tests {
		if got := isMultiline(test.command); got != test.want {
			t.Errorf("isMultiline(%q) = %v, want %v", test.command, got, test.want)
		}
	}
}

func TestScriptSingleLineTrace(t *testing.T) {
	got := Script([]string{"echo hello"})
	if !strings.Contains(got, `echo + "echo hello"`) {
		t.Errorf("single-line command should use echo trace, got:\n%s", got)
	}
	if strings.Contains(got, "set -x") {
		t.Errorf("single-line command should not enable xtrace, got:\n%s", got)
	}
}

func TestScriptMultilineTrace(t *testing.T) {
	got := Script([]string{"echo one\necho two"})
	if strings.Contains(got, "echo + ") {
		t.Errorf("multi-line command should not use echo trace, got:\n%s", got)
	}
	if !strings.Contains(got, "set -x\necho one\necho two\n{ set +x; } 2>/dev/null\n") {
		t.Errorf("multi-line command should enable xtrace around the script, got:\n%s", got)
	}
}

// TestScriptMultilineTraceOrder verifies that commands and their output
// are interleaved instead of every command being printed before any
// output. This is the behavior that keeps CI logs in execution order.
func TestScriptMultilineTraceOrder(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	script := Script([]string{"echo ONE\necho TWO"})
	cmd := exec.Command("sh")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cannot execute script: %v\n%s", err, out)
	}

	want := "+ echo ONE\nONE\n+ echo TWO\nTWO\n"
	if !strings.Contains(string(out), want) {
		t.Errorf("commands and output are not interleaved, got:\n%s", out)
	}
}

// Docker 的日志流不保证 stdout/stderr 的先后顺序；脚本需要把 stderr
// 合并到 stdout，保证命令、trace 和输出在日志中按执行顺序出现。
func TestScriptMergesStderrIntoStdout(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}

	script := Script([]string{"echo OUT1; echo ERR1 >&2; echo OUT2; echo ERR2 >&2"})
	if !strings.Contains(script, "exec 2>&1") {
		t.Fatalf("script should merge stderr into stdout, got:\n%s", script)
	}

	cmd := exec.Command("sh")
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("cannot execute script: %v", err)
	}

	if stderr.Len() != 0 {
		t.Errorf("stderr should be merged into stdout, got stderr:\n%s", stderr.String())
	}
	if want := "OUT1\nERR1\nOUT2\nERR2\n"; !strings.Contains(stdout.String(), want) {
		t.Errorf("stdout does not preserve command order, got:\n%s", stdout.String())
	}
}
