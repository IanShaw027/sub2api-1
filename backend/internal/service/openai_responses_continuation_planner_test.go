package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIResponsesContinuationPlanner_OAuthStoreFalsePrefersHotWS(t *testing.T) {
	planner := openAIResponsesContinuationPlanner{}

	decision := planner.Plan(openAIResponsesContinuationInput{
		AccountType:        AccountTypeOAuth,
		StoreDisabled:      true,
		PreviousResponseID: "resp_hot_1",
		LiveWSAvailable:    true,
		StickyAccountHit:   true,
		StickyConnHit:      true,
	})

	require.Equal(t, openAIResponsesContinuationActionHotWSIncremental, decision.Action)
}

func TestOpenAIResponsesContinuationPlanner_OAuthStoreFalseWithoutLiveWSRebuilds(t *testing.T) {
	planner := openAIResponsesContinuationPlanner{}

	decision := planner.Plan(openAIResponsesContinuationInput{
		AccountType:            AccountTypeOAuth,
		StoreDisabled:          true,
		PreviousResponseID:     "resp_cold_1",
		LiveWSAvailable:        false,
		StickyAccountHit:       true,
		SessionWindowAvailable: true,
	})

	require.Equal(t, openAIResponsesContinuationActionFullRebuild, decision.Action)
}

func TestOpenAIResponsesContinuationPlanner_DurableLaneAllowsColdHTTP(t *testing.T) {
	planner := openAIResponsesContinuationPlanner{}

	decision := planner.Plan(openAIResponsesContinuationInput{
		AccountType:                   AccountTypeOAuth,
		StoreDisabled:                 false,
		PreviousResponseID:            "resp_durable_1",
		LiveWSAvailable:               false,
		StickyAccountHit:              true,
		SessionWindowAvailable:        true,
		DurableContinuationAllowed:    true,
		PersistedContinuationAvailable: true,
	})

	require.Equal(t, openAIResponsesContinuationActionColdHTTPIncremental, decision.Action)
}

func TestOpenAIResponsesContinuationPlanner_RejectsUnsafeToolContinuation(t *testing.T) {
	planner := openAIResponsesContinuationPlanner{}

	decision := planner.Plan(openAIResponsesContinuationInput{
		AccountType:             AccountTypeOAuth,
		StoreDisabled:           true,
		PreviousResponseID:      "resp_tool_1",
		HasFunctionCallOutput:   true,
		HasSafeToolReplayWindow: false,
	})

	require.Equal(t, openAIResponsesContinuationActionRejectUnsafeContinuation, decision.Action)
}
