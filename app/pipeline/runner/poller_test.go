// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package runner

import (
	"testing"

	"github.com/easysoft/gitfox-community/pkg/utils/netutil"
)

func TestDockerRunnerIP(t *testing.T) {
	tests := []struct {
		name       string
		dockerHost string
		want       string
		useLocal   bool
	}{
		{name: "unset", dockerHost: "", useLocal: true},
		{name: "blank", dockerHost: "   ", useLocal: true},
		{name: "tcp", dockerHost: "tcp://10.0.4.20:2375", want: "10.0.4.20"},
		{name: "tcp without port", dockerHost: "tcp://10.0.4.20", want: "10.0.4.20"},
		{name: "https", dockerHost: "https://docker.example.com:2376", want: "docker.example.com"},
		{name: "ipv6", dockerHost: "tcp://[fd00::1]:2375", want: "fd00::1"},
		{name: "unix socket", dockerHost: "unix:///var/run/docker.sock", useLocal: true},
		{name: "npipe", dockerHost: "npipe:////./pipe/docker_engine", useLocal: true},
		{name: "no scheme", dockerHost: "10.0.4.20:2375", useLocal: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DOCKER_HOST", tt.dockerHost)

			want := tt.want
			if tt.useLocal {
				want = netutil.LocalIP()
			}

			if got := dockerRunnerIP(); got != want {
				t.Fatalf("dockerRunnerIP() = %q, want %q", got, want)
			}
		})
	}
}
