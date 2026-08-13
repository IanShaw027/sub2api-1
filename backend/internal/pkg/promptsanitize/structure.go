package promptsanitize

import (
	"regexp"
	"strings"
)

// Structural markers that commonly appear in third-party harness system prompts.
// These are stripped (tags removed, content preserved) to reduce template fingerprinting.
// Only harness-specific compound tags are included; generic tags like "context", "config",
// "instructions", "capabilities" are excluded to avoid mutating user-authored prompts.
var structuralXMLTags = []string{
	"workspace",
	"system-reminder", "system_reminder",
	"system-instructions", "system_instructions",
	"tool-config", "tool_config",
	"environment_details", "env_details",
}

var (
	// markdownHeaderRe matches markdown headers like "## Tooling", "### Workspace"
	markdownHeaderRe = regexp.MustCompile(`(?m)^(#{1,6})\s+`)

	// excessiveNewlinesRe matches 3+ consecutive blank lines
	excessiveNewlinesRe = regexp.MustCompile(`\n{3,}`)
)

// SanitizeStructure strips structural formatting markers from system prompt text
// to reduce template fingerprinting signals. Markdown headers lose their # prefix,
// known XML tags are stripped (content preserved), and excessive blank lines are collapsed.
// Code-fenced blocks (``` ... ```) are protected from modification.
// The function is idempotent and has no side effects.
func SanitizeStructure(text string) string {
	if text == "" {
		return text
	}

	// Split by code fences to protect fenced content
	segments := splitByCodeFences(text)
	for i, seg := range segments {
		if seg.fenced {
			continue
		}
		s := seg.text

		// Strip markdown headers: "## Tooling" → "Tooling"
		s = markdownHeaderRe.ReplaceAllString(s, "")

		// Strip known XML tags but keep content
		for _, tag := range structuralXMLTags {
			s = stripXMLTag(s, tag)
		}

		// Collapse 3+ consecutive newlines to 2
		s = excessiveNewlinesRe.ReplaceAllString(s, "\n\n")

		segments[i].text = s
	}

	// Reassemble
	var b strings.Builder
	for _, seg := range segments {
		_, _ = b.WriteString(seg.text)
	}
	return b.String()
}

type textSegment struct {
	text   string
	fenced bool
}

// splitByCodeFences splits text into alternating segments of unfenced and fenced content.
func splitByCodeFences(text string) []textSegment {
	const fence = "```"
	var segments []textSegment
	remaining := text
	inFence := false

	for {
		idx := strings.Index(remaining, fence)
		if idx == -1 {
			if remaining != "" {
				segments = append(segments, textSegment{text: remaining, fenced: inFence})
			}
			break
		}
		// Find end of fence marker (consume the ``` and optional language tag on same line)
		fenceEnd := idx + len(fence)
		if !inFence {
			// Opening fence: consume to end of line (language tag)
			nlIdx := strings.IndexByte(remaining[fenceEnd:], '\n')
			if nlIdx >= 0 {
				fenceEnd += nlIdx + 1
			} else {
				fenceEnd = len(remaining)
			}
		} else {
			// Closing fence: consume to end of line
			nlIdx := strings.IndexByte(remaining[fenceEnd:], '\n')
			if nlIdx >= 0 {
				fenceEnd += nlIdx + 1
			} else {
				fenceEnd = len(remaining)
			}
		}

		if idx > 0 {
			segments = append(segments, textSegment{text: remaining[:idx], fenced: inFence})
		}
		segments = append(segments, textSegment{text: remaining[idx:fenceEnd], fenced: true})
		remaining = remaining[fenceEnd:]
		inFence = !inFence
	}

	return segments
}

// stripXMLTag removes <tag ...> and </tag> occurrences (case-insensitive) while preserving inner content.
func stripXMLTag(s, tag string) string {
	lowerTag := strings.ToLower(tag)

	// Build patterns for opening and closing tags
	// Opening: <tag>, <tag attr="val">, <tag/>, etc.
	openRe := regexp.MustCompile(`(?i)<` + regexp.QuoteMeta(lowerTag) + `(?:\s[^>]*)?>`)
	closeRe := regexp.MustCompile(`(?i)</` + regexp.QuoteMeta(lowerTag) + `\s*>`)

	s = openRe.ReplaceAllString(s, "")
	s = closeRe.ReplaceAllString(s, "")
	return s
}
