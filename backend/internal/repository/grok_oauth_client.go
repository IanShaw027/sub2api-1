package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	sharedhttp "github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/imroc/req/v3"
)

type grokOAuthClient struct {
	tokenURL string
}

const (
	xaiOAuthIssuer        = "https://auth.x.ai"
	xaiOAuthDeviceCodeURL = xaiOAuthIssuer + "/oauth2/device/code"
	accountsBaseURL       = "https://accounts.x.ai"
	loginRPCEndpoint      = accountsBaseURL + "/api/rpc"
	turnstileWebsiteURL   = accountsBaseURL
	turnstileWebsiteKey   = "0x4AAAAAAAhr9JGVDZbrZOo0"
	yesCaptchaCreateTask  = "https://api.yescaptcha.com/createTask"
	yesCaptchaGetResult   = "https://api.yescaptcha.com/getTaskResult"
)

func NewGrokOAuthClient() service.GrokOAuthClient {
	return &grokOAuthClient{tokenURL: xai.EffectiveTokenURL()}
}

func (c *grokOAuthClient) ExchangeCode(ctx context.Context, code, codeVerifier, redirectURI, proxyURL, clientID string) (*xai.TokenResponse, error) {
	client, err := createGrokReqClient(proxyURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}

	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = xai.EffectiveClientID()
	}

	formData := url.Values{}
	formData.Set("grant_type", "authorization_code")
	formData.Set("client_id", clientID)
	formData.Set("code", code)
	formData.Set("redirect_uri", xai.EffectiveRedirectURI(redirectURI))
	formData.Set("code_verifier", codeVerifier)

	var tokenResp xai.TokenResponse
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("User-Agent", "sub2api-grok-oauth/1.0").
		SetFormDataFromValues(formData).
		SetSuccessResult(&tokenResp).
		Post(c.tokenURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_REQUEST_FAILED", "request failed: %v", err)
	}
	if !resp.IsSuccessState() {
		return nil, grokOAuthStatusError("GROK_OAUTH_TOKEN_EXCHANGE_FAILED", "token exchange failed", resp)
	}
	return &tokenResp, nil
}

func (c *grokOAuthClient) RefreshToken(ctx context.Context, refreshToken, proxyURL, clientID string) (*xai.TokenResponse, error) {
	client, err := createGrokReqClient(proxyURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}

	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = xai.EffectiveClientID()
	}

	formData := url.Values{}
	formData.Set("grant_type", "refresh_token")
	formData.Set("client_id", clientID)
	formData.Set("refresh_token", refreshToken)

	var tokenResp xai.TokenResponse
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("User-Agent", "sub2api-grok-oauth/1.0").
		SetFormDataFromValues(formData).
		SetSuccessResult(&tokenResp).
		Post(c.tokenURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_REQUEST_FAILED", "request failed: %v", err)
	}
	if !resp.IsSuccessState() {
		return nil, grokOAuthStatusError("GROK_OAUTH_TOKEN_REFRESH_FAILED", "token refresh failed", resp)
	}
	return &tokenResp, nil
}

func (c *grokOAuthClient) RequestDeviceCode(ctx context.Context, proxyURL, clientID string) (*service.GrokDeviceCodeResponse, error) {
	client, err := createGrokReqClient(proxyURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = xai.EffectiveClientID()
	}
	formData := url.Values{}
	formData.Set("client_id", clientID)
	formData.Set("scope", xai.EffectiveScope())

	var out service.GrokDeviceCodeResponse
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("User-Agent", "sub2api-grok-oauth/1.0").
		SetFormDataFromValues(formData).
		SetSuccessResult(&out).
		Post(xaiOAuthDeviceCodeURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_REQUEST_FAILED", "request failed: %v", err)
	}
	if !resp.IsSuccessState() {
		return nil, grokOAuthStatusError("GROK_OAUTH_DEVICE_CODE_FAILED", "device code request failed", resp)
	}
	if out.Interval <= 0 {
		out.Interval = 5
	}
	return &out, nil
}

func (c *grokOAuthClient) PollDeviceToken(ctx context.Context, deviceCode string, interval, expiresIn int, proxyURL, clientID string) (*xai.TokenResponse, error) {
	client, err := createGrokReqClient(proxyURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		clientID = xai.EffectiveClientID()
	}
	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	formData.Set("client_id", clientID)
	formData.Set("device_code", strings.TrimSpace(deviceCode))

	if interval <= 0 {
		interval = 5
	}
	if expiresIn <= 0 {
		expiresIn = 900
	}
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	pollEvery := time.Duration(interval) * time.Second
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollEvery):
		}

		var tokenResp xai.TokenResponse
		var errBody map[string]any
		resp, err := client.R().
			SetContext(ctx).
			SetHeader("User-Agent", "sub2api-grok-oauth/1.0").
			SetFormDataFromValues(formData).
			SetSuccessResult(&tokenResp).
			SetErrorResult(&errBody).
			Post(c.tokenURL)
		if err != nil {
			continue
		}
		if resp.IsSuccessState() {
			return &tokenResp, nil
		}
		errCode := strings.TrimSpace(fmt.Sprint(errBody["error"]))
		switch errCode {
		case "authorization_pending":
			continue
		case "slow_down":
			pollEvery += time.Second
			if pollEvery > 30*time.Second {
				pollEvery = 30 * time.Second
			}
			continue
		case "expired_token":
			return nil, infraerrors.New(http.StatusBadGateway, "GROK_OAUTH_DEVICE_CODE_EXPIRED", "device code expired before authorization completed")
		case "access_denied":
			return nil, infraerrors.New(http.StatusForbidden, "GROK_OAUTH_DEVICE_CODE_DENIED", "device authorization was denied")
		default:
			return nil, grokOAuthStatusError("GROK_OAUTH_DEVICE_TOKEN_FAILED", "device token polling failed", resp)
		}
	}
	return nil, infraerrors.New(http.StatusGatewayTimeout, "GROK_OAUTH_DEVICE_CODE_TIMEOUT", "device code authorization timed out")
}

func (c *grokOAuthClient) AutoAuthorizeDeviceCode(ctx context.Context, ssoToken, userCode, proxyURL string) error {
	httpClient, err := createGrokHTTPClient(proxyURL, true)
	if err != nil {
		return infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}
	return autoAuthorizeGrokDeviceCode(ctx, httpClient, ssoToken, userCode,
		xaiOAuthIssuer+"/oauth2/device/verify",
		xaiOAuthIssuer+"/oauth2/device/approve",
	)
}

func autoAuthorizeGrokDeviceCode(ctx context.Context, httpClient *http.Client, ssoToken, userCode, verifyURL, approveURL string) error {
	cookie := buildGrokSSOCookie(ssoToken)

	verifyForm := url.Values{"user_code": {strings.TrimSpace(userCode)}}
	verifyReq, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, strings.NewReader(verifyForm.Encode()))
	if err != nil {
		return infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_DEVICE_VERIFY_FAILED", "build verify request: %v", err)
	}
	verifyReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	verifyReq.Header.Set("Origin", "https://accounts.x.ai")
	verifyReq.Header.Set("Referer", "https://accounts.x.ai/")
	verifyReq.Header.Set("Cookie", cookie)
	verifyResp, err := httpClient.Do(verifyReq)
	if err != nil {
		return infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_DEVICE_VERIFY_FAILED", "verify request failed: %v", err)
	}
	if err := validateGrokDeviceActionResponse("verify", verifyResp); err != nil {
		return err
	}

	approveForm := url.Values{
		"user_code":      {strings.TrimSpace(userCode)},
		"action":         {"allow"},
		"principal_type": {"User"},
		"principal_id":   {""},
	}
	approveReq, err := http.NewRequestWithContext(ctx, http.MethodPost, approveURL, strings.NewReader(approveForm.Encode()))
	if err != nil {
		return infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_DEVICE_APPROVE_FAILED", "build approve request: %v", err)
	}
	approveReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	approveReq.Header.Set("Origin", "https://accounts.x.ai")
	approveReq.Header.Set("Referer", "https://accounts.x.ai/")
	approveReq.Header.Set("Cookie", cookie)
	approveResp, err := httpClient.Do(approveReq)
	if err != nil {
		return infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_DEVICE_APPROVE_FAILED", "approve request failed: %v", err)
	}
	return validateGrokDeviceActionResponse("approve", approveResp)
}

func (c *grokOAuthClient) LoginWithPassword(ctx context.Context, email, password, proxyURL string) (*service.GrokPasswordLoginResult, error) {
	turnstileToken, err := solveTurnstile(ctx)
	if err != nil {
		return nil, err
	}
	httpClient, err := createGrokHTTPClient(proxyURL, true)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}
	cookieSetterURL, err := createGrokPasswordSession(ctx, httpClient, strings.TrimSpace(email), password, turnstileToken)
	if err != nil {
		return nil, err
	}
	ssoToken, err := extractGrokSSOToken(ctx, httpClient, cookieSetterURL)
	if err != nil {
		return nil, err
	}
	return &service.GrokPasswordLoginResult{
		Email:    strings.TrimSpace(email),
		SSOToken: ssoToken,
	}, nil
}

func (c *grokOAuthClient) ConvertSSOToBuild(ctx context.Context, ssoToken, proxyURL string) (*xai.TokenResponse, error) {
	client, err := createGrokSSOHTTPClient(proxyURL)
	if err != nil {
		return nil, infraerrors.Newf(http.StatusBadGateway, "GROK_SSO_CLIENT_INIT_FAILED", "create HTTP client: %v", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, xai.SSOConversionTimeout)
	defer cancel()
	tokenResp, err := xai.ConvertSSOToBuild(requestCtx, ssoToken, &xai.SSODeviceOptions{HTTPClient: client})
	if err != nil {
		return nil, grokSSOConversionError(err)
	}
	return tokenResp, nil
}

func createGrokReqClient(proxyURL string) (*req.Client, error) {
	return getSharedReqClient(ReqClientOptions{
		ProxyURL: proxyURL,
		Timeout:  60 * time.Second,
	})
}

func createGrokSSOHTTPClient(proxyURL string) (*http.Client, error) {
	client, err := sharedhttp.GetClient(sharedhttp.Options{
		ProxyURL:              proxyURL,
		Timeout:               xai.SSOConversionTimeout,
		ResponseHeaderTimeout: 30 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clone, nil
}

func grokSSOConversionError(err error) error {
	if errors.Is(err, xai.ErrSSOUnauthorized) {
		return infraerrors.New(http.StatusUnauthorized, "GROK_SSO_UNAUTHORIZED", "Grok Web SSO cookie is invalid or expired")
	}
	if errors.Is(err, xai.ErrSSOAuthorizationDenied) {
		return infraerrors.New(http.StatusForbidden, "GROK_SSO_AUTHORIZATION_DENIED", "xAI device authorization was denied or expired")
	}
	var statusErr xai.SSOHTTPError
	if errors.As(err, &statusErr) {
		statusCode := http.StatusBadGateway
		if statusErr.Status == http.StatusForbidden {
			statusCode = http.StatusForbidden
		}
		return infraerrors.Newf(statusCode, "GROK_SSO_UPSTREAM_FAILED", "xAI SSO conversion failed: %v", err)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return infraerrors.Newf(http.StatusGatewayTimeout, "GROK_SSO_TIMEOUT", "xAI SSO conversion timed out: %v", err)
	}
	return infraerrors.Newf(http.StatusBadGateway, "GROK_SSO_CONVERSION_FAILED", "xAI SSO conversion failed: %v", err)
}

func grokOAuthStatusError(code, message string, resp *req.Response) error {
	statusCode := http.StatusBadGateway
	errorCode := code
	upstreamStatus := 0
	if resp != nil && resp.StatusCode == http.StatusForbidden {
		statusCode = http.StatusForbidden
		errorCode = "GROK_OAUTH_ENTITLEMENT_DENIED"
	}
	body := ""
	if resp != nil {
		upstreamStatus = resp.StatusCode
		body = logredact.RedactText(resp.String())
	}
	return infraerrors.Newf(statusCode, errorCode, "%s: status %d, body: %s", message, upstreamStatus, body)
}

func createGrokHTTPClient(proxyURL string, noRedirect bool) (*http.Client, error) {
	transport := &http.Transport{}
	if strings.TrimSpace(proxyURL) != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(parsed)
	}
	client := &http.Client{Timeout: 120 * time.Second, Transport: transport}
	if noRedirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client, nil
}

func solveTurnstile(ctx context.Context) (string, error) {
	clientKey := strings.TrimSpace(os.Getenv("YESCAPTCHA_CLIENT_KEY"))
	if clientKey == "" {
		clientKey = strings.TrimSpace(os.Getenv("YESCAPTCHA_API_KEY"))
	}
	if clientKey == "" {
		return "", infraerrors.New(http.StatusBadRequest, "GROK_OAUTH_CAPTCHA_KEY_REQUIRED", "yescaptcha client key is required for Grok password authorization")
	}
	createBody, _ := json.Marshal(map[string]any{
		"clientKey": clientKey,
		"task": map[string]any{
			"type":       "TurnstileTaskProxyless",
			"websiteURL": turnstileWebsiteURL,
			"websiteKey": turnstileWebsiteKey,
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, yesCaptchaCreateTask, bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_CAPTCHA_FAILED", "create captcha task failed: %v", err)
	}
	defer resp.Body.Close()
	var createResp struct {
		ErrorID          int    `json:"errorId"`
		TaskID           string `json:"taskId"`
		ErrorDescription string `json:"errorDescription"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_CAPTCHA_FAILED", "decode captcha create response failed: %v", err)
	}
	if createResp.ErrorID != 0 || strings.TrimSpace(createResp.TaskID) == "" {
		return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_CAPTCHA_FAILED", "captcha create failed: %s", createResp.ErrorDescription)
	}
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(5 * time.Second):
		}
		body, _ := json.Marshal(map[string]any{"clientKey": clientKey, "taskId": createResp.TaskID})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, yesCaptchaGetResult, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		var pollResp struct {
			ErrorID          int    `json:"errorId"`
			Status           string `json:"status"`
			ErrorDescription string `json:"errorDescription"`
			Solution         struct {
				Token string `json:"token"`
			} `json:"solution"`
		}
		err = json.NewDecoder(resp.Body).Decode(&pollResp)
		resp.Body.Close()
		if err != nil {
			continue
		}
		if pollResp.ErrorID != 0 {
			return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_CAPTCHA_FAILED", "captcha poll failed: %s", pollResp.ErrorDescription)
		}
		if pollResp.Status == "ready" && strings.TrimSpace(pollResp.Solution.Token) != "" {
			return pollResp.Solution.Token, nil
		}
	}
	return "", infraerrors.New(http.StatusGatewayTimeout, "GROK_OAUTH_CAPTCHA_TIMEOUT", "captcha solve timed out")
}

func createGrokPasswordSession(ctx context.Context, client *http.Client, email, password, turnstileToken string) (string, error) {
	payload, _ := json.Marshal(map[string]any{
		"rpc": "createSession",
		"req": map[string]any{
			"createSessionRequest": map[string]any{
				"credentials": map[string]any{
					"case": "emailAndPassword",
					"value": map[string]any{
						"email":             email,
						"clearTextPassword": password,
					},
				},
			},
			"turnstileToken": turnstileToken,
		},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, loginRPCEndpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", accountsBaseURL)
	req.Header.Set("Referer", accountsBaseURL+"/sign-in?redirect=grok-com&email=true")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_PASSWORD_LOGIN_FAILED", "password login request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_PASSWORD_LOGIN_FAILED", "password login returned status %d: %s", resp.StatusCode, logredact.RedactText(string(body)))
	}
	var loginResp struct {
		CookieSetterURL string `json:"cookieSetterUrl"`
		Error           string `json:"error"`
	}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_PASSWORD_LOGIN_FAILED", "decode password login response failed: %v", err)
	}
	if strings.TrimSpace(loginResp.Error) != "" {
		return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_PASSWORD_LOGIN_FAILED", "password login error: %s", logredact.RedactText(loginResp.Error))
	}
	if strings.TrimSpace(loginResp.CookieSetterURL) == "" {
		return "", infraerrors.New(http.StatusBadGateway, "GROK_OAUTH_PASSWORD_LOGIN_FAILED", "password login did not return cookieSetterUrl")
	}
	return loginResp.CookieSetterURL, nil
}

func extractGrokSSOToken(ctx context.Context, client *http.Client, cookieSetterURL string) (string, error) {
	safeURL, err := validateGrokCookieSetterURL(cookieSetterURL)
	if err != nil {
		return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_PASSWORD_LOGIN_FAILED", "invalid cookie setter url: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, safeURL.String(), nil)
	if err != nil {
		return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_PASSWORD_LOGIN_FAILED", "build cookie setter request: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Referer", accountsBaseURL+"/")
	resp, err := client.Do(req)
	if err != nil {
		return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_PASSWORD_LOGIN_FAILED", "follow cookie setter url failed: %v", err)
	}
	defer resp.Body.Close()
	for _, cookie := range resp.Header.Values("Set-Cookie") {
		if token, ok := strings.CutPrefix(cookie, "sso="); ok {
			if idx := strings.Index(token, ";"); idx > 0 {
				token = token[:idx]
			}
			return strings.TrimSpace(token), nil
		}
	}
	return "", infraerrors.Newf(http.StatusBadGateway, "GROK_OAUTH_PASSWORD_LOGIN_FAILED", "no sso cookie found in response (status=%d)", resp.StatusCode)
}

func buildGrokSSOCookie(token string) string {
	return fmt.Sprintf("sso=%s; sso-rw=%s", token, token)
}

func validateGrokCookieSetterURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "accounts.x.ai") {
		return nil, fmt.Errorf("url must use https://accounts.x.ai")
	}
	if parsed.User != nil || parsed.Port() != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return nil, fmt.Errorf("url contains disallowed authority or fragment components")
	}
	return parsed, nil
}

func validateGrokDeviceActionResponse(action string, resp *http.Response) error {
	errorCode := "GROK_OAUTH_DEVICE_" + strings.ToUpper(action) + "_FAILED"
	if resp == nil {
		return infraerrors.Newf(http.StatusBadGateway, errorCode, "%s returned no response", action)
	}
	if resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	switch resp.StatusCode {
	case http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		location := strings.TrimSpace(resp.Header.Get("Location"))
		redirectURL, err := resolveTrustedXAIRedirect(resp.Request, location)
		if err != nil || isGrokLoginRedirect(redirectURL) {
			return infraerrors.Newf(http.StatusBadGateway, errorCode, "%s returned unsafe redirect", action)
		}
		return nil
	}
	return infraerrors.Newf(http.StatusBadGateway, errorCode, "%s returned status %d", action, resp.StatusCode)
}

func resolveTrustedXAIRedirect(request *http.Request, location string) (*url.URL, error) {
	if request == nil || request.URL == nil || location == "" {
		return nil, fmt.Errorf("missing redirect target")
	}
	parsed, err := url.Parse(location)
	if err != nil {
		return nil, err
	}
	parsed = request.URL.ResolveReference(parsed)
	host := strings.ToLower(parsed.Hostname())
	if parsed.Scheme != "https" || (host != "auth.x.ai" && host != "accounts.x.ai") {
		return nil, fmt.Errorf("redirect target is not a trusted x.ai endpoint")
	}
	if parsed.User != nil || parsed.Port() != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return nil, fmt.Errorf("redirect target contains disallowed components")
	}
	return parsed, nil
}

func isGrokLoginRedirect(target *url.URL) bool {
	if target == nil {
		return true
	}
	value := strings.ToLower(target.EscapedPath() + "?" + target.RawQuery)
	for _, marker := range []string{"login", "sign-in", "signin", "sign-up", "signup"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}
