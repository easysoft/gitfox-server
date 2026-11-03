// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      lidaogang <lidaogang@chandao.com>

package util

import "testing"

func TestIsUnrelatedHistoriesError(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		want   bool
	}{
		{name: "no merge base", errMsg: "fatal: no merge base", want: true},
		{name: "unrelated histories", errMsg: "fatal: refusing to merge unrelated histories", want: true},
		{name: "unrelated commit histories", errMsg: "unrelated commit histories error", want: true},
		{
			name:   "entirely different commit histories",
			errMsg: "failed to get merge base: main and other have entirely different commit histories.",
			want:   true,
		},
		{name: "other git error", errMsg: "fatal: bad object", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnrelatedHistoriesError(tt.errMsg); got != tt.want {
				t.Errorf("isUnrelatedHistoriesError(%q) = %v, want %v", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestCheckGitVersionAtLeast(t *testing.T) {
	type args struct {
		atLeast string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "TestCheckGitVersionAtLeast229",
			args: args{
				atLeast: "2.29",
			},
			wantErr: false,
		},
		{
			name: "TestCheckGitVersionAtLeast329",
			args: args{
				atLeast: "3.29",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckGitVersionAtLeast(tt.args.atLeast); (err != nil) != tt.wantErr {
				t.Errorf("CheckGitVersionAtLeast() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
