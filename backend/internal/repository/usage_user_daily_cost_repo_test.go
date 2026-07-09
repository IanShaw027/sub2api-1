//go:build integration

package repository

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 复用 UserRepoSuite 的 harness 与 mustCreateUser/mustInsertUsageLogWithCost 等辅助方法。

// truncateUsageTables 清空 usage_logs 与 usage_user_daily_cost，避免跨测试脏数据污染同一业务日的全量重算。
func (s *UserRepoSuite) truncateUsageTables() {
	s.T().Helper()
	_, err := integrationDB.ExecContext(s.ctx, "DELETE FROM usage_logs")
	s.Require().NoError(err)
	_, err = integrationDB.ExecContext(s.ctx, "DELETE FROM usage_user_daily_cost")
	s.Require().NoError(err)
}

// rollupRepo 返回直连 integrationDB 的预聚合仓储。
func (s *UserRepoSuite) rollupRepo() service.UsageUserDailyCostRepository {
	s.T().Helper()
	return NewUsageUserDailyCostRepository(s.client, integrationDB)
}

type dailyCostRow struct {
	actualCost       float64
	balanceCost      float64
	subscriptionCost float64
}

// readDailyCost 读取指定 user + bucketDate 的预聚合行；不存在时 ok=false。
func (s *UserRepoSuite) readDailyCost(userID int64, bucketDate string) (dailyCostRow, bool) {
	s.T().Helper()
	row := integrationDB.QueryRowContext(
		s.ctx,
		"SELECT actual_cost, balance_actual_cost, subscription_actual_cost FROM usage_user_daily_cost WHERE user_id = $1 AND bucket_date = $2::date",
		userID,
		bucketDate,
	)
	var out dailyCostRow
	err := row.Scan(&out.actualCost, &out.balanceCost, &out.subscriptionCost)
	if err != nil {
		return dailyCostRow{}, false
	}
	return out, true
}

func (s *UserRepoSuite) TestRecomputeDay_AggregatesPerUserAndBillingType() {
	s.truncateUsageTables()

	dayStart := timezone.Today()
	dayEnd := dayStart.AddDate(0, 0, 1)
	bucketDate := dayStart.Format("2006-01-02")
	at := time.Now().UTC()

	userA := s.mustCreateUser(&service.User{Email: "rollup-a@example.com"})
	userB := s.mustCreateUser(&service.User{Email: "rollup-b@example.com"})

	// userA: 余额 0.50 + 0.20，订阅 0.10 → total 0.80 / balance 0.70 / subscription 0.10
	s.mustInsertUsageLogWithCost(userA.ID, 0.50, nil, service.BillingTypeBalance, at)
	s.mustInsertUsageLogWithCost(userA.ID, 0.20, nil, service.BillingTypeBalance, at)
	s.mustInsertUsageLogWithCost(userA.ID, 0.10, nil, service.BillingTypeSubscription, at)
	// userB: 订阅 0.90 → total 0.90 / balance 0.00 / subscription 0.90
	s.mustInsertUsageLogWithCost(userB.ID, 0.90, nil, service.BillingTypeSubscription, at)

	s.Require().NoError(s.rollupRepo().RecomputeDay(s.ctx, bucketDate, dayStart.UTC(), dayEnd.UTC()))

	rowA, ok := s.readDailyCost(userA.ID, bucketDate)
	s.Require().True(ok)
	s.Require().InDelta(0.80, rowA.actualCost, 0.0001)
	s.Require().InDelta(0.70, rowA.balanceCost, 0.0001)
	s.Require().InDelta(0.10, rowA.subscriptionCost, 0.0001)

	rowB, ok := s.readDailyCost(userB.ID, bucketDate)
	s.Require().True(ok)
	s.Require().InDelta(0.90, rowB.actualCost, 0.0001)
	s.Require().InDelta(0.00, rowB.balanceCost, 0.0001)
	s.Require().InDelta(0.90, rowB.subscriptionCost, 0.0001)
}

func (s *UserRepoSuite) TestRecomputeDay_Idempotent() {
	s.truncateUsageTables()

	dayStart := timezone.Today()
	dayEnd := dayStart.AddDate(0, 0, 1)
	bucketDate := dayStart.Format("2006-01-02")
	at := time.Now().UTC()

	user := s.mustCreateUser(&service.User{Email: "rollup-idem@example.com"})
	s.mustInsertUsageLogWithCost(user.ID, 0.33, nil, service.BillingTypeBalance, at)

	repo := s.rollupRepo()
	s.Require().NoError(repo.RecomputeDay(s.ctx, bucketDate, dayStart.UTC(), dayEnd.UTC()))
	first, ok := s.readDailyCost(user.ID, bucketDate)
	s.Require().True(ok)

	// 再跑一次：同日先删后插，结果不变（不翻倍）。
	s.Require().NoError(repo.RecomputeDay(s.ctx, bucketDate, dayStart.UTC(), dayEnd.UTC()))
	second, ok := s.readDailyCost(user.ID, bucketDate)
	s.Require().True(ok)

	s.Require().InDelta(first.actualCost, second.actualCost, 0.0001)
	s.Require().InDelta(0.33, second.actualCost, 0.0001)

	// 该 bucket 只应有一行。
	var count int
	s.Require().NoError(integrationDB.QueryRowContext(
		s.ctx,
		"SELECT COUNT(*) FROM usage_user_daily_cost WHERE user_id = $1 AND bucket_date = $2::date",
		user.ID, bucketDate,
	).Scan(&count))
	s.Require().Equal(1, count)
}

func (s *UserRepoSuite) TestDeleteOlderThan_RemovesOnlyBeforeCutoff() {
	s.truncateUsageTables()

	todayStart := timezone.Today()
	oldStart := todayStart.AddDate(0, 0, -40)
	oldEnd := oldStart.AddDate(0, 0, 1)
	oldBucket := oldStart.Format("2006-01-02")
	todayBucket := todayStart.Format("2006-01-02")

	user := s.mustCreateUser(&service.User{Email: "rollup-retention@example.com"})
	s.mustInsertUsageLogWithCost(user.ID, 0.10, nil, service.BillingTypeBalance, time.Now().UTC())
	s.mustInsertUsageLogWithCost(user.ID, 0.20, nil, service.BillingTypeBalance, oldStart.Add(2*time.Hour).UTC())

	repo := s.rollupRepo()
	s.Require().NoError(repo.RecomputeDay(s.ctx, todayBucket, todayStart.UTC(), todayStart.AddDate(0, 0, 1).UTC()))
	s.Require().NoError(repo.RecomputeDay(s.ctx, oldBucket, oldStart.UTC(), oldEnd.UTC()))

	// 保留 35 天：cutoff = today-35，40 天前的行 < cutoff 应删，今天的行保留。
	cutoff := todayStart.AddDate(0, 0, -35).Format("2006-01-02")
	deleted, err := repo.DeleteOlderThan(s.ctx, cutoff)
	s.Require().NoError(err)
	s.Require().Equal(int64(1), deleted)

	_, okOld := s.readDailyCost(user.ID, oldBucket)
	s.Require().False(okOld)
	_, okToday := s.readDailyCost(user.ID, todayBucket)
	s.Require().True(okToday)
}

func (s *UserRepoSuite) TestListWithUsageSort_RollupNotReadyFallsBackToLiveUsage() {
	s.truncateUsageTables()
	service.SetUsageUserDailyCostRollupReady(false)
	s.T().Cleanup(func() {
		service.SetUsageUserDailyCostRollupReady(true)
	})

	s.repo.useUsageRollup = true
	todayAt := time.Now().UTC()

	userLow := s.mustCreateUser(&service.User{Email: "rollup-not-ready-low@example.com"})
	userHigh := s.mustCreateUser(&service.User{Email: "rollup-not-ready-high@example.com"})

	s.mustInsertUsageLogWithCost(userLow.ID, 0.10, nil, service.BillingTypeBalance, todayAt)
	s.mustInsertUsageLogWithCost(userHigh.ID, 0.90, nil, service.BillingTypeBalance, todayAt)

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "today_balance_usage",
		SortOrder: "desc",
	}, service.UserListFilters{})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(users), 2)
	s.Require().Equal(userHigh.ID, users[0].ID)
	s.Require().Equal(userLow.ID, users[1].ID)
}

// TestRollupSortParity 端到端断言：同一批 usage_logs 先 RecomputeDay，
// 再比对 optimizedUsageSortedRowsFromRollup 与 optimizedUsageSortedRows 的排序与金额一致。
func (s *UserRepoSuite) TestRollupSortParity() {
	s.truncateUsageTables()

	todayStart := timezone.Today()
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	todayAt := time.Now().UTC()
	yesterdayAt := time.Now().AddDate(0, 0, -1).UTC()

	userA := s.mustCreateUser(&service.User{Email: "parity-a@example.com"})
	userB := s.mustCreateUser(&service.User{Email: "parity-b@example.com"})
	userC := s.mustCreateUser(&service.User{Email: "parity-c@example.com"})

	// 今天：不同余额/订阅金额；昨天：仅计入近 30 天 total，不计入 today。
	s.mustInsertUsageLogWithCost(userA.ID, 0.50, nil, service.BillingTypeBalance, todayAt)
	s.mustInsertUsageLogWithCost(userA.ID, 0.10, nil, service.BillingTypeSubscription, todayAt)
	s.mustInsertUsageLogWithCost(userA.ID, 0.10, nil, service.BillingTypeBalance, yesterdayAt)
	s.mustInsertUsageLogWithCost(userB.ID, 0.90, nil, service.BillingTypeSubscription, todayAt)
	s.mustInsertUsageLogWithCost(userB.ID, 0.10, nil, service.BillingTypeBalance, yesterdayAt)
	s.mustInsertUsageLogWithCost(userC.ID, 0.20, nil, service.BillingTypeBalance, todayAt)
	s.mustInsertUsageLogWithCost(userC.ID, 1.00, nil, service.BillingTypeBalance, yesterdayAt)

	repo := s.rollupRepo()
	s.Require().NoError(repo.RecomputeDay(s.ctx, todayStart.Format("2006-01-02"), todayStart.UTC(), todayStart.AddDate(0, 0, 1).UTC()))
	s.Require().NoError(repo.RecomputeDay(s.ctx, yesterdayStart.Format("2006-01-02"), yesterdayStart.UTC(), yesterdayStart.AddDate(0, 0, 1).UTC()))

	for _, sortBy := range []string{"today_balance_usage", "today_subscription_usage", "last_30d_usage"} {
		params := pagination.PaginationParams{Page: 1, PageSize: 10, SortBy: sortBy, SortOrder: "desc"}
		live, err := s.repo.optimizedUsageSortedRows(s.ctx, params, service.UserListFilters{})
		s.Require().NoError(err)
		rollup, err := s.repo.optimizedUsageSortedRowsFromRollup(s.ctx, params, service.UserListFilters{})
		s.Require().NoError(err)

		s.Require().Equal(len(live), len(rollup), "row count parity for sortBy=%s", sortBy)
		for i := range live {
			s.Require().Equal(live[i].ID, rollup[i].ID, "user order parity idx=%d sortBy=%s", i, sortBy)
			s.Require().InDelta(live[i].TodayActualCost, rollup[i].TodayActualCost, 0.0001, "today parity idx=%d sortBy=%s", i, sortBy)
			s.Require().InDelta(live[i].TodayBalanceActualCost, rollup[i].TodayBalanceActualCost, 0.0001, "today balance parity idx=%d sortBy=%s", i, sortBy)
			s.Require().InDelta(live[i].TodaySubscriptionActualCost, rollup[i].TodaySubscriptionActualCost, 0.0001, "today subscription parity idx=%d sortBy=%s", i, sortBy)
			s.Require().InDelta(live[i].TotalActualCost, rollup[i].TotalActualCost, 0.0001, "total parity idx=%d sortBy=%s", i, sortBy)
		}
	}
}

func (s *UserRepoSuite) TestRollupSortParity_Last30DaysBoundary() {
	s.truncateUsageTables()

	boundaryStart := timezone.Today().AddDate(0, 0, -30)
	boundaryEnd := boundaryStart.AddDate(0, 0, 1)
	rollingStart := time.Now().UTC().AddDate(0, 0, -30)
	s.Require().True(rollingStart.After(boundaryStart.UTC()))
	s.Require().True(rollingStart.Before(boundaryEnd.UTC()))

	outsideAt := boundaryStart.UTC().Add(rollingStart.Sub(boundaryStart.UTC()) / 2)
	insideAt := rollingStart.Add(boundaryEnd.UTC().Sub(rollingStart) / 2)

	userBoundary := s.mustCreateUser(&service.User{Email: "rollup-boundary@example.com"})
	userRecent := s.mustCreateUser(&service.User{Email: "rollup-recent@example.com"})

	s.mustInsertUsageLogWithCost(userBoundary.ID, 0.80, nil, service.BillingTypeBalance, outsideAt)
	s.mustInsertUsageLogWithCost(userBoundary.ID, 0.20, nil, service.BillingTypeBalance, insideAt)
	s.mustInsertUsageLogWithCost(userRecent.ID, 0.50, nil, service.BillingTypeBalance, time.Now().UTC())

	repo := s.rollupRepo()
	s.Require().NoError(repo.RecomputeDay(s.ctx, boundaryStart.Format("2006-01-02"), boundaryStart.UTC(), boundaryEnd.UTC()))
	s.Require().NoError(repo.RecomputeDay(s.ctx, timezone.Today().Format("2006-01-02"), timezone.Today().UTC(), timezone.Today().AddDate(0, 0, 1).UTC()))

	params := pagination.PaginationParams{Page: 1, PageSize: 10, SortBy: "last_30d_usage", SortOrder: "desc"}
	live, err := s.repo.optimizedUsageSortedRows(s.ctx, params, service.UserListFilters{})
	s.Require().NoError(err)
	rollup, err := s.repo.optimizedUsageSortedRowsFromRollup(s.ctx, params, service.UserListFilters{})
	s.Require().NoError(err)

	s.Require().Equal(len(live), len(rollup))
	for i := range live {
		s.Require().Equal(live[i].ID, rollup[i].ID, "boundary parity idx=%d", i)
		s.Require().InDelta(live[i].TotalActualCost, rollup[i].TotalActualCost, 0.0001, "boundary total parity idx=%d", i)
	}
}
