package service

import (
	"encoding/json"
	"net/url"
	"strings"
)

const (
	maxSupportQRCodes     = 8
	maxSupportQRNoteRunes = 80
)

// SupportQRCodeEntry is a public customer-service QR shown in the header.
type SupportQRCodeEntry struct {
	ImageURL string `json:"image_url"`
	Note     string `json:"note,omitempty"`
}

func ParseSupportQRCodes(raw string) []SupportQRCodeEntry {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" {
		return []SupportQRCodeEntry{}
	}
	var parsed []SupportQRCodeEntry
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return []SupportQRCodeEntry{}
	}
	return NormalizeSupportQRCodes(parsed)
}

func NormalizeSupportQRCodes(entries []SupportQRCodeEntry) []SupportQRCodeEntry {
	out := make([]SupportQRCodeEntry, 0, len(entries))
	for _, entry := range entries {
		imageURL := strings.TrimSpace(entry.ImageURL)
		if imageURL == "" || !isAllowedPublicImageURL(imageURL) {
			continue
		}
		note := strings.TrimSpace(entry.Note)
		if runes := []rune(note); len(runes) > maxSupportQRNoteRunes {
			note = string(runes[:maxSupportQRNoteRunes])
		}
		out = append(out, SupportQRCodeEntry{ImageURL: imageURL, Note: note})
		if len(out) >= maxSupportQRCodes {
			break
		}
	}
	return out
}

func MarshalSupportQRCodes(entries []SupportQRCodeEntry) string {
	normalized := NormalizeSupportQRCodes(entries)
	if len(normalized) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func NormalizeDownloadToolsURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return ""
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return ""
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return ""
	}
	return raw
}

func isAllowedPublicImageURL(raw string) bool {
	if strings.HasPrefix(raw, "/api/v1/media/public/") {
		id := strings.TrimPrefix(raw, "/api/v1/media/public/")
		id = strings.Trim(id, "/")
		if id == "" {
			return false
		}
		for _, ch := range id {
			if ch < '0' || ch > '9' {
				return false
			}
		}
		return true
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return false
	}
	if !strings.EqualFold(parsed.Scheme, "http") && !strings.EqualFold(parsed.Scheme, "https") {
		return false
	}
	return strings.TrimSpace(parsed.Host) != ""
}
