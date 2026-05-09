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

func TestIsValidAIVisibility_RejectsInvalidValues(t *testing.T) {
	t.Parallel()

	validCases := []string{
		AIVisibilityPrivate,
		AIVisibilityUnlisted,
		AIVisibilityPublic,
		" PRIVATE ",
		"UnLiStEd",
	}
	for _, raw := range validCases {
		if !IsValidAIVisibility(raw) {
			t.Fatalf("expected visibility %q to be valid", raw)
		}
	}

	invalidCases := []string{"", "friends_only", "public/private", "123"}
	for _, raw := range invalidCases {
		if IsValidAIVisibility(raw) {
			t.Fatalf("expected visibility %q to be invalid", raw)
		}
	}
}

func TestIsValidAIModerationState_RejectsInvalidValues(t *testing.T) {
	t.Parallel()

	validCases := []string{
		AIModerationStateNormal,
		AIModerationStateForcedPrivate,
		AIModerationStateBlocked,
		" NORMAL ",
		"FoRcEd_PrIvAtE",
	}
	for _, raw := range validCases {
		if !IsValidAIModerationState(raw) {
			t.Fatalf("expected moderation state %q to be valid", raw)
		}
	}

	invalidCases := []string{"", "shadow_banned", "forced-private", "1"}
	for _, raw := range invalidCases {
		if IsValidAIModerationState(raw) {
			t.Fatalf("expected moderation state %q to be invalid", raw)
		}
	}
}

func TestNormalizeAISkillSourceVisibility_RejectsInvalidExplicitValue(t *testing.T) {
	t.Parallel()

	if got := NormalizeAISkillSourceVisibility("", AIVisibilityPublic, 0); got != AISkillSourceVisibilityPublic {
		t.Fatalf("expected empty source visibility to default to public for free public skill, got %q", got)
	}
	if got := NormalizeAISkillSourceVisibility("", AIVisibilityPublic, 9.9); got != AISkillSourceVisibilityHidden {
		t.Fatalf("expected empty source visibility to default to hidden for paid public skill, got %q", got)
	}
	if got := NormalizeAISkillSourceVisibility("friends_only", AIVisibilityPublic, 0); got != "" {
		t.Fatalf("expected invalid explicit source visibility to stay invalid, got %q", got)
	}
}

func TestEffectiveAISkillSourceVisibility_InvalidValueFailsClosed(t *testing.T) {
	t.Parallel()

	if got := EffectiveAISkillSourceVisibility(AIVisibilityPublic, "friends_only", 0); got != AISkillSourceVisibilityHidden {
		t.Fatalf("expected invalid stored source visibility to fail closed, got %q", got)
	}
	if CanReadAISkillSource(10, 99, false, AIVisibilityPublic, "friends_only", 0, ptrInt64(1)) {
		t.Fatalf("invalid stored source visibility should not expose skill source to non-owner")
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}
