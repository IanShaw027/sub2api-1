package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type proxyUpdateRepoStub struct {
	proxyServiceRepoStub
	current *Proxy
	updated *Proxy
}

func (s *proxyUpdateRepoStub) GetByID(ctx context.Context, id int64) (*Proxy, error) {
	if s.current == nil {
		return nil, ErrProxyNotFound
	}
	proxy := *s.current
	return &proxy, nil
}

func (s *proxyUpdateRepoStub) Update(ctx context.Context, proxy *Proxy) error {
	copied := *proxy
	s.updated = &copied
	return nil
}

func TestAdminServiceUpdateProxyPreservesNullableFieldsWhenOmitted(t *testing.T) {
	expiresAt := time.Now().UTC().Add(48 * time.Hour).Round(time.Second)
	backupID := int64(42)
	repo := &proxyUpdateRepoStub{current: &Proxy{
		ID:             7,
		Name:           "primary",
		Protocol:       "http",
		Host:           "127.0.0.1",
		Port:           8080,
		Status:         StatusActive,
		ExpiresAt:      &expiresAt,
		FallbackMode:   FallbackModeProxy,
		BackupProxyID:  &backupID,
		ExpiryWarnDays: 5,
	}}
	svc := &adminServiceImpl{proxyRepo: repo}

	updated, err := svc.UpdateProxy(context.Background(), 7, &UpdateProxyInput{Status: "inactive"})

	require.NoError(t, err)
	require.Equal(t, "inactive", updated.Status)
	require.NotNil(t, updated.ExpiresAt)
	require.WithinDuration(t, expiresAt, *updated.ExpiresAt, time.Second)
	require.Equal(t, FallbackModeProxy, updated.FallbackMode)
	require.NotNil(t, updated.BackupProxyID)
	require.Equal(t, backupID, *updated.BackupProxyID)
	require.Equal(t, 5, updated.ExpiryWarnDays)
	require.Equal(t, updated, repo.updated)
}
