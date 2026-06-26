package service

import "strings"

type openAIResponsesContinuationAction string

const (
	openAIResponsesContinuationActionHotWSIncremental         openAIResponsesContinuationAction = "hot_ws_incremental"
	openAIResponsesContinuationActionColdHTTPIncremental      openAIResponsesContinuationAction = "cold_http_incremental"
	openAIResponsesContinuationActionFullRebuild              openAIResponsesContinuationAction = "full_rebuild"
	openAIResponsesContinuationActionRejectUnsafeContinuation openAIResponsesContinuationAction = "reject_unsafe"
)

type openAIResponsesContinuationInput struct {
	AccountType                    string
	StoreDisabled                  bool
	PreviousResponseID             string
	LiveWSAvailable                bool
	StickyAccountHit               bool
	StickyConnHit                  bool
	SessionWindowAvailable         bool
	HasFunctionCallOutput          bool
	HasSafeToolReplayWindow        bool
	DurableContinuationAllowed     bool
	PersistedContinuationAvailable bool
}

type openAIResponsesContinuationDecision struct {
	Action openAIResponsesContinuationAction
}

type openAIResponsesContinuationPlanner struct{}

func (openAIResponsesContinuationPlanner) Plan(in openAIResponsesContinuationInput) openAIResponsesContinuationDecision {
	if in.HasFunctionCallOutput && !in.HasSafeToolReplayWindow {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionRejectUnsafeContinuation}
	}

	if strings.TrimSpace(in.PreviousResponseID) == "" {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionFullRebuild}
	}

	if in.LiveWSAvailable && in.StickyAccountHit && in.StickyConnHit {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionHotWSIncremental}
	}

	if in.StoreDisabled {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionFullRebuild}
	}

	if in.DurableContinuationAllowed && in.PersistedContinuationAvailable && in.StickyAccountHit && in.SessionWindowAvailable {
		return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionColdHTTPIncremental}
	}

	return openAIResponsesContinuationDecision{Action: openAIResponsesContinuationActionFullRebuild}
}
