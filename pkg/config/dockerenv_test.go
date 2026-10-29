// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@chandao.com>

package config

import (
	"fmt"
	"maps"
	"os"
	"testing"
)

func TestNormalizeDockerEnv(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		want  map[string]string
		warns int
	}{
		{
			name: "gitfox host overrides standard host",
			env: map[string]string{
				envGitfoxDockerHost: "tcp://10.0.4.20:2375",
				envDockerHost:       "unix:///var/run/docker.sock",
			},
			want:  map[string]string{envDockerHost: "tcp://10.0.4.20:2375"},
			warns: 1,
		},
		{
			name: "gitfox host fills empty standard host",
			env:  map[string]string{envGitfoxDockerHost: " tcp://10.0.4.20:2375 "},
			want: map[string]string{envDockerHost: "tcp://10.0.4.20:2375"},
		},
		{
			name: "standard host kept when gitfox host unset",
			env:  map[string]string{envDockerHost: "tcp://legacy:2375"},
			want: map[string]string{envDockerHost: "tcp://legacy:2375"},
		},
		{
			name: "equal values do not warn",
			env: map[string]string{
				envGitfoxDockerHost: "tcp://10.0.4.20:2375",
				envDockerHost:       "tcp://10.0.4.20:2375",
			},
			want: map[string]string{envDockerHost: "tcp://10.0.4.20:2375"},
		},
		{
			name: "blank gitfox host falls back to standard host",
			env: map[string]string{
				envGitfoxDockerHost: "   ",
				envDockerHost:       "tcp://legacy:2375",
			},
			want: map[string]string{envDockerHost: "tcp://legacy:2375"},
		},
		{
			name: "tls aliases are mapped",
			env: map[string]string{
				envGitfoxDockerHost:      "tcp://10.0.4.20:2376",
				envGitfoxDockerTLSVerify: "1",
				envGitfoxDockerCertPath:  "/data/gitfox-community/docker-certs",
			},
			want: map[string]string{
				envDockerHost:      "tcp://10.0.4.20:2376",
				envDockerTLSVerify: "1",
				envDockerCertPath:  "/data/gitfox-community/docker-certs",
			},
		},
		{
			name: "unset aliases leave standard values untouched",
			env:  map[string]string{envDockerTLSVerify: "1", envDockerCertPath: "/certs"},
			want: map[string]string{envDockerTLSVerify: "1", envDockerCertPath: "/certs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := make(map[string]string, len(tt.env))
			maps.Copy(env, tt.env)

			var warnings []string
			normalizeDockerEnv(
				func(key string) string { return env[key] },
				func(key, value string) error {
					env[key] = value
					return nil
				},
				func(format string, args ...any) {
					warnings = append(warnings, fmt.Sprintf(format, args...))
				},
			)

			if len(warnings) != tt.warns {
				t.Fatalf("warnings = %v, want %d", warnings, tt.warns)
			}
			for key, want := range tt.want {
				if got := env[key]; got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestNormalizeDockerEnvIdempotent(t *testing.T) {
	env := map[string]string{envGitfoxDockerHost: "tcp://10.0.4.20:2375"}
	getenv := func(key string) string { return env[key] }
	setenv := func(key, value string) error {
		env[key] = value
		return nil
	}

	var warnings int
	warnf := func(string, ...any) { warnings++ }

	normalizeDockerEnv(getenv, setenv, warnf)
	normalizeDockerEnv(getenv, setenv, warnf)

	if warnings != 0 {
		t.Fatalf("unexpected warnings on repeated normalization: %d", warnings)
	}
	if got := env[envDockerHost]; got != "tcp://10.0.4.20:2375" {
		t.Fatalf("%s = %q, want %q", envDockerHost, got, "tcp://10.0.4.20:2375")
	}
}

func TestNormalizeDockerEnvSetenvFailureWarns(t *testing.T) {
	env := map[string]string{envGitfoxDockerHost: "tcp://10.0.4.20:2375"}
	var warnings int
	normalizeDockerEnv(
		func(key string) string { return env[key] },
		func(string, string) error { return fmt.Errorf("boom") },
		func(string, ...any) { warnings++ },
	)
	if warnings != 1 {
		t.Fatalf("warnings = %d, want 1", warnings)
	}
}

func TestNormalizeDockerEnvProcessEnv(t *testing.T) {
	t.Setenv(envGitfoxDockerHost, "tcp://10.0.4.20:2375")
	t.Setenv(envDockerHost, "unix:///var/run/docker.sock")

	NormalizeDockerEnv()

	if got := os.Getenv(envDockerHost); got != "tcp://10.0.4.20:2375" {
		t.Fatalf("%s = %q, want %q", envDockerHost, got, "tcp://10.0.4.20:2375")
	}
}
