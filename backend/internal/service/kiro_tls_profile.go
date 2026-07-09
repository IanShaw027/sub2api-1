package service

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

func resolveKiroTLSProfile(account *Account, tlsFPProfileService *TLSFingerprintProfileService) *tlsfingerprint.Profile {
	if tlsFPProfileService == nil || !isKiroTLSFingerprintEnabled(account) {
		return nil
	}
	return tlsFPProfileService.ResolveTLSProfileForTransport(account, "http")
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
