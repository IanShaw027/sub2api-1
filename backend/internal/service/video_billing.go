package service

import (
	"strconv"
	"strings"
)

const (
	VideoBillingTier480p  = "480p"
	VideoBillingTier720p  = "720p"
	VideoBillingTier1080p = "1080p"
	VideoBillingTier4K    = "4k"
)

func NormalizeVideoBillingTierOrDefault(size string) string {
	if tier := normalizeVideoBillingTier(size); tier != "" {
		return tier
	}
	return VideoBillingTier720p
}

func normalizeVideoBillingTier(size string) string {
	trimmed := strings.ToLower(strings.TrimSpace(size))
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	trimmed = strings.ReplaceAll(trimmed, "_", "-")
	if strings.Contains(trimmed, "4k") || strings.Contains(trimmed, "2160") {
		return VideoBillingTier4K
	}
	switch {
	case strings.Contains(trimmed, "1080p"):
		return VideoBillingTier1080p
	case strings.Contains(trimmed, "720p"):
		return VideoBillingTier720p
	case strings.Contains(trimmed, "480p"):
		return VideoBillingTier480p
	}
	if width, height, ok := parseVideoSizeDimensions(trimmed); ok {
		shortSide := width
		longSide := height
		if height < shortSide {
			shortSide = height
			longSide = width
		}
		switch {
		case shortSide >= 1440 || longSide >= 2560:
			return VideoBillingTier4K
		case shortSide >= 1080:
			return VideoBillingTier1080p
		case shortSide >= 720:
			return VideoBillingTier720p
		case shortSide >= 480:
			return VideoBillingTier480p
		default:
			return VideoBillingTier480p
		}
	}
	return ""
}

func parseVideoSizeDimensions(size string) (int, int, bool) {
	parts := strings.Split(size, "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, err := strconv.Atoi(leadingDigits(parts[0]))
	if err != nil || width <= 0 {
		return 0, 0, false
	}
	height, err := strconv.Atoi(leadingDigits(parts[1]))
	if err != nil || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func leadingDigits(value string) string {
	value = strings.TrimSpace(value)
	for i, r := range value {
		if r < '0' || r > '9' {
			return value[:i]
		}
	}
	return value
}
