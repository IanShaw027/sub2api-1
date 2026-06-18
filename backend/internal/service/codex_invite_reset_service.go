package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

const (
	codexInviteResetReferralKey = "codex_referral_persistent_invite"
	codexBackendAPIBaseURL      = "https://chatgpt.com/backend-api"
	codexInviteResetMaxEmails   = 5
	// Codex Desktop 的邀请重置请求默认使用 Desktop UA。
	codexInviteResetDefaultUserAgent = "Codex Desktop/0.0.0 (Linux; x86_64)"
	// 上游响应体读取上限，与其他 OpenAI 上游调用保持一致。
	codexInviteResetBodyReadLimit = 2 << 20
	// 上游错误信息透传到管理端时的最大长度，避免把 Cloudflare 挑战页或大 JSON 整段塞进 UI。
	codexInviteResetErrorMessageMaxLen = 200
)

var codexInviteResetEmailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// CodexInviteResetService 封装 Codex Desktop 的邀请重置接口调用。
type CodexInviteResetService struct {
	adminService        AdminService
	httpUpstream        HTTPUpstream
	openAITokenProvider *OpenAITokenProvider
	tlsFPProfileService *TLSFingerprintProfileService
	historyRepo         CodexInviteResetHistoryRepository
}

// NewCodexInviteResetService 创建 Codex 邀请重置服务。
func NewCodexInviteResetService(
	adminService AdminService,
	httpUpstream HTTPUpstream,
	openAITokenProvider *OpenAITokenProvider,
	tlsFPProfileService *TLSFingerprintProfileService,
	historyRepo CodexInviteResetHistoryRepository,
) *CodexInviteResetService {
	return &CodexInviteResetService{
		adminService:        adminService,
		httpUpstream:        httpUpstream,
		openAITokenProvider: openAITokenProvider,
		tlsFPProfileService: tlsFPProfileService,
		historyRepo:         historyRepo,
	}
}

// CodexInviteResetActionType 区分历史记录的动作类型。
const (
	CodexInviteResetActionInvite  = "invite"
	CodexInviteResetActionConsume = "consume"
)

// CodexInviteResetHistoryEntry 是一条邀请/重置操作流水。
type CodexInviteResetHistoryEntry struct {
	ID             int64     `json:"id"`
	AccountID      int64     `json:"account_id"`
	ActionType     string    `json:"action_type"`
	OperatorUserID *int64    `json:"operator_user_id,omitempty"`
	Emails         []string  `json:"emails,omitempty"`
	FailedEmails   []string  `json:"failed_emails,omitempty"`
	CreditID       string    `json:"credit_id,omitempty"`
	Success        bool      `json:"success"`
	ResultCode     string    `json:"result_code,omitempty"`
	Message        string    `json:"message,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// CodexInviteResetHistoryRepository 持久化邀请/重置操作流水。
type CodexInviteResetHistoryRepository interface {
	Create(ctx context.Context, entry *CodexInviteResetHistoryEntry) error
	ListByAccount(ctx context.Context, accountID int64, params pagination.PaginationParams) ([]CodexInviteResetHistoryEntry, *pagination.PaginationResult, error)
}

type CodexInviteResetStatus struct {
	ReferralKey       string                   `json:"referral_key"`
	InviteEligibility map[string]any           `json:"invite_eligibility,omitempty"`
	EligibilityRules  []string                 `json:"eligibility_rules,omitempty"`
	RequiresConsent   bool                     `json:"requires_consent"`
	AvailableCount    int                      `json:"available_count"`
	Credits           []CodexInviteResetCredit `json:"credits"`
}

type CodexInviteResetCredit struct {
	ID              string `json:"id"`
	Status          string `json:"status,omitempty"`
	Title           string `json:"title,omitempty"`
	Description     string `json:"description,omitempty"`
	ProfileUserID   string `json:"profile_user_id,omitempty"`
	ProfileImageURL string `json:"profile_image_url,omitempty"`
}

type CodexInviteResetInviteResult struct {
	Invites      []map[string]any `json:"invites,omitempty"`
	FailedEmails []string         `json:"failed_emails,omitempty"`
	Message      string           `json:"message,omitempty"`
}

type CodexInviteResetConsumeResult struct {
	Code             string           `json:"code,omitempty"`
	CreditID         string           `json:"credit_id"`
	RedeemRequestID  string           `json:"redeem_request_id"`
	AvailableCount   *int             `json:"available_count,omitempty"`
	RemainingCredits []map[string]any `json:"remaining_credits,omitempty"`
}

type codexInviteResetAccountContext struct {
	account    *Account
	token      string
	proxyURL   string
	userAgent  string
	tlsProfile *tlsfingerprint.Profile
}

// GetStatus 查询邀请资格和可用重置次数。
func (s *CodexInviteResetService) GetStatus(ctx context.Context, accountID int64) (*CodexInviteResetStatus, error) {
	accountCtx, err := s.prepareAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}

	// 三个上游查询互不依赖，并发执行以降低管理端等待延迟（任一失败即取消其余）。
	var (
		eligibility map[string]any
		rules       map[string]any
		creditsRaw  map[string]any
	)
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		eligibility, err = s.getJSON(groupCtx, accountCtx, "/referrals/invite/eligibility", map[string]string{
			"referral_key": codexInviteResetReferralKey,
		})
		return err
	})
	group.Go(func() error {
		var err error
		rules, err = s.getJSON(groupCtx, accountCtx, "/wham/referrals/eligibility_rules", map[string]string{
			"referral_key": codexInviteResetReferralKey,
		})
		return err
	})
	group.Go(func() error {
		var err error
		creditsRaw, err = s.getJSON(groupCtx, accountCtx, "/wham/rate-limit-reset-credits", nil)
		return err
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}

	credits := normalizeCodexInviteResetCredits(creditsRaw)
	// 仅在上游未返回 available_count 字段时，才用 credits 数组兜底计数；
	// 上游明确返回的 0（例如 credit 处于冻结/待验证态）不应被覆盖成非 0。
	var availableCount int
	if _, ok := creditsRaw["available_count"]; ok {
		availableCount = codexInviteResetIntFromMap(creditsRaw, "available_count")
	} else {
		for _, credit := range credits {
			if strings.EqualFold(credit.Status, "available") {
				availableCount++
			}
		}
	}

	status := &CodexInviteResetStatus{
		ReferralKey:       codexInviteResetReferralKey,
		InviteEligibility: eligibility,
		EligibilityRules:  normalizeCodexInviteResetRules(rules),
		RequiresConsent:   codexInviteResetBoolFromMapDefault(eligibility, "requires_explicit_confirmation", true),
		AvailableCount:    availableCount,
		Credits:           credits,
	}
	s.persistStatusSnapshot(ctx, accountCtx.account, status)
	return status, nil
}

// SendInvite 发送 Codex 邀请邮件。operatorUserID 为发起操作的管理员（nil 表示系统触发）。
func (s *CodexInviteResetService) SendInvite(ctx context.Context, accountID int64, emails []string, operatorUserID *int64) (*CodexInviteResetInviteResult, error) {
	accountCtx, err := s.prepareAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeCodexInviteEmails(emails)
	if err != nil {
		return nil, err
	}

	raw, err := s.postJSON(ctx, accountCtx, "/wham/referrals/invite", map[string]any{
		"referral_key": codexInviteResetReferralKey,
		"emails":       normalized,
	})
	if err != nil {
		s.recordHistory(ctx, &CodexInviteResetHistoryEntry{
			AccountID:      accountID,
			ActionType:     CodexInviteResetActionInvite,
			OperatorUserID: operatorUserID,
			Emails:         normalized,
			Success:        false,
			Message:        codexInviteResetTruncateError(err.Error()),
		})
		return nil, err
	}

	result := &CodexInviteResetInviteResult{
		Invites:      codexInviteResetMapSliceFromMap(raw, "invites"),
		FailedEmails: codexInviteResetStringSliceFromMap(raw, "failed_emails"),
		Message:      codexInviteResetStringFromMap(raw, "message"),
	}
	s.recordHistory(ctx, &CodexInviteResetHistoryEntry{
		AccountID:      accountID,
		ActionType:     CodexInviteResetActionInvite,
		OperatorUserID: operatorUserID,
		Emails:         normalized,
		FailedEmails:   result.FailedEmails,
		Success:        len(result.FailedEmails) == 0,
		Message:        result.Message,
	})
	return result, nil
}

// Consume 使用一次可用的 Codex 重置机会。operatorUserID 为发起操作的管理员（nil 表示系统触发）。
func (s *CodexInviteResetService) Consume(ctx context.Context, accountID int64, creditID string, operatorUserID *int64) (*CodexInviteResetConsumeResult, error) {
	accountCtx, err := s.prepareAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	creditID = strings.TrimSpace(creditID)
	if creditID == "" {
		return nil, infraerrors.BadRequest("CODEX_INVITE_RESET_CREDIT_ID_REQUIRED", "credit_id is required")
	}
	redeemRequestID := uuid.NewString()

	raw, err := s.postJSON(ctx, accountCtx, "/wham/rate-limit-reset-credits/consume", map[string]any{
		"credit_id":         creditID,
		"redeem_request_id": redeemRequestID,
	})
	if err != nil {
		s.recordHistory(ctx, &CodexInviteResetHistoryEntry{
			AccountID:      accountID,
			ActionType:     CodexInviteResetActionConsume,
			OperatorUserID: operatorUserID,
			CreditID:       creditID,
			Success:        false,
			Message:        codexInviteResetTruncateError(err.Error()),
		})
		return nil, err
	}

	var availableCount *int
	if _, ok := raw["available_count"]; ok {
		v := codexInviteResetIntFromMap(raw, "available_count")
		availableCount = &v
	}
	result := &CodexInviteResetConsumeResult{
		Code:             codexInviteResetStringFromMap(raw, "code"),
		CreditID:         creditID,
		RedeemRequestID:  redeemRequestID,
		AvailableCount:   availableCount,
		RemainingCredits: codexInviteResetMapSliceFromMap(raw, "credits"),
	}
	// 上游 code 为空或 "reset" 视为成功重置，其余（如 nothing_to_reset / already_redeemed）视为未生效。
	success := result.Code == "" || result.Code == "reset"
	s.recordHistory(ctx, &CodexInviteResetHistoryEntry{
		AccountID:      accountID,
		ActionType:     CodexInviteResetActionConsume,
		OperatorUserID: operatorUserID,
		CreditID:       creditID,
		Success:        success,
		ResultCode:     result.Code,
	})
	return result, nil
}

// ListHistory 分页查询某账号的邀请/重置操作流水。
func (s *CodexInviteResetService) ListHistory(ctx context.Context, accountID int64, params pagination.PaginationParams) ([]CodexInviteResetHistoryEntry, *pagination.PaginationResult, error) {
	if s.historyRepo == nil {
		return []CodexInviteResetHistoryEntry{}, &pagination.PaginationResult{Page: params.Page, PageSize: params.PageSize}, nil
	}
	return s.historyRepo.ListByAccount(ctx, accountID, params)
}

// recordHistory 写入一条操作流水，失败仅记日志不阻断主流程。
func (s *CodexInviteResetService) recordHistory(ctx context.Context, entry *CodexInviteResetHistoryEntry) {
	if s.historyRepo == nil {
		return
	}
	if err := s.historyRepo.Create(ctx, entry); err != nil {
		slog.Warn("codex_invite_reset_history_write_failed",
			"account_id", entry.AccountID,
			"action_type", entry.ActionType,
			"error", err)
	}
}

// persistStatusSnapshot stores the latest query result on account.extra so
// list/reload paths can show the last known reset count without re-querying upstream.
func (s *CodexInviteResetService) persistStatusSnapshot(ctx context.Context, account *Account, status *CodexInviteResetStatus) {
	if s == nil || s.adminService == nil || account == nil || account.ID <= 0 || status == nil {
		return
	}
	updates := buildCodexInviteResetStatusExtraUpdates(status, time.Now().UTC())
	if len(updates) == 0 {
		return
	}
	if err := s.adminService.UpdateAccountExtra(ctx, account.ID, updates); err != nil {
		slog.Warn("codex_invite_reset_status_persist_failed",
			"account_id", account.ID,
			"error", err)
		return
	}
	mergeAccountExtra(account, updates)
}

func buildCodexInviteResetStatusExtraUpdates(status *CodexInviteResetStatus, now time.Time) map[string]any {
	if status == nil {
		return nil
	}
	creditIDs := make([]string, 0, len(status.Credits))
	credits := make([]map[string]any, 0, len(status.Credits))
	for _, credit := range status.Credits {
		if strings.TrimSpace(credit.ID) != "" {
			creditIDs = append(creditIDs, credit.ID)
		}
		creditMap := map[string]any{"id": credit.ID}
		if credit.Status != "" {
			creditMap["status"] = credit.Status
		}
		if credit.Title != "" {
			creditMap["title"] = credit.Title
		}
		if credit.Description != "" {
			creditMap["description"] = credit.Description
		}
		if credit.ProfileUserID != "" {
			creditMap["profile_user_id"] = credit.ProfileUserID
		}
		if credit.ProfileImageURL != "" {
			creditMap["profile_image_url"] = credit.ProfileImageURL
		}
		credits = append(credits, creditMap)
	}
	return map[string]any{
		"codex_invite_reset_available_count": status.AvailableCount,
		"codex_invite_reset_updated_at":      now.Format(time.RFC3339),
		"codex_invite_reset_credit_ids":      creditIDs,
		"codex_invite_reset_credits":         credits,
	}
}

func (s *CodexInviteResetService) prepareAccount(ctx context.Context, accountID int64) (*codexInviteResetAccountContext, error) {
	if s == nil || s.adminService == nil {
		return nil, infraerrors.InternalServer("CODEX_INVITE_RESET_SERVICE_NOT_CONFIGURED", "codex invite reset service is not configured")
	}
	account, err := s.adminService.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, infraerrors.NotFound("ACCOUNT_NOT_FOUND", "account not found")
	}
	if !account.IsOpenAIOAuth() {
		return nil, infraerrors.BadRequest("CODEX_INVITE_RESET_UNSUPPORTED_ACCOUNT", "only OpenAI OAuth accounts support Codex invite reset")
	}

	token := ""
	if s.openAITokenProvider != nil {
		token, err = s.openAITokenProvider.GetAccessToken(ctx, account)
		if err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(token) == "" {
		token = account.GetOpenAIAccessToken()
	}
	if strings.TrimSpace(token) == "" {
		return nil, infraerrors.BadRequest("CODEX_INVITE_RESET_MISSING_TOKEN", "missing OpenAI OAuth access token")
	}

	proxyURL := ""
	if account.ProxyID != nil {
		proxy, proxyErr := s.adminService.GetProxy(ctx, *account.ProxyID)
		if proxyErr != nil {
			return nil, proxyErr
		}
		if proxy != nil {
			proxyURL = proxy.URL()
		}
	}

	return &codexInviteResetAccountContext{
		account:    account,
		token:      token,
		proxyURL:   proxyURL,
		userAgent:  codexInviteResetDefaultUserAgent,
		tlsProfile: s.resolveTLSProfile(account),
	}, nil
}

func (s *CodexInviteResetService) resolveTLSProfile(account *Account) *tlsfingerprint.Profile {
	if s == nil || s.tlsFPProfileService == nil {
		return nil
	}
	return s.tlsFPProfileService.ResolveTLSProfile(account)
}

func (s *CodexInviteResetService) getJSON(ctx context.Context, accountCtx *codexInviteResetAccountContext, path string, query map[string]string) (map[string]any, error) {
	target, err := buildCodexInviteResetURL(path, query)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	s.applyHeaders(req, accountCtx)
	return s.doJSON(req, accountCtx)
}

func (s *CodexInviteResetService) postJSON(ctx context.Context, accountCtx *codexInviteResetAccountContext, path string, body map[string]any) (map[string]any, error) {
	target, err := buildCodexInviteResetURL(path, nil)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	s.applyHeaders(req, accountCtx)
	return s.doJSON(req, accountCtx)
}

func (s *CodexInviteResetService) applyHeaders(req *http.Request, accountCtx *codexInviteResetAccountContext) {
	*req = *req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	req.Host = "chatgpt.com"
	req.Header.Set("Authorization", "Bearer "+accountCtx.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("OAI-Language", "zh-CN")
	req.Header.Set("originator", "Codex Desktop")
	req.Header.Set("X-OpenAI-Attach-Auth", "1")
	req.Header.Set("X-OpenAI-Attach-Integrity-State", "1")
	req.Header.Set("User-Agent", accountCtx.userAgent)
	if chatgptAccountID := accountCtx.account.GetChatGPTAccountID(); chatgptAccountID != "" {
		req.Header.Set("chatgpt-account-id", chatgptAccountID)
	}
}

func (s *CodexInviteResetService) doJSON(req *http.Request, accountCtx *codexInviteResetAccountContext) (map[string]any, error) {
	if s.httpUpstream == nil {
		return nil, infraerrors.InternalServer("HTTP_UPSTREAM_NOT_CONFIGURED", "http upstream is not configured")
	}
	req = req.WithContext(WithHTTPUpstreamProfile(req.Context(), HTTPUpstreamProfileOpenAI))
	resp, err := s.httpUpstream.DoWithTLS(req, accountCtx.proxyURL, accountCtx.account.ID, accountCtx.account.Concurrency, accountCtx.tlsProfile)
	if err != nil {
		return nil, err
	}
	// 响应体会被完整读取，关闭失败不影响本次调用结果。
	defer func() { _ = resp.Body.Close() }()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, codexInviteResetBodyReadLimit))
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := codexInviteResetTruncateError(string(body))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}
		return nil, infraerrors.Newf(resp.StatusCode, "CODEX_INVITE_RESET_UPSTREAM_ERROR", "codex invite reset upstream returned %d: %s", resp.StatusCode, message)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return map[string]any{}, nil
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode codex invite reset response: %w", err)
	}
	return result, nil
}

func buildCodexInviteResetURL(path string, query map[string]string) (string, error) {
	base, err := url.Parse(codexBackendAPIBaseURL)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	base.Path = strings.TrimRight(base.Path, "/") + path
	if len(query) > 0 {
		values := base.Query()
		for key, value := range query {
			values.Set(key, value)
		}
		base.RawQuery = values.Encode()
	}
	return base.String(), nil
}

// codexInviteResetTruncateError 截断上游错误体，避免把 Cloudflare 挑战页或大 JSON 整段透传到管理端 UI。
func codexInviteResetTruncateError(body string) string {
	message := strings.TrimSpace(body)
	runes := []rune(message)
	if len(runes) <= codexInviteResetErrorMessageMaxLen {
		return message
	}
	return string(runes[:codexInviteResetErrorMessageMaxLen]) + "…"
}

func normalizeCodexInviteEmails(emails []string) ([]string, error) {
	result := make([]string, 0, len(emails))
	seen := make(map[string]struct{}, len(emails))
	for _, raw := range emails {
		for _, part := range splitCodexInviteEmailInput(raw) {
			email := strings.TrimSpace(part)
			if email == "" {
				continue
			}
			key := strings.ToLower(email)
			if _, exists := seen[key]; exists {
				continue
			}
			if !codexInviteResetEmailPattern.MatchString(email) {
				return nil, infraerrors.BadRequest("CODEX_INVITE_RESET_INVALID_EMAIL", fmt.Sprintf("invalid email: %s", email))
			}
			seen[key] = struct{}{}
			result = append(result, email)
			if len(result) > codexInviteResetMaxEmails {
				return nil, infraerrors.BadRequest("CODEX_INVITE_RESET_EMAIL_LIMIT", fmt.Sprintf("最多一次邀请 %d 个邮箱", codexInviteResetMaxEmails))
			}
		}
	}
	if len(result) == 0 {
		return nil, infraerrors.BadRequest("CODEX_INVITE_RESET_EMAILS_REQUIRED", "emails are required")
	}
	return result, nil
}

func splitCodexInviteEmailInput(input string) []string {
	return strings.FieldsFunc(input, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\r', '\t', ' ':
			return true
		default:
			return false
		}
	})
}

func normalizeCodexInviteResetCredits(raw map[string]any) []CodexInviteResetCredit {
	items := codexInviteResetMapSliceFromMap(raw, "credits")
	credits := make([]CodexInviteResetCredit, 0, len(items))
	for _, item := range items {
		id := codexInviteResetStringFromMap(item, "id")
		if id == "" {
			continue
		}
		credits = append(credits, CodexInviteResetCredit{
			ID:              id,
			Status:          codexInviteResetStringFromMap(item, "status"),
			Title:           codexInviteResetStringFromMap(item, "title"),
			Description:     codexInviteResetStringFromMap(item, "description"),
			ProfileUserID:   codexInviteResetStringFromMap(item, "profile_user_id"),
			ProfileImageURL: codexInviteResetStringFromMap(item, "profile_image_url"),
		})
	}
	return credits
}

func normalizeCodexInviteResetRules(raw map[string]any) []string {
	rulesRaw, ok := raw["rules"].([]any)
	if !ok {
		return nil
	}
	rules := make([]string, 0, len(rulesRaw))
	for _, item := range rulesRaw {
		switch value := item.(type) {
		case string:
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				rules = append(rules, trimmed)
			}
		case map[string]any:
			for _, key := range []string{"text", "description", "message", "title"} {
				if text := codexInviteResetStringFromMap(value, key); text != "" {
					rules = append(rules, text)
					break
				}
			}
		}
	}
	return rules
}

func codexInviteResetStringFromMap(raw map[string]any, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func codexInviteResetIntFromMap(raw map[string]any, key string) int {
	if raw == nil {
		return 0
	}
	switch value := raw[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		i, _ := value.Int64()
		return int(i)
	default:
		return 0
	}
}

func codexInviteResetBoolFromMapDefault(raw map[string]any, key string, fallback bool) bool {
	if raw == nil {
		return fallback
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return fallback
	}
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

func codexInviteResetStringSliceFromMap(raw map[string]any, key string) []string {
	values, ok := raw[key].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if s := strings.TrimSpace(fmt.Sprint(value)); s != "" {
			result = append(result, s)
		}
	}
	return result
}

func codexInviteResetMapSliceFromMap(raw map[string]any, key string) []map[string]any {
	values, ok := raw[key].([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			result = append(result, item)
		}
	}
	return result
}
