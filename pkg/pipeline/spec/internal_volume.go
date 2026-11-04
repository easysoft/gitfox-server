// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package yaml

// Names of the volumes injected by the runner itself. User defined stage
// volumes must not use these names: the runner resolves a volume by
// declaration order, so a user volume with the same name would shadow the
// internal one.
const (
	VolumeNameWorkspace    = "_workspace"
	VolumeNameAddons       = "_addons"
	VolumeNameDockerSocket = "_docker_socket"
	// VolumeNameSocketDir is the shared docker socket directory injected for
	// docker_build steps (see compiler/step_docker_build.go).
	VolumeNameSocketDir = "socketDir"
)
