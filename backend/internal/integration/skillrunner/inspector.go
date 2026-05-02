package skillrunner

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	skillNamePattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{2,63}$`)
	skillVersionPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)
)

var disallowedBundleSuffixes = []string{
	".zip",
	".tar",
	".tgz",
	".gz",
	".bz2",
	".xz",
	".so",
	".dll",
	".dylib",
	".exe",
}

var disallowedTopLevelEntries = map[string]struct{}{
	".git":     {},
	".github":  {},
	"__MACOSX": {},
}

type BundleInspector struct {
	Constraints ArchiveConstraints
	Runtimes    map[string]RuntimeSpec
}

func NewBundleInspector(constraints ArchiveConstraints, runtimes map[string]RuntimeSpec) *BundleInspector {
	if constraints.MaxArchiveBytes == 0 {
		constraints = DefaultArchiveConstraints()
	}
	if len(runtimes) == 0 {
		runtimes = DefaultRuntimeRegistry()
	}
	return &BundleInspector{
		Constraints: constraints,
		Runtimes:    runtimes,
	}
}

func (i *BundleInspector) InspectArchive(ctx context.Context, raw []byte) (*Bundle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(raw) == 0 {
		return nil, fmt.Errorf("skill archive is empty")
	}
	if int64(len(raw)) > i.Constraints.MaxArchiveBytes {
		return nil, fmt.Errorf("skill archive exceeds %d bytes", i.Constraints.MaxArchiveBytes)
	}

	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}

	files := make(map[string]*zip.File, len(reader.File))
	bundleFiles := make([]BundleFile, 0, len(reader.File))
	var totalUncompressed uint64
	regularFiles := 0

	for _, file := range reader.File {
		cleanedPath, err := cleanArchivePath(file.Name, i.Constraints.MaxPathBytes)
		if err != nil {
			return nil, fmt.Errorf("archive entry %q: %w", file.Name, err)
		}
		if cleanedPath == "" {
			continue
		}

		if file.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("archive entry %q: symlinks are not allowed", cleanedPath)
		}
		if file.FileInfo().IsDir() {
			continue
		}

		regularFiles++
		if regularFiles > i.Constraints.MaxFiles {
			return nil, fmt.Errorf("skill archive exceeds %d files", i.Constraints.MaxFiles)
		}
		if file.UncompressedSize64 > i.Constraints.MaxFileBytes {
			return nil, fmt.Errorf("archive entry %q exceeds %d bytes", cleanedPath, i.Constraints.MaxFileBytes)
		}

		totalUncompressed += file.UncompressedSize64
		if totalUncompressed > i.Constraints.MaxUncompressedBytes {
			return nil, fmt.Errorf("skill archive exceeds %d uncompressed bytes", i.Constraints.MaxUncompressedBytes)
		}
		if err := validateBundleFile(cleanedPath); err != nil {
			return nil, fmt.Errorf("archive entry %q: %w", cleanedPath, err)
		}
		if _, exists := files[cleanedPath]; exists {
			return nil, fmt.Errorf("duplicate archive entry %q", cleanedPath)
		}

		files[cleanedPath] = file
		bundleFiles = append(bundleFiles, BundleFile{
			Path: cleanedPath,
			Size: file.UncompressedSize64,
		})
	}

	manifestFile, ok := files[ManifestFile]
	if !ok {
		return nil, fmt.Errorf("archive missing %s", ManifestFile)
	}
	if manifestFile.UncompressedSize64 > i.Constraints.MaxManifestBytes {
		return nil, fmt.Errorf("%s exceeds %d bytes", ManifestFile, i.Constraints.MaxManifestBytes)
	}

	manifest, err := decodeManifest(manifestFile, i.Constraints.MaxManifestBytes)
	if err != nil {
		return nil, err
	}
	if err := validateManifest(manifest, files, i.Runtimes, i.Constraints.MaxPathBytes); err != nil {
		return nil, err
	}

	digest := sha256.Sum256(raw)

	return &Bundle{
		Digest:                 hex.EncodeToString(digest[:]),
		Manifest:               manifest,
		Files:                  bundleFiles,
		ArchiveBytes:           int64(len(raw)),
		TotalUncompressedBytes: totalUncompressed,
		Constraints:            i.Constraints,
	}, nil
}

func decodeManifest(file *zip.File, maxBytes uint64) (Manifest, error) {
	reader, err := file.Open()
	if err != nil {
		return Manifest{}, fmt.Errorf("open %s: %w", ManifestFile, err)
	}
	defer reader.Close()

	limited := io.LimitReader(reader, int64(maxBytes))
	var manifest Manifest
	if err := yaml.NewDecoder(limited).Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode %s: %w", ManifestFile, err)
	}

	return normalizeManifest(manifest), nil
}

func normalizeManifest(manifest Manifest) Manifest {
	manifest.APIVersion = strings.TrimSpace(manifest.APIVersion)
	manifest.Kind = strings.TrimSpace(manifest.Kind)
	manifest.Metadata.Name = strings.TrimSpace(manifest.Metadata.Name)
	manifest.Metadata.Version = strings.TrimSpace(manifest.Metadata.Version)
	manifest.Metadata.DisplayName = strings.TrimSpace(manifest.Metadata.DisplayName)
	manifest.Metadata.Description = strings.TrimSpace(manifest.Metadata.Description)
	manifest.Spec.Type = strings.ToLower(strings.TrimSpace(manifest.Spec.Type))
	manifest.Spec.Runtime = strings.ToLower(strings.TrimSpace(manifest.Spec.Runtime))
	manifest.Spec.Entrypoint = strings.TrimSpace(manifest.Spec.Entrypoint)
	manifest.Spec.Protocol = strings.ToLower(strings.TrimSpace(manifest.Spec.Protocol))
	return manifest
}

func validateManifest(manifest Manifest, files map[string]*zip.File, runtimes map[string]RuntimeSpec, maxPathBytes int) error {
	if manifest.APIVersion != ManifestAPIVersion {
		return fmt.Errorf("manifest apiVersion must be %q", ManifestAPIVersion)
	}
	if manifest.Kind != ManifestKind {
		return fmt.Errorf("manifest kind must be %q", ManifestKind)
	}
	if !skillNamePattern.MatchString(manifest.Metadata.Name) {
		return fmt.Errorf("metadata.name must match %s", skillNamePattern.String())
	}
	if !skillVersionPattern.MatchString(manifest.Metadata.Version) {
		return fmt.Errorf("metadata.version must be semver-like")
	}
	if len(manifest.Metadata.DisplayName) > 80 {
		return fmt.Errorf("metadata.displayName must be <= 80 characters")
	}
	if len(manifest.Metadata.Description) > 512 {
		return fmt.Errorf("metadata.description must be <= 512 characters")
	}
	if manifest.Spec.Type != SkillTypeScript {
		return fmt.Errorf("spec.type must be %q", SkillTypeScript)
	}
	if manifest.Spec.Protocol != ProtocolJSONFileV1 {
		return fmt.Errorf("spec.protocol must be %q", ProtocolJSONFileV1)
	}
	if _, ok := runtimes[manifest.Spec.Runtime]; !ok {
		return fmt.Errorf("%w: %s", ErrUnsupportedRuntime, manifest.Spec.Runtime)
	}

	entrypoint, err := cleanArchivePath(manifest.Spec.Entrypoint, maxPathBytes)
	if err != nil {
		return fmt.Errorf("spec.entrypoint: %w", err)
	}
	if entrypoint == "" {
		return fmt.Errorf("spec.entrypoint is required")
	}
	file, ok := files[entrypoint]
	if !ok || file.FileInfo().IsDir() {
		return fmt.Errorf("spec.entrypoint %q is missing from archive", entrypoint)
	}
	if err := validateEntrypoint(manifest.Spec.Runtime, entrypoint); err != nil {
		return err
	}
	if entrypoint == ManifestFile {
		return fmt.Errorf("spec.entrypoint cannot point to %s", ManifestFile)
	}

	return nil
}

func cleanArchivePath(raw string, maxPathBytes int) (string, error) {
	name := strings.TrimSpace(strings.TrimSuffix(raw, "/"))
	if name == "" {
		return "", nil
	}
	if len(name) > maxPathBytes {
		return "", fmt.Errorf("path exceeds %d bytes", maxPathBytes)
	}
	if strings.Contains(name, `\`) {
		return "", fmt.Errorf("backslashes are not allowed")
	}
	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("absolute paths are not allowed")
	}

	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("path traversal is not allowed")
	}
	if cleaned != name {
		return "", fmt.Errorf("path must be normalized")
	}

	topLevel := strings.Split(cleaned, "/")[0]
	if _, blocked := disallowedTopLevelEntries[topLevel]; blocked {
		return "", fmt.Errorf("top-level entry %q is not allowed", topLevel)
	}

	return cleaned, nil
}

func validateBundleFile(cleanedPath string) error {
	lowerPath := strings.ToLower(cleanedPath)
	baseName := strings.ToLower(path.Base(cleanedPath))

	switch baseName {
	case ".ds_store", "dockerfile", "docker-compose.yml", "docker-compose.yaml":
		return fmt.Errorf("bundle file is not allowed")
	}

	for _, suffix := range disallowedBundleSuffixes {
		if strings.HasSuffix(lowerPath, suffix) {
			return fmt.Errorf("file suffix %q is not allowed in phase-1 script bundles", suffix)
		}
	}

	return nil
}

func validateEntrypoint(runtimeID, entrypoint string) error {
	switch runtimeID {
	case RuntimePython311:
		if !strings.HasSuffix(entrypoint, ".py") {
			return fmt.Errorf("python3.11 entrypoint must end with .py")
		}
	case RuntimeNode20:
		if !(strings.HasSuffix(entrypoint, ".js") || strings.HasSuffix(entrypoint, ".mjs") || strings.HasSuffix(entrypoint, ".cjs")) {
			return fmt.Errorf("node20 entrypoint must end with .js, .mjs or .cjs")
		}
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedRuntime, runtimeID)
	}
	return nil
}
