package repository

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/googleapi"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/imroc/req/v3"
)

type geminiCliCodeAssistClient struct {
	baseURL string
}

func NewGeminiCliCodeAssistClient() service.GeminiCliCodeAssistClient {
	return &geminiCliCodeAssistClient{baseURL: geminicli.GeminiCliBaseURL}
}

func (c *geminiCliCodeAssistClient) LoadCodeAssist(ctx context.Context, accessToken, proxyURL string, reqBody *geminicli.LoadCodeAssistRequest) (*geminicli.LoadCodeAssistResponse, error) {
	if reqBody == nil {
		reqBody = defaultLoadCodeAssistRequest()
	}

	var out geminicli.LoadCodeAssistResponse
	client, err := createGeminiCliReqClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("create HTTP client: %w", err)
	}
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+accessToken).
		SetHeader("Content-Type", "application/json").
		SetHeader("User-Agent", geminicli.GeminiCLIUserAgent).
		SetBody(reqBody).
		SetSuccessResult(&out).
		Post(c.baseURL + "/v1internal:loadCodeAssist")
	if err != nil {
		slog.Warn("codeassist_loadcodeassist_request_error", "error", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if !resp.IsSuccessState() {
		body := resp.String()
		sanitizedBody := geminicli.SanitizeBodyForLogs(body)
		slog.Warn("codeassist_loadcodeassist_failed", "status", resp.StatusCode, "body", sanitizedBody)

		// Check if this is a SERVICE_DISABLED error and extract activation URL
		if googleapi.IsServiceDisabledError(body) {
			activationURL := googleapi.ExtractActivationURL(body)
			if activationURL != "" {
				return nil, fmt.Errorf("gemini API not enabled for this project, please enable it by visiting: %s\n\nAfter enabling the API, wait a few minutes for the changes to propagate, then try again", activationURL)
			}
			return nil, fmt.Errorf("gemini API not enabled for this project, please enable it in the Google Cloud Console at: https://console.cloud.google.com/apis/library/cloudaicompanion.googleapis.com")
		}

		return nil, &geminicli.CodeAssistHTTPError{StatusCode: resp.StatusCode, Body: sanitizedBody, Endpoint: "loadCodeAssist"}
	}
	slog.Debug("codeassist_loadcodeassist_success", "status", resp.StatusCode)
	return &out, nil
}

func (c *geminiCliCodeAssistClient) OnboardUser(ctx context.Context, accessToken, proxyURL string, reqBody *geminicli.OnboardUserRequest) (*geminicli.OnboardUserResponse, error) {
	if reqBody == nil {
		reqBody = defaultOnboardUserRequest()
	}

	slog.Debug("codeassist_onboarduser_request", "tier_id", reqBody.TierID)

	var out geminicli.OnboardUserResponse
	client, err := createGeminiCliReqClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("create HTTP client: %w", err)
	}
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+accessToken).
		SetHeader("Content-Type", "application/json").
		SetHeader("User-Agent", geminicli.GeminiCLIUserAgent).
		SetBody(reqBody).
		SetSuccessResult(&out).
		Post(c.baseURL + "/v1internal:onboardUser")
	if err != nil {
		slog.Warn("codeassist_onboarduser_request_error", "error", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if !resp.IsSuccessState() {
		body := resp.String()
		sanitizedBody := geminicli.SanitizeBodyForLogs(body)
		slog.Warn("codeassist_onboarduser_failed", "status", resp.StatusCode, "body", sanitizedBody)

		// Check if this is a SERVICE_DISABLED error and extract activation URL
		if googleapi.IsServiceDisabledError(body) {
			activationURL := googleapi.ExtractActivationURL(body)
			if activationURL != "" {
				return nil, fmt.Errorf("gemini API not enabled for this project, please enable it by visiting: %s\n\nAfter enabling the API, wait a few minutes for the changes to propagate, then try again", activationURL)
			}
			return nil, fmt.Errorf("gemini API not enabled for this project, please enable it in the Google Cloud Console at: https://console.cloud.google.com/apis/library/cloudaicompanion.googleapis.com")
		}

		return nil, &geminicli.CodeAssistHTTPError{StatusCode: resp.StatusCode, Body: sanitizedBody, Endpoint: "onboardUser"}
	}
	slog.Debug("codeassist_onboarduser_success", "status", resp.StatusCode)
	return &out, nil
}

func (c *geminiCliCodeAssistClient) GetOperation(ctx context.Context, accessToken, proxyURL, name string) (*geminicli.OnboardUserResponse, error) {
	var out geminicli.OnboardUserResponse
	operationName := strings.TrimLeft(strings.TrimSpace(name), "/")
	if operationName == "" {
		return nil, fmt.Errorf("operation name is empty")
	}
	client, err := createGeminiCliReqClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("create HTTP client: %w", err)
	}
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+accessToken).
		SetHeader("User-Agent", geminicli.GeminiCLIUserAgent).
		SetSuccessResult(&out).
		Get(c.baseURL + "/v1internal/" + operationName)
	if err != nil {
		slog.Warn("codeassist_getoperation_request_error", "error", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if !resp.IsSuccessState() {
		body := resp.String()
		sanitizedBody := geminicli.SanitizeBodyForLogs(body)
		slog.Warn("codeassist_getoperation_failed", "status", resp.StatusCode, "body", sanitizedBody)
		return nil, &geminicli.CodeAssistHTTPError{StatusCode: resp.StatusCode, Body: sanitizedBody, Endpoint: "getOperation"}
	}
	slog.Debug("codeassist_getoperation_success", "status", resp.StatusCode)
	return &out, nil
}

func (c *geminiCliCodeAssistClient) RetrieveUserQuota(ctx context.Context, accessToken, proxyURL string, reqBody *geminicli.RetrieveUserQuotaRequest) (*geminicli.RetrieveUserQuotaResponse, error) {
	if reqBody == nil {
		reqBody = &geminicli.RetrieveUserQuotaRequest{}
	}

	var out geminicli.RetrieveUserQuotaResponse
	client, err := createGeminiCliReqClient(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("create HTTP client: %w", err)
	}
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+accessToken).
		SetHeader("Content-Type", "application/json").
		SetHeader("User-Agent", geminicli.GeminiCLIUserAgent).
		SetBody(reqBody).
		SetSuccessResult(&out).
		Post(c.baseURL + "/v1internal:retrieveUserQuota")
	if err != nil {
		slog.Warn("codeassist_retrieveuserquota_request_error", "error", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if !resp.IsSuccessState() {
		body := resp.String()
		sanitizedBody := geminicli.SanitizeBodyForLogs(body)
		slog.Warn("codeassist_retrieveuserquota_failed", "status", resp.StatusCode, "body", sanitizedBody)
		return nil, &geminicli.CodeAssistHTTPError{StatusCode: resp.StatusCode, Body: sanitizedBody, Endpoint: "retrieveUserQuota"}
	}
	slog.Debug("codeassist_retrieveuserquota_success", "status", resp.StatusCode)
	return &out, nil
}

func createGeminiCliReqClient(proxyURL string) (*req.Client, error) {
	return getSharedReqClient(ReqClientOptions{
		ProxyURL: proxyURL,
		Timeout:  30 * time.Second,
	})
}

func defaultLoadCodeAssistRequest() *geminicli.LoadCodeAssistRequest {
	return &geminicli.LoadCodeAssistRequest{
		Metadata: geminicli.LoadCodeAssistMetadata{
			IDEType:       "IDE_UNSPECIFIED",
			Platform:      "PLATFORM_UNSPECIFIED",
			PluginType:    "GEMINI",
			UpdateChannel: "PREVIEW", // 启用 Preview Release Channel 以访问预览版模型
		},
	}
}

func defaultOnboardUserRequest() *geminicli.OnboardUserRequest {
	return &geminicli.OnboardUserRequest{
		TierID: "LEGACY",
		Metadata: geminicli.LoadCodeAssistMetadata{
			IDEType:       "IDE_UNSPECIFIED",
			Platform:      "PLATFORM_UNSPECIFIED",
			PluginType:    "GEMINI",
			UpdateChannel: "PREVIEW", // 启用 Preview Release Channel 以访问预览版模型
		},
	}
}
