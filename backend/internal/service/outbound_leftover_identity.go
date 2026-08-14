package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
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

func leftoverOutboundTLSRoutingUA(req *http.Request) string {
	if req == nil {
		return ""
	}
	return strings.TrimSpace(req.Header.Get("User-Agent"))
}

type leftoverPinnedUAUpstream struct {
	inner HTTPUpstream
	ua    string
}

func pinLeftoverOutboundUserAgent(upstream HTTPUpstream, req *http.Request) HTTPUpstream {
	if upstream == nil {
		return nil
	}
	return &leftoverPinnedUAUpstream{inner: upstream, ua: leftoverOutboundTLSRoutingUA(req)}
}

func (w *leftoverPinnedUAUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	if w == nil || w.inner == nil {
		return nil, fmt.Errorf("leftover outbound upstream is not configured")
	}
	if w.ua != "" && req != nil {
		req.Header.Set("User-Agent", w.ua)
	}
	return w.inner.Do(req, proxyURL, accountID, accountConcurrency)
}

func (w *leftoverPinnedUAUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	if w == nil || w.inner == nil {
		return nil, fmt.Errorf("leftover outbound upstream is not configured")
	}
	if w.ua != "" && req != nil {
		req.Header.Set("User-Agent", w.ua)
	}
	return w.inner.DoWithTLS(req, proxyURL, accountID, accountConcurrency, profile)
}

func doLeftoverAccountHTTP(
	ctx context.Context,
	upstream HTTPUpstream,
	req *http.Request,
	proxyURL string,
	account *Account,
	profileSvc *TLSFingerprintProfileService,
	routerSvc *TLSFingerprintRouterService,
	inboundUA, transport, protocol string,
) (*http.Response, error) {
	routingUA := leftoverOutboundTLSRoutingUA(req)
	if routingUA == "" {
		routingUA = inboundUA
	}
	return doAccountHTTPUpstream(ctx, pinLeftoverOutboundUserAgent(upstream, req), req, proxyURL, account, profileSvc, routerSvc, routingUA, transport, protocol)
}
