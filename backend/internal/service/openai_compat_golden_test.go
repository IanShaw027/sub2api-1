package service

import "testing"

func TestForwardAsAnthropic_GoldenFixtures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		fixtureDir     string
		promptCacheKey string
		requestModel   string
	}{
		{
			name:           "prompt cache key ordering",
			fixtureDir:     "prompt_cache_key_ordering",
			promptCacheKey: "session-ordered",
			requestModel:   "gpt-5.1",
		},
		{
			name:         "previous response id http strip",
			fixtureDir:   "previous_response_id_http_strip",
			requestModel: "gpt-5.1",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runForwardAsAnthropicGoldenFixture(t, tc.fixtureDir, tc.promptCacheKey, tc.requestModel)
		})
	}
}

func TestForwardAsChatCompletions_GoldenFixtures(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		fixtureDir     string
		promptCacheKey string
		requestModel   string
		accountType    string
	}{
		{
			name:           "oauth prompt cache key ordering",
			fixtureDir:     "chat_prompt_cache_key_ordering_oauth",
			promptCacheKey: "session-ordered",
			requestModel:   "gpt-5.1",
			accountType:    AccountTypeOAuth,
		},
		{
			name:           "apikey prompt cache key injection",
			fixtureDir:     "chat_prompt_cache_key_injection_apikey",
			promptCacheKey: "session-apikey",
			requestModel:   "gpt-5.1",
			accountType:    AccountTypeAPIKey,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runForwardAsChatCompletionsGoldenFixture(t, tc.fixtureDir, tc.promptCacheKey, tc.requestModel, tc.accountType)
		})
	}
}
