package openai

import "testing"

func TestIDTokenClaimsGetUserInfo_PreservesDefaultOrganizationRole(t *testing.T) {
	claims := &IDTokenClaims{
		Name:  "User",
		Email: "user@example.com",
		OpenAIAuth: &OpenAIAuthClaims{
			ChatGPTAccountID: "acct-1",
			ChatGPTUserID:    "user-1",
			ChatGPTPlanType:  "team",
			Organizations: []OrganizationClaim{
				{
					ID:        "org-team",
					Role:      "owner",
					Title:     "Team A",
					IsDefault: true,
				},
			},
		},
	}

	info := claims.GetUserInfo()
	if info == nil {
		t.Fatal("expected user info")
	}
	if info.OrganizationID != "org-team" {
		t.Fatalf("expected default organization id, got %q", info.OrganizationID)
	}
	if info.OrganizationTitle != "Team A" {
		t.Fatalf("expected default organization title, got %q", info.OrganizationTitle)
	}
	if info.OrganizationRole != "owner" {
		t.Fatalf("expected default organization role, got %q", info.OrganizationRole)
	}
}
