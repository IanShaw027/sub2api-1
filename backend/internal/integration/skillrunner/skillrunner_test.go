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

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}

	return buf.Bytes()
}
