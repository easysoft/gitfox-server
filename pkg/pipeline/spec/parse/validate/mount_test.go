// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package validate

import (
	"strings"
	"testing"

	schema "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
)

func ciStage(volumes []*schema.Volume, steps ...*schema.Step) *schema.Stage {
	return &schema.Stage{
		Type: "ci",
		Spec: &schema.StageCI{
			Volumes: volumes,
			Steps:   steps,
		},
	}
}

func runStep(name string, mounts ...*schema.Mount) *schema.Step {
	return &schema.Step{
		Name: name,
		Type: "run",
		Spec: &schema.StepRun{Mount: mounts},
	}
}

func tempVolume(name string) *schema.Volume {
	return &schema.Volume{Name: name, Type: "temp", Spec: &schema.VolumeTemp{}}
}

func TestValidateStageMounts(t *testing.T) {
	tests := []struct {
		name    string
		stage   *schema.Stage
		wantErr string
	}{
		{
			name: "合法挂载",
			stage: ciStage(
				[]*schema.Volume{tempVolume("tmp")},
				runStep("build", &schema.Mount{Name: "tmp", Path: "/tmp"}),
			),
		},
		{
			name: "未声明的卷",
			stage: ciStage(
				[]*schema.Volume{tempVolume("tmp")},
				runStep("build", &schema.Mount{Name: "data", Path: "/data"}),
			),
			wantErr: `mount volume "data" is not declared`,
		},
		{
			name: "空挂载项",
			stage: ciStage(
				[]*schema.Volume{tempVolume("tmp")},
				runStep("build", nil),
			),
			wantErr: `mount must not be empty`,
		},
		{
			name: "路径不是绝对路径",
			stage: ciStage(
				[]*schema.Volume{tempVolume("tmp")},
				runStep("build", &schema.Mount{Name: "tmp", Path: "tmp"}),
			),
			wantErr: `mount path must be absolute: "tmp"`,
		},
		{
			name: "同一步骤内路径重复",
			stage: ciStage(
				[]*schema.Volume{tempVolume("tmp"), tempVolume("other")},
				runStep("build",
					&schema.Mount{Name: "tmp", Path: "/data"},
					&schema.Mount{Name: "other", Path: "/data"},
				),
			),
			wantErr: `duplicate mount path "/data"`,
		},
		{
			name: "末尾斜杠等价路径视为重复",
			stage: ciStage(
				[]*schema.Volume{tempVolume("tmp"), tempVolume("other")},
				runStep("build",
					&schema.Mount{Name: "tmp", Path: "/data"},
					&schema.Mount{Name: "other", Path: "/data/"},
				),
			),
			wantErr: `duplicate mount path "/data"`,
		},
		{
			name: "含 .. 的等价路径视为重复",
			stage: ciStage(
				[]*schema.Volume{tempVolume("tmp"), tempVolume("other")},
				runStep("build",
					&schema.Mount{Name: "tmp", Path: "/data"},
					&schema.Mount{Name: "other", Path: "/a/../data"},
				),
			),
			wantErr: `duplicate mount path "/data"`,
		},
		{
			name: "卷名重复",
			stage: ciStage(
				[]*schema.Volume{
					{Name: "cache", Type: "host", Spec: &schema.VolumeHost{Path: "/tmp"}},
					{Name: "cache", Type: "temp", Spec: &schema.VolumeTemp{}},
				},
			),
			wantErr: `stage volume "cache" is declared more than once`,
		},
		{
			name: "卷名为空",
			stage: ciStage(
				[]*schema.Volume{{Type: "temp", Spec: &schema.VolumeTemp{}}},
			),
			wantErr: `stage volume #1: name must not be empty`,
		},
		{
			name: "空卷声明",
			stage: ciStage(
				[]*schema.Volume{nil},
			),
			wantErr: `stage volume #1 must not be empty`,
		},
		{
			name: "保留卷名",
			stage: ciStage(
				[]*schema.Volume{tempVolume("_workspace")},
			),
			wantErr: `stage volume name "_workspace" is reserved`,
		},
		{
			name: "docker_build 内部卷名",
			stage: ciStage(
				[]*schema.Volume{tempVolume(schema.VolumeNameSocketDir)},
			),
			wantErr: `stage volume name "socketDir" is reserved`,
		},
		{
			name: "不支持的卷类型",
			stage: ciStage(
				[]*schema.Volume{{Name: "cache", Type: "claim", Spec: &schema.VolumeClaim{}}},
			),
			wantErr: `stage volume "cache": volume type "claim" is not supported`,
		},
		{
			name: "递归校验 group 内的步骤",
			stage: ciStage(
				[]*schema.Volume{tempVolume("tmp")},
				&schema.Step{
					Name: "group",
					Type: "group",
					Spec: &schema.StepGroup{Steps: []*schema.Step{
						runStep("inner", &schema.Mount{Name: "missing", Path: "/data"}),
					}},
				},
			),
			wantErr: `step "inner": mount volume "missing" is not declared`,
		},
		{
			name: "plugin 步骤自带挂载同样校验",
			stage: ciStage(
				[]*schema.Volume{tempVolume("tmp")},
				&schema.Step{
					Name: "plugin",
					Type: "plugin",
					Spec: &schema.StepPlugin{
						Mount: []*schema.Mount{{Name: "missing", Path: "/data"}},
					},
				},
			),
			wantErr: `step "plugin": mount volume "missing" is not declared`,
		},
		{
			name: "没有挂载声明时通过",
			stage: ciStage(
				[]*schema.Volume{tempVolume("tmp")},
				&schema.Step{Name: "script", Type: "script", Spec: &schema.StepScript{}},
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateStageMounts(test.stage)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateStageMounts: unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateStageMounts: expected error containing %q, got nil", test.wantErr)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateStageMounts error = %q, want it to contain %q", err, test.wantErr)
			}
		})
	}
}

// 校验通过后挂载路径被归一化，下游（引擎、工作区冲突判断）使用与 Docker 一致的路径。
func TestValidateStageMountsNormalizesPath(t *testing.T) {
	mount := &schema.Mount{Name: "tmp", Path: "/a/../data/"}
	stage := ciStage(
		[]*schema.Volume{tempVolume("tmp")},
		runStep("build", mount),
	)

	if err := ValidateStageMounts(stage); err != nil {
		t.Fatalf("ValidateStageMounts: %v", err)
	}
	if mount.Path != "/data" {
		t.Fatalf("mount path = %q, want /data", mount.Path)
	}
}

func TestValidateStageMountsNonCIStage(t *testing.T) {
	stage := &schema.Stage{Type: "template", Spec: &schema.TemplateStage{}}
	if err := ValidateStageMounts(stage); err != nil {
		t.Fatalf("ValidateStageMounts: unexpected error: %v", err)
	}
}
