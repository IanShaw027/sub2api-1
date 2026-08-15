package service

import (
	"net/http"
	"strings"
)

// Codex official OAuth outbound wire casing.
//
// These maps are Codex-specific. Do not merge them into Claude's
// headerWireCasing / headerWireOrder in header_util.go.
//
// Casing evidence (in-repo, not a live capture):
//   - originator: forced lowercase raw key (issue #3901; openai_codex_identity_test.go)
//   - version / session-id / x-codex-*: written lowercase by outbound builders
//     (session-id is the official CLI hyphen form per extractClientSessionID)
//   - session_id: leftover 5 / fingerprint also emit the underscore form; keep it
//   - User-Agent / Authorization / content-type / accept: as listed in the P2 brief
//   - OpenAI-Beta: written that way by ensureCodexIdentityHeaders
//
// Official send order was not captured in-repo. The declared order is a stable
// identity-then-fingerprint-then-transport list for debug comparison only.
var codexHeaderWireCasing = map[string]string{
	"authorization": "Authorization",
	"user-agent":    "User-Agent",
	"accept":        "accept",
	"content-type":  "content-type",
	"originator":    "originator",
	"version":       "version",
	"session-id":    "session-id",
	// session_id is leftover 5's underscore form, not the official CLI header.
	// Keep Go-canonical Session_id so existing Header.Get("session_id") readers
	// and the account-stable overwrite stay intact.
	"chatgpt-account-id":      "chatgpt-account-id",
	"openai-beta":             "OpenAI-Beta",
	"x-codex-installation-id": "x-codex-installation-id",
	"x-codex-window-id":       "x-codex-window-id",
	"x-codex-turn-metadata":   "x-codex-turn-metadata",
	"x-codex-turn-state":      "x-codex-turn-state",
	"conversation_id":         "conversation_id",
	"thread-id":               "thread-id",
	"x-client-request-id":     "x-client-request-id",
	"accept-language":         "accept-language",
	"x-codex-beta-features":   "x-codex-beta-features",
	"x-openai-fedramp":        "x-openai-fedramp",
}

var codexHeaderWireOrder = []string{
	"Authorization",
	"User-Agent",
	"accept",
	"content-type",
	"originator",
	"version",
	"session-id",
	"session_id",
	"chatgpt-account-id",
	"OpenAI-Beta",
	"x-codex-installation-id",
	"x-codex-window-id",
	"x-codex-turn-metadata",
	"x-codex-turn-state",
	"conversation_id",
	"thread-id",
	"x-client-request-id",
	"accept-language",
	"x-codex-beta-features",
	"x-openai-fedramp",
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

// sortCodexHeadersByWireOrder returns header keys in the declared Codex wire
// order. Keys not in the list are appended after the known ones.
func sortCodexHeadersByWireOrder(h http.Header) []string {
	present := make(map[string]string, len(h))
	for k := range h {
		present[strings.ToLower(k)] = k
	}

	result := make([]string, 0, len(h))
	seen := make(map[string]struct{}, len(h))

	for _, wk := range codexHeaderWireOrder {
		lk := strings.ToLower(wk)
		if actual, ok := present[lk]; ok {
			if _, dup := seen[lk]; !dup {
				result = append(result, actual)
				seen[lk] = struct{}{}
			}
		}
	}

	for k := range h {
		lk := strings.ToLower(k)
		if _, ok := seen[lk]; !ok {
			result = append(result, k)
			seen[lk] = struct{}{}
		}
	}

	return result
}
