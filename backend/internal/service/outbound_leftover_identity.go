package service

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
)

func outboundProfileUserAgent(p *AccountDeviceProfile) string {
	if p == nil {
		return ""
	}
	if p.ProfilePayload != nil {
		if ua, ok := p.ProfilePayload["user_agent"].(string); ok && strings.TrimSpace(ua) != "" {
			return ua
		}
	}
	return leftoverFamilyDefaultUserAgent(p)
}

func leftoverFamilyDefaultUserAgent(p *AccountDeviceProfile) string {
	if p == nil {
		return ""
	}
	switch p.ClientFamily {
	case ClientFamilyGeminiCLI:
		return geminicli.GeminiCLIUserAgent
	case ClientFamilyAntigravity:
		return antigravity.BuildUserAgent(p.ClientVersion)
	default:
		return ""
	}
}

func applyOutboundProfileUserAgent(ctx context.Context, account *Account, req *http.Request) error {
	p, err := LoadOutboundDeviceProfile(ctx, account)
	if err != nil {
		return err
	}
	if req != nil {
		if ua := outboundProfileUserAgent(p); ua != "" {
			req.Header.Set("User-Agent", ua)
		}
	}
	return nil
}

func leftoverOutboundMachineID(ctx context.Context, account *Account) (string, error) {
	p, err := LoadOutboundDeviceProfile(ctx, account)
	if err != nil {
		return "", err
	}
	return p.MachineID, nil
}
