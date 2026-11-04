// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

// Package validate provides schema-level validation for pipeline resources
// that cannot be expressed by the yaml unmarshaller alone.
package validate

import (
	"fmt"
	"path"
	"strings"

	schema "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
)

// reservedVolumeNames are volume names used internally by the runner. A user
// defined volume with one of these names would shadow or conflict with the
// built-in volume of the same name. Keep this in sync with the
// schema.VolumeName* constants used by the compiler.
var reservedVolumeNames = map[string]struct{}{
	schema.VolumeNameWorkspace:    {},
	schema.VolumeNameAddons:       {},
	schema.VolumeNameDockerSocket: {},
	schema.VolumeNameSocketDir:    {},
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
	for i, volume := range stageCI.Volumes {
		if volume == nil {
			return fmt.Errorf("stage volume #%d must not be empty", i+1)
		}
		if volume.Name == "" {
			return fmt.Errorf("stage volume #%d: name must not be empty", i+1)
		}
		if _, ok := reservedVolumeNames[volume.Name]; ok {
			return fmt.Errorf("stage volume name %q is reserved", volume.Name)
		}
		// The runtime resolves a volume by returning the first declaration
		// with a matching name, so duplicated declarations would make the
		// runtime use a different volume than the one validated here.
		if _, ok := volumes[volume.Name]; ok {
			return fmt.Errorf("stage volume %q is declared more than once", volume.Name)
		}
		switch vv := volume.Spec.(type) {
		case *schema.VolumeHost:
			// An empty host path is compiled into an empty bind source, which
			// Docker only rejects when the container is created. Reject it here
			// so the error names the offending stage volume.
			if vv.Path == "" {
				return fmt.Errorf("stage volume %q: host path must not be empty", volume.Name)
			}
		case *schema.VolumeTemp:
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
		// Docker normalizes the mount target before it checks for duplicate
		// mount points, so "/data/", "/a/../data" and "/data" are the same
		// target. Normalize here and write the result back so that the
		// duplicate check and all downstream consumers (the workspace mount
		// conflict check in particular) use the same path Docker will use.
		cleanPath := path.Clean(mount.Path)
		// path.Clean collapses paths that climb above the root ("/..",
		// "/a/../..") to "/", which is absolute and would otherwise pass
		// validation. Mounting a volume at the container root is rejected by
		// Docker only at container creation time, with an error that no longer
		// mentions the step, so reject it here.
		if cleanPath == "/" {
			return fmt.Errorf("step %q: mount path must not be the container root: %q", name, mount.Path)
		}
		if _, ok := paths[cleanPath]; ok {
			return fmt.Errorf("step %q: duplicate mount path %q", name, cleanPath)
		}
		paths[cleanPath] = struct{}{}
		mount.Path = cleanPath
		if mount.Name == "" {
			return fmt.Errorf("step %q: mount path %q requires a volume name", name, cleanPath)
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
