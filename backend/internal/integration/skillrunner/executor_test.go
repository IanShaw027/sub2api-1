//go:build unit

package skillrunner

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDockerRunArgsIncludesLockedDownRuntimeOptions(t *testing.T) {
	t.Parallel()

	plan := &SandboxPlan{
		Image:          "python:3.11-slim-bookworm",
		Command:        []string{"python3.11", "/workspace/skill/main.py"},
		NetworkMode:    "none",
		ReadOnlyRootFS: true,
		User:           "65532:65532",
		WorkingDir:     "/workspace/skill",
		SecurityOptions: []string{
			"no-new-privileges:true",
		},
		CapDrop: []string{"ALL"},
		ResourceLimits: ResourceLimits{
			Pids:     64,
			MemoryMB: 256,
			CPUMilli: 500,
		},
		Mounts: []Mount{
			{Source: "/opt/sub2api/skill", Target: "/workspace/skill", ReadOnly: true},
			{Source: "/opt/sub2api/scratch", Target: "/sandbox"},
		},
		TmpFS: []TmpFS{{Target: "/tmp", SizeMB: 64, Mode: "1777"}},
		Environment: map[string]string{
			"TMPDIR":               "/tmp",
			"SUB2API_SKILL_OUTPUT": "/sandbox/output/response.json",
			"SUB2API_SKILL_INPUT":  "/sandbox/input/request.json",
		},
	}

	got, err := dockerRunArgs("sub2api-skill-test", plan)
	if err != nil {
		t.Fatalf("dockerRunArgs() error = %v", err)
	}
	want := []string{
		"run", "--rm", "--name", "sub2api-skill-test",
		"--network", "none",
		"--read-only",
		"--user", "65532:65532",
		"--workdir", "/workspace/skill",
		"--security-opt", "no-new-privileges:true",
		"--cap-drop", "ALL",
		"--pids-limit", "64",
		"--memory", "256m",
		"--cpus", "0.500",
		"--volume", "/opt/sub2api/skill:/workspace/skill:ro",
		"--volume", "/opt/sub2api/scratch:/sandbox:rw",
		"--tmpfs", "/tmp:rw,nosuid,nodev,noexec,size=64m,mode=1777",
		"--env", "SUB2API_SKILL_INPUT=/sandbox/input/request.json",
		"--env", "SUB2API_SKILL_OUTPUT=/sandbox/output/response.json",
		"--env", "TMPDIR=/tmp",
		"python:3.11-slim-bookworm",
		"python3.11", "/workspace/skill/main.py",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dockerRunArgs() mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestDockerRunArgsRejectsInvalidMountsAndEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mounts      []Mount
		environment map[string]string
		wantError   string
	}{
		{
			name:      "relative mount source",
			mounts:    []Mount{{Source: "relative/skill", Target: "/workspace/skill"}},
			wantError: "invalid sandbox mount",
		},
		{
			name:      "relative mount target",
			mounts:    []Mount{{Source: "/opt/sub2api/skill", Target: "workspace/skill"}},
			wantError: "invalid sandbox mount",
		},
		{
			name:      "mount source contains volume separator",
			mounts:    []Mount{{Source: "/opt/sub2api/skill:ro", Target: "/workspace/skill"}},
			wantError: "invalid sandbox mount",
		},
		{
			name:        "empty environment name",
			environment: map[string]string{"": "value"},
			wantError:   "invalid sandbox environment variable",
		},
		{
			name:        "environment name contains equals",
			environment: map[string]string{"BAD=NAME": "value"},
			wantError:   "invalid sandbox environment variable",
		},
		{
			name:        "environment name contains null",
			environment: map[string]string{"BAD\x00NAME": "value"},
			wantError:   "invalid sandbox environment variable",
		},
		{
			name:        "environment value contains null",
			environment: map[string]string{"SAFE_NAME": "bad\x00value"},
			wantError:   "invalid sandbox environment variable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			plan := &SandboxPlan{
				Image:       "python:3.11-slim-bookworm",
				Command:     []string{"python3.11", "main.py"},
				NetworkMode: "none",
				Mounts:      tt.mounts,
				Environment: tt.environment,
			}
			args, err := dockerRunArgs("sub2api-skill-test", plan)
			if err == nil {
				t.Fatalf("dockerRunArgs() = %#v, want error containing %q", args, tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("dockerRunArgs() error = %q, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestReadDispatchOutputRejectsSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	output := filepath.Join(dir, "response.json")
	if err := os.Symlink(target, output); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	got, err := readDispatchOutput(output, 1)
	if err == nil {
		t.Fatalf("readDispatchOutput() = %#v, want symlink rejection", got)
	}
	if !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("readDispatchOutput() error = %q, want regular-file rejection", err)
	}
}

func TestReadDispatchOutputRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	output := filepath.Join(t.TempDir(), "response.json")
	if err := os.WriteFile(output, []byte(strings.Repeat("x", 1025)), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := readDispatchOutput(output, 1)
	if err == nil {
		t.Fatalf("readDispatchOutput() = %#v, want size-limit error", got)
	}
	if !strings.Contains(err.Error(), "exceeded 1024 bytes") {
		t.Fatalf("readDispatchOutput() error = %q, want size-limit detail", err)
	}
}

func TestReadDispatchOutputRejectsNonObjectJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{name: "null", body: "null", wantError: "must be a JSON object"},
		{name: "array", body: `[{"ok":true}]`, wantError: "parse sandbox response"},
		{name: "string", body: `"value"`, wantError: "parse sandbox response"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			output := filepath.Join(t.TempDir(), "response.json")
			if err := os.WriteFile(output, []byte(tt.body), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			got, err := readDispatchOutput(output, 1)
			if err == nil {
				t.Fatalf("readDispatchOutput() = %#v, want non-object rejection", got)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("readDispatchOutput() error = %q, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestDockerCLIExecutorFailsClosedWhenBinaryIsUnavailable(t *testing.T) {
	t.Parallel()

	missingBinary := filepath.Join(t.TempDir(), "docker-does-not-exist")
	executor := NewDockerCLIExecutor(missingBinary)
	result, err := executor.Execute(context.Background(), &SandboxPlan{
		Image:       "python:3.11-slim-bookworm",
		Command:     []string{"python3.11", "main.py"},
		NetworkMode: "none",
	})
	if err == nil {
		t.Fatalf("Execute() result = %#v, want unavailable-runtime error", result)
	}
	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil", result)
	}
	if !strings.Contains(err.Error(), "docker runtime is unavailable") {
		t.Fatalf("Execute() error = %q, want unavailable-runtime detail", err)
	}
}
