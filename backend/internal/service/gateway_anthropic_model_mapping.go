package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

func resolveGatewayAnthropicForwardModel(ctx context.Context, settingService *SettingService, account *Account, requestedModel string) (string, string) {
	if account == nil {
		return requestedModel, ""
	}
	if account.Platform != PlatformAnthropic {
		routing := ResolveEffectiveModelRouting(ctx, settingService, account, requestedModel, false)
		if routing.Model != requestedModel {
			return routing.Model, routing.Source
		}
		return requestedModel, ""
	}
	if account.Type == AccountTypeAPIKey {
		routing := ResolveEffectiveModelRouting(ctx, settingService, account, requestedModel, false)
		if routing.Model != requestedModel {
			return routing.Model, routing.Source
		}
		return requestedModel, ""
	}
	routingAccount := account
	if account.Type == AccountTypeOAuth || account.Type == AccountTypeSetupToken {
		routingAccount = accountWithoutCredentialModelRouting(account)
	}
	routing := ResolveEffectiveModelRouting(ctx, settingService, routingAccount, requestedModel, false)
	if routing.Source == "system" && routing.Model != requestedModel {
		return routing.Model, "system"
	}
	if account.Type == AccountTypeServiceAccount {
		if candidate, matched := account.ResolveMappedModel(requestedModel); matched {
			return candidate, "account"
		}
		normalized := normalizeVertexAnthropicModelID(claude.NormalizeModelID(requestedModel))
		if normalized != requestedModel {
			return normalized, "vertex"
		}
		return requestedModel, ""
	}
	normalized := claude.NormalizeModelID(requestedModel)
	if normalized != requestedModel {
		return normalized, "prefix"
	}
	return requestedModel, ""
}

func accountWithoutCredentialModelRouting(account *Account) *Account {
	if account == nil || len(account.Credentials) == 0 {
		return account
	}
	clone := *account
	credentials := make(map[string]any, len(account.Credentials))
	for key, value := range account.Credentials {
		switch key {
		case "model_mapping", "model_whitelist", "compact_model_mapping":
			continue
		default:
			credentials[key] = value
		}
	}
	clone.Credentials = credentials
	return &clone
}
