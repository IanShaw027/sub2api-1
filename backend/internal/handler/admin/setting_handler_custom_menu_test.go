//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCustomMenuItemTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		url      string
		slug     string
		wantURL  string
		wantSlug string
		wantErr  string
	}{
		{name: "absolute https", url: "https://example.com/page", wantURL: "https://example.com/page"},
		{name: "md prefix", url: "md:help-center", wantURL: "md:help-center", wantSlug: "help-center"},
		{name: "bare slug", url: "help", wantURL: "md:help", wantSlug: "help"},
		{name: "page_slug only", slug: "docs", wantURL: "md:docs", wantSlug: "docs"},
		{name: "same-origin path", url: "/docs/guide", wantURL: "/docs/guide"},
		{name: "rejects protocol-relative", url: "//evil.example/x", wantErr: errCustomMenuItemURL},
		{name: "rejects javascript", url: "javascript:alert(1)", wantErr: errCustomMenuItemURL},
		{name: "rejects empty", wantErr: "Custom menu item URL is required (use md:slug for markdown pages)"},
		{name: "rejects empty md slug", url: "md:", wantErr: "Custom menu item markdown slug cannot be empty (use md:slug format)"},
		{name: "rejects invalid slug chars", url: "md:../etc", wantErr: "Custom menu item markdown slug cannot be empty (use md:slug format)"},
		{name: "rejects mismatched page_slug", url: "md:help", slug: "other", wantErr: "Custom menu item page_slug must match md:<slug>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotURL, gotSlug, errMsg := normalizeCustomMenuItemTarget(tt.url, tt.slug)
			if tt.wantErr != "" {
				require.Equal(t, tt.wantErr, errMsg)
				return
			}
			require.Empty(t, errMsg)
			require.Equal(t, tt.wantURL, gotURL)
			require.Equal(t, tt.wantSlug, gotSlug)
		})
	}
}

func TestUpdateSettingsAcceptsCustomPageURLs(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})

	rec := doUpdateSettings(t, h, map[string]any{
		"custom_menu_items": []map[string]any{
			{"id": "help1", "label": "Help", "url": "help", "visibility": "user", "sort_order": 0},
			{"id": "docs1", "label": "Docs", "url": "/docs", "visibility": "user", "sort_order": 1},
			{"id": "ext1", "label": "Ext", "url": "https://example.com/page", "visibility": "admin", "sort_order": 2},
		},
	}, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	raw := repo.values[service.SettingKeyCustomMenuItems]
	require.NotEmpty(t, raw)

	var items []struct {
		URL      string `json:"url"`
		PageSlug string `json:"page_slug"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &items))
	require.Len(t, items, 3)
	require.Equal(t, "md:help", items[0].URL)
	require.Equal(t, "help", items[0].PageSlug)
	require.Equal(t, "/docs", items[1].URL)
	require.Empty(t, items[1].PageSlug)
	require.Equal(t, "https://example.com/page", items[2].URL)
}

func TestUpdateSettingsRejectsUnsafeCustomMenuURL(t *testing.T) {
	h, _ := newStepUpSwitchTestHandler(t, map[string]string{})

	rec := doUpdateSettings(t, h, map[string]any{
		"custom_menu_items": []map[string]any{
			{"id": "bad1", "label": "Bad", "url": "javascript:alert(1)", "visibility": "user", "sort_order": 0},
		},
	}, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "same-origin path")
}
