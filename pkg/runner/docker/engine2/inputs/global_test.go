/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */
package inputs

import "testing"

func TestSplitGlobal(t *testing.T) {
	params := map[string]string{
		"gitRef":               "refs/heads/main",
		GlobalKey("registry"):  "registry.gitfox.cn/prod",
		GlobalKey("npm_token"): "s3cr3t",
		GlobalMaskKey:          "npm_token",
	}

	plain, global, masked := SplitGlobal(params)

	if plain["gitRef"] != "refs/heads/main" {
		t.Errorf("plain[gitRef] = %q, want refs/heads/main", plain["gitRef"])
	}
	if _, ok := plain[GlobalKey("registry")]; ok {
		t.Error("global key leaked into plain params")
	}
	if _, ok := plain[GlobalMaskKey]; ok {
		t.Error("mask key leaked into plain params")
	}
	if len(plain) != 1 {
		t.Errorf("plain size = %d, want 1 (%v)", len(plain), plain)
	}

	if global["registry"] != "registry.gitfox.cn/prod" {
		t.Errorf("global[registry] = %v", global["registry"])
	}
	if _, ok := global["npm_token"]; !ok {
		t.Error("global missing npm_token")
	}
	if len(global) != 2 {
		t.Errorf("global size = %d, want 2 (%v)", len(global), global)
	}

	if masked["npm_token"] != "s3cr3t" {
		t.Errorf("masked[npm_token] = %q", masked["npm_token"])
	}
	if len(masked) != 1 {
		t.Errorf("masked size = %d, want 1 (%v)", len(masked), masked)
	}
}

func TestSplitGlobalEmpty(t *testing.T) {
	plain, global, masked := SplitGlobal(nil)
	if len(plain) != 0 || len(global) != 0 || len(masked) != 0 {
		t.Errorf("expected empty results, got %v %v %v", plain, global, masked)
	}
}

func TestSplitGlobalMaskWithoutValue(t *testing.T) {
	params := map[string]string{
		"gitURL":               "http://gitfox.example/repo.git",
		GlobalMaskKey:          "missing, ,npm_token",
		GlobalKey("npm_token"): "s3cr3t",
	}

	_, _, masked := SplitGlobal(params)

	if len(masked) != 1 || masked["npm_token"] != "s3cr3t" {
		t.Errorf("masked = %v, want only npm_token", masked)
	}
}

func TestRedactGlobalParams(t *testing.T) {
	params := map[string]string{
		"gitRef":               "refs/heads/main",
		GlobalKey("registry"):  "registry.gitfox.cn/prod",
		GlobalKey("npm_token"): "s3cr3t",
		GlobalMaskKey:          "npm_token",
	}

	out := RedactGlobalParams(params)

	if out["gitRef"] != "refs/heads/main" {
		t.Errorf("gitRef = %q, want refs/heads/main", out["gitRef"])
	}
	if out[GlobalKey("registry")] != "registry.gitfox.cn/prod" {
		t.Errorf("registry = %q, want明文保留", out[GlobalKey("registry")])
	}
	if out[GlobalKey("npm_token")] != RedactedValue {
		t.Errorf("npm_token = %q, want %q", out[GlobalKey("npm_token")], RedactedValue)
	}
	if _, ok := out[GlobalMaskKey]; ok {
		t.Error("内部的掩码清单键不应出现在展示结果里")
	}
	if params[GlobalKey("npm_token")] != "s3cr3t" {
		t.Error("入参不应被修改")
	}
}

func TestRedactGlobalParamsWithoutGlobals(t *testing.T) {
	params := map[string]string{"gitRef": "refs/heads/main"}
	out := RedactGlobalParams(params)
	if len(out) != 1 || out["gitRef"] != "refs/heads/main" {
		t.Errorf("out = %v", out)
	}
}
