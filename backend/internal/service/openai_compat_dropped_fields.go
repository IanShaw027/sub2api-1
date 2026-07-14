package service

import "github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"

func recordOpenAICompatDroppedFields(req *apicompat.ResponsesRequest) {
	if req == nil {
		return
	}
	for _, field := range req.DroppedCompatibilityFields {
		recordOpenAICompatStrippedField(field)
	}
}
