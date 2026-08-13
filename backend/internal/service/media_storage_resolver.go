package service

import (
	"context"
	"strings"
	"sync"
)

// BackupMediaResolver binds media storage to the configured backup S3 profile,
// using a media/ prefix so objects never mix with backup dumps.
type BackupMediaResolver struct {
	backup  *BackupService
	factory MediaObjectStoreFactory

	mu    sync.Mutex
	store MediaObjectStore
	cfg   *BackupS3Config
}

func NewBackupMediaResolver(backup *BackupService, factory MediaObjectStoreFactory) *BackupMediaResolver {
	return &BackupMediaResolver{backup: backup, factory: factory}
}

func (r *BackupMediaResolver) Resolve(ctx context.Context) (*MediaStorageBinding, MediaObjectStore, error) {
	if r == nil || r.backup == nil || r.factory == nil {
		return nil, nil, ErrMediaStorageNotConfigured
	}
	cfg, err := r.backup.ConfiguredS3(ctx)
	if err != nil {
		return nil, nil, err
	}
	if cfg == nil || !cfg.IsConfigured() {
		return nil, nil, ErrMediaStorageNotConfigured
	}
	if !cfg.MediaIsEnabled() {
		return nil, nil, ErrMediaStorageDisabled
	}

	store, err := r.getOrCreateStore(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}

	return &MediaStorageBinding{
		ProfileID:             domainMediaProfileID(),
		Prefix:                cfg.ResolvedMediaPrefix(),
		PublicBaseURL:         strings.TrimRight(strings.TrimSpace(cfg.MediaPublicBaseURL), "/"),
		DownloadSigningSecret: strings.TrimSpace(cfg.MediaDownloadSigningSecret),
	}, store, nil
}

func domainMediaProfileID() string {
	return "backup"
}

func (r *BackupMediaResolver) getOrCreateStore(ctx context.Context, cfg *BackupS3Config) (MediaObjectStore, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.store != nil && r.cfg != nil && sameMediaS3Cfg(r.cfg, cfg) {
		return r.store, nil
	}
	store, err := r.factory(ctx, cfg)
	if err != nil {
		return nil, err
	}
	r.store = store
	cloned := *cfg
	r.cfg = &cloned
	return store, nil
}

func sameMediaS3Cfg(a, b *BackupS3Config) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Endpoint == b.Endpoint &&
		a.Region == b.Region &&
		a.Bucket == b.Bucket &&
		a.AccessKeyID == b.AccessKeyID &&
		a.SecretAccessKey == b.SecretAccessKey &&
		a.ForcePathStyle == b.ForcePathStyle
}
