package skillrunner

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultExecutionOutputLimit = int64(64 << 10)
	dockerCleanupTimeout        = 5 * time.Second
)

type DockerCLIExecutor struct {
	Binary string
}

func NewDockerCLIExecutor(binary string) *DockerCLIExecutor {
	return &DockerCLIExecutor{Binary: strings.TrimSpace(binary)}
}

func (e *DockerCLIExecutor) Execute(ctx context.Context, plan *SandboxPlan) (*ExecutionResult, error) {
	if plan == nil || strings.TrimSpace(plan.Image) == "" || len(plan.Command) == 0 {
		return nil, fmt.Errorf("sandbox execution plan is incomplete")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	binary := e.Binary
	if binary == "" {
		binary = "docker"
	}
	resolved, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("docker runtime is unavailable: %w", err)
	}

	runCtx := ctx
	cancel := func() {}
	if plan.ResourceLimits.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, plan.ResourceLimits.Timeout)
	}
	defer cancel()

	containerName, err := newSandboxContainerName()
	if err != nil {
		return nil, err
	}
	args, err := dockerRunArgs(containerName, plan)
	if err != nil {
		return nil, err
	}
	limit := plan.ResourceLimits.StdoutKB << 10
	if limit <= 0 {
		limit = defaultExecutionOutputLimit
	}
	stdout := newLimitedBuffer(limit)
	stderr := newLimitedBuffer(limit)
	cmd := exec.CommandContext(runCtx, resolved, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	runErr := cmd.Run()
	cleanupDockerContainer(resolved, containerName)
	if runErr != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("sandbox execution timed out after %s", plan.ResourceLimits.Timeout)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return nil, fmt.Errorf("sandbox execution failed: %w: %s", runErr, detail)
		}
		return nil, fmt.Errorf("sandbox execution failed: %w", runErr)
	}
	if stdout.Overflowed() || stderr.Overflowed() {
		return nil, fmt.Errorf("sandbox output exceeded %d bytes", limit)
	}
	return &ExecutionResult{Stdout: stdout.String(), Stderr: stderr.String()}, nil
}

func dockerRunArgs(containerName string, plan *SandboxPlan) ([]string, error) {
	args := []string{"run", "--rm", "--name", containerName}
	if plan.NetworkMode == "" {
		return nil, fmt.Errorf("sandbox network mode is required")
	}
	args = append(args, "--network", plan.NetworkMode)
	if plan.ReadOnlyRootFS {
		args = append(args, "--read-only")
	}
	if plan.User != "" {
		args = append(args, "--user", plan.User)
	}
	if plan.WorkingDir != "" {
		args = append(args, "--workdir", plan.WorkingDir)
	}
	for _, option := range plan.SecurityOptions {
		if option != "" {
			args = append(args, "--security-opt", option)
		}
	}
	for _, capability := range plan.CapDrop {
		if capability != "" {
			args = append(args, "--cap-drop", capability)
		}
	}
	if plan.ResourceLimits.Pids > 0 {
		args = append(args, "--pids-limit", strconv.Itoa(plan.ResourceLimits.Pids))
	}
	if plan.ResourceLimits.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", plan.ResourceLimits.MemoryMB))
	}
	if plan.ResourceLimits.CPUMilli > 0 {
		cpus := float64(plan.ResourceLimits.CPUMilli) / 1000
		args = append(args, "--cpus", strconv.FormatFloat(cpus, 'f', 3, 64))
	}
	for _, mount := range plan.Mounts {
		source := filepath.Clean(strings.TrimSpace(mount.Source))
		target := strings.TrimSpace(mount.Target)
		if !filepath.IsAbs(source) || !strings.HasPrefix(target, "/") || strings.Contains(source, ":") {
			return nil, fmt.Errorf("invalid sandbox mount %q -> %q", mount.Source, mount.Target)
		}
		mode := "rw"
		if mount.ReadOnly {
			mode = "ro"
		}
		args = append(args, "--volume", source+":"+target+":"+mode)
	}
	for _, tmpfs := range plan.TmpFS {
		if !strings.HasPrefix(tmpfs.Target, "/") {
			return nil, fmt.Errorf("invalid sandbox tmpfs target %q", tmpfs.Target)
		}
		options := []string{"rw", "nosuid", "nodev", "noexec"}
		if tmpfs.SizeMB > 0 {
			options = append(options, fmt.Sprintf("size=%dm", tmpfs.SizeMB))
		}
		if tmpfs.Mode != "" {
			options = append(options, "mode="+tmpfs.Mode)
		}
		args = append(args, "--tmpfs", tmpfs.Target+":"+strings.Join(options, ","))
	}
	keys := make([]string, 0, len(plan.Environment))
	for key := range plan.Environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == "" || strings.ContainsAny(key, "=\x00") || strings.ContainsRune(plan.Environment[key], '\x00') {
			return nil, fmt.Errorf("invalid sandbox environment variable %q", key)
		}
		args = append(args, "--env", key+"="+plan.Environment[key])
	}
	args = append(args, plan.Image)
	args = append(args, plan.Command...)
	return args, nil
}

func prepareScratchPermissions(hostScratchDir, inputPath, outputPath string) error {
	if err := os.Chmod(hostScratchDir, 0o711); err != nil {
		return fmt.Errorf("secure scratch dir: %w", err)
	}
	if err := os.Chmod(filepath.Dir(inputPath), 0o755); err != nil {
		return fmt.Errorf("secure input dir: %w", err)
	}
	if err := os.Chmod(filepath.Dir(outputPath), 0o733); err != nil {
		return fmt.Errorf("prepare sandbox output dir: %w", err)
	}
	return nil
}

func readDispatchOutput(outputPath string, stdoutKB int64) (map[string]any, error) {
	info, err := os.Lstat(outputPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("sandbox did not produce response.json")
		}
		return nil, fmt.Errorf("inspect sandbox output: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("sandbox output must be a regular file")
	}
	limit := stdoutKB << 10
	if limit <= 0 {
		limit = defaultExecutionOutputLimit
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("sandbox response exceeded %d bytes", limit)
	}
	file, err := os.Open(outputPath)
	if err != nil {
		return nil, fmt.Errorf("open sandbox output: %w", err)
	}
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read sandbox output: %w", err)
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("sandbox response exceeded %d bytes", limit)
	}
	var output map[string]any
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, fmt.Errorf("parse sandbox response: %w", err)
	}
	if output == nil {
		return nil, fmt.Errorf("sandbox response must be a JSON object")
	}
	return output, nil
}

func cleanupDockerContainer(binary, name string) {
	ctx, cancel := context.WithTimeout(context.Background(), dockerCleanupTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "rm", "--force", name)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}

func newSandboxContainerName() (string, error) {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate sandbox container name: %w", err)
	}
	return "sub2api-skill-" + hex.EncodeToString(suffix[:]), nil
}

type limitedBuffer struct {
	buffer     bytes.Buffer
	remaining  int64
	overflowed bool
}

func newLimitedBuffer(limit int64) *limitedBuffer {
	return &limitedBuffer{remaining: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	if int64(len(p)) > b.remaining {
		p = p[:max(0, int(b.remaining))]
		b.overflowed = true
	}
	if len(p) > 0 {
		_, _ = b.buffer.Write(p)
		b.remaining -= int64(len(p))
	}
	return originalLen, nil
}

func (b *limitedBuffer) String() string   { return b.buffer.String() }
func (b *limitedBuffer) Overflowed() bool { return b.overflowed }
