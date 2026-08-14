package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
)

type accountTLSFingerprintRuntime struct {
	Profile            *tlsfingerprint.Profile
	UpstreamUserAgent  string
	UpstreamOriginator string
	Matched            bool
}

func resolveKiroTLSProfile(account *Account, tlsFPProfileService *TLSFingerprintProfileService) *tlsfingerprint.Profile {
	if tlsFPProfileService == nil {
		return nil
	}
	return tlsFPProfileService.ResolveTLSProfile(account)
}

func (s *KiroGatewayService) resolveTLSFingerprintRuntime(
	ctx context.Context,
	c *gin.Context,
	account *Account,
) accountTLSFingerprintRuntime {
	if ctx != nil {
		if runtime, ok := ctx.Value(kiroTLSFingerprintRuntimeContextKey{}).(accountTLSFingerprintRuntime); ok {
			return runtime
		}
	}
	if s == nil {
		return accountTLSFingerprintRuntime{}
	}
	return resolveAccountTLSFingerprintRuntime(ctx, account, s.tlsFPProfileSvc, s.tlsFPRouterSvc, inboundUserAgentFromGin(c), "http", "kiro")
}

func (s *KiroGatewayService) doKiroUpstream(ctx context.Context, c *gin.Context, account *Account, req *http.Request) (*http.Response, error) {
	if s == nil || s.httpUpstream == nil || account == nil {
		return nil, fmt.Errorf("kiro upstream is not configured")
	}
	runtime := s.resolveTLSFingerprintRuntime(ctx, c, account)
	applyKiroTLSFingerprintRuntime(req, runtime)
	profile := runtime.Profile
	if profile == nil {
		profile = resolveKiroTLSProfile(account, s.tlsFPProfileSvc)
	}
	return s.httpUpstream.DoWithTLS(req, accountProxyURL(account), account.ID, account.Concurrency, profile)
}

func applyKiroTLSFingerprintRuntime(req *http.Request, runtime accountTLSFingerprintRuntime) {
	if req == nil {
		return
	}
	if ua := strings.TrimSpace(runtime.UpstreamUserAgent); ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	if originator := strings.TrimSpace(runtime.UpstreamOriginator); originator != "" {
		deleteHeaderAllForms(req.Header, "X-Originator")
		setHeaderRaw(req.Header, "originator", originator)
	}
}

func newKiroSidecarHTTPClient(account *Account, tlsFPProfileService *TLSFingerprintProfileService, timeout time.Duration) (*http.Client, error) {
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}
	poolSize := normalizeKiroTransportConcurrency(account.Concurrency)
	return httpclient.GetClient(httpclient.Options{
		ProxyURL:              accountProxyURL(account),
		Timeout:               timeout,
		ResponseHeaderTimeout: timeout,
		MaxIdleConns:          poolSize * 2,
		MaxIdleConnsPerHost:   poolSize,
		MaxConnsPerHost:       poolSize,
	})
}
