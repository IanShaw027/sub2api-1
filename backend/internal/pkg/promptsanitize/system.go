package promptsanitize

import (
	"regexp"
	"strings"
)

var (
	identityPlaceholderLinePattern = regexp.MustCompile(`(?i)^\{\{\s*identity\s*\}\}$`)
	upstreamIdentityBannerPattern  = regexp.MustCompile(`(?i)^(?:you are|you're)\s+(?:\{\{\s*identity\s*\}\}|opencode|kiro|amazon q(?: developer)?|aws(?: amazon q(?: developer)?)?)\b`)
)

// SystemText replaces a narrow set of known upstream identity banners with the
// canonical proxy banner while leaving user-authored instructions untouched.
func SystemText(text, canonicalBanner string) string {
	text = strings.TrimSpace(text)
	canonicalBanner = strings.TrimSpace(canonicalBanner)
	if text == "" || canonicalBanner == "" {
		return text
	}

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	replacedBanner := false
	modified := false

	appendBanner := func() {
		if replacedBanner {
			return
		}
		out = append(out, canonicalBanner)
		replacedBanner = true
		modified = true
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			out = append(out, line)
		case identityPlaceholderLinePattern.MatchString(trimmed):
			appendBanner()
		case upstreamIdentityBannerPattern.MatchString(trimmed):
			appendBanner()
		default:
			out = append(out, line)
		}
	}

	if !modified {
		return text
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
