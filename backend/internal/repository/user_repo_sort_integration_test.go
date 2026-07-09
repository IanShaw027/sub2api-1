//go:build integration

package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *UserRepoSuite) mustInsertUsageLog(userID int64, createdAt time.Time) {
	s.T().Helper()

	s.mustInsertUsageLogWithCost(userID, 0.01, nil, service.BillingTypeBalance, createdAt)
}

func (s *UserRepoSuite) mustInsertUsageLogWithCost(userID int64, cost float64, subscriptionID *int64, billingType int8, createdAt time.Time) {
	s.T().Helper()

	account := mustCreateAccount(s.T(), s.client, &service.Account{Name: "usage-log-account"})
	apiKey := mustCreateApiKey(s.T(), s.client, &service.APIKey{UserID: userID})

	_, err := integrationDB.ExecContext(
		s.ctx,
		`INSERT INTO usage_logs (user_id, api_key_id, account_id, subscription_id, model, input_tokens, output_tokens, total_cost, actual_cost, billing_type, created_at)
		 VALUES ($1, $2, $3, $4, 'gpt-test', 1, 1, $5, $5, $6, $7)`,
		userID,
		apiKey.ID,
		account.ID,
		subscriptionID,
		cost,
		billingType,
		createdAt.UTC(),
	)
	s.Require().NoError(err)
}

func (s *UserRepoSuite) TestListWithFilters_SortByEmailAsc() {
	s.mustCreateUser(&service.User{Email: "z-last@example.com", Username: "z-user"})
	s.mustCreateUser(&service.User{Email: "a-first@example.com", Username: "a-user"})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "email",
		SortOrder: "asc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 2)
	s.Require().Equal("a-first@example.com", users[0].Email)
	s.Require().Equal("z-last@example.com", users[1].Email)
}

func (s *UserRepoSuite) TestList_DefaultSortByNewestFirst() {
	first := s.mustCreateUser(&service.User{Email: "first@example.com"})
	second := s.mustCreateUser(&service.User{Email: "second@example.com"})

	users, _, err := s.repo.List(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	s.Require().NoError(err)
	s.Require().Len(users, 2)
	s.Require().Equal(second.ID, users[0].ID)
	s.Require().Equal(first.ID, users[1].ID)
}

func (s *UserRepoSuite) TestCreateAndRead_PreservesSignupSourceAndActivityTimestamps() {
	lastLoginAt := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Microsecond)
	lastActiveAt := time.Now().Add(-30 * time.Minute).UTC().Truncate(time.Microsecond)

	created := s.mustCreateUser(&service.User{
		Email:        "identity-meta@example.com",
		SignupSource: "linuxdo",
		LastLoginAt:  &lastLoginAt,
		LastActiveAt: &lastActiveAt,
	})

	got, err := s.repo.GetByID(s.ctx, created.ID)
	s.Require().NoError(err)
	s.Require().Equal("linuxdo", got.SignupSource)
	s.Require().NotNil(got.LastLoginAt)
	s.Require().NotNil(got.LastActiveAt)
	s.Require().True(got.LastLoginAt.Equal(lastLoginAt))
	s.Require().True(got.LastActiveAt.Equal(lastActiveAt))
}

func (s *UserRepoSuite) TestUpdate_PersistsSignupSourceAndActivityTimestamps() {
	created := s.mustCreateUser(&service.User{Email: "identity-update@example.com"})
	lastLoginAt := time.Now().Add(-90 * time.Minute).UTC().Truncate(time.Microsecond)
	lastActiveAt := time.Now().Add(-15 * time.Minute).UTC().Truncate(time.Microsecond)

	created.SignupSource = "oidc"
	created.LastLoginAt = &lastLoginAt
	created.LastActiveAt = &lastActiveAt

	s.Require().NoError(s.repo.Update(s.ctx, created))

	got, err := s.repo.GetByID(s.ctx, created.ID)
	s.Require().NoError(err)
	s.Require().Equal("oidc", got.SignupSource)
	s.Require().NotNil(got.LastLoginAt)
	s.Require().NotNil(got.LastActiveAt)
	s.Require().True(got.LastLoginAt.Equal(lastLoginAt))
	s.Require().True(got.LastActiveAt.Equal(lastActiveAt))
}

func (s *UserRepoSuite) TestListWithFilters_SortByLastActiveAtAsc() {
	earlier := time.Now().Add(-3 * time.Hour).UTC().Truncate(time.Microsecond)
	later := time.Now().Add(-45 * time.Minute).UTC().Truncate(time.Microsecond)

	s.mustCreateUser(&service.User{Email: "nil-active@example.com"})
	s.mustCreateUser(&service.User{Email: "later-active@example.com", LastActiveAt: &later})
	s.mustCreateUser(&service.User{Email: "earlier-active@example.com", LastActiveAt: &earlier})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "last_active_at",
		SortOrder: "asc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 3)
	s.Require().Equal("earlier-active@example.com", users[0].Email)
	s.Require().Equal("later-active@example.com", users[1].Email)
	s.Require().Equal("nil-active@example.com", users[2].Email)
}

func (s *UserRepoSuite) TestListWithFilters_SortByLastLoginAtAsc() {
	earlier := time.Now().Add(-4 * time.Hour).UTC().Truncate(time.Microsecond)
	later := time.Now().Add(-30 * time.Minute).UTC().Truncate(time.Microsecond)

	s.mustCreateUser(&service.User{Email: "nil-login@example.com"})
	s.mustCreateUser(&service.User{Email: "later-login@example.com", LastLoginAt: &later})
	s.mustCreateUser(&service.User{Email: "earlier-login@example.com", LastLoginAt: &earlier})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "last_login_at",
		SortOrder: "asc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 3)
	s.Require().Equal("earlier-login@example.com", users[0].Email)
	s.Require().Equal("later-login@example.com", users[1].Email)
	s.Require().Equal("nil-login@example.com", users[2].Email)
}

func (s *UserRepoSuite) TestGetLatestUsedAtByUserIDs_UsesUsageLogs() {
	older := time.Now().Add(-4 * time.Hour).UTC().Truncate(time.Second)
	newer := time.Now().Add(-90 * time.Minute).UTC().Truncate(time.Second)

	userWithUsage := s.mustCreateUser(&service.User{Email: "usage-source@example.com"})
	userWithoutUsage := s.mustCreateUser(&service.User{Email: "usage-missing@example.com"})
	s.mustInsertUsageLog(userWithUsage.ID, older)
	s.mustInsertUsageLog(userWithUsage.ID, newer)

	got, err := s.repo.GetLatestUsedAtByUserIDs(s.ctx, []int64{userWithUsage.ID, userWithoutUsage.ID})
	s.Require().NoError(err)
	s.Require().Contains(got, userWithUsage.ID)
	s.Require().NotContains(got, userWithoutUsage.ID)
	s.Require().NotNil(got[userWithUsage.ID])
	s.Require().True(got[userWithUsage.ID].Equal(newer))
}

func (s *UserRepoSuite) TestListWithFilters_SortByLastUsedAtDesc_UsesUsageLogsNotLastActiveAt() {
	lastUsedOlder := time.Now().Add(-6 * time.Hour).UTC().Truncate(time.Second)
	lastUsedNewer := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	lastActiveVeryRecent := time.Now().Add(-10 * time.Minute).UTC().Truncate(time.Second)

	nilUsage := s.mustCreateUser(&service.User{Email: "nil-last-used@example.com"})
	wrongSource := s.mustCreateUser(&service.User{
		Email:        "active-not-usage@example.com",
		LastActiveAt: &lastActiveVeryRecent,
	})
	rightSource := s.mustCreateUser(&service.User{Email: "usage-wins@example.com"})

	s.mustInsertUsageLog(wrongSource.ID, lastUsedOlder)
	s.mustInsertUsageLog(rightSource.ID, lastUsedNewer)

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "last_used_at",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 3)
	s.Require().Equal(rightSource.ID, users[0].ID)
	s.Require().Equal(wrongSource.ID, users[1].ID)
	s.Require().Equal(nilUsage.ID, users[2].ID)
}

func (s *UserRepoSuite) TestListWithFilters_SortByUsageCostFields() {
	today := timezone.Now().UTC().Truncate(time.Second)
	yesterday := today.Add(-24 * time.Hour)

	group := s.mustCreateGroup("usage-sort-subscription")
	userA := s.mustCreateUser(&service.User{Email: "usage-a@example.com"})
	userB := s.mustCreateUser(&service.User{Email: "usage-b@example.com"})
	userC := s.mustCreateUser(&service.User{Email: "usage-c@example.com"})
	subA := s.mustCreateSubscription(userA.ID, group.ID, nil)
	subB := s.mustCreateSubscription(userB.ID, group.ID, nil)

	s.mustInsertUsageLogWithCost(userA.ID, 0.50, nil, service.BillingTypeBalance, today)
	s.mustInsertUsageLogWithCost(userA.ID, 0.10, &subA.ID, service.BillingTypeSubscription, today)
	s.mustInsertUsageLogWithCost(userA.ID, 0.10, nil, service.BillingTypeBalance, yesterday)

	s.mustInsertUsageLogWithCost(userB.ID, 0.90, &subB.ID, service.BillingTypeSubscription, today)
	s.mustInsertUsageLogWithCost(userB.ID, 0.10, nil, service.BillingTypeBalance, yesterday)

	s.mustInsertUsageLogWithCost(userC.ID, 0.20, nil, service.BillingTypeBalance, today)
	s.mustInsertUsageLogWithCost(userC.ID, 1.00, nil, service.BillingTypeBalance, yesterday)

	balanceSorted, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "today_balance_usage",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Equal([]int64{userA.ID, userC.ID, userB.ID}, []int64{balanceSorted[0].ID, balanceSorted[1].ID, balanceSorted[2].ID})
	s.Require().InDelta(0.50, balanceSorted[0].TodayBalanceActualCost, 0.0001)
	s.Require().InDelta(0.20, balanceSorted[1].TodayBalanceActualCost, 0.0001)
	s.Require().InDelta(0.00, balanceSorted[2].TodayBalanceActualCost, 0.0001)

	subscriptionSorted, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "today_subscription_usage",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Equal([]int64{userB.ID, userA.ID, userC.ID}, []int64{subscriptionSorted[0].ID, subscriptionSorted[1].ID, subscriptionSorted[2].ID})
	s.Require().InDelta(0.90, subscriptionSorted[0].TodaySubscriptionActualCost, 0.0001)
	s.Require().InDelta(0.10, subscriptionSorted[1].TodaySubscriptionActualCost, 0.0001)
	s.Require().InDelta(0.00, subscriptionSorted[2].TodaySubscriptionActualCost, 0.0001)

	last30Sorted, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "last_30d_usage",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Equal([]int64{userC.ID, userB.ID, userA.ID}, []int64{last30Sorted[0].ID, last30Sorted[1].ID, last30Sorted[2].ID})
	s.Require().InDelta(1.20, last30Sorted[0].TotalActualCost, 0.0001)
	s.Require().InDelta(1.00, last30Sorted[1].TotalActualCost, 0.0001)
	s.Require().InDelta(0.70, last30Sorted[2].TotalActualCost, 0.0001)
}

func (s *UserRepoSuite) TestListWithFilters_SortByUsageCostUsesBillingTypeNotSubscriptionID() {
	today := timezone.Now().UTC().Truncate(time.Second)

	user := s.mustCreateUser(&service.User{Email: "usage-billing-type@test.com"})
	group := s.mustCreateGroup("usage-billing-type")
	subscription := s.mustCreateSubscription(user.ID, group.ID, nil)

	s.mustInsertUsageLogWithCost(user.ID, 0.75, &subscription.ID, service.BillingTypeBalance, today)
	s.mustInsertUsageLogWithCost(user.ID, 0.25, nil, service.BillingTypeSubscription, today)

	balanceSorted, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "today_balance_usage",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(balanceSorted, 1)
	s.Require().InDelta(0.75, balanceSorted[0].TodayBalanceActualCost, 0.0001)
	s.Require().InDelta(0.25, balanceSorted[0].TodaySubscriptionActualCost, 0.0001)
}

func (s *UserRepoSuite) TestListWithFilters_PopulatesUsageCostFieldsUsingBillingTypeNotSubscriptionID() {
	today := timezone.Now().UTC().Truncate(time.Second)

	user := s.mustCreateUser(&service.User{Email: "usage-populate-billing-type@test.com"})
	group := s.mustCreateGroup("usage-populate-billing-type")
	subscription := s.mustCreateSubscription(user.ID, group.ID, nil)

	s.mustInsertUsageLogWithCost(user.ID, 0.60, &subscription.ID, service.BillingTypeBalance, today)
	s.mustInsertUsageLogWithCost(user.ID, 0.40, nil, service.BillingTypeSubscription, today)

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "email",
		SortOrder: "asc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().Len(users, 1)
	s.Require().InDelta(0.60, users[0].TodayBalanceActualCost, 0.0001)
	s.Require().InDelta(0.40, users[0].TodaySubscriptionActualCost, 0.0001)
}

func (s *UserRepoSuite) TestListWithFilters_SortByUsageCostFallbackUsesBillingTypeNotSubscriptionID() {
	today := timezone.Today().Add(2 * time.Hour).UTC()

	allowedGroup := s.mustCreateGroup("usage-sort-fallback")
	userA := s.mustCreateUser(&service.User{
		Email:         "usage-fallback-a@example.com",
		AllowedGroups: []int64{allowedGroup.ID},
	})
	userB := s.mustCreateUser(&service.User{
		Email:         "usage-fallback-b@example.com",
		AllowedGroups: []int64{allowedGroup.ID},
	})
	subA := s.mustCreateSubscription(userA.ID, allowedGroup.ID, nil)
	subB := s.mustCreateSubscription(userB.ID, allowedGroup.ID, nil)

	s.mustInsertUsageLogWithCost(userA.ID, 0.80, &subA.ID, service.BillingTypeBalance, today)
	s.mustInsertUsageLogWithCost(userA.ID, 0.10, nil, service.BillingTypeSubscription, today)
	s.mustInsertUsageLogWithCost(userB.ID, 0.30, &subB.ID, service.BillingTypeBalance, today)
	s.mustInsertUsageLogWithCost(userB.ID, 0.90, nil, service.BillingTypeSubscription, today)

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "today_balance_usage",
		SortOrder: "desc",
	}, service.UserListFilters{
		GroupName: allowedGroup.Name,
	})
	s.Require().NoError(err)
	s.Require().Len(users, 2)
	s.Require().Equal([]int64{userA.ID, userB.ID}, []int64{users[0].ID, users[1].ID})
	s.Require().InDelta(0.80, users[0].TodayBalanceActualCost, 0.0001)
	s.Require().InDelta(0.10, users[0].TodaySubscriptionActualCost, 0.0001)
	s.Require().InDelta(0.30, users[1].TodayBalanceActualCost, 0.0001)
	s.Require().InDelta(0.90, users[1].TodaySubscriptionActualCost, 0.0001)
}

func TestUserRepoSortSuiteSmoke(_ *testing.T) {}
