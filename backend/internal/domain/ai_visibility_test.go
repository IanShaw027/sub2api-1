package domain

import "testing"

func TestEffectivePromptTemplateVisibility_ForcedPrivateAndBlockedBecomePrivate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		visibility      string
		moderationState string
		want            string
	}{
		{name: "public forced private", visibility: AIVisibilityPublic, moderationState: AIModerationStateForcedPrivate, want: AIVisibilityPrivate},
		{name: "unlisted forced private", visibility: AIVisibilityUnlisted, moderationState: AIModerationStateForcedPrivate, want: AIVisibilityPrivate},
		{name: "public blocked", visibility: AIVisibilityPublic, moderationState: AIModerationStateBlocked, want: AIVisibilityPrivate},
		{name: "normal public", visibility: AIVisibilityPublic, moderationState: AIModerationStateNormal, want: AIVisibilityPublic},
		{name: "normal unlisted", visibility: AIVisibilityUnlisted, moderationState: AIModerationStateNormal, want: AIVisibilityUnlisted},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := EffectivePromptTemplateVisibility(tc.visibility, tc.moderationState)
			if got != tc.want {
				t.Fatalf("unexpected visibility: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestCanReadPromptTemplate_RespectsOwnerAdminAndModeration(t *testing.T) {
	t.Parallel()

	if !CanReadPromptTemplate(10, 99, true, AIVisibilityPrivate, AIModerationStateBlocked) {
		t.Fatalf("admin should always read")
	}
	if !CanReadPromptTemplate(10, 10, false, AIVisibilityPrivate, AIModerationStateBlocked) {
		t.Fatalf("owner should read own prompt even when moderated")
	}
	if !CanReadPromptTemplate(10, 99, false, AIVisibilityPublic, AIModerationStateNormal) {
		t.Fatalf("public normal prompt should be readable")
	}
	if !CanReadPromptTemplate(10, 99, false, AIVisibilityUnlisted, AIModerationStateNormal) {
		t.Fatalf("unlisted normal prompt should be readable")
	}
	if CanReadPromptTemplate(10, 99, false, AIVisibilityPrivate, AIModerationStateNormal) {
		t.Fatalf("private prompt should not be readable by non-owner")
	}
	if CanReadPromptTemplate(10, 99, false, AIVisibilityPublic, AIModerationStateForcedPrivate) {
		t.Fatalf("forced private prompt should not be readable by non-owner")
	}
	if CanReadPromptTemplate(10, 99, false, AIVisibilityPublic, AIModerationStateBlocked) {
		t.Fatalf("blocked prompt should not be readable by non-owner")
	}
}

func TestCanListPromptTemplateInLibrary_RequiresPublicNormalUnlessOwner(t *testing.T) {
	t.Parallel()

	if !CanListPromptTemplateInLibrary(10, 10, AIVisibilityPrivate, AIModerationStateBlocked) {
		t.Fatalf("owner should always see own prompt in library")
	}
	if !CanListPromptTemplateInLibrary(10, 99, AIVisibilityPublic, AIModerationStateNormal) {
		t.Fatalf("public normal prompt should be listed")
	}
	if CanListPromptTemplateInLibrary(10, 99, AIVisibilityUnlisted, AIModerationStateNormal) {
		t.Fatalf("unlisted prompt should not be listed for non-owner")
	}
	if CanListPromptTemplateInLibrary(10, 99, AIVisibilityPublic, AIModerationStateForcedPrivate) {
		t.Fatalf("forced private prompt should not be listed for non-owner")
	}
	if CanListPromptTemplateInLibrary(10, 99, AIVisibilityPublic, AIModerationStateBlocked) {
		t.Fatalf("blocked prompt should not be listed for non-owner")
	}
}
