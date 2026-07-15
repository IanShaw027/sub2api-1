package skillrunner

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type LocalDispatcher struct {
	Planner  *DockerPlanner
	Executor SandboxExecutor
}

func NewLocalDispatcher(policy SandboxPolicy, runtimes map[string]RuntimeSpec) *LocalDispatcher {
	return &LocalDispatcher{
		Planner:  NewDockerPlanner(policy, runtimes),
		Executor: NewDockerCLIExecutor(""),
	}
}

func (d *LocalDispatcher) Dispatch(ctx context.Context, req DispatchRequest) (*DispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if d == nil || d.Planner == nil {
		return nil, fmt.Errorf("dispatcher is not configured")
	}
	if d.Executor == nil {
		return nil, fmt.Errorf("sandbox executor is not configured")
	}
	if req.Bundle == nil {
		return nil, fmt.Errorf("bundle is required")
	}

	hostSkillDir := strings.TrimSpace(req.HostSkillDir)
	hostScratchDir := strings.TrimSpace(req.HostScratchDir)
	var err error

	// Track temp dirs created by this call so error paths can clean them up
	// immediately, while successful dispatches hand cleanup control to the
	// caller that owns the returned host paths.
	var createdDirs []string
	cleanupCreatedDirs := func() error {
		var errs []error
		for _, dir := range createdDirs {
			if dir == "" {
				continue
			}
			if removeErr := os.RemoveAll(dir); removeErr != nil {
				errs = append(errs, removeErr)
			}
		}
		createdDirs = nil
		return errors.Join(errs...)
	}
	success := false
	defer func() {
		if !success {
			_ = cleanupCreatedDirs()
		}
	}()

	if hostSkillDir == "" {
		if len(req.Archive) == 0 {
			return nil, fmt.Errorf("archive or host skill dir is required")
		}
		hostSkillDir, err = os.MkdirTemp("", "skillrunner-skill-*")
		if err != nil {
			return nil, fmt.Errorf("create host skill dir: %w", err)
		}
		createdDirs = append(createdDirs, hostSkillDir)
		if err := extractArchive(req.Archive, hostSkillDir); err != nil {
			return nil, err
		}
	}
	if hostScratchDir == "" {
		hostScratchDir, err = os.MkdirTemp("", "skillrunner-scratch-*")
		if err != nil {
			return nil, fmt.Errorf("create host scratch dir: %w", err)
		}
		createdDirs = append(createdDirs, hostScratchDir)
	}

	plan, err := d.Planner.Plan(ctx, PlanRequest{
		Bundle:         req.Bundle,
		Review:         req.Review,
		HostSkillDir:   hostSkillDir,
		HostScratchDir: hostScratchDir,
	})
	if err != nil {
		return nil, err
	}
	if plan.Environment == nil {
		plan.Environment = map[string]string{}
	}
	for key, value := range req.Environment {
		if strings.TrimSpace(key) == "" || value == nil {
			continue
		}
		plan.Environment[key] = fmt.Sprint(value)
	}
	if req.Timeout > 0 {
		plan.ResourceLimits.Timeout = req.Timeout
	}

	inputPath, err := scratchHostPath(plan.Layout, hostScratchDir, plan.Layout.InputPath)
	if err != nil {
		return nil, err
	}
	outputPath, err := scratchHostPath(plan.Layout, hostScratchDir, plan.Layout.OutputPath)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(inputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create input dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	payload := req.Input
	if payload == nil {
		payload = map[string]any{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal dispatch input: %w", err)
	}
	if err := os.WriteFile(inputPath, raw, 0o644); err != nil {
		return nil, fmt.Errorf("write dispatch input: %w", err)
	}
	if err := prepareScratchPermissions(hostScratchDir, inputPath, outputPath); err != nil {
		return nil, err
	}

	execution, err := d.Executor.Execute(ctx, plan)
	if err != nil {
		return nil, err
	}
	output, err := readDispatchOutput(outputPath, plan.ResourceLimits.StdoutKB)
	if err != nil {
		return nil, err
	}

	result := &DispatchResult{
		Plan:           plan,
		HostSkillDir:   hostSkillDir,
		HostScratchDir: hostScratchDir,
		InputPath:      inputPath,
		OutputPath:     outputPath,
		Output:         output,
		Execution:      execution,
		cleanup:        cleanupCreatedDirs,
	}
	success = true
	return result, nil
}

func extractArchive(raw []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	constraints := DefaultArchiveConstraints()
	var (
		totalUncompressed uint64
		regularFiles      int
	)
	for _, file := range reader.File {
		cleanedPath, err := cleanArchivePath(file.Name, DefaultArchiveConstraints().MaxPathBytes)
		if err != nil {
			return fmt.Errorf("archive entry %q: %w", file.Name, err)
		}
		if cleanedPath == "" {
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive entry %q: symlinks are not allowed", cleanedPath)
		}
		if file.FileInfo().IsDir() {
			continue
		}
		regularFiles++
		if regularFiles > constraints.MaxFiles {
			return fmt.Errorf("skill archive exceeds %d files", constraints.MaxFiles)
		}
		if file.UncompressedSize64 > constraints.MaxFileBytes {
			return fmt.Errorf("archive entry %q exceeds %d bytes", cleanedPath, constraints.MaxFileBytes)
		}
		totalUncompressed += file.UncompressedSize64
		if totalUncompressed > constraints.MaxUncompressedBytes {
			return fmt.Errorf("skill archive exceeds %d uncompressed bytes", constraints.MaxUncompressedBytes)
		}
		if err := validateBundleFile(cleanedPath); err != nil {
			return fmt.Errorf("archive entry %q: %w", cleanedPath, err)
		}
		if err := extractArchiveFile(file, targetDir, cleanedPath, constraints.MaxFileBytes); err != nil {
			return err
		}
	}
	return nil
}

func extractArchiveFile(file *zip.File, targetDir, cleanedPath string, maxFileBytes uint64) error {
	targetPath := filepath.Join(targetDir, filepath.FromSlash(cleanedPath))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}

	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("open archive entry %q: %w", cleanedPath, err)
	}
	defer func() { _ = reader.Close() }()

	dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sanitizeArchiveEntryMode(file.Mode()))
	if err != nil {
		return fmt.Errorf("create archive file %q: %w", cleanedPath, err)
	}
	defer func() { _ = dst.Close() }()

	limited := &io.LimitedReader{R: reader, N: int64(maxFileBytes) + 1}
	written, err := io.Copy(dst, limited)
	if err != nil {
		return fmt.Errorf("extract archive file %q: %w", cleanedPath, err)
	}
	if written > int64(maxFileBytes) {
		return fmt.Errorf("archive entry %q exceeds %d bytes", cleanedPath, maxFileBytes)
	}
	return nil
}

func sanitizeArchiveEntryMode(mode os.FileMode) os.FileMode {
	perm := mode.Perm() & 0o755
	if perm == 0 {
		return 0o644
	}
	return perm
}

func scratchHostPath(layout IOLayout, hostScratchDir, containerPath string) (string, error) {
	scratchDir := path.Clean(layout.ScratchDir)
	cleanContainer := path.Clean(containerPath)
	if cleanContainer == scratchDir {
		return hostScratchDir, nil
	}
	prefix := scratchDir + "/"
	if !strings.HasPrefix(cleanContainer, prefix) {
		return "", fmt.Errorf("container path %q is outside scratch dir %q", containerPath, layout.ScratchDir)
	}
	rel := strings.TrimPrefix(cleanContainer, prefix)
	return filepath.Join(hostScratchDir, filepath.FromSlash(rel)), nil
}
