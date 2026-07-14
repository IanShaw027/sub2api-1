package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
)

func resolveKiroTLSProfile(account *Account, tlsFPProfileService *TLSFingerprintProfileService) *tlsfingerprint.Profile {
	return resolveKiroTLSProfileWithRouter(context.Background(), account, tlsFPProfileService, nil, "", "http")
}

// resolveKiroTLSProfileWithRouter allows optional UA-based router resolution when the
// gateway has inbound request context. Without UA, default_os + bindings / single profile apply.
func resolveKiroTLSProfileWithRouter(
	ctx context.Context,
	account *Account,
	tlsFPProfileService *TLSFingerprintProfileService,
	routerSvc *TLSFingerprintRouterService,
	inboundUA, transport string,
) *tlsfingerprint.Profile {
	if tlsFPProfileService == nil || !isKiroTLSFingerprintEnabled(account) {
		return nil
	}
	runtime := resolveAccountTLSFingerprintRuntime(ctx, account, tlsFPProfileService, routerSvc, inboundUA, transport)
	return runtime.Profile
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
	if s == nil || s.tlsFPProfileSvc == nil || !isKiroTLSFingerprintEnabled(account) {
		return accountTLSFingerprintRuntime{}
	}
	inboundUA := inboundUserAgentFromGin(c)
	if inboundUA == "" {
		inboundUA = tlsFingerprintInboundUserAgentFromContext(ctx)
	}
	return resolveAccountTLSFingerprintRuntime(
		ctx,
		account,
		s.tlsFPProfileSvc,
		s.tlsFPRouterSvc,
		inboundUA,
		"http",
	)
}

// applyKiroTLSFingerprintRuntime applies only identity headers. Authorization,
// host, request IDs, and the remaining AWS headers keep their builder values.
func applyKiroTLSFingerprintRuntime(req *http.Request, runtime accountTLSFingerprintRuntime) {
	if req == nil {
		return
	}
	if ua := strings.TrimSpace(runtime.UpstreamUserAgent); ua != "" {
		req.Header.Set("User-Agent", ua)
		// Kiro's two UA headers describe the same SDK/IDE identity. Only replace
		// x-amz-user-agent when the routed full UA contains both required tokens;
		// otherwise preserve the valid AWS value emitted by the request builder.
		if xAmzUA := kiroXAmzUserAgentFromFullUserAgent(ua); xAmzUA != "" {
			req.Header.Set("x-amz-user-agent", xAmzUA)
		}
	}
	if originator := strings.TrimSpace(runtime.UpstreamOriginator); originator != "" {
		req.Header.Set("Originator", originator)
	}
}

func kiroXAmzUserAgentFromFullUserAgent(userAgent string) string {
	fields := strings.Fields(userAgent)
	if len(fields) < 2 || !strings.HasPrefix(strings.ToLower(fields[0]), "aws-sdk-") {
		return ""
	}
	for _, field := range fields[1:] {
		if strings.HasPrefix(strings.ToLower(field), "kiroide-") {
			return fields[0] + " " + field
		}
	}
	return ""
}

func isKiroTLSFingerprintEnabled(account *Account) bool {
	return account != nil && account.IsKiro() && account.IsTLSFingerprintEnabled()
}

func newKiroSidecarHTTPClient(account *Account, tlsFPProfileService *TLSFingerprintProfileService, timeout time.Duration) (*http.Client, error) {
	if account == nil {
		return nil, fmt.Errorf("account is required")
	}

	poolSize := normalizeKiroTransportConcurrency(account.Concurrency)
	profile := resolveKiroTLSProfile(account, tlsFPProfileService)
	if profile == nil {
		return httpclient.GetClient(httpclient.Options{
			ProxyURL:              accountProxyURL(account),
			Timeout:               timeout,
			ResponseHeaderTimeout: timeout,
			MaxIdleConns:          poolSize * 2,
			MaxIdleConnsPerHost:   poolSize,
			MaxConnsPerHost:       poolSize,
		})
	}

	transport, err := buildKiroTLSFingerprintTransport(accountProxyURL(account), profile, timeout, poolSize)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

func buildKiroTLSFingerprintTransport(proxyURL string, profile *tlsfingerprint.Profile, timeout time.Duration, poolSize int) (*http.Transport, error) {
	transport := &http.Transport{
		MaxIdleConns:          poolSize * 2,
		MaxIdleConnsPerHost:   poolSize,
		MaxConnsPerHost:       poolSize,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: timeout,
		ForceAttemptHTTP2:     false,
	}

	_, parsedProxy, err := proxyurl.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	if parsedProxy == nil {
		dialer := tlsfingerprint.NewDialer(profile, nil)
		transport.DialTLSContext = dialer.DialTLSContext
		return transport, nil
	}

	switch strings.ToLower(parsedProxy.Scheme) {
	case "socks5", "socks5h":
		dialer := tlsfingerprint.NewSOCKS5ProxyDialer(profile, parsedProxy)
		transport.DialTLSContext = dialer.DialTLSContext
	case "http", "https":
		dialer := tlsfingerprint.NewHTTPProxyDialer(profile, parsedProxy)
		transport.DialTLSContext = dialer.DialTLSContext
	default:
		if err := proxyutil.ConfigureTransportProxy(transport, parsedProxy); err != nil {
			return nil, err
		}
	}

	return transport, nil
}
