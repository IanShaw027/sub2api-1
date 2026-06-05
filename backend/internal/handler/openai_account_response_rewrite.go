package handler

import "github.com/Wei-Shaw/sub2api/internal/service"

func (h *OpenAIGatewayHandler) rewriteOpenAIUpstreamErrorForAccount(account *service.Account, upstreamStatusCode int, responseBody []byte) (string, bool) {
	if account == nil {
		return "", false
	}
	rule := account.MatchResponseRewriteRule(upstreamStatusCode, responseBody)
	if rule == nil {
		return "", false
	}
	return rule.ResponseMessage, true
}
