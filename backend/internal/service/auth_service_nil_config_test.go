//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAuthService_NilConfigAllowsTokenMethodsAfterPostInitConfig(t *testing.T) {
	service := NewAuthService(nil, &userRepoStub{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	require.NotNil(t, service)
	require.NotNil(t, service.cfg)

	service.cfg.JWT.Secret = "test-secret"
	service.cfg.JWT.ExpireHour = 1

	user := &User{
		ID:           1,
		Email:        "nil-config@test.com",
		Role:         RoleUser,
		Status:       StatusActive,
		TokenVersion: 1,
	}

	token, err := service.GenerateToken(user)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := service.ValidateToken(token)
	require.NoError(t, err)
	require.NotNil(t, claims)
	require.Equal(t, user.ID, claims.UserID)
}
