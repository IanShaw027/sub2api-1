package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type announcementRepoStub struct {
	item *Announcement
	active []Announcement
}

func (s *announcementRepoStub) Create(_ context.Context, a *Announcement) error {
	s.item = a
	return nil
}

func (s *announcementRepoStub) GetByID(_ context.Context, _ int64) (*Announcement, error) {
	if s.item == nil {
		return nil, ErrAnnouncementNotFound
	}
	return s.item, nil
}

func (s *announcementRepoStub) Update(_ context.Context, a *Announcement) error {
	s.item = a
	return nil
}

func (*announcementRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (*announcementRepoStub) List(context.Context, pagination.PaginationParams, AnnouncementListFilters) ([]Announcement, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *announcementRepoStub) ListActive(context.Context, time.Time) ([]Announcement, error) {
	return s.active, nil
}

type announcementReadRepoStub struct {
	readMap map[int64]time.Time
}

func (*announcementReadRepoStub) MarkRead(context.Context, int64, int64, time.Time) error {
	return nil
}

func (s *announcementReadRepoStub) GetReadMapByUser(context.Context, int64, []int64) (map[int64]time.Time, error) {
	return s.readMap, nil
}

func (*announcementReadRepoStub) GetReadMapByUsers(context.Context, int64, []int64) (map[int64]time.Time, error) {
	return map[int64]time.Time{}, nil
}

func (*announcementReadRepoStub) CountByAnnouncementID(context.Context, int64) (int64, error) {
	return 0, nil
}

type announcementUserRepoStub struct{}

func (*announcementUserRepoStub) Create(context.Context, *User) error { return nil }
func (*announcementUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	return &User{ID: 1, Balance: 10}, nil
}
func (*announcementUserRepoStub) GetByEmail(context.Context, string) (*User, error) { return nil, nil }
func (*announcementUserRepoStub) GetFirstAdmin(context.Context) (*User, error) { return nil, nil }
func (*announcementUserRepoStub) Update(context.Context, *User) error { return nil }
func (*announcementUserRepoStub) Delete(context.Context, int64) error { return nil }
func (*announcementUserRepoStub) GetUserAvatar(context.Context, int64) (*UserAvatar, error) { return nil, nil }
func (*announcementUserRepoStub) UpsertUserAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	return nil, nil
}
func (*announcementUserRepoStub) DeleteUserAvatar(context.Context, int64) error { return nil }
func (*announcementUserRepoStub) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (*announcementUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (*announcementUserRepoStub) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	return nil, nil
}
func (*announcementUserRepoStub) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) { return nil, nil }
func (*announcementUserRepoStub) UpdateUserLastActiveAt(context.Context, int64, time.Time) error { return nil }
func (*announcementUserRepoStub) UpdateBalance(context.Context, int64, float64) error { return nil }
func (*announcementUserRepoStub) DeductBalance(context.Context, int64, float64) error { return nil }
func (*announcementUserRepoStub) UpdateConcurrency(context.Context, int64, int) error { return nil }
func (*announcementUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) { return false, nil }
func (*announcementUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) { return 0, nil }
func (*announcementUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error { return nil }
func (*announcementUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error { return nil }
func (*announcementUserRepoStub) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	return nil, nil
}
func (*announcementUserRepoStub) UnbindUserAuthProvider(context.Context, int64, string) error { return nil }
func (*announcementUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error { return nil }
func (*announcementUserRepoStub) EnableTotp(context.Context, int64) error { return nil }
func (*announcementUserRepoStub) DisableTotp(context.Context, int64) error { return nil }

type announcementUserSubRepoStub struct{}

func (*announcementUserSubRepoStub) Create(context.Context, *UserSubscription) error { return nil }
func (*announcementUserSubRepoStub) GetByID(context.Context, int64) (*UserSubscription, error) { return nil, nil }
func (*announcementUserSubRepoStub) GetByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	return nil, nil
}
func (*announcementUserSubRepoStub) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	return nil, nil
}
func (*announcementUserSubRepoStub) Update(context.Context, *UserSubscription) error { return nil }
func (*announcementUserSubRepoStub) Delete(context.Context, int64) error { return nil }
func (*announcementUserSubRepoStub) ListByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}
func (*announcementUserSubRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}
func (*announcementUserSubRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]UserSubscription, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (*announcementUserSubRepoStub) List(context.Context, pagination.PaginationParams, *int64, *int64, string, string, string, string) ([]UserSubscription, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (*announcementUserSubRepoStub) ExistsByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func (*announcementUserSubRepoStub) ExtendExpiry(context.Context, int64, time.Time) error { return nil }
func (*announcementUserSubRepoStub) UpdateStatus(context.Context, int64, string) error { return nil }
func (*announcementUserSubRepoStub) UpdateNotes(context.Context, int64, string) error { return nil }
func (*announcementUserSubRepoStub) ActivateWindows(context.Context, int64, time.Time) error { return nil }
func (*announcementUserSubRepoStub) ResetDailyUsage(context.Context, int64, time.Time) error { return nil }
func (*announcementUserSubRepoStub) ResetWeeklyUsage(context.Context, int64, time.Time) error { return nil }
func (*announcementUserSubRepoStub) ResetMonthlyUsage(context.Context, int64, time.Time) error { return nil }
func (*announcementUserSubRepoStub) IncrementUsage(context.Context, int64, float64) error { return nil }
func (*announcementUserSubRepoStub) BatchUpdateExpiredStatus(context.Context) (int64, error) { return 0, nil }

func TestAnnouncementServiceCreateRejectsEqualStartEndTimes(t *testing.T) {
	repo := &announcementRepoStub{}
	svc := NewAnnouncementService(repo, nil, nil, nil)
	now := time.Unix(1776790020, 0)

	_, err := svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:      "公告",
		Content:    "内容",
		Status:     AnnouncementStatusActive,
		NotifyMode: AnnouncementNotifyModePopup,
		StartsAt:   &now,
		EndsAt:     &now,
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidSchedule)
}

func TestAnnouncementServiceUpdateRejectsEqualStartEndTimes(t *testing.T) {
	repo := &announcementRepoStub{
		item: &Announcement{
			ID:         1,
			Title:      "公告",
			Content:    "内容",
			Status:     AnnouncementStatusActive,
			NotifyMode: AnnouncementNotifyModePopup,
		},
	}
	svc := NewAnnouncementService(repo, nil, nil, nil)
	now := time.Unix(1776790020, 0)
	startsAt := &now
	endsAt := &now

	_, err := svc.Update(context.Background(), 1, &UpdateAnnouncementInput{
		StartsAt: &startsAt,
		EndsAt:   &endsAt,
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidSchedule)
}

func TestNormalizeAnnouncementReadStatus(t *testing.T) {
	require.Equal(t, AnnouncementReadStatusAll, NormalizeAnnouncementReadStatus(""))
	require.Equal(t, AnnouncementReadStatusAll, NormalizeAnnouncementReadStatus("weird"))
	require.Equal(t, AnnouncementReadStatusRead, NormalizeAnnouncementReadStatus(AnnouncementReadStatusRead))
	require.Equal(t, AnnouncementReadStatusUnread, NormalizeAnnouncementReadStatus(AnnouncementReadStatusUnread))
}

func TestAnnouncementServiceListForUserReadStatus(t *testing.T) {
	now := time.Unix(1776790020, 0)
	repo := &announcementRepoStub{
		active: []Announcement{
			{ID: 1, Title: "unread", Content: "a", Status: AnnouncementStatusActive},
			{ID: 2, Title: "read", Content: "b", Status: AnnouncementStatusActive},
		},
	}
	readRepo := &announcementReadRepoStub{
		readMap: map[int64]time.Time{2: now},
	}
	svc := NewAnnouncementService(repo, readRepo, &announcementUserRepoStub{}, &announcementUserSubRepoStub{})

	all, err := svc.ListForUser(context.Background(), 1, AnnouncementReadStatusAll)
	require.NoError(t, err)
	require.Len(t, all, 2)
	require.Equal(t, int64(1), all[0].Announcement.ID)
	require.Nil(t, all[0].ReadAt)
	require.Equal(t, int64(2), all[1].Announcement.ID)
	require.NotNil(t, all[1].ReadAt)

	unread, err := svc.ListForUser(context.Background(), 1, AnnouncementReadStatusUnread)
	require.NoError(t, err)
	require.Len(t, unread, 1)
	require.Equal(t, int64(1), unread[0].Announcement.ID)

	read, err := svc.ListForUser(context.Background(), 1, AnnouncementReadStatusRead)
	require.NoError(t, err)
	require.Len(t, read, 1)
	require.Equal(t, int64(2), read[0].Announcement.ID)
}
