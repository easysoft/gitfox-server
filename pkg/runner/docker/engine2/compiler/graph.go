// @copyright   Copyright 2009-2026 渠成极客软件（青岛）有限公司(Qucheng Geek Software (Qingdao) Co., Ltd. www.gitfox.net)
// @license     ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
// @author      mazhiyuan <mazhiyuan@gitfox.com>

package compiler

import (
	"context"
	"fmt"

	harness "github.com/easysoft/gitfox-community/pkg/pipeline/spec"
	"github.com/easysoft/gitfox-community/pkg/runner/docker/engine2/engine"

	"github.com/drone/runner-go/environ"
)

type nodeKind int

const (
	nodeLeaf nodeKind = iota
	nodeSerial
	nodeParallel
)

// compiledNode mirrors one node of the YAML step tree. parallel / group
// nodes do not produce engine steps; they only describe how their
// descendants are wired.
type compiledNode struct {
	kind     nodeKind
	steps    []*engine.Step  // nodeLeaf
	children []*compiledNode // nodeSerial / nodeParallel
}

// compileContext carries the shared compilation state used by prepareSteps.
type compileContext struct {
	ctx         context.Context
	stage       *harness.Stage
	spec        *engine.Spec
	args        Args
	inputs      map[string]interface{}
	envs        map[string]string
	stageLabels map[string]string
	mount       *engine.VolumeMount
}

// compileNode recursively compiles a YAML step tree. parallel / group nodes
// only produce child nodes; leaves are converted and post-processed here.
func (c *CompilerImpl) compileNode(
	cc *compileContext,
	src *harness.Step,
	parentWhen *harness.When,
) (*compiledNode, error) {
	switch v := src.Spec.(type) {
	case *harness.StepGroup:
		when := combineWhen(parentWhen, src.When)
		node := &compiledNode{kind: nodeSerial}
		for _, child := range v.Steps {
			n, err := c.compileNode(cc, child, when)
			if err != nil {
				return nil, err
			}
			node.children = append(node.children, n)
		}
		return node, nil

	case *harness.StepParallel:
		when := combineWhen(parentWhen, src.When)
		node := &compiledNode{kind: nodeParallel}
		for _, child := range v.Steps {
			n, err := c.compileNode(cc, child, when)
			if err != nil {
				return nil, err
			}
			node.children = append(node.children, n)
		}
		return node, nil

	default:
		steps, err := c.convertStep(cc.stage, src, cc.args)
		if err != nil {
			return nil, err
		}
		c.prepareSteps(cc, steps, src, parentWhen)
		// Preserve YAML depth-first order for the DB SN / UI display.
		cc.spec.Steps = append(cc.spec.Steps, steps...)
		return &compiledNode{kind: nodeLeaf, steps: steps}, nil
	}
}

// prepareSteps applies the per-leaf post-processing that used to live in the
// Compile loop: secret registration, environment, mounts, labels, the build
// ID suffix and the when/run policy.
func (c *CompilerImpl) prepareSteps(
	cc *compileContext,
	steps []*engine.Step,
	src *harness.Step,
	parentWhen *harness.When,
) {
	for _, step := range steps {
		// Register secrets accessed by this step's expressions so they are
		// masked in the logs.
		cc.inputs["secrets"] = map[string]interface{}{
			"get": func(name string) string {
				s, ok := c.findSecret(context.Background(), cc.args, name)
				if ok {
					step.Secrets = append(step.Secrets, &engine.Secret{
						Name: name,
						Data: []byte(s),
						Mask: true,
					})
				}
				return s
			},
		}

		step.Envs = environ.Combine(cc.envs, step.Envs)
		appendWorkspaceMount(cc.ctx, step, cc.mount)
		step.Labels = cc.stageLabels
		step.ExtraHosts = append(step.ExtraHosts, c.ExtraHosts...)
		step.Privileged = c.isPrivileged(step)
		step.ID = fmt.Sprintf("%s_%d", step.ID, cc.args.Build.ID)

		// 父级 group/parallel 的 when 与叶子自身的 when 合并后求值。
		applyStepRunPolicy(cc.ctx, step, combineWhen(parentWhen, src.When), cc.inputs)
	}
}

// wireNode assigns DependsOn edges from the compiled tree. entry is the set
// of IDs the current node must wait for; the return value is the set of IDs
// that downstream nodes must wait for.
func wireNode(node *compiledNode, entry []string) (exit []string) {
	switch node.kind {
	case nodeLeaf:
		if len(node.steps) == 0 {
			return entry
		}
		deps := entry
		for _, step := range node.steps {
			step.DependsOn = dedupe(deps)
			// 一个叶子产生多个 engine step 时内部串行（pre → main → post）。
			deps = []string{step.ID}
		}
		return deps

	case nodeSerial: // group / 顶层列表
		deps := entry
		for _, child := range node.children {
			deps = wireNode(child, deps)
		}
		return deps

	case nodeParallel:
		var exits []string
		for _, child := range node.children {
			exits = append(exits, wireNode(child, entry)...)
		}
		if len(exits) == 0 {
			return entry
		}
		return dedupe(exits)
	}

	return entry
}

// dedupe removes duplicate IDs while preserving order.
func dedupe(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
