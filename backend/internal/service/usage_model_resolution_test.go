package service

import "testing"

import "github.com/stretchr/testify/require"

func TestResolveUsageModelView_PrefersExplicitBillingModelOverUpstream(t *testing.T) {
	models := resolveUsageModelView(usageModelViewInput{
		ResultModel:          "gpt-5.1",
		ExplicitBillingModel: "gpt-5.1-codex",
		UpstreamModel:        "gpt-5.1-codex",
	})

	require.Equal(t, "gpt-5.1", models.RequestedModel)
	require.Equal(t, "gpt-5.1-codex", models.BillingModel)
	require.Equal(t, "gpt-5.1-codex", models.UpstreamModel)
	require.NotNil(t, models.UsageLogUpstreamModel)
	require.Equal(t, "gpt-5.1-codex", *models.UsageLogUpstreamModel)
}

func TestResolveUsageModelView_ChannelMappedBillingOnlyOverridesWhenActuallyMapped(t *testing.T) {
	models := resolveUsageModelView(usageModelViewInput{
		ResultModel:          "glm",
		ExplicitBillingModel: "gpt-5.1",
		UpstreamModel:        "gpt-5.1",
		OriginalModel:        "glm",
		ChannelMappedModel:   "glm",
		BillingModelSource:   BillingModelSourceChannelMapped,
	})

	require.Equal(t, "glm", models.RequestedModel)
	require.Equal(t, "gpt-5.1", models.BillingModel)
	require.Equal(t, "gpt-5.1", models.UpstreamModel)
}

func TestResolveUsageModelView_ChannelMappedBillingOverridesWhenMapped(t *testing.T) {
	models := resolveUsageModelView(usageModelViewInput{
		ResultModel:        "glm",
		UpstreamModel:      "gpt-5.1-codex",
		OriginalModel:      "glm",
		ChannelMappedModel: "gpt-5.1",
		BillingModelSource: BillingModelSourceChannelMapped,
	})

	require.Equal(t, "glm", models.RequestedModel)
	require.Equal(t, "gpt-5.1", models.BillingModel)
	require.Equal(t, "gpt-5.1-codex", models.UpstreamModel)
}

func TestResolveUsageModelView_RequestedBillingSourcePinsBillingToOriginalModel(t *testing.T) {
	models := resolveUsageModelView(usageModelViewInput{
		ResultModel:          "gpt-5.1",
		ExplicitBillingModel: "gpt-5.1-codex",
		UpstreamModel:        "gpt-5.1-codex",
		OriginalModel:        "gpt-5.1",
		BillingModelSource:   BillingModelSourceRequested,
	})

	require.Equal(t, "gpt-5.1", models.RequestedModel)
	require.Equal(t, "gpt-5.1", models.BillingModel)
	require.Equal(t, "gpt-5.1-codex", models.UpstreamModel)
}
