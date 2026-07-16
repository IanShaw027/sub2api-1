package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryCreateWithInvitationRollsBackUserWhenInvitationUpdateMisses(t *testing.T) {
	repo, client := newUserEntRepo(t)
	ctx := context.Background()
	expiredAt := time.Now().UTC().Add(-time.Minute)
	invitation, err := client.RedeemCode.Create().
		SetCode("EXPIRED-INVITE").
		SetType(service.RedeemTypeInvitation).
		SetStatus(service.StatusUnused).
		SetExpiresAt(expiredAt).
		Save(ctx)
	require.NoError(t, err)

	candidate := &service.User{
		Email:        "atomic-rollback@example.com",
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	}
	err = repo.CreateWithInvitation(ctx, candidate, invitation.ID)
	require.ErrorIs(t, err, service.ErrRedeemCodeUsed)

	exists, queryErr := client.User.Query().
		Where(user.EmailEQ(candidate.Email)).
		Exist(ctx)
	require.NoError(t, queryErr)
	require.False(t, exists, "user creation must roll back when invitation cannot be consumed")

	reloaded, queryErr := client.RedeemCode.Query().
		Where(redeemcode.IDEQ(invitation.ID)).
		Only(ctx)
	require.NoError(t, queryErr)
	require.Equal(t, service.StatusUnused, reloaded.Status)
	require.Nil(t, reloaded.UsedBy)
}

func TestUserRepositoryCreateWithInvitationCommitsBothMutations(t *testing.T) {
	repo, client := newUserEntRepo(t)
	ctx := context.Background()
	invitation, err := client.RedeemCode.Create().
		SetCode("VALID-INVITE").
		SetType(service.RedeemTypeInvitation).
		SetStatus(service.StatusUnused).
		Save(ctx)
	require.NoError(t, err)

	candidate := &service.User{
		Email:        "atomic-success@example.com",
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	}
	require.NoError(t, repo.CreateWithInvitation(ctx, candidate, invitation.ID))
	require.Positive(t, candidate.ID)

	reloaded, err := client.RedeemCode.Get(ctx, invitation.ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusUsed, reloaded.Status)
	require.NotNil(t, reloaded.UsedBy)
	require.Equal(t, candidate.ID, *reloaded.UsedBy)

	_, err = client.User.Query().Where(user.IDEQ(candidate.ID)).Only(ctx)
	require.NoError(t, err)
}
