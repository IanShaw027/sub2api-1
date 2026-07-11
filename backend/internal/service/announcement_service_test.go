package service

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type announcementRepoStub struct {
	item   *Announcement
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
	readMap        map[int64]time.Time
	readMapByUsers map[int64]time.Time
}

func (*announcementReadRepoStub) MarkRead(context.Context, int64, int64, time.Time) error {
	return nil
}

func (s *announcementReadRepoStub) GetReadMapByUser(context.Context, int64, []int64) (map[int64]time.Time, error) {
	return s.readMap, nil
}

func (s *announcementReadRepoStub) GetReadMapByUsers(context.Context, int64, []int64) (map[int64]time.Time, error) {
	if s.readMapByUsers == nil {
		return map[int64]time.Time{}, nil
	}
	return s.readMapByUsers, nil
}

func (*announcementReadRepoStub) CountByAnnouncementID(context.Context, int64) (int64, error) {
	return 0, nil
}

type announcementUserRepoStub struct {
	user            *User
	users           []User
	listWithFilters []UserListFilters
}

func (*announcementUserRepoStub) Create(context.Context, *User) error { return nil }
func (s *announcementUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	if s.user != nil {
		return s.user, nil
	}
	return &User{ID: 1, Balance: 10}, nil
}
func (s *announcementUserRepoStub) GetByIDIncludeDeleted(ctx context.Context, id int64) (*User, error) {
	return s.GetByID(ctx, id)
}
func (*announcementUserRepoStub) GetByEmail(context.Context, string) (*User, error) { return nil, nil }
func (*announcementUserRepoStub) GetFirstAdmin(context.Context) (*User, error)      { return nil, nil }
func (*announcementUserRepoStub) Update(context.Context, *User) error               { return nil }
func (*announcementUserRepoStub) Delete(context.Context, int64) error               { return nil }
func (*announcementUserRepoStub) AddBalanceWithoutRecharge(context.Context, int64, float64) error {
	return nil
}
func (*announcementUserRepoStub) GetUserAvatar(context.Context, int64) (*UserAvatar, error) {
	return nil, nil
}
func (*announcementUserRepoStub) UpsertUserAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	return nil, nil
}
func (*announcementUserRepoStub) DeleteUserAvatar(context.Context, int64) error { return nil }
func (*announcementUserRepoStub) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *announcementUserRepoStub) ListWithFilters(_ context.Context, params pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error) {
	s.listWithFilters = append(s.listWithFilters, filters)

	filtered := make([]User, 0, len(s.users))
	needle := strings.ToLower(strings.TrimSpace(filters.Search))
	for _, user := range s.users {
		if needle != "" {
			email := strings.ToLower(user.Email)
			username := strings.ToLower(user.Username)
			if !strings.Contains(email, needle) && !strings.Contains(username, needle) {
				continue
			}
		}
		filtered = append(filtered, user)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Email < filtered[j].Email
	})

	limit := params.Limit()
	offset := params.Offset()
	if offset >= len(filtered) {
		return []User{}, &pagination.PaginationResult{
			Total:    int64(len(filtered)),
			Page:     params.Page,
			PageSize: limit,
			Pages:    pageCount(len(filtered), limit),
		}, nil
	}

	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end], &pagination.PaginationResult{
		Total:    int64(len(filtered)),
		Page:     params.Page,
		PageSize: limit,
		Pages:    pageCount(len(filtered), limit),
	}, nil
}
func (*announcementUserRepoStub) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	return nil, nil
}
func (*announcementUserRepoStub) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	return nil, nil
}
func (*announcementUserRepoStub) UpdateUserLastActiveAt(context.Context, int64, time.Time) error {
	return nil
}
func (*announcementUserRepoStub) UpdateBalance(context.Context, int64, float64) error { return nil }
func (*announcementUserRepoStub) DeductBalance(context.Context, int64, float64) error { return nil }
func (*announcementUserRepoStub) UpdateConcurrency(context.Context, int64, int) error { return nil }
func (*announcementUserRepoStub) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (*announcementUserRepoStub) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (*announcementUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	return false, nil
}
func (*announcementUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	return 0, nil
}
func (*announcementUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (*announcementUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (*announcementUserRepoStub) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	return nil, nil
}
func (*announcementUserRepoStub) UnbindUserAuthProvider(context.Context, int64, string) error {
	return nil
}
func (*announcementUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error { return nil }
func (*announcementUserRepoStub) EnableTotp(context.Context, int64) error                { return nil }
func (*announcementUserRepoStub) DisableTotp(context.Context, int64) error               { return nil }

type announcementUserSubRepoStub struct {
	activeByUserID map[int64][]UserSubscription
}

func (*announcementUserSubRepoStub) Create(context.Context, *UserSubscription) error { return nil }
func (*announcementUserSubRepoStub) GetByID(context.Context, int64) (*UserSubscription, error) {
	return nil, nil
}
func (*announcementUserSubRepoStub) GetByIDIncludeDeleted(context.Context, int64) (*UserSubscription, error) {
	return nil, nil
}
func (*announcementUserSubRepoStub) GetByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	return nil, nil
}
func (*announcementUserSubRepoStub) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	return nil, nil
}
func (*announcementUserSubRepoStub) Update(context.Context, *UserSubscription) error { return nil }
func (*announcementUserSubRepoStub) Delete(context.Context, int64) error             { return nil }
func (*announcementUserSubRepoStub) Restore(context.Context, int64, string) (*UserSubscription, error) {
	return nil, nil
}
func (*announcementUserSubRepoStub) ListByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}
func (s *announcementUserSubRepoStub) ListActiveByUserID(_ context.Context, userID int64) ([]UserSubscription, error) {
	if s.activeByUserID == nil {
		return nil, nil
	}
	return s.activeByUserID[userID], nil
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
func (*announcementUserSubRepoStub) ExistsActiveByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	return false, nil
}
func (*announcementUserSubRepoStub) ExtendExpiry(context.Context, int64, time.Time) error { return nil }
func (*announcementUserSubRepoStub) UpdateStatus(context.Context, int64, string) error    { return nil }
func (*announcementUserSubRepoStub) UpdateNotes(context.Context, int64, string) error     { return nil }
func (*announcementUserSubRepoStub) ActivateWindows(context.Context, int64, time.Time) error {
	return nil
}
func (*announcementUserSubRepoStub) ResetUsageWindows(context.Context, int64, bool, bool, bool, time.Time) error {
	return nil
}
func (*announcementUserSubRepoStub) ResetDailyUsage(context.Context, int64, *time.Time, time.Time) error {
	return nil
}
func (*announcementUserSubRepoStub) ResetWeeklyUsage(context.Context, int64, *time.Time, time.Time) error {
	return nil
}
func (*announcementUserSubRepoStub) ResetMonthlyUsage(context.Context, int64, *time.Time, time.Time) error {
	return nil
}
func (*announcementUserSubRepoStub) IncrementUsage(context.Context, int64, float64) error { return nil }
func (*announcementUserSubRepoStub) BatchUpdateExpiredStatus(context.Context) (int64, error) {
	return 0, nil
}

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

func TestAnnouncementServiceListUserReadStatusPaginatesEligibleRecipientsOnly(t *testing.T) {
	now := time.Unix(1776790020, 0)
	announcementID := int64(9)
	userRepo := &announcementUserRepoStub{
		users: []User{
			{ID: 1, Email: "eligible-unread@example.com"},
			{ID: 2, Email: "ineligible-read@example.com"},
			{ID: 3, Email: "eligible-read@example.com"},
		},
	}
	readRepo := &announcementReadRepoStub{
		readMapByUsers: map[int64]time.Time{
			2: now,
			3: now,
		},
	}
	userSubRepo := &announcementUserSubRepoStub{
		activeByUserID: map[int64][]UserSubscription{
			1: {{GroupID: 7}},
			3: {{GroupID: 7}},
		},
	}
	repo := &announcementRepoStub{
		item: &Announcement{
			ID: announcementID,
			Targeting: domain.AnnouncementTargeting{
				AnyOf: []domain.AnnouncementConditionGroup{
					{
						AllOf: []domain.AnnouncementCondition{
							{
								Type:     domain.AnnouncementConditionTypeSubscription,
								Operator: domain.AnnouncementOperatorIn,
								GroupIDs: []int64{7},
							},
						},
					},
				},
			},
		},
	}
	svc := NewAnnouncementService(repo, readRepo, userRepo, userSubRepo)

	items, page, err := svc.ListUserReadStatus(
		context.Background(),
		announcementID,
		pagination.PaginationParams{Page: 2, PageSize: 1, SortBy: "email", SortOrder: "asc"},
		"",
		AnnouncementReadStatusAll,
	)

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(2), page.Total)
	require.Equal(t, 2, page.Pages)
	require.Equal(t, int64(1), items[0].UserID)
	require.True(t, items[0].Eligible)
	require.Nil(t, items[0].ReadAt)
	require.Len(t, userRepo.listWithFilters, 1)
	require.Zero(t, userRepo.listWithFilters[0].AnnouncementID)
	require.Empty(t, userRepo.listWithFilters[0].AnnouncementReadStatus)
}

func TestAnnouncementServiceListUserReadStatusFiltersUnreadAfterEligibility(t *testing.T) {
	announcementID := int64(9)
	userRepo := &announcementUserRepoStub{
		users: []User{
			{ID: 1, Email: "eligible-unread@example.com"},
			{ID: 2, Email: "ineligible-unread@example.com"},
			{ID: 3, Email: "eligible-read@example.com"},
		},
	}
	readRepo := &announcementReadRepoStub{
		readMapByUsers: map[int64]time.Time{
			3: time.Unix(1776790020, 0),
		},
	}
	userSubRepo := &announcementUserSubRepoStub{
		activeByUserID: map[int64][]UserSubscription{
			1: {{GroupID: 7}},
			3: {{GroupID: 7}},
		},
	}
	repo := &announcementRepoStub{
		item: &Announcement{
			ID: announcementID,
			Targeting: domain.AnnouncementTargeting{
				AnyOf: []domain.AnnouncementConditionGroup{
					{
						AllOf: []domain.AnnouncementCondition{
							{
								Type:     domain.AnnouncementConditionTypeSubscription,
								Operator: domain.AnnouncementOperatorIn,
								GroupIDs: []int64{7},
							},
						},
					},
				},
			},
		},
	}
	svc := NewAnnouncementService(repo, readRepo, userRepo, userSubRepo)

	items, page, err := svc.ListUserReadStatus(
		context.Background(),
		announcementID,
		pagination.PaginationParams{Page: 1, PageSize: 10},
		"",
		AnnouncementReadStatusUnread,
	)

	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, int64(1), items[0].UserID)
	require.True(t, items[0].Eligible)
	require.Nil(t, items[0].ReadAt)
}

func pageCount(total int, pageSize int) int {
	if total == 0 || pageSize <= 0 {
		return 0
	}
	pages := total / pageSize
	if total%pageSize != 0 {
		pages++
	}
	return pages
}
