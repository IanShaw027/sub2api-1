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
}

func resolveKiroTLSProfile(account *Account, tlsFPProfileService *TLSFingerprintProfileService) *tlsfingerprint.Profile {
	return nil
}

func resolveKiroTLSProfileWithRouter(
	ctx context.Context,
	account *Account,
	tlsFPProfileService *TLSFingerprintProfileService,
	inboundUA, transport string,
) *tlsfingerprint.Profile {
	return nil
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
	return accountTLSFingerprintRuntime{}
}

func applyKiroTLSFingerprintRuntime(req *http.Request, runtime accountTLSFingerprintRuntime) {
	if req == nil {
		return
	}
	if ua := strings.TrimSpace(runtime.UpstreamUserAgent); ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	if originator := strings.TrimSpace(runtime.UpstreamOriginator); originator != "" {
		req.Header.Set("X-Originator", originator)
	}
}

func isKiroTLSFingerprintEnabled(account *Account) bool {
	return false
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
