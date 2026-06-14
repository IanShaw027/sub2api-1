package service

import "strings"

// NormalizeOpenAICodexIdentityExtra clears stale OpenAI OAuth identity-bound
// Codex snapshots when submitted credentials no longer match persisted extra.
func NormalizeOpenAICodexIdentityExtra(platform string, accountType string, credentials map[string]any, extra map[string]any) map[string]any {
	if strings.ToLower(strings.TrimSpace(platform)) != PlatformOpenAI {
		return extra
	}
	if accountType != AccountTypeOAuth || len(credentials) == 0 || len(extra) == 0 {
		return extra
	}
	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: credentials,
		Extra:       extra,
	}
	if openAICodexSnapshotIdentityTrusted(account) {
		return extra
	}

	for _, key := range []string{
		"email",
		"email_address",
		"chatgpt_account_id",
		"account_id",
		"name",
		"user_image",
		"user_picture",
		"workspace_id",
		"chatgpt_workspace_id",
		"organization_id",
		"org_id",
		"workspace_name",
		"organization_role",
		"subscription_type",
		"plan_type",
		"web_profile",
		"cookies",
		"browser_cookies",
		"storage_state",
		"session_token_present",
		"session_expires_at",
		"oai_device_id",
		"openai_device_id",
		"oai_session_id",
		"openai_session_id",
	} {
		delete(extra, key)
	}
	for key := range extra {
		if strings.HasPrefix(key, "codex_") {
			delete(extra, key)
		}
	}
	return extra
}
