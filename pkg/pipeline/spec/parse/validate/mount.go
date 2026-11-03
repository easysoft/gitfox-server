// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

// Package validate provides schema-level validation for pipeline resources
// that cannot be expressed by the yaml unmarshaller alone.
package validate

import (
	"fmt"
	"strings"

	schema "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
)

// reservedVolumeNames are volume names used internally by the runner. A user
// defined volume with one of these names would shadow or conflict with the
// built-in volume of the same name.
var reservedVolumeNames = map[string]struct{}{
	"_workspace":     {},
	"_addons":        {},
	"_docker_socket": {},
}

// ValidateStageMounts checks the volume and mount declarations of a stage.
//
// It must run after plugins and templates have been resolved so that mounts
// inherited by the resolved step are validated as well. Mounts that reference
// an undeclared volume, use a relative path or duplicate a mount path are
// rejected instead of being silently dropped by the runner.
func ValidateStageMounts(stage *schema.Stage) error {
	if stage == nil {
		return nil
	}
	stageCI, ok := stage.Spec.(*schema.StageCI)
	if !ok {
		return nil
	}

	volumes := make(map[string]*schema.Volume, len(stageCI.Volumes))
	for _, volume := range stageCI.Volumes {
		if volume == nil {
			continue
		}
		if _, ok := reservedVolumeNames[volume.Name]; ok {
			return fmt.Errorf("stage volume name %q is reserved", volume.Name)
		}
		switch volume.Spec.(type) {
		case *schema.VolumeHost, *schema.VolumeTemp:
		default:
			return fmt.Errorf("stage volume %q: volume type %q is not supported", volume.Name, volume.Type)
		}
		volumes[volume.Name] = volume
	}

	for _, step := range stageCI.Steps {
		if err := validateStepMounts(step, volumes); err != nil {
			return err
		}
	}
	return nil
}

// validateStepMounts validates the mounts declared on a single step,
// descending into group and parallel steps.
func validateStepMounts(step *schema.Step, volumes map[string]*schema.Volume) error {
	if step == nil {
		return nil
	}
	switch v := step.Spec.(type) {
	case *schema.StepGroup:
		for _, child := range v.Steps {
			if err := validateStepMounts(child, volumes); err != nil {
				return err
			}
		}
		return nil
	case *schema.StepParallel:
		for _, child := range v.Steps {
			if err := validateStepMounts(child, volumes); err != nil {
				return err
			}
		}
		return nil
	}

	mounts := stepMounts(step)
	if len(mounts) == 0 {
		return nil
	}

	name := step.Name
	if name == "" {
		name = step.Id
	}

	paths := make(map[string]struct{}, len(mounts))
	for _, mount := range mounts {
		if mount == nil {
			return fmt.Errorf("step %q: mount must not be empty", name)
		}
		if mount.Path == "" || !strings.HasPrefix(mount.Path, "/") {
			return fmt.Errorf("step %q: mount path must be absolute: %q", name, mount.Path)
		}
		if _, ok := paths[mount.Path]; ok {
			return fmt.Errorf("step %q: duplicate mount path %q", name, mount.Path)
		}
		paths[mount.Path] = struct{}{}
		if mount.Name == "" {
			return fmt.Errorf("step %q: mount path %q requires a volume name", name, mount.Path)
		}
		if _, ok := volumes[mount.Name]; !ok {
			return fmt.Errorf("step %q: mount volume %q is not declared in stage volumes", name, mount.Name)
		}
	}
	return nil
}

// stepMounts returns the mounts declared on the step spec. Plugin steps are
// included because they are also compiled by the runner when a plugin
// definition resolves to a plugin step.
func stepMounts(step *schema.Step) []*schema.Mount {
	switch v := step.Spec.(type) {
	case *schema.StepPlugin:
		return v.Mount
	case schema.Mountable:
		return v.GetMounts()
	default:
		return nil
	}
}
