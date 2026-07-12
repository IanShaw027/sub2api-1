package service

import "strings"

const (
	EndpointChatCompletions = "/v1/chat/completions"
	EndpointResponses       = "/v1/responses"
)

func NormalizeInboundEndpoint(path string) string {
	path = strings.TrimSpace(path)
	switch {
	case strings.Contains(path, EndpointChatCompletions):
		return EndpointChatCompletions
	case strings.Contains(path, EndpointResponses):
		return EndpointResponses
	default:
		return path
	}
}
