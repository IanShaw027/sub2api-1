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
