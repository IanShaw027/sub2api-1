//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OpsRepoIntegrationTestSuite struct {
	suite.Suite
	repo      *OpsRepository
	userID    int64
	accountID int64
	groupID   int64
}

func (s *OpsRepoIntegrationTestSuite) SetupSuite() {
	require.NotNil(s.T(), integrationDB, "integration DB not initialized")
	s.repo = &OpsRepository{sql: integrationDB, usePreaggregatedTables: false}

	// Create test entities

	// Insert user
	err := integrationDB.QueryRow(
		`INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3) RETURNING id`,
		"ops-test@example.com",
		"hash",
		"user",
	).Scan(&s.userID)
	require.NoError(s.T(), err, "failed to insert test user")

	// Insert group
	err = integrationDB.QueryRow(
		`INSERT INTO groups (name, description, platform) VALUES ($1, $2, $3) RETURNING id`,
		"ops-test-group",
		"test group",
		"openai",
	).Scan(&s.groupID)
	require.NoError(s.T(), err, "failed to insert test group")

	// Insert account
	err = integrationDB.QueryRow(
		`INSERT INTO accounts (name, platform, type, credentials, extra, status) VALUES ($1, $2, $3, '{}'::jsonb, '{}'::jsonb, 'active') RETURNING id`,
		"ops-test-account",
		"openai",
		"apikey",
	).Scan(&s.accountID)
	require.NoError(s.T(), err, "failed to insert test account")
}

func (s *OpsRepoIntegrationTestSuite) TearDownSuite() {
	// Clean up test data
	_, _ = integrationDB.Exec(`DELETE FROM ops_error_logs WHERE account_id = $1`, s.accountID)
	_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id = $1`, s.accountID)
	_, _ = integrationDB.Exec(`DELETE FROM groups WHERE id = $1`, s.groupID)
	_, _ = integrationDB.Exec(`DELETE FROM users WHERE id = $1`, s.userID)
}

func (s *OpsRepoIntegrationTestSuite) TestGetErrorStatsByIP_BasicAggregation() {
	ctx := context.Background()

	// Setup: Insert test error logs with different IPs and error types
	now := time.Now().UTC()
	startTime := now.Add(-1 * time.Hour)
	endTime := now

	testData := []struct {
		clientIP  string
		errorType string
		createdAt time.Time
	}{
		// IP1: 3 rate_limit errors, 2 timeout errors
		{"192.168.1.1", "rate_limit", startTime.Add(1 * time.Minute)},
		{"192.168.1.1", "rate_limit", startTime.Add(2 * time.Minute)},
		{"192.168.1.1", "rate_limit", startTime.Add(3 * time.Minute)},
		{"192.168.1.1", "timeout", startTime.Add(4 * time.Minute)},
		{"192.168.1.1", "timeout", startTime.Add(5 * time.Minute)},

		// IP2: 4 upstream_error errors
		{"192.168.1.2", "upstream_error", startTime.Add(6 * time.Minute)},
		{"192.168.1.2", "upstream_error", startTime.Add(7 * time.Minute)},
		{"192.168.1.2", "upstream_error", startTime.Add(8 * time.Minute)},
		{"192.168.1.2", "upstream_error", startTime.Add(9 * time.Minute)},

		// IP3: 1 rate_limit error
		{"192.168.1.3", "rate_limit", startTime.Add(10 * time.Minute)},
	}

	// Insert test data
	for _, td := range testData {
		_, err := integrationDB.ExecContext(ctx,
			`INSERT INTO ops_error_logs (account_id, group_id, client_ip, error_phase, error_type, severity, status_code, platform, error_message, duration_ms, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			s.accountID, s.groupID, td.clientIP, "upstream", td.errorType, "P2", 500, "openai", "test error", 0, td.createdAt,
		)
		require.NoError(s.T(), err, "failed to insert test error log")
	}

	// Execute: Call GetErrorStatsByIP
	stats, err := s.repo.GetErrorStatsByIP(ctx, startTime, endTime, 10, "error_count", "DESC")

	// Assert: Verify results
	require.NoError(s.T(), err, "GetErrorStatsByIP should not return error")
	require.Len(s.T(), stats, 3, "should return 3 unique IPs")

	// Verify IP1 (5 total errors: 3 rate_limit + 2 timeout)
	ip1 := findStatsByIP(stats, "192.168.1.1")
	require.NotNil(s.T(), ip1, "IP 192.168.1.1 should be in results")
	require.Equal(s.T(), int64(5), ip1.ErrorCount, "IP1 should have 5 total errors")
	require.Equal(s.T(), int64(3), ip1.ErrorTypes["rate_limit"], "IP1 should have 3 rate_limit errors")
	require.Equal(s.T(), int64(2), ip1.ErrorTypes["timeout"], "IP1 should have 2 timeout errors")
	require.NotZero(s.T(), ip1.FirstErrorTime, "IP1 should have first_error_time")
	require.NotZero(s.T(), ip1.LastErrorTime, "IP1 should have last_error_time")

	// Verify IP2 (4 total errors: all upstream_error)
	ip2 := findStatsByIP(stats, "192.168.1.2")
	require.NotNil(s.T(), ip2, "IP 192.168.1.2 should be in results")
	require.Equal(s.T(), int64(4), ip2.ErrorCount, "IP2 should have 4 total errors")
	require.Equal(s.T(), int64(4), ip2.ErrorTypes["upstream_error"], "IP2 should have 4 upstream_error errors")

	// Verify IP3 (1 total error: rate_limit)
	ip3 := findStatsByIP(stats, "192.168.1.3")
	require.NotNil(s.T(), ip3, "IP 192.168.1.3 should be in results")
	require.Equal(s.T(), int64(1), ip3.ErrorCount, "IP3 should have 1 total error")
	require.Equal(s.T(), int64(1), ip3.ErrorTypes["rate_limit"], "IP3 should have 1 rate_limit error")

	// Verify sorting (DESC by error_count)
	require.Equal(s.T(), "192.168.1.1", stats[0].ClientIP, "First result should be IP1 (5 errors)")
	require.Equal(s.T(), "192.168.1.2", stats[1].ClientIP, "Second result should be IP2 (4 errors)")
	require.Equal(s.T(), "192.168.1.3", stats[2].ClientIP, "Third result should be IP3 (1 error)")

	// Cleanup
	_, _ = integrationDB.ExecContext(ctx, `DELETE FROM ops_error_logs WHERE account_id = $1`, s.accountID)
}

func (s *OpsRepoIntegrationTestSuite) TestGetErrorStatsByIP_EmptyResult() {
	ctx := context.Background()

	now := time.Now().UTC()
	startTime := now.Add(-1 * time.Hour)
	endTime := now

	// Execute: Query with no matching data
	stats, err := s.repo.GetErrorStatsByIP(ctx, startTime, endTime, 10, "error_count", "DESC")

	// Assert: Should return empty slice, not error
	require.NoError(s.T(), err, "GetErrorStatsByIP should not return error for empty result")
	require.Empty(s.T(), stats, "should return empty slice")
}

func (s *OpsRepoIntegrationTestSuite) TestGetErrorStatsByIP_NullIPFiltering() {
	ctx := context.Background()

	now := time.Now().UTC()
	startTime := now.Add(-1 * time.Hour)
	endTime := now

	// Setup: Insert errors with NULL and empty IPs
	testData := []struct {
		clientIP  *string
		errorType string
	}{
		{nil, "rate_limit"},                       // NULL IP - should be filtered
		{stringPtr(""), "timeout"},                // Empty IP - should be filtered
		{stringPtr("192.168.1.100"), "rate_limit"}, // Valid IP - should be included
	}

	for _, td := range testData {
		_, err := integrationDB.ExecContext(ctx,
			`INSERT INTO ops_error_logs (account_id, group_id, client_ip, error_phase, error_type, severity, status_code, platform, error_message, duration_ms, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			s.accountID, s.groupID, td.clientIP, "upstream", td.errorType, "P2", 500, "openai", "test error", 0, startTime.Add(1*time.Minute),
		)
		require.NoError(s.T(), err, "failed to insert test error log")
	}

	// Execute
	stats, err := s.repo.GetErrorStatsByIP(ctx, startTime, endTime, 10, "error_count", "DESC")

	// Assert: Only valid IP should be returned
	require.NoError(s.T(), err)
	require.Len(s.T(), stats, 1, "should only return 1 valid IP")
	require.Equal(s.T(), "192.168.1.100", stats[0].ClientIP)

	// Cleanup
	_, _ = integrationDB.ExecContext(ctx, `DELETE FROM ops_error_logs WHERE account_id = $1`, s.accountID)
}

func (s *OpsRepoIntegrationTestSuite) TestGetErrorStatsByIP_TimeRangeFiltering() {
	ctx := context.Background()

	now := time.Now().UTC()
	startTime := now.Add(-1 * time.Hour)
	endTime := now

	// Setup: Insert errors before, within, and after the time range
	testData := []struct {
		clientIP  string
		createdAt time.Time
	}{
		{"192.168.1.1", startTime.Add(-10 * time.Minute)}, // Before range
		{"192.168.1.1", startTime.Add(10 * time.Minute)},  // Within range
		{"192.168.1.1", startTime.Add(20 * time.Minute)},  // Within range
		{"192.168.1.1", endTime.Add(10 * time.Minute)},    // After range
	}

	for _, td := range testData {
		_, err := integrationDB.ExecContext(ctx,
			`INSERT INTO ops_error_logs (account_id, group_id, client_ip, error_phase, error_type, severity, status_code, platform, error_message, duration_ms, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			s.accountID, s.groupID, td.clientIP, "upstream", "rate_limit", "P2", 500, "openai", "test error", 0, td.createdAt,
		)
		require.NoError(s.T(), err, "failed to insert test error log")
	}

	// Execute
	stats, err := s.repo.GetErrorStatsByIP(ctx, startTime, endTime, 10, "error_count", "DESC")

	// Assert: Only 2 errors within range should be counted
	require.NoError(s.T(), err)
	require.Len(s.T(), stats, 1)
	require.Equal(s.T(), int64(2), stats[0].ErrorCount, "should only count errors within time range")

	// Cleanup
	_, _ = integrationDB.ExecContext(ctx, `DELETE FROM ops_error_logs WHERE account_id = $1`, s.accountID)
}

func (s *OpsRepoIntegrationTestSuite) TestGetErrorStatsByIP_LimitAndSorting() {
	ctx := context.Background()

	now := time.Now().UTC()
	startTime := now.Add(-1 * time.Hour)
	endTime := now

	// Setup: Insert errors for 5 different IPs with different counts
	ipCounts := map[string]int{
		"192.168.1.1": 5,
		"192.168.1.2": 3,
		"192.168.1.3": 7,
		"192.168.1.4": 2,
		"192.168.1.5": 4,
	}

	for ip, count := range ipCounts {
		for i := 0; i < count; i++ {
			_, err := integrationDB.ExecContext(ctx,
				`INSERT INTO ops_error_logs (account_id, group_id, client_ip, error_phase, error_type, severity, status_code, platform, error_message, duration_ms, created_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
				s.accountID, s.groupID, ip, "upstream", "rate_limit", "P2", 500, "openai", "test error", 0, startTime.Add(time.Duration(i)*time.Minute),
			)
			require.NoError(s.T(), err, "failed to insert test error log")
		}
	}

	// Test 1: DESC order with limit 3
	stats, err := s.repo.GetErrorStatsByIP(ctx, startTime, endTime, 3, "error_count", "DESC")
	require.NoError(s.T(), err)
	require.Len(s.T(), stats, 3, "should respect limit")
	require.Equal(s.T(), "192.168.1.3", stats[0].ClientIP, "first should be IP3 (7 errors)")
	require.Equal(s.T(), "192.168.1.1", stats[1].ClientIP, "second should be IP1 (5 errors)")
	require.Equal(s.T(), "192.168.1.5", stats[2].ClientIP, "third should be IP5 (4 errors)")

	// Test 2: ASC order with limit 2
	stats, err = s.repo.GetErrorStatsByIP(ctx, startTime, endTime, 2, "error_count", "ASC")
	require.NoError(s.T(), err)
	require.Len(s.T(), stats, 2, "should respect limit")
	require.Equal(s.T(), "192.168.1.4", stats[0].ClientIP, "first should be IP4 (2 errors)")
	require.Equal(s.T(), "192.168.1.2", stats[1].ClientIP, "second should be IP2 (3 errors)")

	// Cleanup
	_, _ = integrationDB.ExecContext(ctx, `DELETE FROM ops_error_logs WHERE account_id = $1`, s.accountID)
}

// Helper function to find stats by IP
func findStatsByIP(stats []service.IPErrorStats, ip string) *service.IPErrorStats {
	for _, s := range stats {
		if s.ClientIP == ip {
			return &s
		}
	}
	return nil
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

func TestOpsRepoIntegrationSuite(t *testing.T) {
	suite.Run(t, new(OpsRepoIntegrationTestSuite))
}
