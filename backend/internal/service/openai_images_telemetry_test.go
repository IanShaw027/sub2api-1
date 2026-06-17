package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIImageRouteRateLimitRepoStub struct {
	stubOpenAIAccountRepo
	updatedExtra map[string]any
}

func (r *openAIImageRouteRateLimitRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updatedExtra = updates
	return nil
}

func TestOpenAIImagesTelemetryAppendStageEvent(t *testing.T) {
	c := newOpenAIImagesTelemetryTestContext()

	AppendOpenAIImagesTelemetryStage(c, OpenAIImagesTelemetryStageBootstrap)
	AppendOpenAIImagesTelemetryStage(c, OpenAIImagesTelemetryStageConversation, OpenAIImagesTelemetryStageOption{LatencyMs: 42})

	telemetry := GetOpenAIImagesTelemetry(c)
	require.NotNil(t, telemetry)
	require.Len(t, telemetry.Stages, 2)
	require.Equal(t, OpenAIImagesTelemetryStageBootstrap, telemetry.Stages[0].Stage)
	require.Positive(t, telemetry.Stages[0].AtUnixMs)
	require.Equal(t, OpenAIImagesTelemetryStageConversation, telemetry.Stages[1].Stage)
	require.EqualValues(t, 42, telemetry.Stages[1].LatencyMs)
}

func TestOpenAIImagesTelemetryChallengeFlags(t *testing.T) {
	c := newOpenAIImagesTelemetryTestContext()

	SetOpenAIImagesTelemetryChallenge(c, OpenAIImagesTelemetryChallengeState{
		ArkoseRequired:           true,
		TurnstileRequired:        true,
		PoWRequired:              true,
		RequirementsTokenPresent: true,
		ProofTokenPresent:        true,
		UnsupportedChallenge:     "webauthn",
	})

	telemetry := GetOpenAIImagesTelemetry(c)
	require.NotNil(t, telemetry)
	require.True(t, telemetry.Challenge.ArkoseRequired)
	require.True(t, telemetry.Challenge.TurnstileRequired)
	require.True(t, telemetry.Challenge.PoWRequired)
	require.True(t, telemetry.Challenge.RequirementsTokenPresent)
	require.True(t, telemetry.Challenge.ProofTokenPresent)
	require.Equal(t, "webauthn", telemetry.Challenge.UnsupportedChallenge)
}

func TestOpenAIImagesChallengeTelemetryStateMarksUnsupportedChallenges(t *testing.T) {
	reqs := &openAIChatRequirements{}
	reqs.Arkose.Required = true
	reqs.Turnstile.Required = true
	reqs.ProofOfWork.Required = true
	reqs.Token = "requirements-token"

	state := openAIImagesChallengeTelemetryStateWithProof(reqs, "proof-token", "turnstile")
	require.True(t, state.ArkoseRequired)
	require.True(t, state.TurnstileRequired)
	require.True(t, state.PoWRequired)
	require.True(t, state.RequirementsTokenPresent)
	require.True(t, state.ProofTokenPresent)
	require.Equal(t, "turnstile", state.UnsupportedChallenge)
}

func TestOpenAIImagesTelemetryCookieNameDigestDoesNotContainRawCookieValue(t *testing.T) {
	digest := DigestOpenAIImagesTelemetryCookieNames([]string{
		"__Secure-next-auth.session-token=raw-secret-token-value",
		"oai-did=device-secret",
		"cf_clearance=clearance-secret",
	})

	require.NotEmpty(t, digest)
	require.NotContains(t, digest, "raw-secret-token-value")
	require.NotContains(t, digest, "device-secret")
	require.NotContains(t, digest, "clearance-secret")
	require.NotContains(t, digest, "__Secure-next-auth.session-token")
	require.True(t, strings.HasPrefix(digest, "sha256:"))
}

func TestOpenAIImagesTelemetryProfileAgeCalculation(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	profileTime := now.Add(-49*time.Hour - 30*time.Minute)

	profile := NewOpenAIImagesTelemetryProfileState(OpenAIImagesTelemetryProfileInput{
		HasWebProfile:   true,
		ProfileSource:   "account_profile",
		ProfileTime:     profileTime,
		Now:             now,
		CookieNames:     []string{"oai-did=device-secret"},
		ProxyMatch:      true,
		UserAgent:       "Mozilla/5.0 Chrome/131.0.0.0 Safari/537.36",
		SecCHUAPresent:  true,
		CookieJarExists: true,
	})

	require.True(t, profile.HasWebProfile)
	require.Equal(t, "account_profile", profile.ProfileSource)
	require.EqualValues(t, 49, profile.ProfileAgeHours)
	require.True(t, profile.HasCookieJar)
	require.True(t, profile.ProxyMatch)
	require.EqualValues(t, 131, profile.UAMajor)
	require.True(t, profile.SecCHUAPresent)
}

func TestOpenAIImagesTelemetryProxyHashStable(t *testing.T) {
	first := HashOpenAIImagesTelemetrySecret("http://user:pass@example.test:8080")
	second := HashOpenAIImagesTelemetrySecret("http://user:pass@example.test:8080")
	other := HashOpenAIImagesTelemetrySecret("http://user:pass@example.test:9090")

	require.NotEmpty(t, first)
	require.Equal(t, first, second)
	require.NotEqual(t, first, other)
	require.NotContains(t, first, "user")
	require.NotContains(t, first, "pass")
	require.True(t, strings.HasPrefix(first, "sha256:"))
}

func TestFormatOpenAIImagesTelemetrySummarySafe(t *testing.T) {
	telemetry := &OpenAIImagesTelemetry{
		Stages: []OpenAIImagesTelemetryStageEvent{
			{Stage: OpenAIImagesTelemetryStageBootstrap},
			{Stage: OpenAIImagesTelemetryStageConversation},
		},
		Challenge: OpenAIImagesTelemetryChallengeState{
			UnsupportedChallenge: "turnstile",
		},
		Profile: OpenAIImagesTelemetryProfileState{
			HasWebProfile:     true,
			ProfileSource:     "cookie_profile",
			CookieNamesHash:   HashOpenAIImagesTelemetrySecret("__Secure-next-auth.session-token=raw-cookie-value"),
			CookieNamesDigest: HashOpenAIImagesTelemetrySecret("oai-did=device-secret"),
			ProxyMatch:        true,
			UAMajor:           131,
		},
		Network: OpenAIImagesTelemetryNetworkState{
			ProxyURLHash:     HashOpenAIImagesTelemetrySecret("http://user:pass@example.test:8080"),
			TransportKind:    "impersonate",
			TLSProfileSource: "not_applied",
		},
	}

	summary := FormatOpenAIImagesTelemetrySummary(telemetry)

	require.Contains(t, summary, "last_stage=conversation")
	require.Contains(t, summary, "unsupported_challenge=turnstile")
	require.Contains(t, summary, "has_web_profile=true")
	require.Contains(t, summary, "profile_source=cookie_profile")
	require.Contains(t, summary, "ua_major=131")
	require.Contains(t, summary, "proxy_match=true")
	require.Contains(t, summary, "transport_kind=impersonate")
	require.Contains(t, summary, "tls_profile_source=not_applied")
	require.NotContains(t, summary, "raw-cookie-value")
	require.NotContains(t, summary, "device-secret")
	require.NotContains(t, summary, "user:pass")
	require.NotContains(t, summary, "example.test")
	require.NotContains(t, summary, "sha256:")
}

func TestWrapOpenAIImageBackendErrorAppendsTelemetrySummaryToOpsMessage(t *testing.T) {
	c := newOpenAIImagesTelemetryTestContext()
	AppendOpenAIImagesTelemetryStage(c, OpenAIImagesTelemetryStageConversation)
	SetOpenAIImagesTelemetryChallenge(c, OpenAIImagesTelemetryChallengeState{UnsupportedChallenge: "arkose"})
	SetOpenAIImagesTelemetryProfile(c, OpenAIImagesTelemetryProfileState{
		HasWebProfile: true,
		ProfileSource: "account_profile",
		ProxyMatch:    false,
		UAMajor:       130,
	})
	SetOpenAIImagesTelemetryNetwork(c, NewOpenAIImagesTelemetryNetworkState("proxy-1", "http://user:pass@example.test:8080", "default", "", "", ""))
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 42, Name: "openai-oauth", Platform: PlatformOpenAI}

	err := svc.wrapOpenAIImageBackendError(context.Background(), c, account, GroupImageGenerationRouteWeb2API, newOpenAIImageSyntheticStatusError(400, "backend-api request failed", "https://chatgpt.com/backend-api/conversation"))

	require.Error(t, err)
	rawEvents, ok := c.Get(OpsUpstreamErrorsKey)
	require.True(t, ok)
	events, ok := rawEvents.([]*OpsUpstreamErrorEvent)
	require.True(t, ok)
	require.Len(t, events, 1)
	require.Contains(t, events[0].Message, "openai_images_telemetry:")
	require.Contains(t, events[0].Message, "last_stage=conversation")
	require.Contains(t, events[0].Message, "unsupported_challenge=arkose")
	require.Contains(t, events[0].Message, "has_web_profile=true")
	require.Contains(t, events[0].Message, "profile_source=account_profile")
	require.Contains(t, events[0].Message, "proxy_match=false")
	require.NotContains(t, events[0].Message, "user:pass")
	require.NotContains(t, events[0].Message, "example.test")
}

func TestOpenAIImagesLegacyBridgeDoesNotDegradeHealthyProfile403(t *testing.T) {
	c := newOpenAIImagesTelemetryTestContext()
	SetOpenAIImagesTelemetryProfile(c, OpenAIImagesTelemetryProfileState{HasWebProfile: true, ProxyMatch: true})
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 44, Name: "openai-oauth", Platform: PlatformOpenAI}

	err := svc.wrapOpenAIImageBackendError(context.Background(), c, account, GroupImageGenerationRouteWeb2API, newOpenAIImageSyntheticStatusError(http.StatusForbidden, "backend-api request failed", "https://chatgpt.com/backend-api/conversation"))

	require.Error(t, err)
}

func TestWrapOpenAIImageBackendErrorIgnoresSynthetic429WithoutUpstreamHeaders(t *testing.T) {
	c := newOpenAIImagesTelemetryTestContext()
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 45, Name: "openai-oauth", Platform: PlatformOpenAI}

	err := svc.wrapOpenAIImageBackendError(context.Background(), c, account, GroupImageGenerationRouteWeb2API, newOpenAIImageSyntheticStatusError(http.StatusTooManyRequests, "group rate limited", ""))

	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
}

func TestWrapOpenAIImageBackendErrorFailsoverReal429WithoutHeadersWhenBodyHasReset(t *testing.T) {
	c := newOpenAIImagesTelemetryTestContext()
	repo := &openAIImageRouteRateLimitRepoStub{}
	svc := &OpenAIGatewayService{
		rateLimitService: NewRateLimitService(repo, nil, nil, nil, nil),
	}
	account := &Account{ID: 46, Name: "openai-oauth", Platform: PlatformOpenAI}
	err := svc.wrapOpenAIImageBackendError(
		context.Background(),
		c,
		account,
		GroupImageGenerationRouteWeb2API,
		&openAIImageStatusError{
			StatusCode:      http.StatusTooManyRequests,
			Message:         "usage limit reached",
			ResponseBody:    []byte(`{"error":{"type":"usage_limit_reached","resets_at":1777283883}}`),
			ResponseHeaders: http.Header{},
		},
	)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.NotEmpty(t, repo.updatedExtra)
	require.Contains(t, repo.updatedExtra, "openai_image_web2api_rate_limit_reset_at")
}

func newOpenAIImagesTelemetryTestContext() *gin.Context {
	setGinTestMode()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	return c
}
