// Copyright 2023 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func runGitForTest(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)

	return strings.TrimSpace(string(out))
}

func TestGetMergeBaseUnrelatedHistories(t *testing.T) {
	dir := t.TempDir()
	runGitForTest(t, dir, "init", "-q", "-b", "main")
	runGitForTest(t, dir, "config", "user.email", "test@example.com")
	runGitForTest(t, dir, "config", "user.name", "test")
	runGitForTest(t, dir, "commit", "-q", "--allow-empty", "-m", "main")
	runGitForTest(t, dir, "checkout", "-q", "--orphan", "other")
	runGitForTest(t, dir, "commit", "-q", "--allow-empty", "-m", "other")

	mainSHA := runGitForTest(t, dir, "rev-parse", "main")
	otherSHA := runGitForTest(t, dir, "rev-parse", "other")

	g := &Git{}
	_, _, err := g.GetMergeBase(context.Background(), dir, "origin", mainSHA, otherSHA)

	var unrelated *UnrelatedHistoriesError
	require.ErrorAs(t, err, &unrelated)
	require.Equal(t, mainSHA, unrelated.BaseRef)
	require.Equal(t, otherSHA, unrelated.HeadRef)
}
