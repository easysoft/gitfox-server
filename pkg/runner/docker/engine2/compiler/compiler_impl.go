// Copyright 2019 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package compiler

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/easysoft/gitfox-community/app/constant"
	"github.com/easysoft/gitfox-community/app/service/pipeline"
	"github.com/easysoft/gitfox-community/pkg/logger"
	harness "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/engine"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/inputs"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/internal/docker/image"
	"github.com/easysoft/gitfox-community/pkg/runner/extend"

	"github.com/dchest/uniuri"
	"github.com/drone/drone-go/drone"
	"github.com/drone/runner-go/clone"
	"github.com/drone/runner-go/environ"
	"github.com/drone/runner-go/environ/provider"
	"github.com/drone/runner-go/labels"
	"github.com/drone/runner-go/manifest"
	"github.com/drone/runner-go/secret"
)

// random generator function
var random = func() string {
	return "drone-" + uniuri.NewLen(20)
}

// Resources defines container resource constraints. These
// constraints are per-container, not per-pipeline.
type Resources struct {
	Memory     int64
	MemorySwap int64
	CPUQuota   int64
	CPUPeriod  int64
	CPUShares  int64
	CPUSet     []string
	ShmSize    int64
}

// Tmate defines tmate settings.
type Tmate struct {
	Image          string
	Enabled        bool
	Server         string
	Port           string
	RSA            string
	ED25519        string
	AuthorizedKeys string
}

// CompilerImpl compiles the Yaml configuration file to an
// intermediate representation optimized for simple execution.
type CompilerImpl struct {
	// Environ provides a set of environment variables that
	// should be added to each pipeline step by default.
	Environ provider.Provider

	// Labels provides a set of labels that should be added
	// to each container by default.
	Labels map[string]string

	// Privileged provides a list of docker images that
	// are always privileged.
	Privileged []string

	// Networks provides a set of networks that should be
	// attached to each pipeline container.
	Networks []string

	// NetworkOpts provides a set of network options that
	// are used when creating the docker network.
	NetworkOpts map[string]string

	// NetrcCloneOnly instructs the compiler to only inject
	// the netrc file into the clone step.
	NetrcCloneOnly bool

	// Volumes provides a set of volumes that should be
	// mounted to each pipeline container.
	Volumes map[string]string

	// ExtraHosts provides a set of hostname mappings that
	// should be setup for each pipeline container.
	ExtraHosts []string

	// Clone overrides the default plugin image used
	// when cloning a repository.
	Clone string

	// Resources provides global resource constraints
	// applies to pipeline containers.
	Resources Resources

	// Tmate provides global configuration options for tmate
	// live debugging.
	Tmate Tmate

	// Secret returns a named secret value that can be injected
	// into the pipeline step.
	Secret secret.Provider

	// Mount is an optional field that overrides the default
	// workspace volume and mounts to the host path
	Mount string

	PipelineSvc *pipeline.Interface
}

// Compile compiles the configuration file.
func (c *CompilerImpl) Compile(ctx context.Context, args Args) (*engine.Spec, error) {
	// extract the pipeline resource
	pipeline, ok := args.Config.Spec.(*harness.Pipeline)
	if !ok {
		return nil, errors.New("pipeline resource not found")
	}

	// extract the pipeline stage and stage spec
	var stage *harness.Stage
	for _, resource := range pipeline.Stages {
		// FIXME: need to match to stage.uid (which does not exist yet)
		if resource.Name == args.Stage.Name {
			stage = resource
			break
		}
	}
	if stage == nil {
		return nil, errors.New("stage resource not found")
	}
	stageSpec, ok := stage.Spec.(*harness.StageCI)
	if !ok {
		return nil, errors.New("ci stage resource not found")
	}

	// extract the pipeline stage platform
	platform_ := new(harness.Platform)
	if v := stageSpec.Platform; v != nil {
		platform_ = v
	}

	// extract the pipeline options
	options_ := new(harness.Default)
	if v := pipeline.Options; v != nil {
		options_ = v
	}

	// extract the clone. use the stage settings, else
	// fallback to the pipeline-level clone settings.
	clone_ := new(harness.Clone)
	if v := options_.Clone; v != nil {
		clone_ = v
	}
	if v := stageSpec.Clone; v != nil {
		clone_ = &harness.Clone{
			Depth:    v.Depth,
			Disabled: v.Disabled,
			Insecure: v.Insecure,
			Trace:    v.Trace,
		}
	}

	// create system labels
	stageLabels := labels.Combine(
		c.Labels,
		// labels.FromRepo(args.Repo),
		labels.FromBuild(args.Build),
		labels.FromStage(args.Stage),
		labels.FromSystem(args.System),
		labels.WithTimeout(args.Repo),
	)

	// create the workspace mount
	mount := &engine.VolumeMount{
		Name: "_workspace",
		Path: "/gitfox", // FIXME: handle windows. c://gitfox
	}

	// create the workspace volume
	volume := &engine.Volume{
		EmptyDir: &engine.VolumeEmptyDir{
			ID:     random(),
			Name:   mount.Name,
			Labels: stageLabels,
		},
	}

	// if the repository is mounted from a local volume,
	// we should replace the data volume with a host machine
	// volume declaration.
	if c.Mount != "" {
		volume.EmptyDir = nil
		volume.HostPath = &engine.VolumeHostPath{
			ID:     random(),
			Name:   mount.Name,
			Path:   c.Mount,
			Labels: stageLabels,
		}
	}

	spec := &engine.Spec{
		Network: engine.Network{
			ID:      random(),
			Labels:  stageLabels,
			Options: c.NetworkOpts,
		},
		Platform: engine.Platform{
			OS:   platform_.Os,
			Arch: platform_.Arch,
			// Variant: platform_.Variant,
			// Version: platform_.Version,
		},
		Volumes: []*engine.Volume{volume},
	}

	// list the global environment variables
	// TODO we can probably deprecate and remove this
	ctx = extend.WithContext(ctx, stage, pipeline)
	globals, _ := c.Environ.List(ctx, &provider.Request{
		Build: args.Build,
		Repo:  args.Repo,
	})

	// create the default environment variables.
	envs := environ.Combine(
		provider.ToMap(
			provider.FilterUnmasked(globals),
		),
		args.Build.Params,
		options_.Envs,  // environment from pipeline
		stageSpec.Envs, // environment from stage
		environ.Proxy(),
		environ.System(args.System),
		// environ.Repo(args.Repo),
		environ.Build(args.Build),
		environ.Stage(args.Stage),
		environ.Netrc(args.Netrc),
		clone.Environ(clone.Config{
			SkipVerify: clone_.Insecure,
			Trace:      clone_.Trace,
			User: clone.User{
				Name:  args.Build.AuthorName,
				Email: args.Build.AuthorEmail,
			},
		}),
		extend.Envs(args.Stage, args.Repo),
	)

	// create network reference variables
	envs["GITFOX_DOCKER_NETWORK_ID"] = spec.Network.ID

	// create the workspace variables
	envs["GITFOX_WORKSPACE"] = "/gitfox"
	envs["GITFOX_WORKSPACE_BASE"] = "/gitfox"
	envs["GITFOX_WORKSPACE_PATH"] = ""

	// create volume reference variables
	if volume.EmptyDir != nil {
		envs["GITFOX_DOCKER_VOLUME_ID"] = volume.EmptyDir.ID
	} else {
		envs["GITFOX_DOCKER_VOLUME_PATH"] = volume.HostPath.Path
	}

	// create tmate variables
	if c.Tmate.Server != "" {
		envs["GITFOX_TMATE_HOST"] = c.Tmate.Server
		envs["GITFOX_TMATE_PORT"] = c.Tmate.Port
		envs["GITFOX_TMATE_FINGERPRINT_RSA"] = c.Tmate.RSA
		envs["GITFOX_TMATE_FINGERPRINT_ED25519"] = c.Tmate.ED25519

		if c.Tmate.AuthorizedKeys != "" {
			envs["GITFOX_TMATE_AUTHORIZED_KEYS"] = c.Tmate.AuthorizedKeys
		}
	}

	// create the .netrc environment variables if not
	// explicitly disabled
	if c.NetrcCloneOnly == false {
		envs = environ.Combine(envs, environ.Netrc(args.Netrc))
	}

	// create the clone step
	if clone_.Disabled == false {
		step := createClone(platform_, clone_)
		step.ID = random()
		step.Envs = environ.Combine(envs, step.Envs)
		step.Envs["GIT_HTTP_USER_AGENT"] = constant.UserAgentPipeline
		step.ExtraHosts = append(step.ExtraHosts, c.ExtraHosts...)
		step.WorkingDir = "/gitfox"
		step.Labels = stageLabels
		step.Pull = engine.PullIfNotExists
		step.Volumes = append(step.Volumes, mount)
		spec.Steps = append(spec.Steps, step)

		// if the clone image is customized, override
		// the default image.
		if c.Clone != "" {
			step.Image = c.Clone
		}

		// if the repository is mounted from a local
		// volume we should disable cloning.
		if c.Mount != "" {
			step.RunPolicy = engine.RunNever
		}
	}

	// collate the input params
	inputs := map[string]interface{}{
		// "repo":   inputs.Repo(args.Repo),
		"build": inputs.Build(args.Build),
		// parameters 与运行期表达式上下文保持一致（流水线 inputs 默认值 + 触发参数）
		"parameters": inputs.Inputs(pipeline.Inputs, args.Build.Params),
		"inputs":     inputs.Inputs(pipeline.Inputs, args.Build.Params),
		"global":     argsGlobalParams(args),
		"secrets": map[string]interface{}{
			"get": func(name string) string {
				s, _ := c.findSecret(
					context.Background(), args, name)
				return s
			},
		},
		"env": envs,
	}

	// create steps
	for _, src := range stageSpec.Steps {
		steps_, err := c.convertStep(stage, src, args)
		if err != nil {
			return nil, err
		}
		if len(steps_) != 0 {
			for _, step_ := range steps_ {
				// add secret function.
				// register secrets with the step so we know
				// to mask them.
				inputs["secrets"] = map[string]interface{}{
					"get": func(name string) string {
						s, ok := c.findSecret(context.Background(), args, name)
						if ok {
							// regsiter secret so we know to mask
							step_.Secrets = append(step_.Secrets, &engine.Secret{
								Name: name,
								Data: []byte(s),
								Mask: true,
							})
						}
						return s
					},
				}

				step_.Envs = environ.Combine(envs, step_.Envs)
				step_.Volumes = append(step_.Volumes, mount)
				step_.Labels = stageLabels
				step_.ExtraHosts = append(step_.ExtraHosts, c.ExtraHosts...)
				step_.Privileged = c.isPrivileged(step_)
				step_.ID = fmt.Sprintf("%s_%d", step_.ID, args.Build.ID)

				// 根据 step 级 when 条件计算运行策略；条件无法求值时该步骤失败。
				applyStepRunPolicy(ctx, step_, src.When, inputs)

				spec.Steps = append(spec.Steps, step_)
			}
		}
	}

	appendFinishFunc := func() {
		step := createStageClean()
		step.ID = random()
		step.Envs = environ.Combine(envs, step.Envs)
		step.ExtraHosts = append(step.ExtraHosts, c.ExtraHosts...)
		step.WorkingDir = "/gitfox"
		step.Labels = stageLabels
		step.Pull = engine.PullAlways
		step.Volumes = append(step.Volumes, mount)
		setupScript(step, "clean_stage", "linux")
		spec.Steps = append(spec.Steps, step)
	}
	if platform_.Os != "windows" {
		appendFinishFunc()
	}

	// create internal steps if build running in debug mode
	if c.Tmate.Enabled && args.Build.Debug && platform_.Os != "windows" {
		// first we need to add an internal setup step to the pipeline
		// to copy over the tmate binary. Internal steps are not visible
		// to the end user.
		spec.Internal = append(spec.Internal, &engine.Step{
			ID:         random(),
			Labels:     stageLabels,
			Pull:       engine.PullIfNotExists,
			Image:      image.Expand(c.Tmate.Image),
			Entrypoint: []string{"/bin/drone-runner-docker"},
			Command:    []string{"copy"},
			Network:    "none",
		})

		// next we create a temporary volume to share the tmate binary
		// with the pipeline containers.
		for _, step := range append(spec.Steps, spec.Internal...) {
			step.Volumes = append(step.Volumes, &engine.VolumeMount{
				Name: "_addons",
				Path: "/usr/drone/bin",
			})
		}
		spec.Volumes = append(spec.Volumes, &engine.Volume{
			EmptyDir: &engine.VolumeEmptyDir{
				ID:     random(),
				Name:   "_addons",
				Labels: stageLabels,
			},
		})
	}

	if isGraph(spec) == false {
		configureSerial(spec)
	} else if clone_.Disabled == false {
		configureCloneDeps(spec)
	} else if clone_.Disabled == true {
		removeCloneDeps(spec)
	}

	//
	// TODO re-enable, find matching secrets and inject
	//
	// for _, step := range spec.Steps {
	// 	for _, s := range step.Secrets {
	// 		secret, ok := c.findSecret(ctx, args, s.Name)
	// 		if ok {
	// 			s.Data = []byte(secret)
	// 		}
	// 	}
	// }

	// get registry credentials from registry plugins
	//creds, err := c.Registry.List(ctx, &registry.Request{
	//	Repo:  args.Repo,
	//	Build: args.Build,
	//})
	//if err != nil {
	//	return nil, err
	//}

	//
	// TODO re-enable, get registry credentials from secrets
	//
	// // get registry credentials from secrets
	// for _, name := range pipeline.PullSecrets {
	// 	secret, ok := c.findSecret(ctx, args, name)
	// 	if ok {
	// 		parsed, err := auths.ParseString(secret)
	// 		if err == nil {
	// 			creds = append(parsed, creds...)
	// 		}
	// 	}
	// }

	//for _, step := range append(spec.Steps, spec.Internal...) {
	//STEPS:
	//	for _, cred := range creds {
	//		if image.MatchHostname(step.Image, cred.Address) {
	//			step.Auth = &engine.Auth{
	//				Address:  cred.Address,
	//				Username: cred.Username,
	//				Password: cred.Password,
	//			}
	//			break STEPS
	//		}
	//	}
	//}

	// append global resource limits to steps
	for _, step := range spec.Steps {
		// the resource limits defined in the yaml currently
		// take precedence over global values. This is something
		// we should re-think in a future release.
		if step.MemSwapLimit == 0 {
			step.MemSwapLimit = c.Resources.MemorySwap
		}
		if step.MemLimit == 0 {
			step.MemLimit = c.Resources.Memory
		}
		if step.ShmSize == 0 {
			step.ShmSize = c.Resources.ShmSize
		}
		step.CPUPeriod = c.Resources.CPUPeriod
		step.CPUQuota = c.Resources.CPUQuota
		step.CPUShares = c.Resources.CPUShares
		step.CPUSet = c.Resources.CPUSet
	}

	// append global networks to the steps.
	// append step labels to steps.
	for n, step := range spec.Steps {
		step.Networks = append(step.Networks, c.Networks...)
		step.Labels = labels.Combine(step.Labels, labels.FromStep(&drone.Step{
			Number: n + 1,
			Name:   step.Name,
		}))
	}

	// append global volumes to the steps.
	for k, v := range c.Volumes {
		id := random()
		ro := strings.HasSuffix(v, ":ro")
		v = strings.TrimSuffix(v, ":ro")
		volume := &engine.Volume{
			HostPath: &engine.VolumeHostPath{
				ID:       id,
				Name:     id,
				Path:     k,
				ReadOnly: ro,
			},
		}
		spec.Volumes = append(spec.Volumes, volume)
		for _, step := range spec.Steps {
			mount := &engine.VolumeMount{
				Name: id,
				Path: v,
			}
			step.Volumes = append(step.Volumes, mount)
		}
	}

	// append volumes
	for _, v := range stageSpec.Volumes {
		id := random()
		src := new(engine.Volume)

		switch vv := v.Spec.(type) {
		case *harness.VolumeHost:
			src.HostPath = &engine.VolumeHostPath{
				ID:   id,
				Name: v.Name,
				Path: vv.Path,
			}
			spec.Volumes = append(spec.Volumes, src)
		case *harness.VolumeTemp:
			src.EmptyDir = &engine.VolumeEmptyDir{
				ID:        id,
				Name:      v.Name,
				Medium:    vv.Medium,
				SizeLimit: int64(vv.Limit),
				Labels:    stageLabels,
			}
			spec.Volumes = append(spec.Volumes, src)
		}
	}

	return spec, nil
}

// applyStepRunPolicy 根据 step 的 when 条件设置容器运行策略。
//
// 表达式无法求值时不让整份配置编译失败：把该步骤置为不执行（RunNever），失败原因写入
// WhenError 交给运行时——运行时会把这步标记为失败，同一 stage 的其他步骤照常调度。
func applyStepRunPolicy(ctx context.Context, step *engine.Step, when *harness.When, inputs map[string]interface{}) {
	if when == nil {
		return
	}

	code := strings.TrimSpace(when.Eval)
	if code == "" {
		return
	}

	// 记录原始 when 条件，运行时据此输出执行/跳过日志
	step.WhenCondition = code

	// 直接填写运行策略值（always/onSuccess/onFailure）时无需做表达式求值。
	if slices.Contains(constant.RunPolicyList, code) {
		switch constant.RunPolicy(code) {
		case constant.RunPolicyAlways:
			step.RunPolicy = engine.RunAlways
		case constant.RunPolicyOnFailure:
			step.RunPolicy = engine.RunOnFailure
		case constant.RunPolicyOnSuccess:
			step.RunPolicy = engine.RunOnSuccess
		}
		return
	}

	onSuccess, onFailure, err := evalif(code, inputs)
	if err != nil {
		logger.GetLogger(ctx).Errorf("[pipeline]: cannot evaluate step when condition %q: %v", code, err)
		step.RunPolicy = engine.RunNever
		step.WhenError = fmt.Sprintf(harness.WhenEvalFailedStepTmpl, code, err)
		return
	}

	switch {
	case onSuccess && onFailure:
		step.RunPolicy = engine.RunAlways
	case onSuccess:
		step.RunPolicy = engine.RunOnSuccess
	case onFailure:
		step.RunPolicy = engine.RunOnFailure
	default:
		// (false, false)：条件不成立，跳过该 step
		step.RunPolicy = engine.RunNever
	}
}

// isPrivileged checks whether the privileged flag should be set to true for the step or not.
func (c *CompilerImpl) isPrivileged(step *engine.Step) bool {
	if step.Privileged {
		return true
	}
	// privileged-by-default containers are only
	// enabled for plugins steps that do not define
	// commands, command, or entrypoint.
	if len(step.Command) > 0 {
		return false
	}
	if len(step.Entrypoint) > 0 {
		return false
	}

	// privileged-by-default mode is disabled if the
	// pipeline step attempts to use an environment
	// variable restricted for internal use only.
	if isRestrictedVariable(step.Envs) {
		return false
	}
	// if the container image matches any image
	// in the whitelist, return true.
	for _, img := range c.Privileged {
		a := img
		b := step.Image
		if image.Match(a, b) {
			return true
		}
	}
	return false
}

// argsGlobalParams 返回供全局命名空间使用的全局参数，
// 未提供时返回空 map，避免表达式求值取到 nil。
func argsGlobalParams(args Args) map[string]interface{} {
	if args.Global == nil {
		return map[string]interface{}{}
	}
	return args.Global
}

// helper function attempts to find and return the named secret.
// from the secret provider.
func (c *CompilerImpl) findSecret(ctx context.Context, args Args, name string) (s string, ok bool) {
	if name == "" {
		return
	}

	// source secrets from the global secret provider
	// and the repository secret provider.
	provider := secret.Combine(
		args.Secret,
		c.Secret,
	)

	// TODO return an error to the caller if the provider
	// returns an error.
	found, _ := provider.Find(ctx, &secret.Request{
		Name:  name,
		Build: args.Build,
		Repo:  args.Repo,
		Conf:  &manifest.Manifest{}, // TODO remove me
	})
	if found == nil {
		return
	}
	return found.Data, true
}
