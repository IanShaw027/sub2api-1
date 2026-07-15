package skillrunner

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type writingSandboxExecutor struct{}

func (writingSandboxExecutor) Execute(_ context.Context, plan *SandboxPlan) (*ExecutionResult, error) {
	for _, mount := range plan.Mounts {
		if mount.Target != plan.Layout.ScratchDir {
			continue
		}
		rel := strings.TrimPrefix(plan.Layout.OutputPath, plan.Layout.ScratchDir+"/")
		outputPath := filepath.Join(mount.Source, filepath.FromSlash(rel))
		if err := os.WriteFile(outputPath, []byte(`{"ok":true}`), 0o644); err != nil {
			return nil, err
		}
		return &ExecutionResult{Stdout: "done"}, nil
	}
	return nil, errors.New("scratch mount not found")
}

func TestInspectArchiveAcceptsPythonSample(t *testing.T) {
	t.Parallel()

	archive := zipSampleDir(t, "script_python_echo")
	inspector := NewBundleInspector(ArchiveConstraints{}, nil)

	bundle, err := inspector.InspectArchive(context.Background(), archive)
	if err != nil {
		t.Fatalf("InspectArchive() error = %v", err)
	}
	if bundle.Manifest.Spec.Type != SkillTypeScript {
		t.Fatalf("unexpected type %q", bundle.Manifest.Spec.Type)
	}
	if bundle.Manifest.Spec.Runtime != RuntimePython311 {
		t.Fatalf("unexpected runtime %q", bundle.Manifest.Spec.Runtime)
	}
	if bundle.Manifest.Spec.Protocol != ProtocolJSONFileV1 {
		t.Fatalf("unexpected protocol %q", bundle.Manifest.Spec.Protocol)
	}
	if bundle.Digest == "" {
		t.Fatal("expected non-empty digest")
	}
}

func TestInspectArchiveAcceptsNodeSample(t *testing.T) {
	t.Parallel()

	archive := zipSampleDir(t, "script_node_echo")
	inspector := NewBundleInspector(ArchiveConstraints{}, nil)

	bundle, err := inspector.InspectArchive(context.Background(), archive)
	if err != nil {
		t.Fatalf("InspectArchive() error = %v", err)
	}
	if bundle.Manifest.Spec.Runtime != RuntimeNode20 {
		t.Fatalf("unexpected runtime %q", bundle.Manifest.Spec.Runtime)
	}
}

func TestInspectArchiveRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	archive := zipMap(t, map[string]string{
		ManifestFile: minimalManifest(RuntimePython311, "main.py"),
		"../main.py": "print('hello')\n",
	})
	inspector := NewBundleInspector(ArchiveConstraints{}, nil)

	_, err := inspector.InspectArchive(context.Background(), archive)
	if err == nil {
		t.Fatal("expected traversal error")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("expected traversal error, got %v", err)
	}
}

func TestReviewGateApprovedRequiresArtifactDigest(t *testing.T) {
	t.Parallel()

	approved := ReviewGate{Status: ReviewStatusApproved}
	if approved.Approved("bundle-digest") {
		t.Fatal("expected approved gate without artifact digest to fail closed")
	}
}

func TestPlanRequiresApprovedReview(t *testing.T) {
	t.Parallel()

	skillDir := sampleSkillDir(t, "script_python_echo")
	archive := zipSampleDir(t, "script_python_echo")
	inspector := NewBundleInspector(ArchiveConstraints{}, nil)
	bundle, err := inspector.InspectArchive(context.Background(), archive)
	if err != nil {
		t.Fatalf("InspectArchive() error = %v", err)
	}

	planner := NewDockerPlanner(SandboxPolicy{}, nil)
	_, err = planner.Plan(context.Background(), PlanRequest{
		Bundle:         bundle,
		Review:         ReviewGate{ArtifactDigest: bundle.Digest, Status: ReviewStatusPending},
		HostSkillDir:   skillDir,
		HostScratchDir: t.TempDir(),
	})
	if !errors.Is(err, ErrReviewRequired) {
		t.Fatalf("expected ErrReviewRequired, got %v", err)
	}
}

func TestPlanBuildsLockedDownDockerSpec(t *testing.T) {
	t.Parallel()

	skillDir := sampleSkillDir(t, "script_python_echo")
	archive := zipSampleDir(t, "script_python_echo")
	inspector := NewBundleInspector(ArchiveConstraints{}, nil)
	bundle, err := inspector.InspectArchive(context.Background(), archive)
	if err != nil {
		t.Fatalf("InspectArchive() error = %v", err)
	}

	planner := NewDockerPlanner(SandboxPolicy{}, nil)
	plan, err := planner.Plan(context.Background(), PlanRequest{
		Bundle: bundle,
		Review: ReviewGate{
			ArtifactDigest: bundle.Digest,
			Status:         ReviewStatusApproved,
		},
		HostSkillDir:   skillDir,
		HostScratchDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if plan.Image != "python:3.11-slim-bookworm" {
		t.Fatalf("unexpected image %q", plan.Image)
	}
	if plan.NetworkMode != "none" {
		t.Fatalf("unexpected network mode %q", plan.NetworkMode)
	}
	if !plan.ReadOnlyRootFS {
		t.Fatal("expected read-only rootfs")
	}
	if got := plan.Environment["SUB2API_SKILL_INPUT"]; got != "/sandbox/input/request.json" {
		t.Fatalf("unexpected input path %q", got)
	}
	if got := plan.Environment["SUB2API_SKILL_OUTPUT"]; got != "/sandbox/output/response.json" {
		t.Fatalf("unexpected output path %q", got)
	}
	if len(plan.Mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %d", len(plan.Mounts))
	}
	if len(plan.TmpFS) != 1 || plan.TmpFS[0].Target != "/tmp" {
		t.Fatalf("unexpected tmpfs %#v", plan.TmpFS)
	}
	if plan.Command[0] != "python3.11" {
		t.Fatalf("unexpected command %#v", plan.Command)
	}
}

func TestDispatchReturnsCreatedTempDirsUntilCleanup(t *testing.T) {
	t.Parallel()

	archive := zipSampleDir(t, "script_python_echo")
	inspector := NewBundleInspector(ArchiveConstraints{}, nil)
	bundle, err := inspector.InspectArchive(context.Background(), archive)
	if err != nil {
		t.Fatalf("InspectArchive() error = %v", err)
	}

	dispatcher := NewLocalDispatcher(SandboxPolicy{}, nil)
	dispatcher.Executor = writingSandboxExecutor{}
	result, err := dispatcher.Dispatch(context.Background(), DispatchRequest{
		Bundle: bundle,
		Review: ReviewGate{
			ArtifactDigest: bundle.Digest,
			Status:         ReviewStatusApproved,
		},
		Archive: archive,
		Input:   map[string]any{"hello": "world"},
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	// Dispatch must keep its created temp dirs alive until the caller finishes
	// using the returned host paths and explicitly calls Cleanup.
	if _, statErr := os.Stat(result.HostSkillDir); statErr != nil {
		t.Fatalf("expected host skill dir to exist before cleanup, stat err = %v", statErr)
	}
	if _, statErr := os.Stat(result.HostScratchDir); statErr != nil {
		t.Fatalf("expected host scratch dir to exist before cleanup, stat err = %v", statErr)
	}

	if err := result.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	if _, statErr := os.Stat(result.HostSkillDir); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("expected host skill dir to be removed after cleanup, stat err = %v", statErr)
	}
	if _, statErr := os.Stat(result.HostScratchDir); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("expected host scratch dir to be removed after cleanup, stat err = %v", statErr)
	}
}

func TestDispatchPreservesCallerSuppliedDirs(t *testing.T) {
	t.Parallel()

	skillDir := sampleSkillDir(t, "script_python_echo")
	scratchDir := t.TempDir()
	archive := zipSampleDir(t, "script_python_echo")
	inspector := NewBundleInspector(ArchiveConstraints{}, nil)
	bundle, err := inspector.InspectArchive(context.Background(), archive)
	if err != nil {
		t.Fatalf("InspectArchive() error = %v", err)
	}

	dispatcher := NewLocalDispatcher(SandboxPolicy{}, nil)
	dispatcher.Executor = writingSandboxExecutor{}
	result, err := dispatcher.Dispatch(context.Background(), DispatchRequest{
		Bundle: bundle,
		Review: ReviewGate{
			ArtifactDigest: bundle.Digest,
			Status:         ReviewStatusApproved,
		},
		HostSkillDir:   skillDir,
		HostScratchDir: scratchDir,
		Input:          map[string]any{"hello": "world"},
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	// Caller-supplied dirs must not be removed by Dispatch.
	if _, statErr := os.Stat(skillDir); statErr != nil {
		t.Fatalf("caller skill dir should be preserved, stat err = %v", statErr)
	}
	if _, statErr := os.Stat(scratchDir); statErr != nil {
		t.Fatalf("caller scratch dir should be preserved, stat err = %v", statErr)
	}
	if err := result.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if _, statErr := os.Stat(skillDir); statErr != nil {
		t.Fatalf("caller skill dir should remain after cleanup, stat err = %v", statErr)
	}
	if _, statErr := os.Stat(scratchDir); statErr != nil {
		t.Fatalf("caller scratch dir should remain after cleanup, stat err = %v", statErr)
	}
}

func TestExtractArchiveRejectsSymlinkEntries(t *testing.T) {
	t.Parallel()

	archive := zipEntries(t, []zipTestEntry{
		{name: "skill.yaml", content: minimalManifest(RuntimePython311, "main.py"), mode: 0o644},
		{name: "main.py", content: "print('hello')\n", mode: os.ModeSymlink | 0o777},
	})

	err := extractArchive(archive, t.TempDir())
	if err == nil {
		t.Fatal("expected symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlinks are not allowed") {
		t.Fatalf("expected symlink rejection error, got %v", err)
	}
}

func TestExtractArchiveRejectsOversizedFile(t *testing.T) {
	t.Parallel()

	constraints := DefaultArchiveConstraints()
	archive := zipEntries(t, []zipTestEntry{
		{name: "skill.yaml", content: minimalManifest(RuntimePython311, "main.py"), mode: 0o644},
		{name: "main.py", content: strings.Repeat("a", int(constraints.MaxFileBytes)+1), mode: 0o644},
	})

	err := extractArchive(archive, t.TempDir())
	if err == nil {
		t.Fatal("expected oversized file rejection")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized file error, got %v", err)
	}
}

func TestExtractArchiveSanitizesPermissionBits(t *testing.T) {
	t.Parallel()

	archive := zipEntries(t, []zipTestEntry{
		{name: "skill.yaml", content: minimalManifest(RuntimePython311, "main.py"), mode: 0o644},
		{name: "main.py", content: "print('hello')\n", mode: os.FileMode(0o6777)},
	})

	targetDir := t.TempDir()
	if err := extractArchive(archive, targetDir); err != nil {
		t.Fatalf("extractArchive() error = %v", err)
	}

	info, err := os.Stat(filepath.Join(targetDir, "main.py"))
	if err != nil {
		t.Fatalf("stat extracted file: %v", err)
	}
	if info.Mode()&os.ModeSetuid != 0 || info.Mode()&os.ModeSetgid != 0 || info.Mode()&os.ModeSticky != 0 {
		t.Fatalf("expected special permission bits to be stripped, got mode %v", info.Mode())
	}
	if got := info.Mode().Perm(); got != 0o755 {
		t.Fatalf("expected sanitized permissions 0755, got %04o", got)
	}
}

func minimalManifest(runtimeID, entrypoint string) string {
	return strings.Join([]string{
		"apiVersion: " + ManifestAPIVersion,
		"kind: " + ManifestKind,
		"metadata:",
		"  name: sample_skill",
		"  version: 1.0.0",
		"spec:",
		"  type: " + SkillTypeScript,
		"  runtime: " + runtimeID,
		"  entrypoint: " + entrypoint,
		"  protocol: " + ProtocolJSONFileV1,
		"",
	}, "\n")
}

func sampleSkillDir(t *testing.T, name string) string {
	t.Helper()

	dir, err := filepath.Abs(filepath.Join("..", "..", "service", "testdata", "skills", name))
	if err != nil {
		t.Fatalf("resolve sample dir: %v", err)
	}
	return dir
}

func zipSampleDir(t *testing.T, name string) []byte {
	t.Helper()

	root := sampleSkillDir(t, name)
	files := make(map[string]string)

	err := filepath.WalkDir(root, func(filePath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("walk sample dir: %v", err)
	}

	return zipMap(t, files)
}

func zipMap(t *testing.T, files map[string]string) []byte {
	t.Helper()

	entries := make([]zipTestEntry, 0, len(files))
	for name, content := range files {
		entries = append(entries, zipTestEntry{name: name, content: content, mode: 0o644})
	}
	return zipEntries(t, entries)
}

type zipTestEntry struct {
	name    string
	content string
	mode    os.FileMode
}

func zipEntries(t *testing.T, entries []zipTestEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	for _, item := range entries {
		header := &zip.FileHeader{Name: item.name}
		if item.mode != 0 {
			header.SetMode(item.mode)
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", item.name, err)
		}
		if _, err := entry.Write([]byte(item.content)); err != nil {
			t.Fatalf("write zip entry %q: %v", item.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	return buf.Bytes()
}
