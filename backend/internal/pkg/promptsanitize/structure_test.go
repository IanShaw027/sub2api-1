package promptsanitize

import (
	"strings"
	"testing"
)

func TestSanitizeStructure_StripsHeadersAndXMLOutsideCodeFences(t *testing.T) {
	input := "## Workspace\n<workspace>repo</workspace>\n\n```xml\n## Keep Header\n<workspace>keep</workspace>\n```\n"
	got := SanitizeStructure(input)

	if strings.Contains(got, "## Workspace") {
		t.Fatalf("expected markdown header marker outside fence to be removed, got %q", got)
	}
	if strings.Contains(got, "<workspace>repo</workspace>") {
		t.Fatalf("expected workspace tags outside fence to be stripped, got %q", got)
	}
	if !strings.Contains(got, "repo") {
		t.Fatalf("expected workspace content to be preserved, got %q", got)
	}
	if !strings.Contains(got, "## Keep Header") || !strings.Contains(got, "<workspace>keep</workspace>") {
		t.Fatalf("expected fenced content to be preserved, got %q", got)
	}
}

func TestSanitizeStructure_PreservesBillingAttributionLine(t *testing.T) {
	input := "x-anthropic-billing-header: cc_version=2.1.161.abc; cc_entrypoint=cli; cch=00000;\n# Task\n"
	got := SanitizeStructure(input)

	if !strings.Contains(got, "x-anthropic-billing-header:") || !strings.Contains(got, "cch=00000") {
		t.Fatalf("expected billing attribution line to remain intact, got %q", got)
	}
	if strings.Contains(got, "# Task") {
		t.Fatalf("expected markdown task header marker to be stripped, got %q", got)
	}
}

func TestSanitizeStructure_UnclosedFenceProtectsRemainingText(t *testing.T) {
	input := "Before\n```go\n## keep fenced header\n<workspace>keep</workspace>"
	got := SanitizeStructure(input)

	if !strings.Contains(got, "## keep fenced header") || !strings.Contains(got, "<workspace>keep</workspace>") {
		t.Fatalf("expected unclosed fenced content to be preserved, got %q", got)
	}
}
