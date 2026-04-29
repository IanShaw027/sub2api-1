package service

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const openAIImagesTelemetryContextKey = "openai_images_telemetry"

type OpenAIImagesTelemetryStage string

const (
	OpenAIImagesTelemetryStageBootstrap        OpenAIImagesTelemetryStage = "bootstrap"
	OpenAIImagesTelemetryStageChatRequirements OpenAIImagesTelemetryStage = "chat_requirements"
	OpenAIImagesTelemetryStageConversationInit OpenAIImagesTelemetryStage = "conversation_init"
	OpenAIImagesTelemetryStagePrepare          OpenAIImagesTelemetryStage = "prepare"
	OpenAIImagesTelemetryStageUploadCreate     OpenAIImagesTelemetryStage = "upload_create"
	OpenAIImagesTelemetryStageUploadPut        OpenAIImagesTelemetryStage = "upload_put"
	OpenAIImagesTelemetryStageUploadUploaded   OpenAIImagesTelemetryStage = "upload_uploaded"
	OpenAIImagesTelemetryStageConversation     OpenAIImagesTelemetryStage = "conversation"
	OpenAIImagesTelemetryStagePoll             OpenAIImagesTelemetryStage = "poll"
	OpenAIImagesTelemetryStageDownloadURL      OpenAIImagesTelemetryStage = "download_url"
	OpenAIImagesTelemetryStageDownloadBytes    OpenAIImagesTelemetryStage = "download_bytes"
)

type OpenAIImagesTelemetry struct {
	Stages    []OpenAIImagesTelemetryStageEvent   `json:"stages,omitempty"`
	Challenge OpenAIImagesTelemetryChallengeState `json:"challenge,omitempty"`
	Profile   OpenAIImagesTelemetryProfileState   `json:"profile,omitempty"`
	Network   OpenAIImagesTelemetryNetworkState   `json:"network,omitempty"`
}

type OpenAIImagesTelemetryStageEvent struct {
	Stage     OpenAIImagesTelemetryStage `json:"stage,omitempty"`
	AtUnixMs  int64                      `json:"at_unix_ms,omitempty"`
	LatencyMs int64                      `json:"latency_ms,omitempty"`
}

type OpenAIImagesTelemetryStageOption struct {
	At        time.Time
	LatencyMs int64
}

type OpenAIImagesTelemetryChallengeState struct {
	ArkoseRequired           bool   `json:"arkose_required,omitempty"`
	TurnstileRequired        bool   `json:"turnstile_required,omitempty"`
	PoWRequired              bool   `json:"pow_required,omitempty"`
	RequirementsTokenPresent bool   `json:"requirements_token_present,omitempty"`
	ProofTokenPresent        bool   `json:"proof_token_present,omitempty"`
	UnsupportedChallenge     string `json:"unsupported_challenge,omitempty"`
}

type OpenAIImagesTelemetryProfileState struct {
	HasWebProfile     bool   `json:"has_web_profile,omitempty"`
	ProfileSource     string `json:"profile_source,omitempty"`
	ProfileAgeHours   int64  `json:"profile_age_hours,omitempty"`
	HasCookieJar      bool   `json:"has_cookie_jar,omitempty"`
	CookieNamesHash   string `json:"cookie_names_hash,omitempty"`
	CookieNamesDigest string `json:"cookie_names_digest,omitempty"`
	ProxyMatch        bool   `json:"proxy_match,omitempty"`
	UAMajor           int    `json:"ua_major,omitempty"`
	SecCHUAPresent    bool   `json:"sec_ch_ua_present,omitempty"`
}

type OpenAIImagesTelemetryNetworkState struct {
	ProxyID          string `json:"proxy_id,omitempty"`
	ProxyURLHash     string `json:"proxy_url_hash,omitempty"`
	TransportKind    string `json:"transport_kind,omitempty"`
	Impersonate      string `json:"impersonate,omitempty"`
	HTTPProto        string `json:"http_proto,omitempty"`
	TLSProfileID     string `json:"tls_profile_id,omitempty"`
	TLSProfileSource string `json:"tls_profile_source,omitempty"`
}

type OpenAIImagesTelemetryProfileInput struct {
	HasWebProfile   bool
	ProfileSource   string
	ProfileTime     time.Time
	Now             time.Time
	CookieNames     []string
	ProxyMatch      bool
	UserAgent       string
	SecCHUAPresent  bool
	CookieJarExists bool
}

func GetOpenAIImagesTelemetry(c *gin.Context) *OpenAIImagesTelemetry {
	if c == nil {
		return nil
	}
	if v, ok := c.Get(openAIImagesTelemetryContextKey); ok {
		if telemetry, ok := v.(*OpenAIImagesTelemetry); ok {
			return telemetry
		}
	}
	return nil
}

func EnsureOpenAIImagesTelemetry(c *gin.Context) *OpenAIImagesTelemetry {
	if c == nil {
		return nil
	}
	if telemetry := GetOpenAIImagesTelemetry(c); telemetry != nil {
		return telemetry
	}
	telemetry := &OpenAIImagesTelemetry{}
	c.Set(openAIImagesTelemetryContextKey, telemetry)
	return telemetry
}

func AppendOpenAIImagesTelemetryStage(c *gin.Context, stage OpenAIImagesTelemetryStage, opts ...OpenAIImagesTelemetryStageOption) {
	stage = OpenAIImagesTelemetryStage(strings.TrimSpace(string(stage)))
	if stage == "" {
		return
	}
	telemetry := EnsureOpenAIImagesTelemetry(c)
	if telemetry == nil {
		return
	}
	option := OpenAIImagesTelemetryStageOption{At: time.Now()}
	if len(opts) > 0 {
		option = opts[0]
		if option.At.IsZero() {
			option.At = time.Now()
		}
	}
	event := OpenAIImagesTelemetryStageEvent{
		Stage:    stage,
		AtUnixMs: option.At.UnixMilli(),
	}
	if option.LatencyMs >= 0 {
		event.LatencyMs = option.LatencyMs
	}
	telemetry.Stages = append(telemetry.Stages, event)
}

func SetOpenAIImagesTelemetryChallenge(c *gin.Context, state OpenAIImagesTelemetryChallengeState) {
	telemetry := EnsureOpenAIImagesTelemetry(c)
	if telemetry == nil {
		return
	}
	state.UnsupportedChallenge = strings.TrimSpace(state.UnsupportedChallenge)
	telemetry.Challenge = state
}

func SetOpenAIImagesTelemetryProfile(c *gin.Context, state OpenAIImagesTelemetryProfileState) {
	telemetry := EnsureOpenAIImagesTelemetry(c)
	if telemetry == nil {
		return
	}
	state.ProfileSource = strings.TrimSpace(state.ProfileSource)
	state.CookieNamesHash = strings.TrimSpace(state.CookieNamesHash)
	state.CookieNamesDigest = strings.TrimSpace(state.CookieNamesDigest)
	telemetry.Profile = state
}

func SetOpenAIImagesTelemetryNetwork(c *gin.Context, state OpenAIImagesTelemetryNetworkState) {
	telemetry := EnsureOpenAIImagesTelemetry(c)
	if telemetry == nil {
		return
	}
	state.ProxyID = strings.TrimSpace(state.ProxyID)
	state.ProxyURLHash = strings.TrimSpace(state.ProxyURLHash)
	state.TransportKind = strings.TrimSpace(state.TransportKind)
	state.Impersonate = strings.TrimSpace(state.Impersonate)
	state.HTTPProto = strings.TrimSpace(state.HTTPProto)
	state.TLSProfileID = strings.TrimSpace(state.TLSProfileID)
	state.TLSProfileSource = strings.TrimSpace(state.TLSProfileSource)
	telemetry.Network = state
}

func NewOpenAIImagesTelemetryProfileState(input OpenAIImagesTelemetryProfileInput) OpenAIImagesTelemetryProfileState {
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	profileAgeHours := int64(0)
	if !input.ProfileTime.IsZero() && !now.Before(input.ProfileTime) {
		profileAgeHours = int64(now.Sub(input.ProfileTime).Hours())
	}
	digest := DigestOpenAIImagesTelemetryCookieNames(input.CookieNames)
	return OpenAIImagesTelemetryProfileState{
		HasWebProfile:     input.HasWebProfile,
		ProfileSource:     strings.TrimSpace(input.ProfileSource),
		ProfileAgeHours:   profileAgeHours,
		HasCookieJar:      input.CookieJarExists || len(input.CookieNames) > 0,
		CookieNamesHash:   digest,
		CookieNamesDigest: digest,
		ProxyMatch:        input.ProxyMatch,
		UAMajor:           ParseOpenAIImagesTelemetryUAMajor(input.UserAgent),
		SecCHUAPresent:    input.SecCHUAPresent,
	}
}

func NewOpenAIImagesTelemetryNetworkState(proxyID, proxyURL, transportKind, impersonate, httpProto, tlsProfileID string, tlsProfileSource ...string) OpenAIImagesTelemetryNetworkState {
	source := ""
	if len(tlsProfileSource) > 0 {
		source = tlsProfileSource[0]
	}
	return OpenAIImagesTelemetryNetworkState{
		ProxyID:          strings.TrimSpace(proxyID),
		ProxyURLHash:     HashOpenAIImagesTelemetrySecret(proxyURL),
		TransportKind:    strings.TrimSpace(transportKind),
		Impersonate:      strings.TrimSpace(impersonate),
		HTTPProto:        strings.TrimSpace(httpProto),
		TLSProfileID:     strings.TrimSpace(tlsProfileID),
		TLSProfileSource: strings.TrimSpace(source),
	}
}

func FormatOpenAIImagesTelemetrySummary(telemetry *OpenAIImagesTelemetry) string {
	if telemetry == nil {
		return ""
	}
	lastStage := ""
	for i := len(telemetry.Stages) - 1; i >= 0; i-- {
		if stage := strings.TrimSpace(string(telemetry.Stages[i].Stage)); stage != "" {
			lastStage = stage
			break
		}
	}
	fields := []string{
		"last_stage=" + openAIImagesTelemetrySummaryValue(lastStage),
		"unsupported_challenge=" + openAIImagesTelemetrySummaryValue(telemetry.Challenge.UnsupportedChallenge),
		"has_web_profile=" + strconv.FormatBool(telemetry.Profile.HasWebProfile),
		"profile_source=" + openAIImagesTelemetrySummaryValue(telemetry.Profile.ProfileSource),
		"ua_major=" + strconv.Itoa(telemetry.Profile.UAMajor),
		"proxy_match=" + strconv.FormatBool(telemetry.Profile.ProxyMatch),
		"transport_kind=" + openAIImagesTelemetrySummaryValue(telemetry.Network.TransportKind),
		"tls_profile_source=" + openAIImagesTelemetrySummaryValue(telemetry.Network.TLSProfileSource),
	}
	return strings.Join(fields, " ")
}

func openAIImagesTelemetrySummaryValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "none"
	}
	value = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			return r
		}
		return '_'
	}, value)
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func DigestOpenAIImagesTelemetryCookieNames(cookieNames []string) string {
	names := make([]string, 0, len(cookieNames))
	seen := make(map[string]struct{}, len(cookieNames))
	for _, raw := range cookieNames {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if idx := strings.Index(name, "="); idx >= 0 {
			name = strings.TrimSpace(name[:idx])
		}
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)
	return HashOpenAIImagesTelemetrySecret(strings.Join(names, "\n"))
}

func HashOpenAIImagesTelemetrySecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

var openAIImagesTelemetryChromeMajorRe = regexp.MustCompile(`(?:Chrome|Chromium|CriOS|Edg|OPR)/(\d+)`)

func ParseOpenAIImagesTelemetryUAMajor(userAgent string) int {
	matches := openAIImagesTelemetryChromeMajorRe.FindStringSubmatch(userAgent)
	if len(matches) != 2 {
		return 0
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil || major < 0 {
		return 0
	}
	return major
}
