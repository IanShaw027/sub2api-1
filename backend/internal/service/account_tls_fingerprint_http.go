package service

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

func doAccountHTTPUpstream(
	ctx context.Context,
	upstream HTTPUpstream,
	req *http.Request,
	proxyURL string,
	account *Account,
	profileSvc *TLSFingerprintProfileService,
	routerSvc *TLSFingerprintRouterService,
	inboundUA, transport, protocol string,
) (*http.Response, error) {
	if protocol == "" && req != nil && req.URL != nil {
		protocol = inboundProtocolFromPath(req.URL.Path)
	}
	runtime := resolveAccountTLSFingerprintRuntime(ctx, account, profileSvc, routerSvc, inboundUA, transport, protocol)
	applyTLSFingerprintRuntimeHeaders(req, runtime)
	return upstream.DoWithTLS(req, proxyURL, account.ID, account.EffectiveConcurrency(), runtime.Profile)
}

func doAccountHTTPUpstreamFromGin(
	ctx context.Context,
	c *gin.Context,
	upstream HTTPUpstream,
	req *http.Request,
	proxyURL string,
	account *Account,
	profileSvc *TLSFingerprintProfileService,
	routerSvc *TLSFingerprintRouterService,
	transport, protocol string,
) (*http.Response, error) {
	if protocol == "" && c != nil && c.Request != nil && c.Request.URL != nil {
		protocol = inboundProtocolFromPath(c.Request.URL.Path)
	}
	return doAccountHTTPUpstream(ctx, upstream, req, proxyURL, account, profileSvc, routerSvc, inboundUserAgentFromGin(c), transport, protocol)
}
