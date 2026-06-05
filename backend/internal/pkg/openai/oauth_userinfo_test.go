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

func TestIDTokenClaimsGetUserInfo_TeamPlanPrefersWorkspaceOrganization(t *testing.T) {
	claims := &IDTokenClaims{
		Name:  "User",
		Email: "user@example.com",
		OpenAIAuth: &OpenAIAuthClaims{
			ChatGPTPlanType: "team",
			Organizations: []OrganizationClaim{
				{
					ID:    "org-personal",
					Role:  "owner",
					Title: "Personal",
				},
				{
					ID:    "org-team",
					Role:  "member",
					Title: "Workspace A",
				},
			},
		},
	}

	info := claims.GetUserInfo()
	if info == nil {
		t.Fatal("expected user info")
	}
	if info.OrganizationID != "org-team" {
		t.Fatalf("expected workspace organization id, got %q", info.OrganizationID)
	}
	if info.OrganizationTitle != "Workspace A" {
		t.Fatalf("expected workspace organization title, got %q", info.OrganizationTitle)
	}
	if info.OrganizationRole != "member" {
		t.Fatalf("expected workspace organization role, got %q", info.OrganizationRole)
	}
}

func TestIDTokenClaimsGetUserInfo_TeamPlanPrefersPOIDMatchedWorkspace(t *testing.T) {
	claims := &IDTokenClaims{
		Name:  "User",
		Email: "user@example.com",
		OpenAIAuth: &OpenAIAuthClaims{
			ChatGPTPlanType: "team",
			POID:            "org-team-b",
			Organizations: []OrganizationClaim{
				{
					ID:    "org-personal",
					Role:  "owner",
					Title: "Personal",
				},
				{
					ID:    "org-team-a",
					Role:  "member",
					Title: "Workspace A",
				},
				{
					ID:    "org-team-b",
					Role:  "member",
					Title: "Workspace B",
				},
			},
		},
	}

	info := claims.GetUserInfo()
	if info == nil {
		t.Fatal("expected user info")
	}
	if info.OrganizationID != "org-team-b" {
		t.Fatalf("expected poid-matched workspace organization id, got %q", info.OrganizationID)
	}
	if info.OrganizationTitle != "Workspace B" {
		t.Fatalf("expected poid-matched workspace organization title, got %q", info.OrganizationTitle)
	}
	if info.OrganizationRole != "member" {
		t.Fatalf("expected poid-matched workspace organization role, got %q", info.OrganizationRole)
	}
}

func TestIDTokenClaimsGetUserInfo_TeamPlanPrefersDefaultWorkspaceOverFirstNonPersonal(t *testing.T) {
	claims := &IDTokenClaims{
		Name:  "User",
		Email: "user@example.com",
		OpenAIAuth: &OpenAIAuthClaims{
			ChatGPTPlanType: "team",
			Organizations: []OrganizationClaim{
				{
					ID:    "org-personal",
					Role:  "owner",
					Title: "Personal",
				},
				{
					ID:    "org-team-a",
					Role:  "owner",
					Title: "Workspace A",
				},
				{
					ID:        "org-team-b",
					Role:      "owner",
					Title:     "Workspace B",
					IsDefault: true,
				},
			},
		},
	}

	info := claims.GetUserInfo()
	if info == nil {
		t.Fatal("expected user info")
	}
	if info.OrganizationID != "org-team-b" {
		t.Fatalf("expected default workspace organization id, got %q", info.OrganizationID)
	}
	if info.OrganizationTitle != "Workspace B" {
		t.Fatalf("expected default workspace organization title, got %q", info.OrganizationTitle)
	}
	if info.OrganizationRole != "owner" {
		t.Fatalf("expected default workspace organization role, got %q", info.OrganizationRole)
	}
}
