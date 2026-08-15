package service

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

func maybeLearnOfficialDeviceProfile(ctx context.Context, account *Account, headers http.Header) {
	if account == nil || headers == nil || !accountExtraDeviceLearningEnabled(account) {
		return
	}
	switch account.Platform {
	case PlatformAnthropic, PlatformOpenAI:
	default:
		return
	}
	inbound := officialInboundFromHeaders(headers)
	if inbound.UserAgent == "" || !isOfficialInbound(account.Platform, inbound) {
		return
	}
	svc := OutboundDeviceProfileService()
	if svc == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	learnCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), outboundDeviceProfileLoadTimeout)
	defer cancel()
	if _, err := svc.LearnIfOfficial(learnCtx, account, inbound); err != nil {
		slog.Warn("device profile learn failed", "account_id", account.ID, "error", err)
	}
}

func accountExtraDeviceLearningEnabled(account *Account) bool {
	if account == nil || account.Extra == nil {
		return false
	}
	enabled, ok := account.Extra["device_learning_enabled"].(bool)
	return ok && enabled
}

func officialInboundFromHeaders(headers http.Header) OfficialInbound {
	inbound := OfficialInbound{
		UserAgent:      strings.TrimSpace(getHeaderRaw(headers, "User-Agent")),
		Originator:     strings.TrimSpace(firstNonEmptyHeader(headers, "originator", "Originator")),
		ClientVersion:  strings.TrimSpace(firstNonEmptyHeader(headers, "version", "X-Client-Version")),
		Runtime:        strings.TrimSpace(getHeaderRaw(headers, "X-Stainless-Runtime")),
		RuntimeVersion: strings.TrimSpace(getHeaderRaw(headers, "X-Stainless-Runtime-Version")),
	}
	payload := map[string]any{}
	for key, header := range map[string]string{
		"stainless_lang":            "X-Stainless-Lang",
		"stainless_package_version": "X-Stainless-Package-Version",
		"stainless_os":              "X-Stainless-OS",
		"stainless_arch":            "X-Stainless-Arch",
		"stainless_runtime":         "X-Stainless-Runtime",
		"stainless_runtime_version": "X-Stainless-Runtime-Version",
	} {
		if value := strings.TrimSpace(getHeaderRaw(headers, header)); value != "" {
			payload[key] = value
		}
	}
	if len(payload) > 0 {
		inbound.Payload = payload
	}
	return inbound
}

func firstNonEmptyHeader(headers http.Header, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(getHeaderRaw(headers, key)); value != "" {
			return value
		}
	}
	return ""
}
