package skillrunner

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type LocalDispatcher struct {
	Planner *DockerPlanner
}

func NewLocalDispatcher(policy SandboxPolicy, runtimes map[string]RuntimeSpec) *LocalDispatcher {
	return &LocalDispatcher{
		Planner: NewDockerPlanner(policy, runtimes),
	}
}

func (d *LocalDispatcher) Dispatch(ctx context.Context, req DispatchRequest) (*DispatchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if d == nil || d.Planner == nil {
		return nil, fmt.Errorf("dispatcher is not configured")
	}
	if req.Bundle == nil {
		return nil, fmt.Errorf("bundle is required")
	}

	hostSkillDir := strings.TrimSpace(req.HostSkillDir)
	hostScratchDir := strings.TrimSpace(req.HostScratchDir)
	var err error

	// Track temp dirs created by this call so we can clean them up before
	// returning. Dirs supplied by the caller are left untouched. The cleanup is
	// deferred so it covers every error path; once real container execution is
	// wired in, that execution happens before this function returns and thus
	// before the temp dirs (holding the extracted plaintext source and the
	// rendered input) are removed.
	var createdDirs []string
	defer func() {
		for _, dir := range createdDirs {
			_ = os.RemoveAll(dir)
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

	return &DispatchResult{
		Plan:           plan,
		HostSkillDir:   hostSkillDir,
		HostScratchDir: hostScratchDir,
		InputPath:      inputPath,
		OutputPath:     outputPath,
	}, nil
}

func extractArchive(raw []byte, targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	for _, file := range reader.File {
		cleanedPath, err := cleanArchivePath(file.Name, DefaultArchiveConstraints().MaxPathBytes)
		if err != nil {
			return fmt.Errorf("archive entry %q: %w", file.Name, err)
		}
		if cleanedPath == "" {
			continue
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if err := extractArchiveFile(file, targetDir, cleanedPath); err != nil {
			return err
		}
	}
	return nil
}

func extractArchiveFile(file *zip.File, targetDir, cleanedPath string) error {
	targetPath := filepath.Join(targetDir, filepath.FromSlash(cleanedPath))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}

	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("open archive entry %q: %w", cleanedPath, err)
	}
	defer func() { _ = reader.Close() }()

	dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.Mode())
	if err != nil {
		return fmt.Errorf("create archive file %q: %w", cleanedPath, err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, reader); err != nil {
		return fmt.Errorf("extract archive file %q: %w", cleanedPath, err)
	}
	return nil
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
