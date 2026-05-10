//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestFallbackObjectStorageSettings_DoesNotEnableBackupFromMediaEnv(t *testing.T) {
	cfg := &config.Config{}
	cfg.Media.Enabled = true
	cfg.Media.Endpoint = "https://account.r2.cloudflarestorage.com"
	cfg.Media.Region = "auto"
	cfg.Media.Bucket = "media-bucket"
	cfg.Media.AccessKeyID = "media-ak"
	cfg.Media.SecretAccessKey = "media-secret"
	cfg.Media.PublicBaseURL = "https://source.example.com"

	settings := fallbackObjectStorageSettings(cfg)
	require.Len(t, settings.Profiles, 1)
	require.Empty(t, settings.BackupProfileID)
	require.True(t, settings.MediaEnabled)
	require.Equal(t, "default", settings.MediaProfileID)
}

func TestResolveMediaStorageRuntimeConfigForAsset_PrefersStoredProfileThenBucket(t *testing.T) {
	settings := ObjectStorageSettings{
		Profiles: []ObjectStorageProfile{
			{
				ID:              "old",
				Endpoint:        "https://old.example.com",
				Region:          "auto",
				Bucket:          "old-bucket",
				AccessKeyID:     "old-ak",
				SecretAccessKey: "old-sk",
			},
			{
				ID:              "new",
				Endpoint:        "https://new.example.com",
				Region:          "auto",
				Bucket:          "new-bucket",
				AccessKeyID:     "new-ak",
				SecretAccessKey: "new-sk",
			},
		},
		MediaEnabled:       true,
		MediaProfileID:     "new",
		MediaPublicBaseURL: "https://source.example.com",
		MediaPrefix:        "media",
	}

	byProfile := resolveMediaStorageRuntimeConfigForAsset(settings, nil, "old", "")
	require.True(t, byProfile.Enabled)
	require.Equal(t, "old", byProfile.ProfileID)
	require.Equal(t, "https://old.example.com", byProfile.Endpoint)
	require.Equal(t, "old-bucket", byProfile.Bucket)

	byBucket := resolveMediaStorageRuntimeConfigForAsset(settings, nil, "", "old-bucket")
	require.True(t, byBucket.Enabled)
	require.Equal(t, "old", byBucket.ProfileID)
	require.Equal(t, "https://old.example.com", byBucket.Endpoint)
	require.Equal(t, "old-bucket", byBucket.Bucket)
}
