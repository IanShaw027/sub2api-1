package skillrunner

import (
	"context"
	"strings"
	"sync"
	"time"
)

const (
	ManifestFile       = "skill.yaml"
	ManifestAPIVersion = "skill.sub2api.io/v1alpha1"
	ManifestKind       = "Skill"

	SkillTypeScript    = "script"
	ProtocolJSONFileV1 = "json-file-v1"

	RuntimePython311 = "python3.11"
	RuntimeNode20    = "node20"
)

type ReviewStatus string

const (
	ReviewStatusPending  ReviewStatus = "pending"
	ReviewStatusApproved ReviewStatus = "approved"
	ReviewStatusRejected ReviewStatus = "rejected"
)

type Manifest struct {
	APIVersion string       `json:"apiVersion" yaml:"apiVersion"`
	Kind       string       `json:"kind" yaml:"kind"`
	Metadata   ManifestMeta `json:"metadata" yaml:"metadata"`
	Spec       ScriptSpec   `json:"spec" yaml:"spec"`
}

type ManifestMeta struct {
	Name        string `json:"name" yaml:"name"`
	Version     string `json:"version" yaml:"version"`
	DisplayName string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type ScriptSpec struct {
	Type       string `json:"type" yaml:"type"`
	Runtime    string `json:"runtime" yaml:"runtime"`
	Entrypoint string `json:"entrypoint" yaml:"entrypoint"`
	Protocol   string `json:"protocol" yaml:"protocol"`
}

type BundleFile struct {
	Path string `json:"path"`
	Size uint64 `json:"size"`
}

type ArchiveConstraints struct {
	MaxArchiveBytes      int64
	MaxManifestBytes     uint64
	MaxUncompressedBytes uint64
	MaxFileBytes         uint64
	MaxFiles             int
	MaxPathBytes         int
}

func DefaultArchiveConstraints() ArchiveConstraints {
	return ArchiveConstraints{
		MaxArchiveBytes:      16 << 20,
		MaxManifestBytes:     64 << 10,
		MaxUncompressedBytes: 16 << 20,
		MaxFileBytes:         4 << 20,
		MaxFiles:             128,
		MaxPathBytes:         160,
	}
}

type Bundle struct {
	Digest                 string             `json:"digest"`
	Manifest               Manifest           `json:"manifest"`
	Files                  []BundleFile       `json:"files"`
	ArchiveBytes           int64              `json:"archiveBytes"`
	TotalUncompressedBytes uint64             `json:"totalUncompressedBytes"`
	Constraints            ArchiveConstraints `json:"constraints"`
}

type ReviewGate struct {
	ArtifactDigest string       `json:"artifactDigest"`
	Status         ReviewStatus `json:"status"`
	ApprovedAt     *time.Time   `json:"approvedAt,omitempty"`
	ApprovedBy     string       `json:"approvedBy,omitempty"`
}

func (g ReviewGate) Approved(bundleDigest string) bool {
	if g.Status != ReviewStatusApproved {
		return false
	}
	if bundleDigest == "" {
		return false
	}
	if g.ArtifactDigest == "" {
		return false
	}
	return strings.EqualFold(g.ArtifactDigest, bundleDigest)
}

type ResourceLimits struct {
	CPUMilli int           `json:"cpuMilli"`
	MemoryMB int64         `json:"memoryMB"`
	Pids     int           `json:"pids"`
	Timeout  time.Duration `json:"timeout"`
	TmpMB    int64         `json:"tmpMB"`
	StdoutKB int64         `json:"stdoutKB"`
}

type SandboxPolicy struct {
	RequireApprovedReview bool           `json:"requireApprovedReview"`
	ReadOnlyRootFS        bool           `json:"readOnlyRootFS"`
	DisableNetwork        bool           `json:"disableNetwork"`
	WorkingDir            string         `json:"workingDir"`
	ScratchDir            string         `json:"scratchDir"`
	TmpDir                string         `json:"tmpDir"`
	RunAsUser             string         `json:"runAsUser"`
	Limits                ResourceLimits `json:"limits"`
}

func DefaultSandboxPolicy() SandboxPolicy {
	return SandboxPolicy{
		RequireApprovedReview: true,
		ReadOnlyRootFS:        true,
		DisableNetwork:        true,
		WorkingDir:            "/workspace/skill",
		ScratchDir:            "/sandbox",
		TmpDir:                "/tmp",
		RunAsUser:             "65532:65532",
		Limits: ResourceLimits{
			CPUMilli: 500,
			MemoryMB: 256,
			Pids:     64,
			Timeout:  10 * time.Second,
			TmpMB:    64,
			StdoutKB: 64,
		},
	}
}

type RuntimeSpec struct {
	ID         string `json:"id"`
	Image      string `json:"image"`
	Executable string `json:"executable"`
}

func DefaultRuntimeRegistry() map[string]RuntimeSpec {
	return map[string]RuntimeSpec{
		RuntimePython311: {
			ID:         RuntimePython311,
			Image:      "python:3.11-slim-bookworm",
			Executable: "python3.11",
		},
		RuntimeNode20: {
			ID:         RuntimeNode20,
			Image:      "node:20-bookworm-slim",
			Executable: "node",
		},
	}
}

type Mount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"readOnly"`
}

type TmpFS struct {
	Target string `json:"target"`
	SizeMB int64  `json:"sizeMB"`
	Mode   string `json:"mode"`
}

type IOLayout struct {
	ScratchDir string `json:"scratchDir"`
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
	TmpDir     string `json:"tmpDir"`
}

type SandboxPlan struct {
	Runtime         RuntimeSpec       `json:"runtime"`
	Image           string            `json:"image"`
	Command         []string          `json:"command"`
	WorkingDir      string            `json:"workingDir"`
	User            string            `json:"user"`
	Environment     map[string]string `json:"environment"`
	Mounts          []Mount           `json:"mounts"`
	TmpFS           []TmpFS           `json:"tmpfs"`
	Layout          IOLayout          `json:"layout"`
	ReadOnlyRootFS  bool              `json:"readOnlyRootFS"`
	NetworkMode     string            `json:"networkMode"`
	SecurityOptions []string          `json:"securityOptions"`
	CapDrop         []string          `json:"capDrop"`
	ResourceLimits  ResourceLimits    `json:"resourceLimits"`
}

type PlanRequest struct {
	Bundle         *Bundle    `json:"bundle"`
	Review         ReviewGate `json:"review"`
	HostSkillDir   string     `json:"hostSkillDir"`
	HostScratchDir string     `json:"hostScratchDir"`
}

type DispatchRequest struct {
	Bundle         *Bundle        `json:"bundle"`
	Review         ReviewGate     `json:"review"`
	Archive        []byte         `json:"archive,omitempty"`
	HostSkillDir   string         `json:"hostSkillDir,omitempty"`
	HostScratchDir string         `json:"hostScratchDir,omitempty"`
	Input          any            `json:"input,omitempty"`
	Environment    map[string]any `json:"environment,omitempty"`
}

type DispatchResult struct {
	Plan           *SandboxPlan `json:"plan,omitempty"`
	HostSkillDir   string       `json:"hostSkillDir,omitempty"`
	HostScratchDir string       `json:"hostScratchDir,omitempty"`
	InputPath      string       `json:"inputPath,omitempty"`
	OutputPath     string       `json:"outputPath,omitempty"`
	cleanup        func() error
	cleanupOnce    sync.Once
	cleanupErr     error
}

func (r *DispatchResult) Cleanup() error {
	if r == nil || r.cleanup == nil {
		return nil
	}
	r.cleanupOnce.Do(func() {
		r.cleanupErr = r.cleanup()
		r.cleanup = nil
	})
	return r.cleanupErr
}

type Runner interface {
	InspectArchive(ctx context.Context, raw []byte) (*Bundle, error)
	Plan(ctx context.Context, req PlanRequest) (*SandboxPlan, error)
	Dispatch(ctx context.Context, req DispatchRequest) (*DispatchResult, error)
}

type ScriptRunner struct {
	Inspector  *BundleInspector
	Planner    *DockerPlanner
	Dispatcher *LocalDispatcher
}

func NewScriptRunner() *ScriptRunner {
	return &ScriptRunner{
		Inspector:  NewBundleInspector(ArchiveConstraints{}, nil),
		Planner:    NewDockerPlanner(SandboxPolicy{}, nil),
		Dispatcher: NewLocalDispatcher(SandboxPolicy{}, nil),
	}
}

func (r *ScriptRunner) InspectArchive(ctx context.Context, raw []byte) (*Bundle, error) {
	return r.Inspector.InspectArchive(ctx, raw)
}

func (r *ScriptRunner) Plan(ctx context.Context, req PlanRequest) (*SandboxPlan, error) {
	return r.Planner.Plan(ctx, req)
}

func (r *ScriptRunner) Dispatch(ctx context.Context, req DispatchRequest) (*DispatchResult, error) {
	return r.Dispatcher.Dispatch(ctx, req)
}
