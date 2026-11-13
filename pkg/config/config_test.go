// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)

package config

import (
	"os"
	"testing"

	"github.com/kelseyhightower/envconfig"
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

func TestCiConfigMaxParallelStepsEnv(t *testing.T) {
	t.Run("默认 8", func(t *testing.T) {
		old, had := os.LookupEnv("GITFOX_CI_PARALLEL_STEPS")
		if err := os.Unsetenv("GITFOX_CI_PARALLEL_STEPS"); err != nil {
			t.Fatalf("os.Unsetenv: %v", err)
		}
		t.Cleanup(func() {
			if had {
				_ = os.Setenv("GITFOX_CI_PARALLEL_STEPS", old)
				return
			}
			_ = os.Unsetenv("GITFOX_CI_PARALLEL_STEPS")
		})

		var cfg CiConfig
		if err := envconfig.Process("", &cfg); err != nil {
			t.Fatalf("envconfig.Process: %v", err)
		}
		if cfg.MaxParallelSteps != DefaultMaxParallelSteps {
			t.Fatalf("MaxParallelSteps = %d, want %d", cfg.MaxParallelSteps, DefaultMaxParallelSteps)
		}
	})

	t.Run("环境变量覆盖", func(t *testing.T) {
		t.Setenv("GITFOX_CI_PARALLEL_STEPS", "3")
		var cfg CiConfig
		if err := envconfig.Process("", &cfg); err != nil {
			t.Fatalf("envconfig.Process: %v", err)
		}
		if cfg.MaxParallelSteps != 3 {
			t.Fatalf("MaxParallelSteps = %d, want 3", cfg.MaxParallelSteps)
		}
	})
}
