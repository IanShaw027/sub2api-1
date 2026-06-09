package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	objectStorageSettingsCacheTTL = 5 * time.Second
	settingKeyObjectStorageConfig = "object_storage_settings"
)

type ObjectStorageProfile struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Provider         string `json:"provider"`
	Endpoint         string `json:"endpoint"`
	Region           string `json:"region"`
	Bucket           string `json:"bucket"`
	AccessKeyID      string `json:"access_key_id"`
	SecretAccessKey  string `json:"secret_access_key,omitempty"` //nolint:revive // field name follows AWS convention
	SecretConfigured bool   `json:"secret_configured,omitempty"`
	ForcePathStyle   bool   `json:"force_path_style"`
}

type ObjectStorageSettings struct {
	Profiles           []ObjectStorageProfile `json:"profiles"`
	BackupProfileID    string                 `json:"backup_profile_id"`
	BackupPrefix       string                 `json:"backup_prefix"`
	MediaEnabled       bool                   `json:"media_enabled"`
	MediaProfileID     string                 `json:"media_profile_id"`
	MediaPublicBaseURL string                 `json:"media_public_base_url"`
	MediaPrefix        string                 `json:"media_prefix"`
}

type MediaStorageRuntimeConfig struct {
	ProfileID             string
	Enabled               bool
	Endpoint              string
	PublicBaseURL         string
	AccessKeyID           string
	SecretAccessKey       string
	Region                string
	Bucket                string
	ForcePathStyle        bool
	PresignExpiryMinutes  int
	MaxUploadSizeBytes    int64
	DownloadSigningSecret string
	ObjectPrefix          string
}

type MediaStorageConfigProvider struct {
	settingRepo SettingRepository
	encryptor   SecretEncryptor
	fallback    *config.Config

	mu        sync.RWMutex
	cached    ObjectStorageSettings
	expiresAt time.Time
}

func NewMediaStorageConfigProvider(settingRepo SettingRepository, encryptor SecretEncryptor, fallback *config.Config) *MediaStorageConfigProvider {
	return &MediaStorageConfigProvider{
		settingRepo: settingRepo,
		encryptor:   encryptor,
		fallback:    fallback,
	}
}

func (p *MediaStorageConfigProvider) Settings(ctx context.Context) (ObjectStorageSettings, error) {
	if p == nil {
		return fallbackObjectStorageSettings(nil), nil
	}

	now := time.Now()
	p.mu.RLock()
	if !p.expiresAt.IsZero() && now.Before(p.expiresAt) {
		cfg := p.cached
		p.mu.RUnlock()
		return cloneObjectStorageSettings(cfg), nil
	}
	p.mu.RUnlock()

	cfg, err := effectiveObjectStorageSettings(ctx, p.settingRepo, p.encryptor, p.fallback)
	if err != nil {
		return ObjectStorageSettings{}, err
	}

	p.mu.Lock()
	p.cached = cloneObjectStorageSettings(cfg)
	p.expiresAt = now.Add(objectStorageSettingsCacheTTL)
	p.mu.Unlock()

	return cloneObjectStorageSettings(cfg), nil
}

func (p *MediaStorageConfigProvider) Current(ctx context.Context) (MediaStorageRuntimeConfig, error) {
	settings, err := p.Settings(ctx)
	if err != nil {
		logger.LegacyPrintf("service.storage", "[Storage] load current media storage config failed, falling back to env config: %v", err)
		return defaultMediaStorageRuntimeConfig(p.fallback), nil
	}
	return resolveMediaStorageRuntimeConfig(settings, p.fallback), nil
}

func (p *MediaStorageConfigProvider) CurrentForAsset(ctx context.Context, profileID, bucket string) (MediaStorageRuntimeConfig, error) {
	if strings.TrimSpace(profileID) == "" {
		settings, err := p.Settings(ctx)
		if err != nil {
			cfg := defaultMediaStorageRuntimeConfig(p.fallback)
			if strings.TrimSpace(bucket) != "" && strings.TrimSpace(cfg.Bucket) == strings.TrimSpace(bucket) {
				logger.LegacyPrintf("service.storage", "[Storage] load media asset config failed, falling back to env config for bucket %q: %v", bucket, err)
				return cfg, nil
			}
			return MediaStorageRuntimeConfig{}, err
		}
		return resolveMediaStorageRuntimeConfigForAsset(settings, p.fallback, profileID, bucket), nil
	}

	settings, err := p.Settings(ctx)
	if err != nil {
		return MediaStorageRuntimeConfig{}, err
	}
	return resolveMediaStorageRuntimeConfigForAsset(settings, p.fallback, profileID, bucket), nil
}

func (p *MediaStorageConfigProvider) BackupConfig(ctx context.Context) (*BackupS3Config, error) {
	settings, err := p.Settings(ctx)
	if err != nil {
		return nil, err
	}
	return resolveBackupS3Config(settings), nil
}

// APIBaseURL returns the configured API endpoint base URL (system setting
// api_base_url), used to build absolute backend URLs when no inbound request
// context is available. Returns an empty string when unset or unavailable.
func (p *MediaStorageConfigProvider) APIBaseURL(ctx context.Context) string {
	if p == nil || p.settingRepo == nil {
		return ""
	}
	raw, err := p.settingRepo.GetValue(ctx, SettingKeyAPIBaseURL)
	if err != nil {
		return ""
	}
	return absoluteHTTPOrigin(raw)
}

func absoluteHTTPOrigin(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return ""
	}
	return (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String()
}

func (p *MediaStorageConfigProvider) Invalidate() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.cached = ObjectStorageSettings{}
	p.expiresAt = time.Time{}
	p.mu.Unlock()
}

func effectiveObjectStorageSettings(ctx context.Context, settingRepo SettingRepository, encryptor SecretEncryptor, fallback *config.Config) (ObjectStorageSettings, error) {
	if stored, err := loadStoredObjectStorageSettings(ctx, settingRepo, encryptor); err != nil {
		return fallbackObjectStorageSettings(fallback), err
	} else if stored != nil {
		return normalizeObjectStorageSettings(*stored), nil
	}

	if legacy, err := loadStoredLegacyBackupS3Config(ctx, settingRepo, encryptor); err != nil {
		return fallbackObjectStorageSettings(fallback), err
	} else if legacy != nil {
		return normalizeObjectStorageSettings(legacyObjectStorageSettings(*legacy, fallback)), nil
	}

	return fallbackObjectStorageSettings(fallback), nil
}

func loadStoredObjectStorageSettings(ctx context.Context, settingRepo SettingRepository, encryptor SecretEncryptor) (*ObjectStorageSettings, error) {
	if settingRepo == nil {
		return nil, nil
	}
	raw, err := settingRepo.GetValue(ctx, settingKeyObjectStorageConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil, nil
		}
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var cfg ObjectStorageSettings
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, ErrBackupS3ConfigCorrupt
	}
	for i := range cfg.Profiles {
		if cfg.Profiles[i].SecretAccessKey == "" || encryptor == nil {
			continue
		}
		decrypted, err := encryptor.Decrypt(cfg.Profiles[i].SecretAccessKey)
		if err != nil {
			logger.LegacyPrintf("service.storage", "[Storage] SecretAccessKey decrypt failed for profile %q (possibly legacy plaintext): %v", cfg.Profiles[i].ID, err)
		} else {
			cfg.Profiles[i].SecretAccessKey = decrypted
		}
	}
	return &cfg, nil
}

func loadStoredLegacyBackupS3Config(ctx context.Context, settingRepo SettingRepository, encryptor SecretEncryptor) (*BackupS3Config, error) {
	if settingRepo == nil {
		return nil, nil
	}
	raw, err := settingRepo.GetValue(ctx, settingKeyBackupS3Config)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil, nil
		}
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	var cfg BackupS3Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, ErrBackupS3ConfigCorrupt
	}
	if cfg.SecretAccessKey != "" && encryptor != nil {
		decrypted, err := encryptor.Decrypt(cfg.SecretAccessKey)
		if err != nil {
			logger.LegacyPrintf("service.storage", "[Storage] legacy SecretAccessKey decrypt failed (possibly plaintext): %v", err)
		} else {
			cfg.SecretAccessKey = decrypted
		}
	}
	return &cfg, nil
}

func fallbackObjectStorageSettings(fallback *config.Config) ObjectStorageSettings {
	settings := ObjectStorageSettings{
		BackupProfileID: "__disabled__",
		BackupPrefix:    "backups/",
	}
	if fallback == nil {
		return settings
	}

	profile := ObjectStorageProfile{
		ID:              "default",
		Name:            "Default Storage",
		Provider:        detectStorageProvider(strings.TrimSpace(fallback.Media.Endpoint)),
		Endpoint:        strings.TrimSpace(fallback.Media.Endpoint),
		Region:          firstNonEmpty(strings.TrimSpace(fallback.Media.Region), "auto"),
		Bucket:          strings.TrimSpace(fallback.Media.Bucket),
		AccessKeyID:     strings.TrimSpace(fallback.Media.AccessKeyID),
		SecretAccessKey: strings.TrimSpace(fallback.Media.SecretAccessKey),
		ForcePathStyle:  fallback.Media.ForcePathStyle,
	}
	if isObjectStorageProfileConfigured(profile) {
		settings.Profiles = []ObjectStorageProfile{profile}
		if fallback.Media.Enabled {
			settings.MediaEnabled = true
			settings.MediaProfileID = profile.ID
		}
	}
	settings.MediaPublicBaseURL = strings.TrimSpace(fallback.Media.PublicBaseURL)
	settings.MediaPrefix = ""
	return normalizeObjectStorageSettings(settings)
}

func legacyObjectStorageSettings(legacy BackupS3Config, fallback *config.Config) ObjectStorageSettings {
	profile := ObjectStorageProfile{
		ID:              "default",
		Name:            "Default Storage",
		Provider:        detectStorageProvider(strings.TrimSpace(legacy.Endpoint)),
		Endpoint:        strings.TrimSpace(legacy.Endpoint),
		Region:          firstNonEmpty(strings.TrimSpace(legacy.Region), "auto"),
		Bucket:          strings.TrimSpace(legacy.Bucket),
		AccessKeyID:     strings.TrimSpace(legacy.AccessKeyID),
		SecretAccessKey: strings.TrimSpace(legacy.SecretAccessKey),
		ForcePathStyle:  legacy.ForcePathStyle,
	}

	settings := ObjectStorageSettings{
		Profiles:           []ObjectStorageProfile{profile},
		BackupProfileID:    profile.ID,
		BackupPrefix:       firstNonEmpty(strings.TrimSpace(legacy.Prefix), "backups/"),
		MediaEnabled:       legacy.MediaEnabled != nil && *legacy.MediaEnabled,
		MediaProfileID:     profile.ID,
		MediaPublicBaseURL: strings.TrimSpace(legacy.MediaPublicBaseURL),
		MediaPrefix:        strings.TrimSpace(legacy.MediaPrefix),
	}
	if settings.MediaPublicBaseURL == "" && fallback != nil {
		settings.MediaPublicBaseURL = strings.TrimSpace(fallback.Media.PublicBaseURL)
	}
	return normalizeObjectStorageSettings(settings)
}

func normalizeObjectStorageSettings(settings ObjectStorageSettings) ObjectStorageSettings {
	settings.BackupPrefix = strings.TrimSpace(settings.BackupPrefix)
	if settings.BackupPrefix == "" {
		settings.BackupPrefix = "backups/"
	}
	settings.MediaPublicBaseURL = strings.TrimSpace(settings.MediaPublicBaseURL)
	settings.MediaPrefix = strings.Trim(strings.TrimSpace(settings.MediaPrefix), "/")

	normalizedProfiles := make([]ObjectStorageProfile, 0, len(settings.Profiles))
	seenIDs := make(map[string]struct{}, len(settings.Profiles))
	for i := range settings.Profiles {
		profile := settings.Profiles[i]
		profile.ID = strings.TrimSpace(profile.ID)
		if profile.ID == "" {
			profile.ID = uuid.NewString()
		}
		if _, exists := seenIDs[profile.ID]; exists {
			profile.ID = uuid.NewString()
		}
		seenIDs[profile.ID] = struct{}{}

		profile.Name = strings.TrimSpace(profile.Name)
		if profile.Name == "" {
			shortID := profile.ID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			profile.Name = "Storage " + shortID
		}
		profile.Provider = strings.ToLower(strings.TrimSpace(profile.Provider))
		if profile.Provider == "" {
			profile.Provider = detectStorageProvider(profile.Endpoint)
		}
		profile.Endpoint = strings.TrimSpace(profile.Endpoint)
		profile.Region = firstNonEmpty(strings.TrimSpace(profile.Region), "auto")
		profile.Bucket = strings.TrimSpace(profile.Bucket)
		profile.AccessKeyID = strings.TrimSpace(profile.AccessKeyID)
		profile.SecretAccessKey = strings.TrimSpace(profile.SecretAccessKey)
		profile.SecretConfigured = profile.SecretAccessKey != ""
		normalizedProfiles = append(normalizedProfiles, profile)
	}
	settings.Profiles = normalizedProfiles

	if settings.BackupProfileID == "" && len(settings.Profiles) == 1 {
		settings.BackupProfileID = settings.Profiles[0].ID
	}
	if settings.MediaProfileID == "" && len(settings.Profiles) == 1 {
		settings.MediaProfileID = settings.Profiles[0].ID
	}
	if !profileExists(settings.Profiles, settings.BackupProfileID) {
		settings.BackupProfileID = ""
	}
	if !profileExists(settings.Profiles, settings.MediaProfileID) {
		settings.MediaProfileID = ""
	}
	if !settings.MediaEnabled {
		settings.MediaProfileID = firstNonEmpty(settings.MediaProfileID, "")
	}
	return settings
}

func cloneObjectStorageSettings(settings ObjectStorageSettings) ObjectStorageSettings {
	cloned := settings
	cloned.Profiles = append([]ObjectStorageProfile(nil), settings.Profiles...)
	return cloned
}

func resolveBackupS3Config(settings ObjectStorageSettings) *BackupS3Config {
	profile, ok := findObjectStorageProfile(settings.Profiles, settings.BackupProfileID)
	if !ok || !isObjectStorageProfileConfigured(profile) {
		return nil
	}
	return &BackupS3Config{
		Endpoint:        profile.Endpoint,
		Region:          profile.Region,
		Bucket:          profile.Bucket,
		AccessKeyID:     profile.AccessKeyID,
		SecretAccessKey: profile.SecretAccessKey,
		Prefix:          settings.BackupPrefix,
		ForcePathStyle:  profile.ForcePathStyle,
	}
}

func resolveMediaStorageRuntimeConfig(settings ObjectStorageSettings, fallback *config.Config) MediaStorageRuntimeConfig {
	return resolveMediaStorageRuntimeConfigForAsset(settings, fallback, settings.MediaProfileID, "")
}

func resolveMediaStorageRuntimeConfigForAsset(settings ObjectStorageSettings, fallback *config.Config, profileID, bucket string) MediaStorageRuntimeConfig {
	cfg := defaultMediaStorageRuntimeConfig(fallback)
	if !settings.MediaEnabled {
		cfg.Enabled = false
		return cfg
	}

	profile, ok := selectMediaStorageProfile(settings.Profiles, profileID, bucket, settings.MediaProfileID)
	if !ok || !isObjectStorageProfileConfigured(profile) {
		cfg.Enabled = false
		return cfg
	}

	cfg.ProfileID = profile.ID
	cfg.Endpoint = profile.Endpoint
	cfg.AccessKeyID = profile.AccessKeyID
	cfg.SecretAccessKey = profile.SecretAccessKey
	cfg.Region = profile.Region
	cfg.Bucket = profile.Bucket
	cfg.ForcePathStyle = profile.ForcePathStyle
	cfg.PublicBaseURL = settings.MediaPublicBaseURL
	cfg.ObjectPrefix = settings.MediaPrefix
	cfg.Enabled = cfg.PublicBaseURL != ""
	return cfg
}

func selectMediaStorageProfile(profiles []ObjectStorageProfile, requestedID, bucket, defaultID string) (ObjectStorageProfile, bool) {
	if profile, ok := findObjectStorageProfile(profiles, requestedID); ok {
		return profile, true
	}
	if profile, ok := findUniqueObjectStorageProfileByBucket(profiles, bucket); ok {
		return profile, true
	}
	return findObjectStorageProfile(profiles, defaultID)
}

func defaultMediaStorageRuntimeConfig(fallback *config.Config) MediaStorageRuntimeConfig {
	cfg := MediaStorageRuntimeConfig{
		Region:               "auto",
		PresignExpiryMinutes: int(defaultMediaPresignTTL / time.Minute),
		MaxUploadSizeBytes:   defaultMediaMaxUploadSizeBytes,
	}
	if fallback == nil {
		return cfg
	}

	cfg.Enabled = fallback.Media.Enabled
	cfg.Endpoint = strings.TrimSpace(fallback.Media.Endpoint)
	cfg.PublicBaseURL = strings.TrimSpace(fallback.Media.PublicBaseURL)
	cfg.AccessKeyID = strings.TrimSpace(fallback.Media.AccessKeyID)
	cfg.SecretAccessKey = strings.TrimSpace(fallback.Media.SecretAccessKey)
	cfg.Region = firstNonEmpty(strings.TrimSpace(fallback.Media.Region), "auto")
	cfg.Bucket = strings.TrimSpace(fallback.Media.Bucket)
	cfg.ForcePathStyle = fallback.Media.ForcePathStyle
	if fallback.Media.PresignExpiryMinutes > 0 {
		cfg.PresignExpiryMinutes = fallback.Media.PresignExpiryMinutes
	}
	if fallback.Media.MaxUploadSizeBytes > 0 {
		cfg.MaxUploadSizeBytes = fallback.Media.MaxUploadSizeBytes
	}
	cfg.DownloadSigningSecret = strings.TrimSpace(fallback.Media.DownloadSigningSecret)
	cfg.Enabled = cfg.Enabled &&
		cfg.Endpoint != "" &&
		cfg.PublicBaseURL != "" &&
		cfg.AccessKeyID != "" &&
		cfg.SecretAccessKey != "" &&
		cfg.Bucket != ""
	return cfg
}

func isObjectStorageProfileConfigured(profile ObjectStorageProfile) bool {
	return strings.TrimSpace(profile.Bucket) != "" &&
		strings.TrimSpace(profile.AccessKeyID) != "" &&
		strings.TrimSpace(profile.SecretAccessKey) != ""
}

func profileExists(profiles []ObjectStorageProfile, id string) bool {
	_, ok := findObjectStorageProfile(profiles, id)
	return ok
}

func findObjectStorageProfile(profiles []ObjectStorageProfile, id string) (ObjectStorageProfile, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ObjectStorageProfile{}, false
	}
	for i := range profiles {
		if strings.TrimSpace(profiles[i].ID) == id {
			return profiles[i], true
		}
	}
	return ObjectStorageProfile{}, false
}

func findUniqueObjectStorageProfileByBucket(profiles []ObjectStorageProfile, bucket string) (ObjectStorageProfile, bool) {
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		return ObjectStorageProfile{}, false
	}
	var matched ObjectStorageProfile
	matchCount := 0
	for i := range profiles {
		if strings.TrimSpace(profiles[i].Bucket) != bucket {
			continue
		}
		matched = profiles[i]
		matchCount++
		if matchCount > 1 {
			return ObjectStorageProfile{}, false
		}
	}
	if matchCount == 1 {
		return matched, true
	}
	return ObjectStorageProfile{}, false
}

func detectStorageProvider(endpoint string) string {
	host := strings.ToLower(strings.TrimSpace(endpoint))
	switch {
	case strings.Contains(host, "cloudflarestorage.com"):
		return "r2"
	case strings.Contains(host, "aliyuncs.com"):
		return "oss"
	case strings.Contains(host, "minio"):
		return "minio"
	default:
		return "s3"
	}
}
