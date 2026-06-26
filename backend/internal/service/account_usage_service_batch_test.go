package service

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestAccountUsageService_GetUsageBatch_BestEffortByAccount(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second)

	repo := &stubOpenAIAccountRepo{
		accounts: []Account{
			{
				ID:       7001,
				Platform: PlatformAnthropic,
				Type:     AccountTypeServiceAccount,
				Extra: map[string]any{
					"passive_usage_7d_utilization": 0.62,
				},
			},
			{
				ID:       7002,
				Platform: PlatformOpenAI,
				Type:     AccountTypeOAuth,
				Extra: map[string]any{
					"codex_usage_updated_at":  time.Now().UTC().Format(time.RFC3339),
					"codex_5h_used_percent":   18.0,
					"codex_5h_reset_at":       resetAt.Format(time.RFC3339),
					"codex_7d_used_percent":   34.0,
					"codex_7d_reset_at":       resetAt.Add(24 * time.Hour).Format(time.RFC3339),
					"workspace_id":            "org-test",
					"chatgpt_account_id":      "acct-test",
					"openai_snapshot_version": "test",
				},
			},
			{
				ID:       7003,
				Platform: PlatformOpenAI,
				Type:     AccountTypeAPIKey,
			},
		},
	}

	svc := &AccountUsageService{
		accountRepo:  repo,
		usageLogRepo: &geminiUsageLogRepoStub{},
		cache:        NewUsageCache(),
	}

	usageByAccount, errorsByAccount, err := svc.GetUsageBatch(context.Background(), []int64{7001, 7002, 7003, 7002}, false)
	if err != nil {
		t.Fatalf("GetUsageBatch() error = %v", err)
	}

	if usageByAccount[7001] == nil || usageByAccount[7001].Source != "passive" {
		t.Fatalf("expected anthropic passive usage, got %#v", usageByAccount[7001])
	}

	if usageByAccount[7002] == nil || usageByAccount[7002].FiveHour == nil || usageByAccount[7002].FiveHour.Utilization != 18.0 {
		t.Fatalf("expected openai snapshot usage, got %#v", usageByAccount[7002])
	}

	if !strings.Contains(strings.ToLower(errorsByAccount[7003]), "bad request") {
		t.Fatalf("expected API key account error to be preserved, got %q", errorsByAccount[7003])
	}
}
