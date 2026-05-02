package skillrunner

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
)

type DockerPlanner struct {
	Policy   SandboxPolicy
	Runtimes map[string]RuntimeSpec
}

func NewDockerPlanner(policy SandboxPolicy, runtimes map[string]RuntimeSpec) *DockerPlanner {
	if policy.WorkingDir == "" {
		policy = DefaultSandboxPolicy()
	}
	if len(runtimes) == 0 {
		runtimes = DefaultRuntimeRegistry()
	}
	return &DockerPlanner{
		Policy:   policy,
		Runtimes: runtimes,
	}
}

func (p *DockerPlanner) Plan(ctx context.Context, req PlanRequest) (*SandboxPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.Bundle == nil {
		return nil, fmt.Errorf("bundle is required")
	}
	if p.Policy.RequireApprovedReview && !req.Review.Approved(req.Bundle.Digest) {
		return nil, ErrReviewRequired
	}
	if req.HostSkillDir == "" {
		return nil, fmt.Errorf("host skill dir is required")
	}
	if req.HostScratchDir == "" {
		return nil, fmt.Errorf("host scratch dir is required")
	}
	if !filepath.IsAbs(req.HostSkillDir) {
		return nil, fmt.Errorf("host skill dir must be absolute: %s", req.HostSkillDir)
	}
	if !filepath.IsAbs(req.HostScratchDir) {
		return nil, fmt.Errorf("host scratch dir must be absolute: %s", req.HostScratchDir)
	}

	runtimeSpec, ok := p.Runtimes[req.Bundle.Manifest.Spec.Runtime]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedRuntime, req.Bundle.Manifest.Spec.Runtime)
	}

	layout := IOLayout{
		ScratchDir: p.Policy.ScratchDir,
		InputPath:  path.Join(p.Policy.ScratchDir, "input", "request.json"),
		OutputPath: path.Join(p.Policy.ScratchDir, "output", "response.json"),
		TmpDir:     p.Policy.TmpDir,
	}

	plan := &SandboxPlan{
		Runtime:    runtimeSpec,
		Image:      runtimeSpec.Image,
		Command:    []string{runtimeSpec.Executable, path.Join(p.Policy.WorkingDir, req.Bundle.Manifest.Spec.Entrypoint)},
		WorkingDir: p.Policy.WorkingDir,
		User:       p.Policy.RunAsUser,
		Environment: map[string]string{
			"SUB2API_SKILL_INPUT":  layout.InputPath,
			"SUB2API_SKILL_OUTPUT": layout.OutputPath,
			"TMPDIR":               layout.TmpDir,
		},
		Mounts: []Mount{
			{
				Source:   req.HostSkillDir,
				Target:   p.Policy.WorkingDir,
				ReadOnly: true,
			},
			{
				Source:   req.HostScratchDir,
				Target:   p.Policy.ScratchDir,
				ReadOnly: false,
			},
		},
		TmpFS: []TmpFS{
			{
				Target: p.Policy.TmpDir,
				SizeMB: p.Policy.Limits.TmpMB,
				Mode:   "1777",
			},
		},
		Layout:          layout,
		ReadOnlyRootFS:  p.Policy.ReadOnlyRootFS,
		NetworkMode:     "none",
		SecurityOptions: []string{"no-new-privileges:true"},
		CapDrop:         []string{"ALL"},
		ResourceLimits:  p.Policy.Limits,
	}
	if !p.Policy.DisableNetwork {
		plan.NetworkMode = "bridge"
	}

	return plan, nil
}
