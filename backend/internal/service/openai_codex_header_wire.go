package service

import (
	"net/http"
	"strings"
)

// Codex official OAuth outbound wire casing.
//
// This map is Codex-specific. Do not merge it into Claude's
// headerWireCasing / headerWireOrder in header_util.go.
//
// Only keys with in-repo casing evidence are rewritten. Uncaptured keys
// (chatgpt-account-id, conversation_id, thread-id, x-client-request-id,
// accept-language, x-codex-beta-features, x-openai-fedramp, leftover-5
// session_id, and Go-Set content-type/accept/version) keep whatever the
// builder already emitted (Content-Type / Accept / Version).
//
// Casing evidence:
//   - originator: forced lowercase raw key (issue #3901; openai_codex_identity_test.go)
//   - session-id / x-codex-*: written lowercase by outbound builders
//     (session-id is the official CLI hyphen form per extractClientSessionID)
//   - User-Agent / Authorization: P2 brief
//   - OpenAI-Beta: written that way by ensureCodexIdentityHeaders
//
// Official send order was not captured and is not applied. http.Header
// cannot preserve it, and no Codex debug dump exists.
//
// Apply only at the last mutation before send (doAccountHTTP). Callers
// Header.Set/Get after buildUpstreamRequest must still see Go-canonical keys.
var codexHeaderWireCasing = map[string]string{
	"authorization":           "Authorization",
	"user-agent":              "User-Agent",
	"originator":              "originator",
	"session-id":              "session-id",
	"openai-beta":             "OpenAI-Beta",
	"x-codex-installation-id": "x-codex-installation-id",
	"x-codex-window-id":       "x-codex-window-id",
	"x-codex-turn-metadata":   "x-codex-turn-metadata",
	"x-codex-turn-state":      "x-codex-turn-state",
}

func resolveCodexWireCasing(key string) string {
	if wk, ok := codexHeaderWireCasing[strings.ToLower(key)]; ok {
		return wk
	}
	return key
}

// applyCodexHeaderWireCasing rewrites present Codex OAuth outbound keys to
// official wire casing and collapses Go-canonical duplicates. Unknown keys
// keep their current emitted casing. originator stays lowercase; X-Originator
// / Originator are never promoted.
func applyCodexHeaderWireCasing(h http.Header) {
	if h == nil {
		return
	}
	if v := strings.TrimSpace(getHeaderRaw(h, "originator")); v != "" {
		deleteHeaderAllForms(h, "X-Originator")
		deleteHeaderAllForms(h, "Originator")
		setHeaderRaw(h, "originator", v)
	}

	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	for _, k := range keys {
		vals := h[k]
		if len(vals) == 0 {
			continue
		}
		wire := resolveCodexWireCasing(k)
		setHeaderRaw(h, wire, vals[0])
		if len(vals) > 1 {
			for _, v := range vals[1:] {
				addHeaderRaw(h, wire, v)
			}
		}
	}
}
