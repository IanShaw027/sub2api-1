package geminicli

import (
	"encoding/json"
	"testing"
)

func TestLoadCodeAssistResponse_UnmarshalValidationAndIneligibleFields(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"currentTier": {
			"id": "standard-tier",
			"name": "Standard",
			"description": "standard plan",
			"userDefinedCloudaicompanionProject": true,
			"isDefault": true,
			"privacyNotice": {
				"showNotice": true,
				"noticeText": "notice"
			},
			"hasAcceptedTos": true,
			"hasOnboardedPreviously": true,
			"availableCredits": [
				{"creditType":"GOOGLE_ONE_AI","creditAmount":"12"}
			]
		},
		"allowedTiers": [
			{
				"id": "free-tier",
				"name": "Free",
				"userDefinedCloudaicompanionProject": false
			}
		],
		"ineligibleTiers": [
			{
				"reasonCode": "VALIDATION_REQUIRED",
				"reasonMessage": "verify identity",
				"tierId": "free-tier",
				"tierName": "Free",
				"validationErrorMessage": "complete verification",
				"validationUrl": "https://accounts.google.com/verify",
				"validationUrlLinkText": "Verify",
				"validationLearnMoreUrl": "https://support.google.com",
				"validationLearnMoreLinkText": "Learn more"
			}
		]
	}`)

	var resp LoadCodeAssistResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if resp.CurrentTier == nil {
		t.Fatal("CurrentTier should not be nil")
	}
	if resp.CurrentTier.Description != "standard plan" {
		t.Fatalf("CurrentTier.Description = %q", resp.CurrentTier.Description)
	}
	if resp.CurrentTier.UserDefinedCloudAICompanionProject == nil || !*resp.CurrentTier.UserDefinedCloudAICompanionProject {
		t.Fatal("CurrentTier.UserDefinedCloudAICompanionProject should be true")
	}
	if resp.CurrentTier.PrivacyNotice == nil || !resp.CurrentTier.PrivacyNotice.ShowNotice {
		t.Fatal("CurrentTier.PrivacyNotice should be populated")
	}
	if len(resp.CurrentTier.AvailableCredits) != 1 || resp.CurrentTier.AvailableCredits[0].CreditAmount != "12" {
		t.Fatalf("CurrentTier.AvailableCredits parsed incorrectly: %+v", resp.CurrentTier.AvailableCredits)
	}
	if len(resp.IneligibleTiers) != 1 {
		t.Fatalf("expected 1 ineligible tier, got %d", len(resp.IneligibleTiers))
	}
	if len(resp.AllowedTiers) != 1 {
		t.Fatalf("expected 1 allowed tier, got %d", len(resp.AllowedTiers))
	}
	got := resp.IneligibleTiers[0]
	if got.ReasonCode != IneligibleTierReasonCodeValidationRequired {
		t.Fatalf("ReasonCode = %q", got.ReasonCode)
	}
	if got.ValidationURL != "https://accounts.google.com/verify" {
		t.Fatalf("ValidationURL = %q", got.ValidationURL)
	}
	if got.ValidationLearnMoreLinkText != "Learn more" {
		t.Fatalf("ValidationLearnMoreLinkText = %q", got.ValidationLearnMoreLinkText)
	}
	if resp.AllowedTiers[0].UserDefinedCloudAICompanionProject == nil {
		t.Fatal("AllowedTiers[0].UserDefinedCloudAICompanionProject should be populated")
	}
}

func TestLoadCodeAssistRequest_MarshalIncludesModeAndMetadataFields(t *testing.T) {
	t.Parallel()

	req := LoadCodeAssistRequest{
		CloudAICompanionProject: "project-1",
		Mode:                    LoadCodeAssistModeHealthCheck,
		Metadata: LoadCodeAssistMetadata{
			IDEType:       "GEMINI_CLI",
			IDEVersion:    "1.2.3",
			IDEName:       "Gemini CLI",
			Platform:      "LINUX_AMD64",
			PluginType:    "GEMINI",
			PluginVersion: "2.3.4",
			UpdateChannel: "stable",
			DuetProject:   "project-1",
		},
	}

	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded["mode"] != string(LoadCodeAssistModeHealthCheck) {
		t.Fatalf("mode = %v", decoded["mode"])
	}
	metadata, ok := decoded["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata should be an object")
	}
	if metadata["ideVersion"] != "1.2.3" {
		t.Fatalf("ideVersion = %v", metadata["ideVersion"])
	}
	if metadata["pluginVersion"] != "2.3.4" {
		t.Fatalf("pluginVersion = %v", metadata["pluginVersion"])
	}
	if metadata["updateChannel"] != "stable" {
		t.Fatalf("updateChannel = %v", metadata["updateChannel"])
	}
}

func TestRetrieveUserQuotaRequestAndResponse_JSONShape(t *testing.T) {
	t.Parallel()

	req := RetrieveUserQuotaRequest{
		Project:   "project-1",
		UserAgent: "gemini-cli/1.0",
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var reqMap map[string]any
	if err := json.Unmarshal(raw, &reqMap); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if reqMap["project"] != "project-1" {
		t.Fatalf("project = %v", reqMap["project"])
	}
	if reqMap["userAgent"] != "gemini-cli/1.0" {
		t.Fatalf("userAgent = %v", reqMap["userAgent"])
	}

	respRaw := []byte(`{
		"buckets": [
			{
				"modelId":"gemini-2.5-pro",
				"remainingFraction":0.5,
				"remainingAmount":"120",
				"resetTime":"2026-05-08T00:00:00Z",
				"tokenType":"TOKENS"
			}
		]
	}`)
	var resp RetrieveUserQuotaResponse
	if err := json.Unmarshal(respRaw, &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(resp.Buckets))
	}
	if resp.Buckets[0].TokenType != "TOKENS" {
		t.Fatalf("tokenType = %q", resp.Buckets[0].TokenType)
	}
}
