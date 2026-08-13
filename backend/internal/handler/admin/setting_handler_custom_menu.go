package admin

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func isValidCustomPageSlug(slug string) bool {
	return slug != "" && len(slug) <= maxCustomPageSlugLen && customPageSlugPattern.MatchString(slug)
}

// normalizeCustomMenuItemTarget accepts:
//   - md:<slug> markdown pages
//   - a bare page slug (normalized to md:<slug>)
//   - page_slug with an empty URL
//   - same-origin relative paths starting with / (not //)
//   - absolute http(s) URLs
func normalizeCustomMenuItemTarget(rawURL, rawSlug string) (urlOut, slugOut, errMsg string) {
	urlTrimmed := strings.TrimSpace(rawURL)
	slugTrimmed := strings.TrimSpace(rawSlug)

	if slugTrimmed != "" && !isValidCustomPageSlug(slugTrimmed) {
		return "", "", "Custom menu item page_slug is invalid"
	}

	switch {
	case strings.HasPrefix(urlTrimmed, "md:"):
		slug := strings.TrimSpace(strings.TrimPrefix(urlTrimmed, "md:"))
		if !isValidCustomPageSlug(slug) {
			return "", "", "Custom menu item markdown slug cannot be empty (use md:slug format)"
		}
		if slugTrimmed != "" && slugTrimmed != slug {
			return "", "", "Custom menu item page_slug must match md:<slug>"
		}
		return "md:" + slug, slug, ""

	case urlTrimmed == "":
		if slugTrimmed == "" {
			return "", "", "Custom menu item URL is required (use md:slug for markdown pages)"
		}
		return "md:" + slugTrimmed, slugTrimmed, ""

	case isValidCustomPageSlug(urlTrimmed):
		if slugTrimmed != "" && slugTrimmed != urlTrimmed {
			return "", "", "Custom menu item page_slug must match URL slug"
		}
		return "md:" + urlTrimmed, urlTrimmed, ""

	case strings.HasPrefix(urlTrimmed, "/"):
		if err := config.ValidateFrontendRedirectURL(urlTrimmed); err != nil {
			return "", "", errCustomMenuItemURL
		}
		return urlTrimmed, "", ""

	default:
		if err := config.ValidateAbsoluteHTTPURL(urlTrimmed); err != nil {
			return "", "", errCustomMenuItemURL
		}
		return urlTrimmed, "", ""
	}
}
