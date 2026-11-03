// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@chandao.com>

package config

import (
	"fmt"
	"os"
	"strings"
)

// Docker daemon 连接的 GitFox 专用变量及对应的标准 Docker 变量。
// Docker SDK 的 client.FromEnv 只识别标准 DOCKER_* 变量，因此 GITFOX_DOCKER_*
// 需要在配置加载后归一化到进程环境；后续代码（引擎、CLI 预检、runner IP）无需感知。
const (
	envDockerHost      = "DOCKER_HOST"
	envDockerTLSVerify = "DOCKER_TLS_VERIFY"
	envDockerCertPath  = "DOCKER_CERT_PATH"

	envGitfoxDockerHost      = "GITFOX_DOCKER_HOST"
	envGitfoxDockerTLSVerify = "GITFOX_DOCKER_TLS_VERIFY"
	envGitfoxDockerCertPath  = "GITFOX_DOCKER_CERT_PATH"
)

// dockerEnvAliases 定义 GITFOX_DOCKER_* 到标准 DOCKER_* 的映射。
var dockerEnvAliases = []struct {
	gitfox string
	docker string
}{
	{envGitfoxDockerHost, envDockerHost},
	{envGitfoxDockerTLSVerify, envDockerTLSVerify},
	{envGitfoxDockerCertPath, envDockerCertPath},
}

// NormalizeDockerEnv 将 GITFOX_DOCKER_* 归一化为 Docker SDK/CLI 识别的标准变量。
//
// 优先级：GITFOX_DOCKER_* > DOCKER_* > SDK 默认值（本机 unix socket）。
// godotenv.Load 不覆盖进程已有变量，因此 prod.env 与 docker -e / systemd / shell
// 可能同时提供两者；这里显式以 GitFox 专用变量为准，并在取值不一致时告警，
// 避免“配置了 GITFOX_DOCKER_HOST 却连了本机 socket”这类静默回退。
func NormalizeDockerEnv() {
	normalizeDockerEnv(os.Getenv, os.Setenv, func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "[config] warning: "+format+"\n", args...)
	})
}

func normalizeDockerEnv(getenv func(string) string, setenv func(string, string) error, warnf func(string, ...any)) {
	for _, alias := range dockerEnvAliases {
		gitfoxVal := strings.TrimSpace(getenv(alias.gitfox))
		if gitfoxVal == "" {
			continue
		}

		if current := strings.TrimSpace(getenv(alias.docker)); current != "" && current != gitfoxVal {
			warnf("%s=%q overrides %s=%q", alias.gitfox, gitfoxVal, alias.docker, current)
		}

		if err := setenv(alias.docker, gitfoxVal); err != nil {
			warnf("failed to set %s from %s: %v", alias.docker, alias.gitfox, err)
		}
	}
}
