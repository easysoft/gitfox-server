// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      lidaogang <lidaogang@chandao.com>

package util

import (
	"strings"
	"testing"
)

// TestUpdateHookContainsPushRequestArm ensures the refs/for case arm is part of
// the community update-hook template. The arm must not be injected per edition:
// hook scripts are only written when a repository is created, so existing
// repositories would keep a stale script after a server upgrade.
func TestUpdateHookContainsPushRequestArm(t *testing.T) {
	_, _, giteaHookTpls := GetHookTemplates()
	updateHook := giteaHookTpls[1]

	want := []string{
		`refs/for/*,commit)`,
		`new_branch="refs/heads/ztflow-$branch_number"`,
		`[ ! -z "$mail" ] && new_branch="refs/heads/ztflow-$mail-$branch_number"`,
		`git update-ref $new_branch $newrev`,
	}
	for _, s := range want {
		if !strings.Contains(updateHook, s) {
			t.Errorf("update hook template is missing %q", s)
		}
	}
}

// TestHookTemplatesHaveNoUnresolvedFormatVerbs guards against a missing
// fmt.Sprintf argument leaving "%!s(MISSING)" in a generated hook script.
func TestHookTemplatesHaveNoUnresolvedFormatVerbs(t *testing.T) {
	_, hookTpls, giteaHookTpls := GetHookTemplates()

	templates := make([]string, 0, len(hookTpls)+len(giteaHookTpls))
	templates = append(templates, hookTpls...)
	templates = append(templates, giteaHookTpls...)

	for i, tpl := range templates {
		if strings.Contains(tpl, "%!") {
			t.Errorf("hook template %d has an unresolved fmt verb:\n%s", i, tpl)
		}
	}
}
