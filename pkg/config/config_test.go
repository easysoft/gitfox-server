// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)

package config

import (
	"testing"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

func TestCiConfigMaxParallelStepsOrDefault(t *testing.T) {
	tests := []struct {
		name string
		in   uint
		want uint
	}{
		{name: "未配置时回退默认值", in: 0, want: DefaultMaxParallelSteps},
		{name: "显式配置", in: 12, want: 12},
		{name: "配置为 1", in: 1, want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := CiConfig{MaxParallelSteps: test.in}
			if got := cfg.MaxParallelStepsOrDefault(); got != test.want {
				t.Fatalf("MaxParallelStepsOrDefault() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestCiConfigMaxParallelStepsYaml(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want uint
	}{
		{name: "未配置时为 0，由 OrDefault 回退", in: "", want: 0},
		{name: "yaml 中配置", in: "maxParallelSteps: 5\n", want: 5},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var cfg CiConfig
			if err := yaml.Unmarshal([]byte(test.in), &cfg); err != nil {
				t.Fatalf("yaml.Unmarshal: %v", err)
			}
			if cfg.MaxParallelSteps != test.want {
				t.Fatalf("MaxParallelSteps = %d, want %d", cfg.MaxParallelSteps, test.want)
			}
		})
	}
}

func TestCiConfigMaxParallelStepsNotFromEnv(t *testing.T) {
	// 已迁移到 prod.yaml：旧的 GITFOX_CI_PARALLEL_STEPS 环境变量不应再覆盖 yaml 中的值。
	t.Setenv("GITFOX_CI_PARALLEL_STEPS", "3")

	var cfg CiConfig
	if err := yaml.Unmarshal([]byte("maxParallelSteps: 5\n"), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if err := envconfig.Process("", &cfg); err != nil {
		t.Fatalf("envconfig.Process: %v", err)
	}
	if cfg.MaxParallelSteps != 5 {
		t.Fatalf("MaxParallelSteps = %d, want 5 (env should be ignored)", cfg.MaxParallelSteps)
	}
}
