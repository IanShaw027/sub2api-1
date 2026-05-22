package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestRedeemService_BatchUpdate_PartialFields(t *testing.T) {
	status := StatusDisabled
	notes := "maintenance window"
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	repo := &redeemRepoStub{
		getByID: map[int64]*RedeemCode{
			1: {ID: 1, Type: RedeemTypeBalance, Status: StatusUnused},
			2: {ID: 2, Type: RedeemTypeBalance, Status: StatusUnused},
		},
	}
	svc := &RedeemService{redeemRepo: repo}

	result, err := svc.BatchUpdate(context.Background(), &RedeemCodeBatchUpdateInput{
		IDs: []int64{1, 2, 2},
		Fields: RedeemCodeBatchUpdateFields{
			Status:    &status,
			ExpiresAt: NullableTimeUpdate{Set: true, Value: &expiresAt},
			Notes:     &notes,
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(2), result.Updated)
	require.True(t, repo.batchUpdateCalled)
	require.Equal(t, []int64{1, 2}, repo.batchUpdateIDs)
	require.Equal(t, &status, repo.batchUpdateFields.Status)
	require.True(t, repo.batchUpdateFields.ExpiresAt.Set)
	require.WithinDuration(t, expiresAt, *repo.batchUpdateFields.ExpiresAt.Value, time.Second)
	require.Equal(t, &notes, repo.batchUpdateFields.Notes)
	require.False(t, repo.batchUpdateFields.GroupID.Set)
	require.Nil(t, repo.batchUpdateFields.Type)
	require.Nil(t, repo.batchUpdateFields.Value)
}

func TestRedeemService_BatchUpdate_RejectsInvalidID(t *testing.T) {
	repo := &redeemRepoStub{}
	svc := &RedeemService{redeemRepo: repo}
	notes := "bad id"

	result, err := svc.BatchUpdate(context.Background(), &RedeemCodeBatchUpdateInput{
		IDs:    []int64{1, 0},
		Fields: RedeemCodeBatchUpdateFields{Notes: &notes},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.False(t, repo.batchUpdateCalled)
}

func TestRedeemService_BatchUpdate_RejectsCoreFieldsForUsedCodes(t *testing.T) {
	repo := &redeemRepoStub{}
	svc := &RedeemService{redeemRepo: repo}
	newValue := 100.0

	result, err := svc.BatchUpdate(context.Background(), &RedeemCodeBatchUpdateInput{
		IDs: []int64{42},
		Fields: RedeemCodeBatchUpdateFields{
			Value: &newValue,
		},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.False(t, repo.batchUpdateCalled)
}

func TestRedeemService_BatchUpdate_RejectsSubscriptionCodeWithoutFinalGroup(t *testing.T) {
	repo := &redeemRepoStub{
		getByID: map[int64]*RedeemCode{
			42: {
				ID:     42,
				Type:   RedeemTypeSubscription,
				Status: StatusUnused,
			},
		},
	}
	svc := &RedeemService{redeemRepo: repo}
	notes := "touch"

	result, err := svc.BatchUpdate(context.Background(), &RedeemCodeBatchUpdateInput{
		IDs: []int64{42},
		Fields: RedeemCodeBatchUpdateFields{
			Notes: &notes,
		},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.False(t, repo.batchUpdateCalled)
}

func TestRedeemService_BatchUpdate_RejectsSubscriptionCodeWithNonSubscriptionGroup(t *testing.T) {
	currentGroupID := int64(7)
	newGroupID := int64(9)
	repo := &redeemRepoStub{
		getByID: map[int64]*RedeemCode{
			42: {
				ID:      42,
				Type:    RedeemTypeSubscription,
				Status:  StatusUnused,
				GroupID: &currentGroupID,
			},
		},
	}
	svc := &RedeemService{
		redeemRepo: repo,
		subscriptionService: &SubscriptionService{
			groupRepo: &kiroDefaultGroupRepoStub{
				getByID: &Group{ID: newGroupID, SubscriptionType: SubscriptionTypeStandard},
			},
		},
	}

	result, err := svc.BatchUpdate(context.Background(), &RedeemCodeBatchUpdateInput{
		IDs: []int64{42},
		Fields: RedeemCodeBatchUpdateFields{
			GroupID: NullableInt64Update{Set: true, Value: &newGroupID},
		},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.False(t, repo.batchUpdateCalled)
}

func TestRedeemService_BatchUpdate_AllowsSubscriptionCodeWhenExistingGroupRemainsValid(t *testing.T) {
	currentGroupID := int64(7)
	repo := &redeemRepoStub{
		getByID: map[int64]*RedeemCode{
			42: {
				ID:      42,
				Type:    RedeemTypeSubscription,
				Status:  StatusUnused,
				GroupID: &currentGroupID,
			},
		},
	}
	svc := &RedeemService{
		redeemRepo: repo,
		subscriptionService: &SubscriptionService{
			groupRepo: &kiroDefaultGroupRepoStub{
				getByID: &Group{ID: currentGroupID, SubscriptionType: SubscriptionTypeSubscription},
			},
		},
	}
	notes := "touch"

	result, err := svc.BatchUpdate(context.Background(), &RedeemCodeBatchUpdateInput{
		IDs: []int64{42},
		Fields: RedeemCodeBatchUpdateFields{
			Notes: &notes,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, int64(1), result.Updated)
	require.True(t, repo.batchUpdateCalled)
	require.Equal(t, []int64{42}, repo.batchUpdateIDs)
}

func TestRedeemService_BatchUpdate_RejectsPastExpiresAt(t *testing.T) {
	repo := &redeemRepoStub{}
	svc := &RedeemService{redeemRepo: repo}
	expiresAt := time.Now().UTC().Add(-time.Hour)

	result, err := svc.BatchUpdate(context.Background(), &RedeemCodeBatchUpdateInput{
		IDs: []int64{42},
		Fields: RedeemCodeBatchUpdateFields{
			ExpiresAt: NullableTimeUpdate{Set: true, Value: &expiresAt},
		},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.False(t, repo.batchUpdateCalled)
}

func TestRedeemService_BatchUpdate_RejectsNonPositiveGroupID(t *testing.T) {
	repo := &redeemRepoStub{}
	svc := &RedeemService{redeemRepo: repo}
	groupID := int64(0)

	result, err := svc.BatchUpdate(context.Background(), &RedeemCodeBatchUpdateInput{
		IDs: []int64{42},
		Fields: RedeemCodeBatchUpdateFields{
			GroupID: NullableInt64Update{Set: true, Value: &groupID},
		},
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.False(t, repo.batchUpdateCalled)
}
