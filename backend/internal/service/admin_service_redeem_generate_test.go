//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdminService_GenerateRedeemCodes_PersistsExpiresAt(t *testing.T) {
	expiresAt := time.Now().UTC().Add(48 * time.Hour).Round(time.Second)
	redeemRepo := &redeemRepoStub{}
	svc := &adminServiceImpl{
		redeemCodeRepo: redeemRepo,
		groupRepo: &kiroDefaultGroupRepoStub{
			getByID: &Group{ID: 11, SubscriptionType: SubscriptionTypeSubscription},
		},
	}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:     1,
		Type:      RedeemTypeBalance,
		Value:     10,
		ExpiresAt: &expiresAt,
	})

	require.NoError(t, err)
	require.Len(t, codes, 1)
	require.NotNil(t, codes[0].ExpiresAt)
	require.WithinDuration(t, expiresAt, *codes[0].ExpiresAt, time.Second)
	require.Len(t, redeemRepo.created, 1)
	require.NotNil(t, redeemRepo.created[0].ExpiresAt)
	require.WithinDuration(t, expiresAt, *redeemRepo.created[0].ExpiresAt, time.Second)
}

func TestAdminService_GenerateRedeemCodes_RejectsNilInput(t *testing.T) {
	redeemRepo := &redeemRepoStub{}
	svc := &adminServiceImpl{redeemCodeRepo: redeemRepo}

	codes, err := svc.GenerateRedeemCodes(context.Background(), nil)

	require.Nil(t, codes)
	require.Error(t, err)
	require.ErrorContains(t, err, "redeem input is required")
	require.Empty(t, redeemRepo.created)
}

func TestAdminService_GenerateRedeemCodes_RejectsPastExpiresAt(t *testing.T) {
	expiresAt := time.Now().UTC().Add(-time.Hour)
	redeemRepo := &redeemRepoStub{}
	svc := &adminServiceImpl{
		redeemCodeRepo: redeemRepo,
		groupRepo: &kiroDefaultGroupRepoStub{
			getByID: &Group{ID: 11, SubscriptionType: SubscriptionTypeSubscription},
		},
	}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:     1,
		Type:      RedeemTypeBalance,
		Value:     10,
		ExpiresAt: &expiresAt,
	})

	require.Nil(t, codes)
	require.ErrorContains(t, err, "expires_at must be in the future")
	require.Empty(t, redeemRepo.created)
}

func TestAdminService_GenerateRedeemCodes_SubscriptionRequiresGroupID(t *testing.T) {
	svc := &adminServiceImpl{redeemCodeRepo: &redeemRepoStub{}}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 1,
		Type:  RedeemTypeSubscription,
		Value: 10,
	})

	require.Nil(t, codes)
	require.ErrorContains(t, err, "group_id is required for subscription type")
}

func TestAdminService_GenerateRedeemCodes_SubscriptionRejectsNonSubscriptionGroup(t *testing.T) {
	groupID := int64(11)
	svc := &adminServiceImpl{
		redeemCodeRepo: &redeemRepoStub{},
		groupRepo: &kiroDefaultGroupRepoStub{
			getByID: &Group{ID: groupID, SubscriptionType: SubscriptionTypeStandard},
		},
	}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:   1,
		Type:    RedeemTypeSubscription,
		Value:   10,
		GroupID: &groupID,
	})

	require.Nil(t, codes)
	require.ErrorContains(t, err, "group must be subscription type")
}

func TestAdminService_GenerateRedeemCodes_SubscriptionDefaultsNonPositiveValidityDaysTo30(t *testing.T) {
	groupID := int64(22)
	redeemRepo := &redeemRepoStub{}
	svc := &adminServiceImpl{
		redeemCodeRepo: redeemRepo,
		groupRepo: &kiroDefaultGroupRepoStub{
			getByID: &Group{ID: groupID, SubscriptionType: SubscriptionTypeSubscription},
		},
	}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:        1,
		Type:         RedeemTypeSubscription,
		Value:        10,
		GroupID:      &groupID,
		ValidityDays: 0,
	})

	require.NoError(t, err)
	require.Len(t, codes, 1)
	require.NotNil(t, codes[0].GroupID)
	require.Equal(t, groupID, *codes[0].GroupID)
	require.Equal(t, 30, codes[0].ValidityDays)
	require.Len(t, redeemRepo.created, 1)
	require.Equal(t, 30, redeemRepo.created[0].ValidityDays)
}

func TestAdminService_GenerateRedeemCodes_SubscriptionPreservesPositiveValidityDays(t *testing.T) {
	groupID := int64(23)
	redeemRepo := &redeemRepoStub{}
	svc := &adminServiceImpl{
		redeemCodeRepo: redeemRepo,
		groupRepo: &kiroDefaultGroupRepoStub{
			getByID: &Group{ID: groupID, SubscriptionType: SubscriptionTypeSubscription},
		},
	}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:        1,
		Type:         RedeemTypeSubscription,
		Value:        10,
		GroupID:      &groupID,
		ValidityDays: 45,
	})

	require.NoError(t, err)
	require.Len(t, codes, 1)
	require.Equal(t, 45, codes[0].ValidityDays)
	require.Len(t, redeemRepo.created, 1)
	require.Equal(t, 45, redeemRepo.created[0].ValidityDays)
}

func TestAdminService_GenerateRedeemCodes_SubscriptionGroupValidationUnavailable(t *testing.T) {
	groupID := int64(24)
	svc := &adminServiceImpl{redeemCodeRepo: &redeemRepoStub{}}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:   1,
		Type:    RedeemTypeSubscription,
		Value:   10,
		GroupID: &groupID,
	})

	require.Nil(t, codes)
	require.ErrorContains(t, err, "subscription group validation unavailable")
}

func TestAdminService_GenerateRedeemCodes_SubscriptionRejectsNilGroupResult(t *testing.T) {
	groupID := int64(25)
	svc := &adminServiceImpl{
		redeemCodeRepo: &redeemRepoStub{},
		groupRepo:      &kiroDefaultGroupRepoStub{},
	}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:   1,
		Type:    RedeemTypeSubscription,
		Value:   10,
		GroupID: &groupID,
	})

	require.Nil(t, codes)
	require.ErrorContains(t, err, "group must be subscription type")
}
