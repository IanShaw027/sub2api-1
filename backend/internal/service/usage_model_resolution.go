package service

import "strings"

type usageModelViewInput struct {
	ResultModel          string
	ExplicitBillingModel string
	UpstreamModel        string
	OriginalModel        string
	ChannelMappedModel   string
	BillingModelSource   string
}

type usageModelView struct {
	RequestedModel        string
	BillingModel          string
	UpstreamModel         string
	UsageLogUpstreamModel *string
}

func resolveUsageModelView(input usageModelViewInput) usageModelView {
	requestedModel := strings.TrimSpace(input.ResultModel)
	if originalModel := strings.TrimSpace(input.OriginalModel); originalModel != "" {
		requestedModel = originalModel
	}

	billingModel := forwardResultBillingModel(input.ResultModel, input.UpstreamModel)
	if explicitBillingModel := strings.TrimSpace(input.ExplicitBillingModel); explicitBillingModel != "" {
		billingModel = explicitBillingModel
	}

	channelMappedModel := strings.TrimSpace(input.ChannelMappedModel)
	switch input.BillingModelSource {
	case BillingModelSourceChannelMapped:
		if channelMappedModel != "" && channelMappedModel != strings.TrimSpace(input.OriginalModel) {
			billingModel = channelMappedModel
		}
	case BillingModelSourceRequested:
		if originalModel := strings.TrimSpace(input.OriginalModel); originalModel != "" {
			billingModel = originalModel
		}
	}

	upstreamModel := strings.TrimSpace(input.UpstreamModel)

	return usageModelView{
		RequestedModel:        requestedModel,
		BillingModel:          billingModel,
		UpstreamModel:         upstreamModel,
		UsageLogUpstreamModel: optionalNonEqualStringPtr(upstreamModel, strings.TrimSpace(input.ResultModel)),
	}
}
