package admin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// semverPattern 预编译 semver 格式校验正则
var semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// menuItemIDPattern validates custom menu item IDs: alphanumeric, hyphens, underscores only.
var menuItemIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// generateMenuItemID generates a short random hex ID for a custom menu item.
func generateMenuItemID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate menu item ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func scopesContainOpenID(scopes string) bool {
	for _, scope := range strings.Fields(strings.ToLower(strings.TrimSpace(scopes))) {
		if scope == "openid" {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func validateCustomMenuPageURL(raw string) error {
	// Custom menu URLs intentionally allow fragments because admin tools may
	// carry client-side tokens or anchors in the hash segment.
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("empty url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if !u.IsAbs() {
		return fmt.Errorf("must be absolute")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	if strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("missing host")
	}
	return nil
}

func mustMarshalSupportQRCodes(entries *[]dto.SupportQRCodeEntry) string {
	if entries == nil || len(*entries) == 0 {
		return "[]"
	}

	normalized := make([]dto.SupportQRCodeEntry, 0, len(*entries))
	for _, entry := range *entries {
		imageURL := strings.TrimSpace(entry.ImageURL)
		if imageURL == "" {
			continue
		}
		normalized = append(normalized, dto.SupportQRCodeEntry{
			ImageURL: imageURL,
			Note:     strings.TrimSpace(entry.Note),
		})
	}
	if len(normalized) == 0 {
		return "[]"
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

// SettingHandler 系统设置处理器
type SettingHandler struct {
	settingService           *service.SettingService
	notificationEmailService *service.NotificationEmailService
	emailService             *service.EmailService
	turnstileService         *service.TurnstileService
	opsService               *service.OpsService
	paymentConfigService     *service.PaymentConfigService
	paymentService           *service.PaymentService
	mediaService             *service.MediaService
}

// NewSettingHandler 创建系统设置处理器
func NewSettingHandler(
	settingService *service.SettingService,
	emailService *service.EmailService,
	turnstileService *service.TurnstileService,
	opsService *service.OpsService,
	paymentConfigService *service.PaymentConfigService,
	paymentService *service.PaymentService,
	mediaServices ...*service.MediaService,
) *SettingHandler {
	handler := &SettingHandler{
		settingService:       settingService,
		emailService:         emailService,
		turnstileService:     turnstileService,
		opsService:           opsService,
		paymentConfigService: paymentConfigService,
		paymentService:       paymentService,
	}
	if len(mediaServices) > 0 {
		handler.mediaService = mediaServices[0]
	}
	return handler
}

// SetNotificationEmailService attaches the notification email service without
// making it a constructor dependency for legacy test helpers.
func (h *SettingHandler) SetNotificationEmailService(notificationEmailService *service.NotificationEmailService) {
	h.notificationEmailService = notificationEmailService
}

func (h *SettingHandler) ingestSettingsMediaReferences(ctx context.Context, req *UpdateSettingsRequest) ([]int64, error) {
	if h == nil || h.mediaService == nil || !h.mediaService.RuntimeInfo().Enabled {
		return nil, nil
	}
	createdAssetIDs := make([]int64, 0, 3)
	if value := strings.TrimSpace(req.SiteLogo); value != "" {
		publicURL, assetID, err := h.ingestPublicImage(ctx, "site_logo", "default", value, "site-logo")
		if assetID > 0 {
			createdAssetIDs = append(createdAssetIDs, assetID)
		}
		if err != nil {
			return createdAssetIDs, err
		}
		req.SiteLogo = publicURL
	}
	if req.SupportQRCodes != nil {
		for idx := range *req.SupportQRCodes {
			value := strings.TrimSpace((*req.SupportQRCodes)[idx].ImageURL)
			if value == "" {
				continue
			}
			publicURL, assetID, err := h.ingestPublicImage(ctx, "support_qr", fmt.Sprintf("%d", idx+1), value, fmt.Sprintf("support-qr-%d", idx+1))
			if assetID > 0 {
				createdAssetIDs = append(createdAssetIDs, assetID)
			}
			if err != nil {
				return createdAssetIDs, err
			}
			(*req.SupportQRCodes)[idx].ImageURL = publicURL
		}
	}
	if req.PaymentHelpImageURL != nil && strings.TrimSpace(*req.PaymentHelpImageURL) != "" {
		publicURL, assetID, err := h.ingestPublicImage(ctx, "payment_help", "default", strings.TrimSpace(*req.PaymentHelpImageURL), "payment-help")
		if assetID > 0 {
			createdAssetIDs = append(createdAssetIDs, assetID)
		}
		if err != nil {
			return createdAssetIDs, err
		}
		req.PaymentHelpImageURL = &publicURL
	}
	return createdAssetIDs, nil
}

func (h *SettingHandler) ingestPublicImage(ctx context.Context, bizType, bizID, raw, fileName string) (string, int64, error) {
	if publicURL, ok := h.normalizeManagedMediaURL(ctx, raw); ok {
		return publicURL, 0, nil
	}
	asset, err := h.mediaService.IngestImageReference(ctx, service.IngestImageReferenceInput{
		BizType:    bizType,
		BizID:      bizID,
		Visibility: service.MediaVisibilityPublic,
		Source:     raw,
		FileName:   fileName,
	})
	if err != nil {
		return "", 0, err
	}
	publicURL := strings.TrimSpace(h.mediaService.PublicURL(asset))
	if publicURL == "" {
		return "", asset.ID, service.ErrMediaStorageDisabled
	}
	return publicURL, asset.ID, nil
}

func (h *SettingHandler) cleanupIngestedSettingsMediaReferences(ctx context.Context, assetIDs []int64) {
	if h == nil || h.mediaService == nil {
		return
	}
	for i := len(assetIDs) - 1; i >= 0; i-- {
		if assetIDs[i] <= 0 {
			continue
		}
		if err := h.mediaService.DeleteForAdmin(ctx, assetIDs[i]); err != nil {
			slog.Warn("failed to cleanup migrated settings media", "media_id", assetIDs[i], "error", err)
		}
	}
}

func (h *SettingHandler) normalizeManagedMediaURL(ctx context.Context, raw string) (string, bool) {
	id, ok := h.parseManagedMediaID(raw)
	if !ok {
		return "", false
	}
	asset, err := h.mediaService.GetForAdmin(ctx, id)
	if err != nil || asset == nil {
		return "", false
	}
	publicURL := strings.TrimSpace(h.mediaService.PublicURL(asset))
	if publicURL == "" {
		return "", false
	}
	return publicURL, true
}

func (h *SettingHandler) parseManagedMediaID(raw string) (int64, bool) {
	if h == nil || h.mediaService == nil {
		return 0, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if id, ok := service.ParseManagedMediaID(h.mediaService, raw); ok {
		return id, true
	}
	if id, ok := parseManagedMediaIDFromPath(raw); ok {
		return id, true
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return 0, false
	}
	return parseManagedMediaIDFromPath(parsed.Path)
}

func (h *SettingHandler) emailTemplateService() *service.NotificationEmailService {
	if h == nil {
		return nil
	}
	return h.notificationEmailService
}

func (h *SettingHandler) ListEmailTemplates(c *gin.Context) {
	svc := h.emailTemplateService()
	if svc == nil {
		response.InternalError(c, "Notification email service is not configured")
		return
	}
	events := svc.ListEventInfos()
	eventOptions := make([]dto.EmailTemplateEventOption, 0, len(events))
	placeholderSet := make(map[string]struct{})
	for _, event := range events {
		eventOptions = append(eventOptions, dto.EmailTemplateEventOption{
			Value:       event.Event,
			Label:       event.Label,
			Description: event.Description,
			Category:    event.Category,
			Optional:    event.Optional,
		})
		for _, placeholder := range event.Placeholders {
			if placeholder = strings.TrimSpace(placeholder); placeholder != "" {
				placeholderSet[placeholder] = struct{}{}
			}
		}
	}
	placeholders := make([]string, 0, len(placeholderSet))
	for placeholder := range placeholderSet {
		placeholders = append(placeholders, placeholder)
	}
	sort.Strings(placeholders)

	templates, err := svc.ListTemplates(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	summaries := make([]dto.EmailTemplateSummary, 0, len(templates))
	for _, tmpl := range templates {
		summary := dto.EmailTemplateSummary{
			Event:    tmpl.Event,
			Locale:   tmpl.Locale,
			Subject:  tmpl.Subject,
			IsCustom: tmpl.IsCustom,
		}
		if tmpl.UpdatedAt != nil {
			summary.UpdatedAt = tmpl.UpdatedAt.UTC().Format(time.RFC3339)
		}
		summaries = append(summaries, summary)
	}

	response.Success(c, dto.EmailTemplateListResponse{
		Events:       eventOptions,
		Locales:      svc.SupportedLocales(),
		Templates:    summaries,
		Placeholders: placeholders,
	})
}

func (h *SettingHandler) GetEmailTemplate(c *gin.Context) {
	svc := h.emailTemplateService()
	if svc == nil {
		response.InternalError(c, "Notification email service is not configured")
		return
	}
	tmpl, err := svc.GetTemplate(c.Request.Context(), c.Param("event"), c.Param("locale"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	detail := dto.EmailTemplateDetail{
		Event:        tmpl.Event,
		Locale:       tmpl.Locale,
		Subject:      tmpl.Subject,
		HTML:         tmpl.HTML,
		IsCustom:     tmpl.IsCustom,
		Placeholders: append([]string(nil), tmpl.Placeholders...),
	}
	if tmpl.UpdatedAt != nil {
		detail.UpdatedAt = tmpl.UpdatedAt.UTC().Format(time.RFC3339)
	}
	response.Success(c, detail)
}

func (h *SettingHandler) UpdateEmailTemplate(c *gin.Context) {
	svc := h.emailTemplateService()
	if svc == nil {
		response.InternalError(c, "Notification email service is not configured")
		return
	}
	var req dto.UpdateEmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	tmpl, err := svc.UpdateTemplate(c.Request.Context(), c.Param("event"), c.Param("locale"), req.Subject, req.HTML)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	detail := dto.EmailTemplateDetail{
		Event:        tmpl.Event,
		Locale:       tmpl.Locale,
		Subject:      tmpl.Subject,
		HTML:         tmpl.HTML,
		IsCustom:     tmpl.IsCustom,
		Placeholders: append([]string(nil), tmpl.Placeholders...),
	}
	if tmpl.UpdatedAt != nil {
		detail.UpdatedAt = tmpl.UpdatedAt.UTC().Format(time.RFC3339)
	}
	response.Success(c, detail)
}

func (h *SettingHandler) RestoreOfficialEmailTemplate(c *gin.Context) {
	svc := h.emailTemplateService()
	if svc == nil {
		response.InternalError(c, "Notification email service is not configured")
		return
	}
	tmpl, err := svc.RestoreOfficialTemplate(c.Request.Context(), c.Param("event"), c.Param("locale"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	detail := dto.EmailTemplateDetail{
		Event:        tmpl.Event,
		Locale:       tmpl.Locale,
		Subject:      tmpl.Subject,
		HTML:         tmpl.HTML,
		IsCustom:     tmpl.IsCustom,
		Placeholders: append([]string(nil), tmpl.Placeholders...),
	}
	if tmpl.UpdatedAt != nil {
		detail.UpdatedAt = tmpl.UpdatedAt.UTC().Format(time.RFC3339)
	}
	response.Success(c, detail)
}

func (h *SettingHandler) PreviewEmailTemplate(c *gin.Context) {
	svc := h.emailTemplateService()
	if svc == nil {
		response.InternalError(c, "Notification email service is not configured")
		return
	}
	var req dto.PreviewEmailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	preview, err := svc.PreviewTemplate(c.Request.Context(), service.NotificationEmailPreviewInput{
		Event:     req.Event,
		Locale:    req.Locale,
		Subject:   req.Subject,
		HTML:      req.HTML,
		Variables: req.Variables,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.EmailTemplatePreviewResponse{
		Subject: preview.Subject,
		HTML:    preview.HTML,
	})
}

func parseManagedMediaIDFromPath(raw string) (int64, bool) {
	remainder := strings.TrimSpace(raw)
	if remainder == "" {
		return 0, false
	}
	if idx := strings.IndexAny(remainder, "?#"); idx >= 0 {
		remainder = remainder[:idx]
	}
	for _, prefix := range []string{
		"/api/v1/media/public/",
		"/api/v1/media/download/",
	} {
		if !strings.HasPrefix(remainder, prefix) {
			continue
		}
		remainder = strings.TrimPrefix(remainder, prefix)
		if idx := strings.IndexRune(remainder, '/'); idx >= 0 {
			remainder = remainder[:idx]
		}
		id, err := strconv.ParseInt(strings.TrimSpace(remainder), 10, 64)
		if err == nil && id > 0 {
			return id, true
		}
		return 0, false
	}
	return 0, false
}

// GetSettings 获取所有系统设置
// GET /api/v1/admin/settings
func (h *SettingHandler) GetSettings(c *gin.Context) {
	settings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	authSourceDefaults, err := h.settingService.GetAuthSourceDefaultSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Check if ops monitoring is enabled (respects config.ops.enabled)
	opsEnabled := h.opsService != nil && h.opsService.IsMonitoringEnabled(c.Request.Context())
	defaultSubscriptions := make([]dto.DefaultSubscriptionSetting, 0, len(settings.DefaultSubscriptions))
	for _, sub := range settings.DefaultSubscriptions {
		defaultSubscriptions = append(defaultSubscriptions, dto.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	// Load payment config
	var paymentCfg *service.PaymentConfig
	if h.paymentConfigService != nil {
		paymentCfg, _ = h.paymentConfigService.GetPaymentConfig(c.Request.Context())
	}
	if paymentCfg == nil {
		paymentCfg = &service.PaymentConfig{}
	}

	payload := dto.SystemSettings{
		RegistrationEnabled:                    settings.RegistrationEnabled,
		EmailVerifyEnabled:                     settings.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist:       settings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                       settings.PromoCodeEnabled,
		PasswordResetEnabled:                   settings.PasswordResetEnabled,
		FrontendURL:                            settings.FrontendURL,
		InvitationCodeEnabled:                  settings.InvitationCodeEnabled,
		TotpEnabled:                            settings.TotpEnabled,
		TotpEncryptionKeyConfigured:            h.settingService.IsTotpEncryptionKeyConfigured(),
		LoginAgreementEnabled:                  settings.LoginAgreementEnabled,
		LoginAgreementMode:                     settings.LoginAgreementMode,
		LoginAgreementUpdatedAt:                settings.LoginAgreementUpdatedAt,
		LoginAgreementDocuments:                loginAgreementDocumentsToDTO(settings.LoginAgreementDocuments),
		SMTPHost:                               settings.SMTPHost,
		SMTPPort:                               settings.SMTPPort,
		SMTPUsername:                           settings.SMTPUsername,
		SMTPPasswordConfigured:                 settings.SMTPPasswordConfigured,
		SMTPFrom:                               settings.SMTPFrom,
		SMTPFromName:                           settings.SMTPFromName,
		SMTPUseTLS:                             settings.SMTPUseTLS,
		TurnstileEnabled:                       settings.TurnstileEnabled,
		TurnstileSiteKey:                       settings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:           settings.TurnstileSecretKeyConfigured,
		APIKeyACLTrustForwardedIP:              settings.APIKeyACLTrustForwardedIP,
		LinuxDoConnectEnabled:                  settings.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:                 settings.LinuxDoConnectClientID,
		LinuxDoConnectClientSecretConfigured:   settings.LinuxDoConnectClientSecretConfigured,
		LinuxDoConnectRedirectURL:              settings.LinuxDoConnectRedirectURL,
		DingTalkConnectEnabled:                 settings.DingTalkConnectEnabled,
		DingTalkConnectClientID:                settings.DingTalkConnectClientID,
		DingTalkConnectClientSecretConfigured:  settings.DingTalkConnectClientSecretConfigured,
		DingTalkConnectRedirectURL:             settings.DingTalkConnectRedirectURL,
		DingTalkConnectCorpRestrictionPolicy:   settings.DingTalkConnectCorpRestrictionPolicy,
		DingTalkConnectInternalCorpID:          settings.DingTalkConnectInternalCorpID,
		DingTalkConnectBypassRegistration:      settings.DingTalkConnectBypassRegistration,
		DingTalkConnectSyncCorpEmail:           settings.DingTalkConnectSyncCorpEmail,
		DingTalkConnectSyncDisplayName:         settings.DingTalkConnectSyncDisplayName,
		DingTalkConnectSyncDept:                settings.DingTalkConnectSyncDept,
		DingTalkConnectSyncCorpEmailAttrKey:    settings.DingTalkConnectSyncCorpEmailAttrKey,
		DingTalkConnectSyncDisplayNameAttrKey:  settings.DingTalkConnectSyncDisplayNameAttrKey,
		DingTalkConnectSyncDeptAttrKey:         settings.DingTalkConnectSyncDeptAttrKey,
		DingTalkConnectSyncCorpEmailAttrName:   settings.DingTalkConnectSyncCorpEmailAttrName,
		DingTalkConnectSyncDisplayNameAttrName: settings.DingTalkConnectSyncDisplayNameAttrName,
		DingTalkConnectSyncDeptAttrName:        settings.DingTalkConnectSyncDeptAttrName,
		WeChatConnectEnabled:                   settings.WeChatConnectEnabled,
		WeChatConnectAppID:                     settings.WeChatConnectAppID,
		WeChatConnectAppSecretConfigured:       settings.WeChatConnectAppSecretConfigured,
		WeChatConnectOpenAppID:                 settings.WeChatConnectOpenAppID,
		WeChatConnectOpenAppSecretConfigured:   settings.WeChatConnectOpenAppSecretConfigured,
		WeChatConnectMPAppID:                   settings.WeChatConnectMPAppID,
		WeChatConnectMPAppSecretConfigured:     settings.WeChatConnectMPAppSecretConfigured,
		WeChatConnectMobileAppID:               settings.WeChatConnectMobileAppID,
		WeChatConnectMobileAppSecretConfigured: settings.WeChatConnectMobileAppSecretConfigured,
		WeChatConnectOpenEnabled:               settings.WeChatConnectOpenEnabled,
		WeChatConnectMPEnabled:                 settings.WeChatConnectMPEnabled,
		WeChatConnectMobileEnabled:             settings.WeChatConnectMobileEnabled,
		WeChatConnectMode:                      settings.WeChatConnectMode,
		WeChatConnectScopes:                    settings.WeChatConnectScopes,
		WeChatConnectRedirectURL:               settings.WeChatConnectRedirectURL,
		WeChatConnectFrontendRedirectURL:       settings.WeChatConnectFrontendRedirectURL,
		OIDCConnectEnabled:                     settings.OIDCConnectEnabled,
		OIDCConnectProviderName:                settings.OIDCConnectProviderName,
		OIDCConnectClientID:                    settings.OIDCConnectClientID,
		OIDCConnectClientSecretConfigured:      settings.OIDCConnectClientSecretConfigured,
		OIDCConnectIssuerURL:                   settings.OIDCConnectIssuerURL,
		OIDCConnectDiscoveryURL:                settings.OIDCConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:                settings.OIDCConnectAuthorizeURL,
		OIDCConnectTokenURL:                    settings.OIDCConnectTokenURL,
		OIDCConnectUserInfoURL:                 settings.OIDCConnectUserInfoURL,
		OIDCConnectJWKSURL:                     settings.OIDCConnectJWKSURL,
		OIDCConnectScopes:                      settings.OIDCConnectScopes,
		OIDCConnectRedirectURL:                 settings.OIDCConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:         settings.OIDCConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:             settings.OIDCConnectTokenAuthMethod,
		OIDCConnectUsePKCE:                     settings.OIDCConnectUsePKCE,
		OIDCConnectValidateIDToken:             settings.OIDCConnectValidateIDToken,
		OIDCConnectAllowedSigningAlgs:          settings.OIDCConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:            settings.OIDCConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:        settings.OIDCConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:           settings.OIDCConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:              settings.OIDCConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:        settings.OIDCConnectUserInfoUsernamePath,
		GitHubOAuthEnabled:                     settings.GitHubOAuthEnabled,
		GitHubOAuthClientID:                    settings.GitHubOAuthClientID,
		GitHubOAuthClientSecretConfigured:      settings.GitHubOAuthClientSecretConfigured,
		GitHubOAuthRedirectURL:                 settings.GitHubOAuthRedirectURL,
		GitHubOAuthFrontendRedirectURL:         settings.GitHubOAuthFrontendRedirectURL,
		GoogleOAuthEnabled:                     settings.GoogleOAuthEnabled,
		GoogleOAuthClientID:                    settings.GoogleOAuthClientID,
		GoogleOAuthClientSecretConfigured:      settings.GoogleOAuthClientSecretConfigured,
		GoogleOAuthRedirectURL:                 settings.GoogleOAuthRedirectURL,
		GoogleOAuthFrontendRedirectURL:         settings.GoogleOAuthFrontendRedirectURL,
		SiteName:                               settings.SiteName,
		SiteLogo:                               settings.SiteLogo,
		SiteSubtitle:                           settings.SiteSubtitle,
		APIBaseURL:                             settings.APIBaseURL,
		ContactInfo:                            settings.ContactInfo,
		SupportQRCodes:                         dto.ParseSupportQRCodes(settings.SupportQRCodes),
		DocURL:                                 settings.DocURL,
		HomeContent:                            settings.HomeContent,
		HideCcsImportButton:                    settings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:            settings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:                settings.PurchaseSubscriptionURL,
		TableDefaultPageSize:                   settings.TableDefaultPageSize,
		TablePageSizeOptions:                   settings.TablePageSizeOptions,
		CustomMenuItems:                        dto.ParseCustomMenuItems(settings.CustomMenuItems),
		CustomEndpoints:                        dto.ParseCustomEndpoints(settings.CustomEndpoints),
		DefaultConcurrency:                     settings.DefaultConcurrency,
		DefaultBalance:                         settings.DefaultBalance,
		AffiliateEnabled:                       settings.AffiliateEnabled,
		RiskControlEnabled:                     settings.RiskControlEnabled,
		CyberSessionBlockEnabled:               settings.CyberSessionBlockEnabled,
		CyberSessionBlockTTLSeconds:            settings.CyberSessionBlockTTLSeconds,
		AffiliateRebateRate:                    settings.AffiliateRebateRate,
		AffiliateRebateCap:                     settings.AffiliateRebateCap,
		AffiliateRebateInviteeLimit:            settings.AffiliateRebateInviteeLimit,
		AffiliateSignupBonus:                   settings.AffiliateSignupBonus,
		TicketEnabled:                          settings.TicketEnabled,
		AIStudioEnabled:                        settings.AIStudioEnabled,
		AffiliateRebateFreezeHours:             settings.AffiliateRebateFreezeHours,
		AffiliateRebateDurationDays:            settings.AffiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap:           settings.AffiliateRebatePerInviteeCap,
		DefaultUserRPMLimit:                    settings.DefaultUserRPMLimit,
		DefaultSubscriptions:                   defaultSubscriptions,
		EnableModelFallback:                    settings.EnableModelFallback,
		FallbackModelAnthropic:                 settings.FallbackModelAnthropic,
		FallbackModelOpenAI:                    settings.FallbackModelOpenAI,
		FallbackModelGemini:                    settings.FallbackModelGemini,
		FallbackModelAntigravity:               settings.FallbackModelAntigravity,
		PlatformDefaultAccountModelConfig:      toDTODefaultAccountModelConfig(settings.PlatformDefaultAccountModelConfig),
		EnableIdentityPatch:                    settings.EnableIdentityPatch,
		IdentityPatchPrompt:                    settings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                   opsEnabled && settings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:           settings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                    settings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:              settings.OpsMetricsIntervalSeconds,
		MinClaudeCodeVersion:                   settings.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:                   settings.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling:            settings.AllowUngroupedKeyScheduling,
		BackendModeEnabled:                     settings.BackendModeEnabled,
		EnableFingerprintUnification:           settings.EnableFingerprintUnification,
		EnableMetadataPassthrough:              settings.EnableMetadataPassthrough,
		ClaudeTelemetryMode:                    settings.ClaudeTelemetryMode,
		GatewayDebugTimelineEnabled:            settings.GatewayDebugTimelineEnabled,
		GatewayDebugTimelineDirectory:          settings.GatewayDebugTimelineDirectory,
		GatewayDebugTimelineRetentionDays:      settings.GatewayDebugTimelineRetentionDays,
		GatewayDebugTimelineMaxSizeMB:          settings.GatewayDebugTimelineMaxSizeMB,
		GatewayDebugTimelineIncludeBody:        settings.GatewayDebugTimelineIncludeBody,
		GatewayDebugTimelineBodyMaxKB:          settings.GatewayDebugTimelineBodyMaxKB,
		EnableClaudeOAuthSystemPromptInjection: settings.EnableClaudeOAuthSystemPromptInjection,
		ClaudeOAuthSystemPrompt:                settings.ClaudeOAuthSystemPrompt,
		ClaudeOAuthSystemPromptBlocks:          settings.ClaudeOAuthSystemPromptBlocks,
		EnableAnthropicCacheTTL1hInjection:     settings.EnableAnthropicCacheTTL1hInjection,
		RewriteMessageCacheControl:             settings.RewriteMessageCacheControl,
		AntigravityUserAgentVersion:            settings.AntigravityUserAgentVersion,
		OpenAICodexUserAgent:                   settings.OpenAICodexUserAgent,

		// Anti-ban per platform
		AntiBanPlatforms:                          settings.AntiBanPlatforms,
		MinCodexVersion:                           settings.MinCodexVersion,
		MaxCodexVersion:                           settings.MaxCodexVersion,
		CodexCLIOnlyBlacklist:                     settings.CodexCLIOnlyBlacklist,
		CodexCLIOnlyWhitelist:                     settings.CodexCLIOnlyWhitelist,
		CodexCLIOnlyAllowAppServerClients:         settings.CodexCLIOnlyAllowAppServerClients,
		CodexCLIOnlyEngineFingerprintSignals:      settings.CodexCLIOnlyEngineFingerprintSignals,
		WebSearchEmulationEnabled:                 settings.WebSearchEmulationEnabled,
		KiroDefaultVersion:                        settings.KiroDefaultVersion,
		KiroDefaultCommit:                         settings.KiroDefaultCommit,
		KiroDefaultSystemVersion:                  settings.KiroDefaultSystemVersion,
		KiroDefaultNodeVersion:                    settings.KiroDefaultNodeVersion,
		KiroCacheHitRateScale:                     settings.KiroCacheHitRateScale,
		KiroCacheMinBlockTokens:                   settings.KiroCacheMinBlockTokens,
		KiroCacheIndependentTTLSeconds:            settings.KiroCacheIndependentTTLSeconds,
		KiroCachePrefixTTLSeconds:                 settings.KiroCachePrefixTTLSeconds,
		PaymentVisibleMethodAlipaySource:          settings.PaymentVisibleMethodAlipaySource,
		PaymentVisibleMethodWxpaySource:           settings.PaymentVisibleMethodWxpaySource,
		PaymentVisibleMethodAlipayEnabled:         settings.PaymentVisibleMethodAlipayEnabled,
		PaymentVisibleMethodWxpayEnabled:          settings.PaymentVisibleMethodWxpayEnabled,
		OpenAIAdvancedSchedulerEnabled:            settings.OpenAIAdvancedSchedulerEnabled,
		OpenAIStickyReservePercent:                settings.OpenAIStickyReservePercent,
		OpenAIStickyWaitTimeoutSeconds:            settings.OpenAIStickyWaitTimeoutSeconds,
		OpenAIWSMinIdlePerAccount:                 settings.OpenAIWSMinIdlePerAccount,
		OpenAIWSMaxIdlePerAccount:                 settings.OpenAIWSMaxIdlePerAccount,
		OpenAIWSNeutralPrewarmPercent:             settings.OpenAIWSNeutralPrewarmPercent,
		OpenAIWSSessionIdleTTLSeconds:             settings.OpenAIWSSessionIdleTTLSeconds,
		OpenAIOAuthImageBridgeDisableKeepAlives:   settings.OpenAIOAuthImageBridgeDisableKeepAlives,
		OpenAIOAuthImageBridgeFreshUpstreamClient: settings.OpenAIOAuthImageBridgeFreshUpstreamClient,
		BalanceLowNotifyEnabled:                   settings.BalanceLowNotifyEnabled,
		BalanceLowNotifyThreshold:                 settings.BalanceLowNotifyThreshold,
		BalanceLowNotifyRechargeURL:               settings.BalanceLowNotifyRechargeURL,
		SubscriptionExpiryNotifyEnabled:           settings.SubscriptionExpiryNotifyEnabled,
		AccountQuotaNotifyEnabled:                 settings.AccountQuotaNotifyEnabled,
		AccountQuotaNotifyEmails:                  dto.NotifyEmailEntriesFromService(settings.AccountQuotaNotifyEmails),
		PaymentEnabled:                            paymentCfg.Enabled,
		PaymentMinAmount:                          paymentCfg.MinAmount,
		PaymentMaxAmount:                          paymentCfg.MaxAmount,
		PaymentDailyLimit:                         paymentCfg.DailyLimit,
		PaymentOrderTimeoutMin:                    paymentCfg.OrderTimeoutMin,
		PaymentMaxPendingOrders:                   paymentCfg.MaxPendingOrders,
		PaymentEnabledTypes:                       paymentCfg.EnabledTypes,
		PaymentBalanceDisabled:                    paymentCfg.BalanceDisabled,
		PaymentBalanceRechargeMultiplier:          paymentCfg.BalanceRechargeMultiplier,
		PaymentRechargeFeeRate:                    paymentCfg.RechargeFeeRate,
		PaymentLoadBalanceStrat:                   paymentCfg.LoadBalanceStrategy,
		PaymentProductNamePrefix:                  paymentCfg.ProductNamePrefix,
		PaymentProductNameSuffix:                  paymentCfg.ProductNameSuffix,
		PaymentHelpImageURL:                       paymentCfg.HelpImageURL,
		PaymentHelpText:                           paymentCfg.HelpText,
		PaymentCancelRateLimitEnabled:             paymentCfg.CancelRateLimitEnabled,
		PaymentCancelRateLimitMax:                 paymentCfg.CancelRateLimitMax,
		PaymentCancelRateLimitWindow:              paymentCfg.CancelRateLimitWindow,
		PaymentCancelRateLimitUnit:                paymentCfg.CancelRateLimitUnit,
		PaymentCancelRateLimitMode:                paymentCfg.CancelRateLimitMode,
		PaymentAlipayForceQRCode:                  paymentCfg.AlipayForceQRCode,

		ChannelMonitorEnabled:                settings.ChannelMonitorEnabled,
		ChannelMonitorDefaultIntervalSeconds: settings.ChannelMonitorDefaultIntervalSeconds,

		AvailableChannelsEnabled:    settings.AvailableChannelsEnabled,
		AccountSchedulingThresholds: settings.AccountSchedulingThresholds,
	}

	// OpenAI fast policy (stored under a dedicated setting key)
	if fastPolicy, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context()); err != nil {
		slog.Error("openai_fast_policy_settings_get_failed", "error", err)
	} else if fastPolicy != nil {
		payload.OpenAIFastPolicySettings = openaiFastPolicySettingsToDTO(fastPolicy)
	}

	// Default platform quotas（JSON map）
	if platformQuotas, err := h.settingService.GetDefaultPlatformQuotas(c.Request.Context()); err != nil {
		slog.Error("default_platform_quotas_get_failed", "error", err)
	} else {
		payload.DefaultPlatformQuotas = platformQuotas
	}

	response.Success(c, systemSettingsResponseData(payload, authSourceDefaults))
}

// openaiFastPolicySettingsToDTO converts service -> dto for OpenAI fast policy.
func openaiFastPolicySettingsToDTO(s *service.OpenAIFastPolicySettings) *dto.OpenAIFastPolicySettings {
	if s == nil {
		return nil
	}
	rules := make([]dto.OpenAIFastPolicyRule, len(s.Rules))
	for i, r := range s.Rules {
		rules[i] = dto.OpenAIFastPolicyRule(r)
	}
	return &dto.OpenAIFastPolicySettings{Rules: rules}
}

// openaiFastPolicySettingsFromDTO converts dto -> service for OpenAI fast policy.
//
// 规范化 ServiceTier：在 DTO 进入 service 层之前统一把空字符串归一为
// service.OpenAIFastTierAny ("all")，避免管理员保存时空串与 "all" 同时
// 表达"匹配任意 tier"造成数据库取值的二义性。其它非空值原样透传，由
// service.SetOpenAIFastPolicySettings 负责合法值校验。
func openaiFastPolicySettingsFromDTO(s *dto.OpenAIFastPolicySettings) *service.OpenAIFastPolicySettings {
	if s == nil {
		return nil
	}
	rules := make([]service.OpenAIFastPolicyRule, len(s.Rules))
	for i, r := range s.Rules {
		rules[i] = service.OpenAIFastPolicyRule(r)
		tier := strings.ToLower(strings.TrimSpace(rules[i].ServiceTier))
		if tier == "" {
			tier = service.OpenAIFastTierAny
		}
		rules[i].ServiceTier = tier
	}
	return &service.OpenAIFastPolicySettings{Rules: rules}
}

func loginAgreementDocumentsToDTO(items []service.LoginAgreementDocument) []dto.LoginAgreementDocument {
	result := make([]dto.LoginAgreementDocument, 0, len(items))
	for _, item := range items {
		result = append(result, dto.LoginAgreementDocument{
			ID:        item.ID,
			Title:     item.Title,
			ContentMD: item.ContentMD,
		})
	}
	return result
}

func loginAgreementDocumentsToService(items []dto.LoginAgreementDocument) []service.LoginAgreementDocument {
	result := make([]service.LoginAgreementDocument, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		content := strings.TrimSpace(item.ContentMD)
		if title == "" && content == "" {
			continue
		}
		result = append(result, service.LoginAgreementDocument{
			ID:        strings.TrimSpace(item.ID),
			Title:     title,
			ContentMD: content,
		})
	}
	return result
}

// UpdateSettingsRequest 更新设置请求
type UpdateSettingsRequest struct {
	// 注册设置
	RegistrationEnabled              bool                         `json:"registration_enabled"`
	EmailVerifyEnabled               bool                         `json:"email_verify_enabled"`
	RegistrationEmailSuffixWhitelist []string                     `json:"registration_email_suffix_whitelist"`
	PromoCodeEnabled                 bool                         `json:"promo_code_enabled"`
	PasswordResetEnabled             bool                         `json:"password_reset_enabled"`
	FrontendURL                      string                       `json:"frontend_url"`
	InvitationCodeEnabled            bool                         `json:"invitation_code_enabled"`
	TotpEnabled                      bool                         `json:"totp_enabled"` // TOTP 双因素认证
	LoginAgreementEnabled            bool                         `json:"login_agreement_enabled"`
	LoginAgreementMode               string                       `json:"login_agreement_mode"`
	LoginAgreementUpdatedAt          string                       `json:"login_agreement_updated_at"`
	LoginAgreementDocuments          []dto.LoginAgreementDocument `json:"login_agreement_documents"`

	// 邮件服务设置
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`

	// Cloudflare Turnstile 设置
	TurnstileEnabled          bool   `json:"turnstile_enabled"`
	TurnstileSiteKey          string `json:"turnstile_site_key"`
	TurnstileSecretKey        string `json:"turnstile_secret_key"`
	APIKeyACLTrustForwardedIP *bool  `json:"api_key_acl_trust_forwarded_ip"`

	// LinuxDo Connect OAuth 登录
	LinuxDoConnectEnabled      bool   `json:"linuxdo_connect_enabled"`
	LinuxDoConnectClientID     string `json:"linuxdo_connect_client_id"`
	LinuxDoConnectClientSecret string `json:"linuxdo_connect_client_secret"`
	LinuxDoConnectRedirectURL  string `json:"linuxdo_connect_redirect_url"`

	// DingTalk Connect OAuth 登录
	DingTalkConnectEnabled                 bool   `json:"dingtalk_connect_enabled"`
	DingTalkConnectClientID                string `json:"dingtalk_connect_client_id"`
	DingTalkConnectClientSecret            string `json:"dingtalk_connect_client_secret"`
	DingTalkConnectRedirectURL             string `json:"dingtalk_connect_redirect_url"`
	DingTalkConnectCorpRestrictionPolicy   string `json:"dingtalk_connect_corp_restriction_policy"`
	DingTalkConnectInternalCorpID          string `json:"dingtalk_connect_internal_corp_id"`
	DingTalkConnectBypassRegistration      bool   `json:"dingtalk_connect_bypass_registration"`
	DingTalkConnectSyncCorpEmail           bool   `json:"dingtalk_connect_sync_corp_email"`
	DingTalkConnectSyncDisplayName         bool   `json:"dingtalk_connect_sync_display_name"`
	DingTalkConnectSyncDept                bool   `json:"dingtalk_connect_sync_dept"`
	DingTalkConnectSyncCorpEmailAttrKey    string `json:"dingtalk_connect_sync_corp_email_attr_key"`
	DingTalkConnectSyncDisplayNameAttrKey  string `json:"dingtalk_connect_sync_display_name_attr_key"`
	DingTalkConnectSyncDeptAttrKey         string `json:"dingtalk_connect_sync_dept_attr_key"`
	DingTalkConnectSyncCorpEmailAttrName   string `json:"dingtalk_connect_sync_corp_email_attr_name"`
	DingTalkConnectSyncDisplayNameAttrName string `json:"dingtalk_connect_sync_display_name_attr_name"`
	DingTalkConnectSyncDeptAttrName        string `json:"dingtalk_connect_sync_dept_attr_name"`

	// WeChat Connect OAuth 登录
	WeChatConnectEnabled             bool   `json:"wechat_connect_enabled"`
	WeChatConnectAppID               string `json:"wechat_connect_app_id"`
	WeChatConnectAppSecret           string `json:"wechat_connect_app_secret"`
	WeChatConnectOpenAppID           string `json:"wechat_connect_open_app_id"`
	WeChatConnectOpenAppSecret       string `json:"wechat_connect_open_app_secret"`
	WeChatConnectMPAppID             string `json:"wechat_connect_mp_app_id"`
	WeChatConnectMPAppSecret         string `json:"wechat_connect_mp_app_secret"`
	WeChatConnectMobileAppID         string `json:"wechat_connect_mobile_app_id"`
	WeChatConnectMobileAppSecret     string `json:"wechat_connect_mobile_app_secret"`
	WeChatConnectOpenEnabled         bool   `json:"wechat_connect_open_enabled"`
	WeChatConnectMPEnabled           bool   `json:"wechat_connect_mp_enabled"`
	WeChatConnectMobileEnabled       bool   `json:"wechat_connect_mobile_enabled"`
	WeChatConnectMode                string `json:"wechat_connect_mode"`
	WeChatConnectScopes              string `json:"wechat_connect_scopes"`
	WeChatConnectRedirectURL         string `json:"wechat_connect_redirect_url"`
	WeChatConnectFrontendRedirectURL string `json:"wechat_connect_frontend_redirect_url"`

	// Generic OIDC OAuth 登录
	OIDCConnectEnabled              bool   `json:"oidc_connect_enabled"`
	OIDCConnectProviderName         string `json:"oidc_connect_provider_name"`
	OIDCConnectClientID             string `json:"oidc_connect_client_id"`
	OIDCConnectClientSecret         string `json:"oidc_connect_client_secret"`
	OIDCConnectIssuerURL            string `json:"oidc_connect_issuer_url"`
	OIDCConnectDiscoveryURL         string `json:"oidc_connect_discovery_url"`
	OIDCConnectAuthorizeURL         string `json:"oidc_connect_authorize_url"`
	OIDCConnectTokenURL             string `json:"oidc_connect_token_url"`
	OIDCConnectUserInfoURL          string `json:"oidc_connect_userinfo_url"`
	OIDCConnectJWKSURL              string `json:"oidc_connect_jwks_url"`
	OIDCConnectScopes               string `json:"oidc_connect_scopes"`
	OIDCConnectRedirectURL          string `json:"oidc_connect_redirect_url"`
	OIDCConnectFrontendRedirectURL  string `json:"oidc_connect_frontend_redirect_url"`
	OIDCConnectTokenAuthMethod      string `json:"oidc_connect_token_auth_method"`
	OIDCConnectUsePKCE              *bool  `json:"oidc_connect_use_pkce"`
	OIDCConnectValidateIDToken      *bool  `json:"oidc_connect_validate_id_token"`
	OIDCConnectAllowedSigningAlgs   string `json:"oidc_connect_allowed_signing_algs"`
	OIDCConnectClockSkewSeconds     int    `json:"oidc_connect_clock_skew_seconds"`
	OIDCConnectRequireEmailVerified bool   `json:"oidc_connect_require_email_verified"`
	OIDCConnectUserInfoEmailPath    string `json:"oidc_connect_userinfo_email_path"`
	OIDCConnectUserInfoIDPath       string `json:"oidc_connect_userinfo_id_path"`
	OIDCConnectUserInfoUsernamePath string `json:"oidc_connect_userinfo_username_path"`

	GitHubOAuthEnabled             bool   `json:"github_oauth_enabled"`
	GitHubOAuthClientID            string `json:"github_oauth_client_id"`
	GitHubOAuthClientSecret        string `json:"github_oauth_client_secret"`
	GitHubOAuthRedirectURL         string `json:"github_oauth_redirect_url"`
	GitHubOAuthFrontendRedirectURL string `json:"github_oauth_frontend_redirect_url"`
	GoogleOAuthEnabled             bool   `json:"google_oauth_enabled"`
	GoogleOAuthClientID            string `json:"google_oauth_client_id"`
	GoogleOAuthClientSecret        string `json:"google_oauth_client_secret"`
	GoogleOAuthRedirectURL         string `json:"google_oauth_redirect_url"`
	GoogleOAuthFrontendRedirectURL string `json:"google_oauth_frontend_redirect_url"`

	// OEM设置
	SiteName                    string                    `json:"site_name"`
	SiteLogo                    string                    `json:"site_logo"`
	SiteSubtitle                string                    `json:"site_subtitle"`
	APIBaseURL                  string                    `json:"api_base_url"`
	ContactInfo                 string                    `json:"contact_info"`
	SupportQRCodes              *[]dto.SupportQRCodeEntry `json:"support_qr_codes"`
	DocURL                      string                    `json:"doc_url"`
	HomeContent                 string                    `json:"home_content"`
	HideCcsImportButton         bool                      `json:"hide_ccs_import_button"`
	PurchaseSubscriptionEnabled *bool                     `json:"purchase_subscription_enabled"`
	PurchaseSubscriptionURL     *string                   `json:"purchase_subscription_url"`
	TableDefaultPageSize        int                       `json:"table_default_page_size"`
	TablePageSizeOptions        []int                     `json:"table_page_size_options"`
	CustomMenuItems             *[]dto.CustomMenuItem     `json:"custom_menu_items"`
	CustomEndpoints             *[]dto.CustomEndpoint     `json:"custom_endpoints"`

	// 默认配置
	DefaultConcurrency                        int                               `json:"default_concurrency"`
	DefaultBalance                            float64                           `json:"default_balance"`
	AffiliateEnabled                          *bool                             `json:"affiliate_enabled"`
	AffiliateRebateRate                       *float64                          `json:"affiliate_rebate_rate"`
	AffiliateRebateCap                        *float64                          `json:"affiliate_rebate_cap"`
	AffiliateRebateInviteeLimit               *int                              `json:"affiliate_rebate_invitee_limit"`
	AffiliateSignupBonus                      *float64                          `json:"affiliate_signup_bonus"`
	TicketEnabled                             *bool                             `json:"ticket_enabled"`
	AffiliateRebateFreezeHours                *int                              `json:"affiliate_rebate_freeze_hours"`
	AffiliateRebateDurationDays               *int                              `json:"affiliate_rebate_duration_days"`
	AffiliateRebatePerInviteeCap              *float64                          `json:"affiliate_rebate_per_invitee_cap"`
	DefaultUserRPMLimit                       int                               `json:"default_user_rpm_limit"`
	DefaultSubscriptions                      []dto.DefaultSubscriptionSetting  `json:"default_subscriptions"`
	AuthSourceDefaultEmailBalance             *float64                          `json:"auth_source_default_email_balance"`
	AuthSourceDefaultEmailConcurrency         *int                              `json:"auth_source_default_email_concurrency"`
	AuthSourceDefaultEmailSubscriptions       *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_email_subscriptions"`
	AuthSourceDefaultEmailGrantOnSignup       *bool                             `json:"auth_source_default_email_grant_on_signup"`
	AuthSourceDefaultEmailGrantOnFirstBind    *bool                             `json:"auth_source_default_email_grant_on_first_bind"`
	AuthSourceDefaultLinuxDoBalance           *float64                          `json:"auth_source_default_linuxdo_balance"`
	AuthSourceDefaultLinuxDoConcurrency       *int                              `json:"auth_source_default_linuxdo_concurrency"`
	AuthSourceDefaultLinuxDoSubscriptions     *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_linuxdo_subscriptions"`
	AuthSourceDefaultLinuxDoGrantOnSignup     *bool                             `json:"auth_source_default_linuxdo_grant_on_signup"`
	AuthSourceDefaultLinuxDoGrantOnFirstBind  *bool                             `json:"auth_source_default_linuxdo_grant_on_first_bind"`
	AuthSourceDefaultOIDCBalance              *float64                          `json:"auth_source_default_oidc_balance"`
	AuthSourceDefaultOIDCConcurrency          *int                              `json:"auth_source_default_oidc_concurrency"`
	AuthSourceDefaultOIDCSubscriptions        *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_oidc_subscriptions"`
	AuthSourceDefaultOIDCGrantOnSignup        *bool                             `json:"auth_source_default_oidc_grant_on_signup"`
	AuthSourceDefaultOIDCGrantOnFirstBind     *bool                             `json:"auth_source_default_oidc_grant_on_first_bind"`
	AuthSourceDefaultWeChatBalance            *float64                          `json:"auth_source_default_wechat_balance"`
	AuthSourceDefaultWeChatConcurrency        *int                              `json:"auth_source_default_wechat_concurrency"`
	AuthSourceDefaultWeChatSubscriptions      *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_wechat_subscriptions"`
	AuthSourceDefaultWeChatGrantOnSignup      *bool                             `json:"auth_source_default_wechat_grant_on_signup"`
	AuthSourceDefaultWeChatGrantOnFirstBind   *bool                             `json:"auth_source_default_wechat_grant_on_first_bind"`
	AuthSourceDefaultGitHubBalance            *float64                          `json:"auth_source_default_github_balance"`
	AuthSourceDefaultGitHubConcurrency        *int                              `json:"auth_source_default_github_concurrency"`
	AuthSourceDefaultGitHubSubscriptions      *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_github_subscriptions"`
	AuthSourceDefaultGitHubGrantOnSignup      *bool                             `json:"auth_source_default_github_grant_on_signup"`
	AuthSourceDefaultGitHubGrantOnFirstBind   *bool                             `json:"auth_source_default_github_grant_on_first_bind"`
	AuthSourceDefaultGoogleBalance            *float64                          `json:"auth_source_default_google_balance"`
	AuthSourceDefaultGoogleConcurrency        *int                              `json:"auth_source_default_google_concurrency"`
	AuthSourceDefaultGoogleSubscriptions      *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_google_subscriptions"`
	AuthSourceDefaultGoogleGrantOnSignup      *bool                             `json:"auth_source_default_google_grant_on_signup"`
	AuthSourceDefaultGoogleGrantOnFirstBind   *bool                             `json:"auth_source_default_google_grant_on_first_bind"`
	AuthSourceDefaultDingTalkBalance          *float64                          `json:"auth_source_default_dingtalk_balance"`
	AuthSourceDefaultDingTalkConcurrency      *int                              `json:"auth_source_default_dingtalk_concurrency"`
	AuthSourceDefaultDingTalkSubscriptions    *[]dto.DefaultSubscriptionSetting `json:"auth_source_default_dingtalk_subscriptions"`
	AuthSourceDefaultDingTalkGrantOnSignup    *bool                             `json:"auth_source_default_dingtalk_grant_on_signup"`
	AuthSourceDefaultDingTalkGrantOnFirstBind *bool                             `json:"auth_source_default_dingtalk_grant_on_first_bind"`
	ForceEmailOnThirdPartySignup              *bool                             `json:"force_email_on_third_party_signup"`

	// Model fallback configuration
	EnableModelFallback               bool                                      `json:"enable_model_fallback"`
	FallbackModelAnthropic            string                                    `json:"fallback_model_anthropic"`
	FallbackModelOpenAI               string                                    `json:"fallback_model_openai"`
	FallbackModelGemini               string                                    `json:"fallback_model_gemini"`
	FallbackModelAntigravity          string                                    `json:"fallback_model_antigravity"`
	PlatformDefaultAccountModelConfig *map[string]dto.DefaultAccountModelConfig `json:"platform_default_account_model_config"`

	// Identity patch configuration (Claude -> Gemini)
	EnableIdentityPatch bool   `json:"enable_identity_patch"`
	IdentityPatchPrompt string `json:"identity_patch_prompt"`

	// Ops monitoring (vNext)
	OpsMonitoringEnabled         *bool   `json:"ops_monitoring_enabled"`
	OpsRealtimeMonitoringEnabled *bool   `json:"ops_realtime_monitoring_enabled"`
	OpsQueryModeDefault          *string `json:"ops_query_mode_default"`
	OpsMetricsIntervalSeconds    *int    `json:"ops_metrics_interval_seconds"`

	MinClaudeCodeVersion string `json:"min_claude_code_version"`
	MaxClaudeCodeVersion string `json:"max_claude_code_version"`

	// 分组隔离
	AllowUngroupedKeyScheduling bool `json:"allow_ungrouped_key_scheduling"`

	// Backend Mode
	BackendModeEnabled bool `json:"backend_mode_enabled"`

	// Gateway forwarding behavior
	EnableFingerprintUnification           *bool   `json:"enable_fingerprint_unification"`
	EnableMetadataPassthrough              *bool   `json:"enable_metadata_passthrough"`
	ClaudeTelemetryMode                    *string `json:"claude_telemetry_mode"`
	GatewayDebugTimelineEnabled            *bool   `json:"gateway_debug_timeline_enabled"`
	GatewayDebugTimelineDirectory          *string `json:"gateway_debug_timeline_directory"`
	GatewayDebugTimelineRetentionDays      *int    `json:"gateway_debug_timeline_retention_days"`
	GatewayDebugTimelineMaxSizeMB          *int64  `json:"gateway_debug_timeline_max_size_mb"`
	GatewayDebugTimelineIncludeBody        *bool   `json:"gateway_debug_timeline_include_body"`
	GatewayDebugTimelineBodyMaxKB          *int    `json:"gateway_debug_timeline_body_max_kb"`
	EnableClaudeOAuthSystemPromptInjection *bool   `json:"enable_claude_oauth_system_prompt_injection"`
	ClaudeOAuthSystemPrompt                *string `json:"claude_oauth_system_prompt"`
	ClaudeOAuthSystemPromptBlocks          *string `json:"claude_oauth_system_prompt_blocks"`
	EnableAnthropicCacheTTL1hInjection     *bool   `json:"enable_anthropic_cache_ttl_1h_injection"`
	RewriteMessageCacheControl             *bool   `json:"rewrite_message_cache_control"`
	AntigravityUserAgentVersion            *string `json:"antigravity_user_agent_version"`
	OpenAICodexUserAgent                   *string `json:"openai_codex_user_agent"`

	// Anti-ban / 防封控 per-platform toggles (anti-fingerprint normalizer, TLS spoofing suite etc.)
	// Keyed by platform name (anthropic, openai, gemini, grok, kiro, antigravity...)
	AntiBanPlatforms map[string]bool `json:"anti_ban_platforms"`

	// codex_cli_only 加固（global-only）
	MinCodexVersion                      string `json:"min_codex_version"`
	MaxCodexVersion                      string `json:"max_codex_version"`
	CodexCLIOnlyBlacklist                string `json:"codex_cli_only_blacklist"`
	CodexCLIOnlyWhitelist                string `json:"codex_cli_only_whitelist"`
	CodexCLIOnlyAllowAppServerClients    *bool  `json:"codex_cli_only_allow_app_server_clients"`
	CodexCLIOnlyEngineFingerprintSignals string `json:"codex_cli_only_engine_fingerprint_signals"`

	// Kiro runtime defaults
	KiroDefaultVersion             *string `json:"kiro_version"`
	KiroDefaultCommit              *string `json:"kiro_commit"`
	KiroDefaultSystemVersion       *string `json:"system_version"`
	KiroDefaultNodeVersion         *string `json:"node_version"`
	KiroCacheHitRateScale          *int    `json:"cache_hit_rate_scale"`
	KiroCacheMinBlockTokens        *int    `json:"cache_min_block_tokens"`
	KiroCacheIndependentTTLSeconds *int    `json:"cache_independent_ttl_seconds"`
	KiroCachePrefixTTLSeconds      *int    `json:"cache_prefix_ttl_seconds"`

	// Payment visible method routing
	PaymentVisibleMethodAlipaySource  *string `json:"payment_visible_method_alipay_source"`
	PaymentVisibleMethodWxpaySource   *string `json:"payment_visible_method_wxpay_source"`
	PaymentVisibleMethodAlipayEnabled *bool   `json:"payment_visible_method_alipay_enabled"`
	PaymentVisibleMethodWxpayEnabled  *bool   `json:"payment_visible_method_wxpay_enabled"`

	// OpenAI account scheduling
	OpenAIAdvancedSchedulerEnabled            *bool `json:"openai_advanced_scheduler_enabled"`
	OpenAIStickyReservePercent                *int  `json:"openai_sticky_reserve_percent"`
	OpenAIStickyWaitTimeoutSeconds            *int  `json:"openai_sticky_wait_timeout_seconds"`
	OpenAIWSMinIdlePerAccount                 *int  `json:"openai_ws_min_idle_per_account"`
	OpenAIWSMaxIdlePerAccount                 *int  `json:"openai_ws_max_idle_per_account"`
	OpenAIWSNeutralPrewarmPercent             *int  `json:"openai_ws_neutral_prewarm_percent"`
	OpenAIWSSessionIdleTTLSeconds             *int  `json:"openai_ws_session_idle_ttl_seconds"`
	OpenAIOAuthImageBridgeDisableKeepAlives   *bool `json:"openai_oauth_image_bridge_disable_keepalives"`
	OpenAIOAuthImageBridgeFreshUpstreamClient *bool `json:"openai_oauth_image_bridge_fresh_upstream_client"`

	// Balance low notification
	BalanceLowNotifyEnabled         *bool                   `json:"balance_low_notify_enabled"`
	BalanceLowNotifyThreshold       *float64                `json:"balance_low_notify_threshold"`
	BalanceLowNotifyRechargeURL     *string                 `json:"balance_low_notify_recharge_url"`
	SubscriptionExpiryNotifyEnabled *bool                   `json:"subscription_expiry_notify_enabled"`
	AccountQuotaNotifyEnabled       *bool                   `json:"account_quota_notify_enabled"`
	AccountQuotaNotifyEmails        *[]dto.NotifyEmailEntry `json:"account_quota_notify_emails"`

	// Payment configuration (integrated into settings, full replace)
	PaymentEnabled                   *bool    `json:"payment_enabled"`
	PaymentMinAmount                 *float64 `json:"payment_min_amount"`
	PaymentMaxAmount                 *float64 `json:"payment_max_amount"`
	PaymentDailyLimit                *float64 `json:"payment_daily_limit"`
	PaymentOrderTimeoutMin           *int     `json:"payment_order_timeout_minutes"`
	PaymentMaxPendingOrders          *int     `json:"payment_max_pending_orders"`
	PaymentEnabledTypes              []string `json:"payment_enabled_types"`
	PaymentBalanceDisabled           *bool    `json:"payment_balance_disabled"`
	PaymentBalanceRechargeMultiplier *float64 `json:"payment_balance_recharge_multiplier"`
	PaymentRechargeFeeRate           *float64 `json:"payment_recharge_fee_rate"`
	PaymentLoadBalanceStrat          *string  `json:"payment_load_balance_strategy"`
	PaymentProductNamePrefix         *string  `json:"payment_product_name_prefix"`
	PaymentProductNameSuffix         *string  `json:"payment_product_name_suffix"`
	PaymentHelpImageURL              *string  `json:"payment_help_image_url"`
	PaymentHelpText                  *string  `json:"payment_help_text"`

	// Cancel rate limit
	PaymentCancelRateLimitEnabled *bool   `json:"payment_cancel_rate_limit_enabled"`
	PaymentCancelRateLimitMax     *int    `json:"payment_cancel_rate_limit_max"`
	PaymentCancelRateLimitWindow  *int    `json:"payment_cancel_rate_limit_window"`
	PaymentCancelRateLimitUnit    *string `json:"payment_cancel_rate_limit_unit"`
	PaymentCancelRateLimitMode    *string `json:"payment_cancel_rate_limit_window_mode"`
	PaymentAlipayForceQRCode      *bool   `json:"payment_alipay_force_qrcode"`

	// Channel Monitor feature switch
	AIStudioEnabled                      *bool `json:"ai_studio_enabled"`
	ChannelMonitorEnabled                *bool `json:"channel_monitor_enabled"`
	ChannelMonitorDefaultIntervalSeconds *int  `json:"channel_monitor_default_interval_seconds"`

	// Available Channels feature switch (user-facing)
	AvailableChannelsEnabled *bool `json:"available_channels_enabled"`

	// 风控中心功能开关
	RiskControlEnabled *bool `json:"risk_control_enabled"`

	// cyber 会话屏蔽开关 + TTL
	CyberSessionBlockEnabled    *bool `json:"cyber_session_block_enabled"`
	CyberSessionBlockTTLSeconds *int  `json:"cyber_session_block_ttl_seconds"`

	// OpenAI fast/flex policy (optional, only updated when provided)
	OpenAIFastPolicySettings *dto.OpenAIFastPolicySettings `json:"openai_fast_policy_settings,omitempty"`

	// 系统全局 platform quota 默认值（整体替换语义：nil = 不修改，non-nil = 整体覆盖）。
	DefaultPlatformQuotas map[string]*service.DefaultPlatformQuotaSetting `json:"default_platform_quotas"`

	// 各平台账号自动停调阈值（整体替换语义：nil = 不修改，non-nil = 整体覆盖）。
	AccountSchedulingThresholds map[string]int `json:"account_scheduling_thresholds"`

	// auth-source 层 platform quota 覆盖（override 语义：nil = 不修改，non-nil = 整体覆盖该 source 的 quota 配置）。
	AuthSourceEmailPlatformQuotas    map[string]*service.DefaultPlatformQuotaSetting `json:"auth_source_default_email_platform_quotas"`
	AuthSourceLinuxDoPlatformQuotas  map[string]*service.DefaultPlatformQuotaSetting `json:"auth_source_default_linuxdo_platform_quotas"`
	AuthSourceOIDCPlatformQuotas     map[string]*service.DefaultPlatformQuotaSetting `json:"auth_source_default_oidc_platform_quotas"`
	AuthSourceWeChatPlatformQuotas   map[string]*service.DefaultPlatformQuotaSetting `json:"auth_source_default_wechat_platform_quotas"`
	AuthSourceGitHubPlatformQuotas   map[string]*service.DefaultPlatformQuotaSetting `json:"auth_source_default_github_platform_quotas"`
	AuthSourceGooglePlatformQuotas   map[string]*service.DefaultPlatformQuotaSetting `json:"auth_source_default_google_platform_quotas"`
	AuthSourceDingTalkPlatformQuotas map[string]*service.DefaultPlatformQuotaSetting `json:"auth_source_default_dingtalk_platform_quotas"`

	AllowUserViewErrorRequests *bool `json:"allow_user_view_error_requests"`
}

func toDTODefaultAccountModelConfig(src map[string]service.DefaultAccountModelConfig) map[string]dto.DefaultAccountModelConfig {
	if len(src) == 0 {
		return map[string]dto.DefaultAccountModelConfig{}
	}
	out := make(map[string]dto.DefaultAccountModelConfig, len(src))
	for platform, cfg := range src {
		out[platform] = toDTODefaultAccountModelConfigEntry(cfg)
	}
	return out
}

func toDTODefaultAccountModelConfigEntry(cfg service.DefaultAccountModelConfig) dto.DefaultAccountModelConfig {
	out := dto.DefaultAccountModelConfig{
		ModelWhitelist:           append([]string(nil), cfg.ModelWhitelist...),
		ModelMapping:             copyStringMapForSettingsDTO(cfg.ModelMapping),
		CompactModelMapping:      copyStringMapForSettingsDTO(cfg.CompactModelMapping),
		TempUnschedulableEnabled: cfg.TempUnschedulableEnabled,
		TempUnschedulableRules:   append([]service.TempUnschedulableRule(nil), cfg.TempUnschedulableRules...),
		CustomErrorCodesEnabled:  cfg.CustomErrorCodesEnabled,
		CustomErrorCodes:         append([]int(nil), cfg.CustomErrorCodes...),
	}
	if len(cfg.KiroSubscriptionTypeModelMap) > 0 {
		nested := make(map[string]dto.DefaultAccountModelConfig, len(cfg.KiroSubscriptionTypeModelMap))
		for key, variant := range cfg.KiroSubscriptionTypeModelMap {
			nested[key] = toDTODefaultAccountModelConfigEntry(variant)
		}
		out.KiroSubscriptionTypeModelMap = nested
	}
	return out
}

func fromDTODefaultAccountModelConfig(src map[string]dto.DefaultAccountModelConfig) map[string]service.DefaultAccountModelConfig {
	if len(src) == 0 {
		return map[string]service.DefaultAccountModelConfig{}
	}
	out := make(map[string]service.DefaultAccountModelConfig, len(src))
	for platform, cfg := range src {
		out[platform] = fromDTODefaultAccountModelConfigEntry(cfg)
	}
	return out
}

func fromDTODefaultAccountModelConfigEntry(cfg dto.DefaultAccountModelConfig) service.DefaultAccountModelConfig {
	out := service.DefaultAccountModelConfig{
		ModelWhitelist:           append([]string(nil), cfg.ModelWhitelist...),
		ModelMapping:             copyStringMapForSettingsDTO(cfg.ModelMapping),
		CompactModelMapping:      copyStringMapForSettingsDTO(cfg.CompactModelMapping),
		TempUnschedulableEnabled: cfg.TempUnschedulableEnabled,
		TempUnschedulableRules:   append([]service.TempUnschedulableRule(nil), cfg.TempUnschedulableRules...),
		CustomErrorCodesEnabled:  cfg.CustomErrorCodesEnabled,
		CustomErrorCodes:         append([]int(nil), cfg.CustomErrorCodes...),
	}
	if len(cfg.KiroSubscriptionTypeModelMap) > 0 {
		nested := make(map[string]service.DefaultAccountModelConfig, len(cfg.KiroSubscriptionTypeModelMap))
		for key, variant := range cfg.KiroSubscriptionTypeModelMap {
			nested[key] = fromDTODefaultAccountModelConfigEntry(variant)
		}
		out.KiroSubscriptionTypeModelMap = nested
	}
	return out
}

func fromOptionalDTODefaultAccountModelConfig(
	src *map[string]dto.DefaultAccountModelConfig,
	fallback map[string]service.DefaultAccountModelConfig,
) map[string]service.DefaultAccountModelConfig {
	if src == nil {
		return fallback
	}
	return fromDTODefaultAccountModelConfig(*src)
}

func copyStringMapForSettingsDTO(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func defaultAccountModelConfigEqual(a, b map[string]service.DefaultAccountModelConfig) bool {
	aData, _ := json.Marshal(a)
	bData, _ := json.Marshal(b)
	return string(aData) == string(bData)
}

// UpdateSettings 更新系统设置
// PUT /api/v1/admin/settings
func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.BadRequest(c, "Invalid request: unable to read request body")
		return
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &rawFields); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(rawBody))

	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	_, hasSiteLogoField := rawFields["site_logo"]
	previousSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	preserveOmittedSettingsFields(rawFields, &req, previousSettings)
	previousAuthSourceDefaults, err := h.settingService.GetAuthSourceDefaultSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	previousFastPolicy, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var previousPaymentCfg *service.PaymentConfig
	if h.paymentConfigService != nil && hasPaymentFields(req) {
		previousPaymentCfg, err = h.paymentConfigService.GetPaymentConfig(c.Request.Context())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
	}

	// 验证参数
	if req.DefaultConcurrency < 1 {
		req.DefaultConcurrency = 1
	}
	if req.DefaultBalance < 0 {
		req.DefaultBalance = 0
	}
	affiliateRebateRate := previousSettings.AffiliateRebateRate
	if req.AffiliateRebateRate != nil {
		affiliateRebateRate = *req.AffiliateRebateRate
	}
	if affiliateRebateRate < service.AffiliateRebateRateMin {
		affiliateRebateRate = service.AffiliateRebateRateMin
	}
	if affiliateRebateRate > service.AffiliateRebateRateMax {
		affiliateRebateRate = service.AffiliateRebateRateMax
	}
	affiliateRebateCap := previousSettings.AffiliateRebateCap
	if req.AffiliateRebateCap != nil {
		affiliateRebateCap = *req.AffiliateRebateCap
	}
	if affiliateRebateCap < 0 {
		affiliateRebateCap = 0
	}
	affiliateRebateInviteeLimit := previousSettings.AffiliateRebateInviteeLimit
	if req.AffiliateRebateInviteeLimit != nil {
		affiliateRebateInviteeLimit = *req.AffiliateRebateInviteeLimit
	}
	if affiliateRebateInviteeLimit < 0 {
		affiliateRebateInviteeLimit = 0
	}
	affiliateSignupBonus := previousSettings.AffiliateSignupBonus
	if req.AffiliateSignupBonus != nil {
		affiliateSignupBonus = *req.AffiliateSignupBonus
	}
	if affiliateSignupBonus < 0 {
		affiliateSignupBonus = 0
	}
	affiliateRebateFreezeHours := previousSettings.AffiliateRebateFreezeHours
	if req.AffiliateRebateFreezeHours != nil {
		affiliateRebateFreezeHours = *req.AffiliateRebateFreezeHours
	}
	if affiliateRebateFreezeHours < 0 {
		affiliateRebateFreezeHours = service.AffiliateRebateFreezeHoursDefault
	}
	if affiliateRebateFreezeHours > service.AffiliateRebateFreezeHoursMax {
		affiliateRebateFreezeHours = service.AffiliateRebateFreezeHoursMax
	}
	affiliateRebateDurationDays := previousSettings.AffiliateRebateDurationDays
	if req.AffiliateRebateDurationDays != nil {
		affiliateRebateDurationDays = *req.AffiliateRebateDurationDays
	}
	if affiliateRebateDurationDays < 0 {
		affiliateRebateDurationDays = service.AffiliateRebateDurationDaysDefault
	}
	if affiliateRebateDurationDays > service.AffiliateRebateDurationDaysMax {
		affiliateRebateDurationDays = service.AffiliateRebateDurationDaysMax
	}
	affiliateRebatePerInviteeCap := previousSettings.AffiliateRebatePerInviteeCap
	if req.AffiliateRebatePerInviteeCap != nil {
		affiliateRebatePerInviteeCap = *req.AffiliateRebatePerInviteeCap
	}
	if affiliateRebatePerInviteeCap < 0 {
		affiliateRebatePerInviteeCap = service.AffiliateRebatePerInviteeCapDefault
	}
	// 通用表格配置：兼容旧客户端未传字段时保留当前值。
	if req.TableDefaultPageSize <= 0 {
		req.TableDefaultPageSize = previousSettings.TableDefaultPageSize
	}
	if req.TablePageSizeOptions == nil {
		req.TablePageSizeOptions = previousSettings.TablePageSizeOptions
	}
	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)
	req.SMTPPassword = strings.TrimSpace(req.SMTPPassword)
	req.SMTPFrom = strings.TrimSpace(req.SMTPFrom)
	req.SMTPFromName = strings.TrimSpace(req.SMTPFromName)
	if req.SMTPPort <= 0 {
		req.SMTPPort = 587
	}
	req.DefaultSubscriptions = normalizeDefaultSubscriptions(req.DefaultSubscriptions)
	req.AuthSourceDefaultEmailSubscriptions = normalizeOptionalDefaultSubscriptions(req.AuthSourceDefaultEmailSubscriptions)
	req.AuthSourceDefaultLinuxDoSubscriptions = normalizeOptionalDefaultSubscriptions(req.AuthSourceDefaultLinuxDoSubscriptions)
	req.AuthSourceDefaultOIDCSubscriptions = normalizeOptionalDefaultSubscriptions(req.AuthSourceDefaultOIDCSubscriptions)
	req.AuthSourceDefaultWeChatSubscriptions = normalizeOptionalDefaultSubscriptions(req.AuthSourceDefaultWeChatSubscriptions)
	req.AuthSourceDefaultDingTalkSubscriptions = normalizeOptionalDefaultSubscriptions(req.AuthSourceDefaultDingTalkSubscriptions)
	if req.KiroCacheHitRateScale != nil && (*req.KiroCacheHitRateScale < 0 || *req.KiroCacheHitRateScale > 100) {
		response.BadRequest(c, "Kiro cache hit rate scale must be between 0 and 100")
		return
	}
	if req.KiroCacheMinBlockTokens != nil &&
		(*req.KiroCacheMinBlockTokens < 0 || *req.KiroCacheMinBlockTokens > service.KiroCacheMinBlockTokensMax) {
		response.BadRequest(c, fmt.Sprintf("Kiro cache min block tokens must be between 0 and %d", service.KiroCacheMinBlockTokensMax))
		return
	}
	if req.KiroCacheIndependentTTLSeconds != nil &&
		(*req.KiroCacheIndependentTTLSeconds < 60 || *req.KiroCacheIndependentTTLSeconds > 86400) {
		response.BadRequest(c, "Kiro independent TTL must be between 60 and 86400 seconds")
		return
	}
	if req.KiroCachePrefixTTLSeconds != nil &&
		(*req.KiroCachePrefixTTLSeconds < 60 || *req.KiroCachePrefixTTLSeconds > 3600) {
		response.BadRequest(c, "Kiro prefix TTL must be between 60 and 3600 seconds")
		return
	}
	effectiveKiroIndependentTTLSeconds := previousSettings.KiroCacheIndependentTTLSeconds
	if req.KiroCacheIndependentTTLSeconds != nil {
		effectiveKiroIndependentTTLSeconds = *req.KiroCacheIndependentTTLSeconds
	}
	effectiveKiroPrefixTTLSeconds := previousSettings.KiroCachePrefixTTLSeconds
	if req.KiroCachePrefixTTLSeconds != nil {
		effectiveKiroPrefixTTLSeconds = *req.KiroCachePrefixTTLSeconds
	}
	if effectiveKiroPrefixTTLSeconds > effectiveKiroIndependentTTLSeconds {
		response.BadRequest(c, "Kiro prefix TTL cannot exceed independent TTL")
		return
	}

	// SMTP 配置保护：如果请求中 smtp_host 为空但数据库中已有配置，则保留已有 SMTP 配置
	// 防止前端加载设置失败时空表单覆盖已保存的 SMTP 配置
	if req.SMTPHost == "" && previousSettings.SMTPHost != "" {
		req.SMTPHost = previousSettings.SMTPHost
		req.SMTPPort = previousSettings.SMTPPort
		req.SMTPUsername = previousSettings.SMTPUsername
		req.SMTPFrom = previousSettings.SMTPFrom
		req.SMTPFromName = previousSettings.SMTPFromName
		req.SMTPUseTLS = previousSettings.SMTPUseTLS
	}

	// Turnstile 参数验证
	if req.TurnstileEnabled {
		// 检查必填字段
		if req.TurnstileSiteKey == "" {
			response.BadRequest(c, "Turnstile Site Key is required when enabled")
			return
		}
		// 如果未提供 secret key，使用已保存的值（留空保留当前值）
		if req.TurnstileSecretKey == "" {
			if previousSettings.TurnstileSecretKey == "" {
				response.BadRequest(c, "Turnstile Secret Key is required when enabled")
				return
			}
			req.TurnstileSecretKey = previousSettings.TurnstileSecretKey
		}

		// 当 site_key 或 secret_key 任一变化时验证（避免配置错误导致无法登录）
		siteKeyChanged := previousSettings.TurnstileSiteKey != req.TurnstileSiteKey
		secretKeyChanged := previousSettings.TurnstileSecretKey != req.TurnstileSecretKey
		if siteKeyChanged || secretKeyChanged {
			if err := h.turnstileService.ValidateSecretKey(c.Request.Context(), req.TurnstileSecretKey); err != nil {
				response.ErrorFrom(c, err)
				return
			}
		}
	}

	// TOTP 双因素认证参数验证
	// 只有手动配置了加密密钥才允许启用 TOTP 功能
	if req.TotpEnabled && !previousSettings.TotpEnabled {
		// 尝试启用 TOTP，检查加密密钥是否已手动配置
		if !h.settingService.IsTotpEncryptionKeyConfigured() {
			response.BadRequest(c, "Cannot enable TOTP: TOTP_ENCRYPTION_KEY environment variable must be configured first. Generate a key with 'openssl rand -hex 32' and set it in your environment.")
			return
		}
	}
	loginAgreementMode := strings.ToLower(strings.TrimSpace(req.LoginAgreementMode))
	if loginAgreementMode == "" {
		loginAgreementMode = strings.ToLower(strings.TrimSpace(previousSettings.LoginAgreementMode))
	}
	switch loginAgreementMode {
	case "", "modal":
		loginAgreementMode = "modal"
	case "checkbox":
	default:
		response.BadRequest(c, "Login agreement mode must be modal or checkbox")
		return
	}
	loginAgreementUpdatedAt := strings.TrimSpace(req.LoginAgreementUpdatedAt)
	if loginAgreementUpdatedAt == "" {
		loginAgreementUpdatedAt = strings.TrimSpace(previousSettings.LoginAgreementUpdatedAt)
	}
	loginAgreementDocuments := loginAgreementDocumentsToService(req.LoginAgreementDocuments)
	if len(loginAgreementDocuments) == 0 {
		loginAgreementDocuments = previousSettings.LoginAgreementDocuments
	}
	for _, doc := range loginAgreementDocuments {
		if strings.TrimSpace(doc.Title) == "" {
			response.BadRequest(c, "Login agreement document title is required")
			return
		}
		if len(doc.Title) > 80 {
			response.BadRequest(c, "Login agreement document title is too long (max 80 characters)")
			return
		}
		if len(doc.ContentMD) > 200*1024 {
			response.BadRequest(c, "Login agreement document content is too large (max 200KB)")
			return
		}
	}
	if req.LoginAgreementEnabled && len(loginAgreementDocuments) == 0 {
		response.BadRequest(c, "Login agreement documents are required when enabled")
		return
	}

	// LinuxDo Connect 参数验证
	if req.LinuxDoConnectEnabled {
		req.LinuxDoConnectClientID = strings.TrimSpace(req.LinuxDoConnectClientID)
		req.LinuxDoConnectClientSecret = strings.TrimSpace(req.LinuxDoConnectClientSecret)
		req.LinuxDoConnectRedirectURL = strings.TrimSpace(req.LinuxDoConnectRedirectURL)
		req.LinuxDoConnectClientID = stringJSONFieldOrFirstNonEmpty(
			rawFields,
			"linuxdo_connect_client_id",
			req.LinuxDoConnectClientID,
			previousSettings.LinuxDoConnectClientID,
		)
		req.LinuxDoConnectRedirectURL = stringJSONFieldOrFirstNonEmpty(
			rawFields,
			"linuxdo_connect_redirect_url",
			req.LinuxDoConnectRedirectURL,
			previousSettings.LinuxDoConnectRedirectURL,
		)

		if req.LinuxDoConnectClientID == "" {
			response.BadRequest(c, "LinuxDo Client ID is required when enabled")
			return
		}
		if req.LinuxDoConnectRedirectURL == "" {
			response.BadRequest(c, "LinuxDo Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.LinuxDoConnectRedirectURL); err != nil {
			response.BadRequest(c, "LinuxDo Redirect URL must be an absolute http(s) URL")
			return
		}

		// 如果未提供 client_secret，则保留现有值（如有）。
		if req.LinuxDoConnectClientSecret == "" {
			if previousSettings.LinuxDoConnectClientSecret == "" {
				response.BadRequest(c, "LinuxDo Client Secret is required when enabled")
				return
			}
			req.LinuxDoConnectClientSecret = previousSettings.LinuxDoConnectClientSecret
		}
	}

	if req.DingTalkConnectEnabled {
		req.DingTalkConnectClientID = strings.TrimSpace(req.DingTalkConnectClientID)
		req.DingTalkConnectClientSecret = strings.TrimSpace(req.DingTalkConnectClientSecret)
		req.DingTalkConnectRedirectURL = strings.TrimSpace(req.DingTalkConnectRedirectURL)
		req.DingTalkConnectCorpRestrictionPolicy = strings.ToLower(strings.TrimSpace(req.DingTalkConnectCorpRestrictionPolicy))
		req.DingTalkConnectInternalCorpID = strings.TrimSpace(req.DingTalkConnectInternalCorpID)
		req.DingTalkConnectSyncCorpEmailAttrKey = strings.TrimSpace(req.DingTalkConnectSyncCorpEmailAttrKey)
		req.DingTalkConnectSyncDisplayNameAttrKey = strings.TrimSpace(req.DingTalkConnectSyncDisplayNameAttrKey)
		req.DingTalkConnectSyncDeptAttrKey = strings.TrimSpace(req.DingTalkConnectSyncDeptAttrKey)
		req.DingTalkConnectSyncCorpEmailAttrName = strings.TrimSpace(req.DingTalkConnectSyncCorpEmailAttrName)
		req.DingTalkConnectSyncDisplayNameAttrName = strings.TrimSpace(req.DingTalkConnectSyncDisplayNameAttrName)
		req.DingTalkConnectSyncDeptAttrName = strings.TrimSpace(req.DingTalkConnectSyncDeptAttrName)

		if req.DingTalkConnectClientID == "" {
			response.BadRequest(c, "DingTalk Client ID is required when enabled")
			return
		}
		if req.DingTalkConnectRedirectURL == "" {
			response.BadRequest(c, "DingTalk Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.DingTalkConnectRedirectURL); err != nil {
			response.BadRequest(c, "DingTalk Redirect URL must be an absolute http(s) URL")
			return
		}
		if req.DingTalkConnectClientSecret == "" {
			if previousSettings.DingTalkConnectClientSecret == "" {
				response.BadRequest(c, "DingTalk Client Secret is required when enabled")
				return
			}
			req.DingTalkConnectClientSecret = previousSettings.DingTalkConnectClientSecret
		}

		switch req.DingTalkConnectCorpRestrictionPolicy {
		case "", "none", "whitelist":
			req.DingTalkConnectCorpRestrictionPolicy = "none"
		case "internal_only":
			// keep as-is
		default:
			response.BadRequest(c, "DingTalk corp restriction policy must be none or internal_only")
			return
		}

		if req.DingTalkConnectCorpRestrictionPolicy != "internal_only" {
			req.DingTalkConnectInternalCorpID = ""
			req.DingTalkConnectBypassRegistration = false
			req.DingTalkConnectSyncCorpEmail = false
			req.DingTalkConnectSyncDisplayName = false
			req.DingTalkConnectSyncDept = false
		}

		req.DingTalkConnectSyncCorpEmailAttrKey = firstNonEmpty(req.DingTalkConnectSyncCorpEmailAttrKey, previousSettings.DingTalkConnectSyncCorpEmailAttrKey, "dingtalk_email")
		req.DingTalkConnectSyncDisplayNameAttrKey = firstNonEmpty(req.DingTalkConnectSyncDisplayNameAttrKey, previousSettings.DingTalkConnectSyncDisplayNameAttrKey, "dingtalk_name")
		req.DingTalkConnectSyncDeptAttrKey = firstNonEmpty(req.DingTalkConnectSyncDeptAttrKey, previousSettings.DingTalkConnectSyncDeptAttrKey, "dingtalk_department")
	}

	if req.WeChatConnectEnabled {
		req.WeChatConnectAppID = strings.TrimSpace(req.WeChatConnectAppID)
		req.WeChatConnectAppSecret = strings.TrimSpace(req.WeChatConnectAppSecret)
		req.WeChatConnectOpenAppID = strings.TrimSpace(req.WeChatConnectOpenAppID)
		req.WeChatConnectOpenAppSecret = strings.TrimSpace(req.WeChatConnectOpenAppSecret)
		req.WeChatConnectMPAppID = strings.TrimSpace(req.WeChatConnectMPAppID)
		req.WeChatConnectMPAppSecret = strings.TrimSpace(req.WeChatConnectMPAppSecret)
		req.WeChatConnectMobileAppID = strings.TrimSpace(req.WeChatConnectMobileAppID)
		req.WeChatConnectMobileAppSecret = strings.TrimSpace(req.WeChatConnectMobileAppSecret)
		req.WeChatConnectMode = strings.ToLower(strings.TrimSpace(req.WeChatConnectMode))
		req.WeChatConnectScopes = strings.TrimSpace(req.WeChatConnectScopes)
		req.WeChatConnectRedirectURL = strings.TrimSpace(req.WeChatConnectRedirectURL)
		req.WeChatConnectFrontendRedirectURL = strings.TrimSpace(req.WeChatConnectFrontendRedirectURL)
		req.WeChatConnectAppID = stringJSONFieldOrFirstNonEmpty(
			rawFields,
			"wechat_connect_app_id",
			req.WeChatConnectAppID,
			previousSettings.WeChatConnectAppID,
		)
		req.WeChatConnectRedirectURL = stringJSONFieldOrFirstNonEmpty(
			rawFields,
			"wechat_connect_redirect_url",
			req.WeChatConnectRedirectURL,
			previousSettings.WeChatConnectRedirectURL,
		)
		req.WeChatConnectFrontendRedirectURL = stringJSONFieldOrFirstNonEmpty(
			rawFields,
			"wechat_connect_frontend_redirect_url",
			req.WeChatConnectFrontendRedirectURL,
			previousSettings.WeChatConnectFrontendRedirectURL,
		)
		if req.WeChatConnectMode == "" {
			req.WeChatConnectMode = strings.ToLower(strings.TrimSpace(previousSettings.WeChatConnectMode))
		}
		if req.WeChatConnectScopes == "" {
			req.WeChatConnectScopes = strings.TrimSpace(previousSettings.WeChatConnectScopes)
		}

		if req.WeChatConnectMPEnabled && req.WeChatConnectMobileEnabled {
			response.BadRequest(c, "WeChat Official Account and Mobile App cannot be enabled at the same time")
			return
		}
		if req.WeChatConnectMode != "" {
			switch req.WeChatConnectMode {
			case "open", "mp", "mobile":
			default:
				response.BadRequest(c, "WeChat mode must be open, mp, or mobile")
				return
			}
		}
		if !req.WeChatConnectOpenEnabled && !req.WeChatConnectMPEnabled && !req.WeChatConnectMobileEnabled {
			switch req.WeChatConnectMode {
			case "mp":
				req.WeChatConnectMPEnabled = true
			case "mobile":
				req.WeChatConnectMobileEnabled = true
			default:
				req.WeChatConnectOpenEnabled = true
			}
		}
		if req.WeChatConnectMode == "" {
			if req.WeChatConnectMPEnabled {
				req.WeChatConnectMode = "mp"
			} else if req.WeChatConnectMobileEnabled {
				req.WeChatConnectMode = "mobile"
			} else {
				req.WeChatConnectMode = "open"
			}
		}

		req.WeChatConnectOpenAppID = stringJSONFieldOrFirstNonEmpty(
			rawFields,
			"wechat_connect_open_app_id",
			req.WeChatConnectOpenAppID,
			req.WeChatConnectAppID,
			previousSettings.WeChatConnectOpenAppID,
			previousSettings.WeChatConnectAppID,
		)
		req.WeChatConnectMPAppID = stringJSONFieldOrFirstNonEmpty(
			rawFields,
			"wechat_connect_mp_app_id",
			req.WeChatConnectMPAppID,
			req.WeChatConnectAppID,
			previousSettings.WeChatConnectMPAppID,
			previousSettings.WeChatConnectAppID,
		)
		req.WeChatConnectMobileAppID = stringJSONFieldOrFirstNonEmpty(
			rawFields,
			"wechat_connect_mobile_app_id",
			req.WeChatConnectMobileAppID,
			req.WeChatConnectAppID,
			previousSettings.WeChatConnectMobileAppID,
			previousSettings.WeChatConnectAppID,
		)

		if req.WeChatConnectOpenAppSecret == "" {
			req.WeChatConnectOpenAppSecret = strings.TrimSpace(firstNonEmpty(previousSettings.WeChatConnectOpenAppSecret, previousSettings.WeChatConnectAppSecret, req.WeChatConnectAppSecret))
		}
		if req.WeChatConnectMPAppSecret == "" {
			req.WeChatConnectMPAppSecret = strings.TrimSpace(firstNonEmpty(previousSettings.WeChatConnectMPAppSecret, previousSettings.WeChatConnectAppSecret, req.WeChatConnectAppSecret))
		}
		if req.WeChatConnectMobileAppSecret == "" {
			req.WeChatConnectMobileAppSecret = strings.TrimSpace(firstNonEmpty(previousSettings.WeChatConnectMobileAppSecret, previousSettings.WeChatConnectAppSecret, req.WeChatConnectAppSecret))
		}
		if req.WeChatConnectAppSecret == "" {
			req.WeChatConnectAppSecret = strings.TrimSpace(firstNonEmpty(req.WeChatConnectOpenAppSecret, req.WeChatConnectMPAppSecret, req.WeChatConnectMobileAppSecret, previousSettings.WeChatConnectAppSecret))
		}

		if req.WeChatConnectOpenEnabled {
			if req.WeChatConnectOpenAppID == "" {
				response.BadRequest(c, "WeChat PC App ID is required when enabled")
				return
			}
			if req.WeChatConnectOpenAppSecret == "" {
				response.BadRequest(c, "WeChat PC App Secret is required when enabled")
				return
			}
		}
		if req.WeChatConnectMPEnabled {
			if req.WeChatConnectMPAppID == "" {
				response.BadRequest(c, "WeChat Official Account App ID is required when enabled")
				return
			}
			if req.WeChatConnectMPAppSecret == "" {
				response.BadRequest(c, "WeChat Official Account App Secret is required when enabled")
				return
			}
		}
		if req.WeChatConnectMobileEnabled {
			if req.WeChatConnectMobileAppID == "" {
				response.BadRequest(c, "WeChat Mobile App ID is required when enabled")
				return
			}
			if req.WeChatConnectMobileAppSecret == "" {
				response.BadRequest(c, "WeChat Mobile App Secret is required when enabled")
				return
			}
		}

		if req.WeChatConnectScopes == "" {
			if req.WeChatConnectMPEnabled {
				req.WeChatConnectScopes = service.DefaultWeChatConnectScopesForMode("mp")
			} else {
				req.WeChatConnectScopes = service.DefaultWeChatConnectScopesForMode(req.WeChatConnectMode)
			}
		}
		if req.WeChatConnectOpenEnabled || req.WeChatConnectMPEnabled {
			if req.WeChatConnectRedirectURL == "" {
				response.BadRequest(c, "WeChat Redirect URL is required when web oauth is enabled")
				return
			}
			if err := config.ValidateAbsoluteHTTPURL(req.WeChatConnectRedirectURL); err != nil {
				response.BadRequest(c, "WeChat Redirect URL must be an absolute http(s) URL")
				return
			}
			if req.WeChatConnectFrontendRedirectURL == "" {
				req.WeChatConnectFrontendRedirectURL = "/auth/wechat/callback"
			}
			if err := config.ValidateFrontendRedirectURL(req.WeChatConnectFrontendRedirectURL); err != nil {
				response.BadRequest(c, "WeChat Frontend Redirect URL is invalid")
				return
			}
		}
	}

	// Generic OIDC 参数验证
	oidcUsePKCE, oidcValidateIDToken, err := h.settingService.OIDCSecurityWriteDefaults(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if req.OIDCConnectEnabled {
		req.OIDCConnectProviderName = strings.TrimSpace(req.OIDCConnectProviderName)
		req.OIDCConnectClientID = strings.TrimSpace(req.OIDCConnectClientID)
		req.OIDCConnectClientSecret = strings.TrimSpace(req.OIDCConnectClientSecret)
		req.OIDCConnectIssuerURL = strings.TrimSpace(req.OIDCConnectIssuerURL)
		req.OIDCConnectDiscoveryURL = strings.TrimSpace(req.OIDCConnectDiscoveryURL)
		req.OIDCConnectAuthorizeURL = strings.TrimSpace(req.OIDCConnectAuthorizeURL)
		req.OIDCConnectTokenURL = strings.TrimSpace(req.OIDCConnectTokenURL)
		req.OIDCConnectUserInfoURL = strings.TrimSpace(req.OIDCConnectUserInfoURL)
		req.OIDCConnectJWKSURL = strings.TrimSpace(req.OIDCConnectJWKSURL)
		req.OIDCConnectScopes = strings.TrimSpace(req.OIDCConnectScopes)
		req.OIDCConnectRedirectURL = strings.TrimSpace(req.OIDCConnectRedirectURL)
		req.OIDCConnectFrontendRedirectURL = strings.TrimSpace(req.OIDCConnectFrontendRedirectURL)
		req.OIDCConnectTokenAuthMethod = strings.ToLower(strings.TrimSpace(req.OIDCConnectTokenAuthMethod))
		req.OIDCConnectAllowedSigningAlgs = strings.TrimSpace(req.OIDCConnectAllowedSigningAlgs)
		req.OIDCConnectUserInfoEmailPath = strings.TrimSpace(req.OIDCConnectUserInfoEmailPath)
		req.OIDCConnectUserInfoIDPath = strings.TrimSpace(req.OIDCConnectUserInfoIDPath)
		req.OIDCConnectUserInfoUsernamePath = strings.TrimSpace(req.OIDCConnectUserInfoUsernamePath)
		req.OIDCConnectProviderName = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_provider_name", req.OIDCConnectProviderName, previousSettings.OIDCConnectProviderName, "OIDC")
		req.OIDCConnectClientID = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_client_id", req.OIDCConnectClientID, previousSettings.OIDCConnectClientID)
		req.OIDCConnectIssuerURL = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_issuer_url", req.OIDCConnectIssuerURL, previousSettings.OIDCConnectIssuerURL)
		req.OIDCConnectDiscoveryURL = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_discovery_url", req.OIDCConnectDiscoveryURL, previousSettings.OIDCConnectDiscoveryURL)
		req.OIDCConnectAuthorizeURL = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_authorize_url", req.OIDCConnectAuthorizeURL, previousSettings.OIDCConnectAuthorizeURL)
		req.OIDCConnectTokenURL = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_token_url", req.OIDCConnectTokenURL, previousSettings.OIDCConnectTokenURL)
		req.OIDCConnectUserInfoURL = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_userinfo_url", req.OIDCConnectUserInfoURL, previousSettings.OIDCConnectUserInfoURL)
		req.OIDCConnectJWKSURL = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_jwks_url", req.OIDCConnectJWKSURL, previousSettings.OIDCConnectJWKSURL)
		req.OIDCConnectScopes = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_scopes", req.OIDCConnectScopes, previousSettings.OIDCConnectScopes, "openid email profile")
		req.OIDCConnectRedirectURL = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_redirect_url", req.OIDCConnectRedirectURL, previousSettings.OIDCConnectRedirectURL)
		req.OIDCConnectFrontendRedirectURL = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_frontend_redirect_url", req.OIDCConnectFrontendRedirectURL, previousSettings.OIDCConnectFrontendRedirectURL, "/auth/oidc/callback")
		req.OIDCConnectTokenAuthMethod = strings.ToLower(stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_token_auth_method", req.OIDCConnectTokenAuthMethod, previousSettings.OIDCConnectTokenAuthMethod, "client_secret_post"))
		req.OIDCConnectAllowedSigningAlgs = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_allowed_signing_algs", req.OIDCConnectAllowedSigningAlgs, previousSettings.OIDCConnectAllowedSigningAlgs, "RS256,ES256,PS256")
		req.OIDCConnectUserInfoEmailPath = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_userinfo_email_path", req.OIDCConnectUserInfoEmailPath, previousSettings.OIDCConnectUserInfoEmailPath)
		req.OIDCConnectUserInfoIDPath = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_userinfo_id_path", req.OIDCConnectUserInfoIDPath, previousSettings.OIDCConnectUserInfoIDPath)
		req.OIDCConnectUserInfoUsernamePath = stringJSONFieldOrFirstNonEmpty(rawFields, "oidc_connect_userinfo_username_path", req.OIDCConnectUserInfoUsernamePath, previousSettings.OIDCConnectUserInfoUsernamePath)
		if req.OIDCConnectUsePKCE != nil {
			oidcUsePKCE = *req.OIDCConnectUsePKCE
		}
		if req.OIDCConnectValidateIDToken != nil {
			oidcValidateIDToken = *req.OIDCConnectValidateIDToken
		}
		if req.OIDCConnectClockSkewSeconds == 0 {
			req.OIDCConnectClockSkewSeconds = previousSettings.OIDCConnectClockSkewSeconds
			if req.OIDCConnectClockSkewSeconds == 0 {
				req.OIDCConnectClockSkewSeconds = 120
			}
		}

		if req.OIDCConnectClientID == "" {
			response.BadRequest(c, "OIDC Client ID is required when enabled")
			return
		}
		if req.OIDCConnectIssuerURL == "" {
			response.BadRequest(c, "OIDC Issuer URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectIssuerURL); err != nil {
			response.BadRequest(c, "OIDC Issuer URL must be an absolute http(s) URL")
			return
		}
		if req.OIDCConnectDiscoveryURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectDiscoveryURL); err != nil {
				response.BadRequest(c, "OIDC Discovery URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectAuthorizeURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectAuthorizeURL); err != nil {
				response.BadRequest(c, "OIDC Authorize URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectTokenURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectTokenURL); err != nil {
				response.BadRequest(c, "OIDC Token URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectUserInfoURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectUserInfoURL); err != nil {
				response.BadRequest(c, "OIDC UserInfo URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectRedirectURL == "" {
			response.BadRequest(c, "OIDC Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectRedirectURL); err != nil {
			response.BadRequest(c, "OIDC Redirect URL must be an absolute http(s) URL")
			return
		}
		if req.OIDCConnectFrontendRedirectURL == "" {
			response.BadRequest(c, "OIDC Frontend Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateFrontendRedirectURL(req.OIDCConnectFrontendRedirectURL); err != nil {
			response.BadRequest(c, "OIDC Frontend Redirect URL is invalid")
			return
		}
		if !scopesContainOpenID(req.OIDCConnectScopes) {
			response.BadRequest(c, "OIDC scopes must contain openid")
			return
		}
		switch req.OIDCConnectTokenAuthMethod {
		case "", "client_secret_post", "client_secret_basic", "none":
		default:
			response.BadRequest(c, "OIDC Token Auth Method must be one of client_secret_post/client_secret_basic/none")
			return
		}
		if req.OIDCConnectClockSkewSeconds < 0 || req.OIDCConnectClockSkewSeconds > 600 {
			response.BadRequest(c, "OIDC clock skew seconds must be between 0 and 600")
			return
		}
		if oidcValidateIDToken && req.OIDCConnectAllowedSigningAlgs == "" {
			response.BadRequest(c, "OIDC Allowed Signing Algs is required when validate_id_token=true")
			return
		}
		if req.OIDCConnectJWKSURL != "" {
			if err := config.ValidateAbsoluteHTTPURL(req.OIDCConnectJWKSURL); err != nil {
				response.BadRequest(c, "OIDC JWKS URL must be an absolute http(s) URL")
				return
			}
		}
		if req.OIDCConnectTokenAuthMethod == "" || req.OIDCConnectTokenAuthMethod == "client_secret_post" || req.OIDCConnectTokenAuthMethod == "client_secret_basic" {
			if req.OIDCConnectClientSecret == "" {
				if previousSettings.OIDCConnectClientSecret == "" {
					response.BadRequest(c, "OIDC Client Secret is required when enabled")
					return
				}
				req.OIDCConnectClientSecret = previousSettings.OIDCConnectClientSecret
			}
		}
	}

	if req.GitHubOAuthEnabled {
		req.GitHubOAuthClientID = stringJSONFieldOrFirstNonEmpty(rawFields, "github_oauth_client_id", req.GitHubOAuthClientID, previousSettings.GitHubOAuthClientID)
		req.GitHubOAuthClientSecret = strings.TrimSpace(req.GitHubOAuthClientSecret)
		req.GitHubOAuthRedirectURL = stringJSONFieldOrFirstNonEmpty(rawFields, "github_oauth_redirect_url", req.GitHubOAuthRedirectURL, previousSettings.GitHubOAuthRedirectURL)
		req.GitHubOAuthFrontendRedirectURL = stringJSONFieldOrFirstNonEmpty(rawFields, "github_oauth_frontend_redirect_url", req.GitHubOAuthFrontendRedirectURL, previousSettings.GitHubOAuthFrontendRedirectURL, "/auth/oauth/callback")

		if req.GitHubOAuthClientID == "" {
			response.BadRequest(c, "GitHub OAuth Client ID is required when enabled")
			return
		}
		if req.GitHubOAuthClientSecret == "" {
			if previousSettings.GitHubOAuthClientSecret == "" {
				response.BadRequest(c, "GitHub OAuth Client Secret is required when enabled")
				return
			}
			req.GitHubOAuthClientSecret = previousSettings.GitHubOAuthClientSecret
		}
		if req.GitHubOAuthRedirectURL == "" {
			response.BadRequest(c, "GitHub OAuth Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.GitHubOAuthRedirectURL); err != nil {
			response.BadRequest(c, "GitHub OAuth Redirect URL must be an absolute http(s) URL")
			return
		}
		if req.GitHubOAuthFrontendRedirectURL == "" {
			response.BadRequest(c, "GitHub OAuth Frontend Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateFrontendRedirectURL(req.GitHubOAuthFrontendRedirectURL); err != nil {
			response.BadRequest(c, "GitHub OAuth Frontend Redirect URL is invalid")
			return
		}
	}

	if req.GoogleOAuthEnabled {
		req.GoogleOAuthClientID = stringJSONFieldOrFirstNonEmpty(rawFields, "google_oauth_client_id", req.GoogleOAuthClientID, previousSettings.GoogleOAuthClientID)
		req.GoogleOAuthClientSecret = strings.TrimSpace(req.GoogleOAuthClientSecret)
		req.GoogleOAuthRedirectURL = stringJSONFieldOrFirstNonEmpty(rawFields, "google_oauth_redirect_url", req.GoogleOAuthRedirectURL, previousSettings.GoogleOAuthRedirectURL)
		req.GoogleOAuthFrontendRedirectURL = stringJSONFieldOrFirstNonEmpty(rawFields, "google_oauth_frontend_redirect_url", req.GoogleOAuthFrontendRedirectURL, previousSettings.GoogleOAuthFrontendRedirectURL, "/auth/oauth/callback")

		if req.GoogleOAuthClientID == "" {
			response.BadRequest(c, "Google OAuth Client ID is required when enabled")
			return
		}
		if req.GoogleOAuthClientSecret == "" {
			if previousSettings.GoogleOAuthClientSecret == "" {
				response.BadRequest(c, "Google OAuth Client Secret is required when enabled")
				return
			}
			req.GoogleOAuthClientSecret = previousSettings.GoogleOAuthClientSecret
		}
		if req.GoogleOAuthRedirectURL == "" {
			response.BadRequest(c, "Google OAuth Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(req.GoogleOAuthRedirectURL); err != nil {
			response.BadRequest(c, "Google OAuth Redirect URL must be an absolute http(s) URL")
			return
		}
		if req.GoogleOAuthFrontendRedirectURL == "" {
			response.BadRequest(c, "Google OAuth Frontend Redirect URL is required when enabled")
			return
		}
		if err := config.ValidateFrontendRedirectURL(req.GoogleOAuthFrontendRedirectURL); err != nil {
			response.BadRequest(c, "Google OAuth Frontend Redirect URL is invalid")
			return
		}
	}

	createdMediaAssetIDs := []int64{}
	rollbackMigratedMedia := true
	defer func() {
		if rollbackMigratedMedia {
			h.cleanupIngestedSettingsMediaReferences(c.Request.Context(), createdMediaAssetIDs)
		}
	}()

	// “购买订阅”页面配置验证
	purchaseEnabled := previousSettings.PurchaseSubscriptionEnabled
	if req.PurchaseSubscriptionEnabled != nil {
		purchaseEnabled = *req.PurchaseSubscriptionEnabled
	}
	purchaseURL := previousSettings.PurchaseSubscriptionURL
	if req.PurchaseSubscriptionURL != nil {
		purchaseURL = strings.TrimSpace(*req.PurchaseSubscriptionURL)
	}

	// - 启用时要求 URL 合法且非空
	// - 禁用时允许为空；若提供了 URL 也做基本校验，避免误配置
	if purchaseEnabled {
		if purchaseURL == "" {
			response.BadRequest(c, "Purchase Subscription URL is required when enabled")
			return
		}
		if err := config.ValidateAbsoluteHTTPURL(purchaseURL); err != nil {
			response.BadRequest(c, "Purchase Subscription URL must be an absolute http(s) URL")
			return
		}
	} else if purchaseURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(purchaseURL); err != nil {
			response.BadRequest(c, "Purchase Subscription URL must be an absolute http(s) URL")
			return
		}
	}

	// Frontend URL 验证
	req.FrontendURL = strings.TrimSpace(req.FrontendURL)
	if req.FrontendURL != "" {
		if err := config.ValidateAbsoluteHTTPURL(req.FrontendURL); err != nil {
			response.BadRequest(c, "Frontend URL must be an absolute http(s) URL")
			return
		}
	}

	// 自定义菜单项验证
	const (
		maxCustomMenuItems    = 20
		maxMenuItemLabelLen   = 50
		maxMenuItemURLLen     = 2048
		maxMenuItemIconSVGLen = 10 * 1024 // 10KB
		maxMenuItemIDLen      = 32
	)

	customMenuJSON := previousSettings.CustomMenuItems
	if req.CustomMenuItems != nil {
		items := *req.CustomMenuItems
		if len(items) > maxCustomMenuItems {
			response.BadRequest(c, "Too many custom menu items (max 20)")
			return
		}
		for i, item := range items {
			if strings.TrimSpace(item.Label) == "" {
				response.BadRequest(c, "Custom menu item label is required")
				return
			}
			if len(item.Label) > maxMenuItemLabelLen {
				response.BadRequest(c, "Custom menu item label is too long (max 50 characters)")
				return
			}
			urlTrimmed := strings.TrimSpace(item.URL)
			if strings.HasPrefix(urlTrimmed, "md:") {
				// Markdown page mode: URL = "md:<slug>"
				slug := strings.TrimPrefix(urlTrimmed, "md:")
				if slug == "" {
					response.BadRequest(c, "Custom menu item markdown slug cannot be empty (use md:slug format)")
					return
				}
			} else {
				if urlTrimmed == "" {
					response.BadRequest(c, "Custom menu item URL is required (use md:slug for markdown pages)")
					return
				}
				if len(item.URL) > maxMenuItemURLLen {
					response.BadRequest(c, "Custom menu item URL is too long (max 2048 characters)")
					return
				}
				if err := validateCustomMenuPageURL(urlTrimmed); err != nil {
					response.BadRequest(c, "Custom menu item URL must be an absolute http(s) URL or md:<slug>")
					return
				}
			}
			items[i].URL = urlTrimmed
			if item.Visibility != "user" && item.Visibility != "admin" {
				response.BadRequest(c, "Custom menu item visibility must be 'user' or 'admin'")
				return
			}
			if len(item.IconSVG) > maxMenuItemIconSVGLen {
				response.BadRequest(c, "Custom menu item icon SVG is too large (max 10KB)")
				return
			}
			// Auto-generate ID if missing
			if strings.TrimSpace(item.ID) == "" {
				id, err := generateMenuItemID()
				if err != nil {
					response.Error(c, http.StatusInternalServerError, "Failed to generate menu item ID")
					return
				}
				items[i].ID = id
			} else if len(item.ID) > maxMenuItemIDLen {
				response.BadRequest(c, "Custom menu item ID is too long (max 32 characters)")
				return
			} else if !menuItemIDPattern.MatchString(item.ID) {
				response.BadRequest(c, "Custom menu item ID contains invalid characters (only a-z, A-Z, 0-9, - and _ are allowed)")
				return
			}
		}
		// ID uniqueness check
		seen := make(map[string]struct{}, len(items))
		for _, item := range items {
			if _, exists := seen[item.ID]; exists {
				response.BadRequest(c, "Duplicate custom menu item ID: "+item.ID)
				return
			}
			seen[item.ID] = struct{}{}
		}
		menuBytes, err := json.Marshal(items)
		if err != nil {
			response.BadRequest(c, "Failed to serialize custom menu items")
			return
		}
		customMenuJSON = string(menuBytes)
	}

	// 自定义端点验证
	const (
		maxCustomEndpoints        = 10
		maxEndpointNameLen        = 50
		maxEndpointURLLen         = 2048
		maxEndpointDescriptionLen = 200
	)

	customEndpointsJSON := previousSettings.CustomEndpoints
	if req.CustomEndpoints != nil {
		endpoints := *req.CustomEndpoints
		if len(endpoints) > maxCustomEndpoints {
			response.BadRequest(c, "Too many custom endpoints (max 10)")
			return
		}
		for _, ep := range endpoints {
			if strings.TrimSpace(ep.Name) == "" {
				response.BadRequest(c, "Custom endpoint name is required")
				return
			}
			if len(ep.Name) > maxEndpointNameLen {
				response.BadRequest(c, "Custom endpoint name is too long (max 50 characters)")
				return
			}
			if strings.TrimSpace(ep.Endpoint) == "" {
				response.BadRequest(c, "Custom endpoint URL is required")
				return
			}
			if len(ep.Endpoint) > maxEndpointURLLen {
				response.BadRequest(c, "Custom endpoint URL is too long (max 2048 characters)")
				return
			}
			if err := config.ValidateAbsoluteHTTPURL(strings.TrimSpace(ep.Endpoint)); err != nil {
				response.BadRequest(c, "Custom endpoint URL must be an absolute http(s) URL")
				return
			}
			if len(ep.Description) > maxEndpointDescriptionLen {
				response.BadRequest(c, "Custom endpoint description is too long (max 200 characters)")
				return
			}
		}
		endpointBytes, err := json.Marshal(endpoints)
		if err != nil {
			response.BadRequest(c, "Failed to serialize custom endpoints")
			return
		}
		customEndpointsJSON = string(endpointBytes)
	}

	// Ops metrics collector interval validation (seconds).
	if req.OpsMetricsIntervalSeconds != nil {
		v := *req.OpsMetricsIntervalSeconds
		if v < 60 {
			v = 60
		}
		if v > 3600 {
			v = 3600
		}
		req.OpsMetricsIntervalSeconds = &v
	}
	defaultSubscriptions := make([]service.DefaultSubscriptionSetting, 0, len(req.DefaultSubscriptions))
	for _, sub := range req.DefaultSubscriptions {
		defaultSubscriptions = append(defaultSubscriptions, service.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	// 验证最低版本号格式（空字符串=禁用，或合法 semver）
	if req.MinClaudeCodeVersion != "" {
		if !semverPattern.MatchString(req.MinClaudeCodeVersion) {
			response.Error(c, http.StatusBadRequest, "min_claude_code_version must be empty or a valid semver (e.g. 2.1.63)")
			return
		}
	}

	// 验证最高版本号格式（空字符串=禁用，或合法 semver）
	if req.MaxClaudeCodeVersion != "" {
		if !semverPattern.MatchString(req.MaxClaudeCodeVersion) {
			response.Error(c, http.StatusBadRequest, "max_claude_code_version must be empty or a valid semver (e.g. 3.0.0)")
			return
		}
	}
	if req.AntigravityUserAgentVersion != nil {
		normalized := strings.TrimSpace(*req.AntigravityUserAgentVersion)
		req.AntigravityUserAgentVersion = &normalized
		if normalized != "" && !semverPattern.MatchString(normalized) {
			response.Error(c, http.StatusBadRequest, "antigravity_user_agent_version must be empty or a valid semver (e.g. 1.23.2)")
			return
		}
	}
	if req.OpenAICodexUserAgent != nil {
		normalized := strings.TrimSpace(*req.OpenAICodexUserAgent)
		req.OpenAICodexUserAgent = &normalized
	}

	// codex_cli_only 加固：最低/最高 Codex 版本（空=禁用，或合法 semver；max>=min）
	if req.MinCodexVersion != "" && !semverPattern.MatchString(req.MinCodexVersion) {
		response.Error(c, http.StatusBadRequest, "min_codex_version must be empty or a valid semver (e.g. 0.141.0)")
		return
	}
	if req.MaxCodexVersion != "" && !semverPattern.MatchString(req.MaxCodexVersion) {
		response.Error(c, http.StatusBadRequest, "max_codex_version must be empty or a valid semver (e.g. 0.200.0)")
		return
	}
	if req.MinCodexVersion != "" && req.MaxCodexVersion != "" && service.CompareVersions(req.MaxCodexVersion, req.MinCodexVersion) < 0 {
		response.Error(c, http.StatusBadRequest, "max_codex_version must be greater than or equal to min_codex_version")
		return
	}
	// codex_cli_only 黑/白名单：非空须为合法 []AllowedClientEntry JSON。
	// 黑名单 OR 宽 deny（允许 originator-only）；白名单双因子 AND，额外要求每条可命中（非空 originator + ua_contains）。
	if err := service.ValidateCodexClientEntriesJSON(req.CodexCLIOnlyBlacklist); err != nil {
		response.Error(c, http.StatusBadRequest, "codex_cli_only_blacklist "+err.Error())
		return
	}
	if err := service.ValidateCodexWhitelistEntriesJSON(req.CodexCLIOnlyWhitelist); err != nil {
		response.Error(c, http.StatusBadRequest, "codex_cli_only_whitelist "+err.Error())
		return
	}
	if err := service.ValidateEngineFingerprintSignalsJSON(req.CodexCLIOnlyEngineFingerprintSignals); err != nil {
		response.Error(c, http.StatusBadRequest, "codex_cli_only_engine_fingerprint_signals "+err.Error())
		return
	}

	// 交叉验证：如果同时设置了最低和最高版本号，最高版本号必须 >= 最低版本号
	if req.MinClaudeCodeVersion != "" && req.MaxClaudeCodeVersion != "" {
		if service.CompareVersions(req.MaxClaudeCodeVersion, req.MinClaudeCodeVersion) < 0 {
			response.Error(c, http.StatusBadRequest, "max_claude_code_version must be greater than or equal to min_claude_code_version")
			return
		}
	}

	createdMediaAssetIDs, err = h.ingestSettingsMediaReferences(c.Request.Context(), &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var paymentReq *service.UpdatePaymentConfigRequest
	if previousPaymentCfg != nil {
		mergedPaymentReq := mergePaymentConfigUpdate(req, previousPaymentCfg)
		paymentReq = &mergedPaymentReq
	}

	supportQRCodes := previousSettings.SupportQRCodes
	if req.SupportQRCodes != nil {
		supportQRCodes = mustMarshalSupportQRCodes(req.SupportQRCodes)
	}
	linuxDoConnectEnabled := boolJSONFieldOrDefault(rawFields, "linuxdo_connect_enabled", req.LinuxDoConnectEnabled, previousSettings.LinuxDoConnectEnabled)
	linuxDoConnectClientID := stringJSONFieldOrDefault(rawFields, "linuxdo_connect_client_id", req.LinuxDoConnectClientID, previousSettings.LinuxDoConnectClientID)
	linuxDoConnectClientSecret := stringJSONFieldOrDefault(rawFields, "linuxdo_connect_client_secret", req.LinuxDoConnectClientSecret, previousSettings.LinuxDoConnectClientSecret)
	linuxDoConnectRedirectURL := stringJSONFieldOrDefault(rawFields, "linuxdo_connect_redirect_url", req.LinuxDoConnectRedirectURL, previousSettings.LinuxDoConnectRedirectURL)
	oidcConnectEnabled := boolJSONFieldOrDefault(rawFields, "oidc_connect_enabled", req.OIDCConnectEnabled, previousSettings.OIDCConnectEnabled)
	oidcConnectProviderName := stringJSONFieldOrDefault(rawFields, "oidc_connect_provider_name", req.OIDCConnectProviderName, previousSettings.OIDCConnectProviderName)
	oidcConnectClientID := stringJSONFieldOrDefault(rawFields, "oidc_connect_client_id", req.OIDCConnectClientID, previousSettings.OIDCConnectClientID)
	oidcConnectClientSecret := stringJSONFieldOrDefault(rawFields, "oidc_connect_client_secret", req.OIDCConnectClientSecret, previousSettings.OIDCConnectClientSecret)
	oidcConnectIssuerURL := stringJSONFieldOrDefault(rawFields, "oidc_connect_issuer_url", req.OIDCConnectIssuerURL, previousSettings.OIDCConnectIssuerURL)
	oidcConnectDiscoveryURL := stringJSONFieldOrDefault(rawFields, "oidc_connect_discovery_url", req.OIDCConnectDiscoveryURL, previousSettings.OIDCConnectDiscoveryURL)
	oidcConnectAuthorizeURL := stringJSONFieldOrDefault(rawFields, "oidc_connect_authorize_url", req.OIDCConnectAuthorizeURL, previousSettings.OIDCConnectAuthorizeURL)
	oidcConnectTokenURL := stringJSONFieldOrDefault(rawFields, "oidc_connect_token_url", req.OIDCConnectTokenURL, previousSettings.OIDCConnectTokenURL)
	oidcConnectUserInfoURL := stringJSONFieldOrDefault(rawFields, "oidc_connect_userinfo_url", req.OIDCConnectUserInfoURL, previousSettings.OIDCConnectUserInfoURL)
	oidcConnectJWKSURL := stringJSONFieldOrDefault(rawFields, "oidc_connect_jwks_url", req.OIDCConnectJWKSURL, previousSettings.OIDCConnectJWKSURL)
	oidcConnectScopes := stringJSONFieldOrDefault(rawFields, "oidc_connect_scopes", req.OIDCConnectScopes, previousSettings.OIDCConnectScopes)
	oidcConnectRedirectURL := stringJSONFieldOrDefault(rawFields, "oidc_connect_redirect_url", req.OIDCConnectRedirectURL, previousSettings.OIDCConnectRedirectURL)
	oidcConnectFrontendRedirectURL := stringJSONFieldOrDefault(rawFields, "oidc_connect_frontend_redirect_url", req.OIDCConnectFrontendRedirectURL, previousSettings.OIDCConnectFrontendRedirectURL)
	oidcConnectTokenAuthMethod := stringJSONFieldOrDefault(rawFields, "oidc_connect_token_auth_method", req.OIDCConnectTokenAuthMethod, previousSettings.OIDCConnectTokenAuthMethod)
	oidcConnectAllowedSigningAlgs := stringJSONFieldOrDefault(rawFields, "oidc_connect_allowed_signing_algs", req.OIDCConnectAllowedSigningAlgs, previousSettings.OIDCConnectAllowedSigningAlgs)
	oidcConnectClockSkewSeconds := intJSONFieldOrDefault(rawFields, "oidc_connect_clock_skew_seconds", req.OIDCConnectClockSkewSeconds, previousSettings.OIDCConnectClockSkewSeconds)
	oidcConnectRequireEmailVerified := boolJSONFieldOrDefault(rawFields, "oidc_connect_require_email_verified", req.OIDCConnectRequireEmailVerified, previousSettings.OIDCConnectRequireEmailVerified)
	oidcConnectUserInfoEmailPath := stringJSONFieldOrDefault(rawFields, "oidc_connect_userinfo_email_path", req.OIDCConnectUserInfoEmailPath, previousSettings.OIDCConnectUserInfoEmailPath)
	oidcConnectUserInfoIDPath := stringJSONFieldOrDefault(rawFields, "oidc_connect_userinfo_id_path", req.OIDCConnectUserInfoIDPath, previousSettings.OIDCConnectUserInfoIDPath)
	oidcConnectUserInfoUsernamePath := stringJSONFieldOrDefault(rawFields, "oidc_connect_userinfo_username_path", req.OIDCConnectUserInfoUsernamePath, previousSettings.OIDCConnectUserInfoUsernamePath)
	siteName := stringJSONFieldOrDefault(rawFields, "site_name", req.SiteName, previousSettings.SiteName)
	siteSubtitle := stringJSONFieldOrDefault(rawFields, "site_subtitle", req.SiteSubtitle, previousSettings.SiteSubtitle)
	enableModelFallback := boolJSONFieldOrDefault(rawFields, "enable_model_fallback", req.EnableModelFallback, previousSettings.EnableModelFallback)
	fallbackModelAnthropic := stringJSONFieldOrDefault(rawFields, "fallback_model_anthropic", req.FallbackModelAnthropic, previousSettings.FallbackModelAnthropic)
	fallbackModelOpenAI := stringJSONFieldOrDefault(rawFields, "fallback_model_openai", req.FallbackModelOpenAI, previousSettings.FallbackModelOpenAI)
	fallbackModelGemini := stringJSONFieldOrDefault(rawFields, "fallback_model_gemini", req.FallbackModelGemini, previousSettings.FallbackModelGemini)
	fallbackModelAntigravity := stringJSONFieldOrDefault(rawFields, "fallback_model_antigravity", req.FallbackModelAntigravity, previousSettings.FallbackModelAntigravity)

	// cyber 会话屏蔽 TTL 校验：提供时必须 > 0
	if req.CyberSessionBlockTTLSeconds != nil && *req.CyberSessionBlockTTLSeconds <= 0 {
		response.BadRequest(c, "cyber_session_block_ttl_seconds must be > 0")
		return
	}

	settings := &service.SystemSettings{
		// 系统全局 platform quota 默认值（整体替换语义）
		DefaultPlatformQuotas:       req.DefaultPlatformQuotas,
		AccountSchedulingThresholds: req.AccountSchedulingThresholds,

		RegistrationEnabled:              req.RegistrationEnabled,
		EmailVerifyEnabled:               req.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist: req.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                 req.PromoCodeEnabled,
		PasswordResetEnabled:             req.PasswordResetEnabled,
		FrontendURL:                      req.FrontendURL,
		InvitationCodeEnabled:            req.InvitationCodeEnabled,
		TotpEnabled:                      req.TotpEnabled,
		LoginAgreementEnabled:            req.LoginAgreementEnabled,
		LoginAgreementMode:               loginAgreementMode,
		LoginAgreementUpdatedAt:          loginAgreementUpdatedAt,
		LoginAgreementDocuments:          loginAgreementDocuments,
		SMTPHost:                         req.SMTPHost,
		SMTPPort:                         req.SMTPPort,
		SMTPUsername:                     req.SMTPUsername,
		SMTPPassword:                     req.SMTPPassword,
		SMTPFrom:                         req.SMTPFrom,
		SMTPFromName:                     req.SMTPFromName,
		SMTPUseTLS:                       req.SMTPUseTLS,
		TurnstileEnabled:                 req.TurnstileEnabled,
		TurnstileSiteKey:                 req.TurnstileSiteKey,
		TurnstileSecretKey:               req.TurnstileSecretKey,
		APIKeyACLTrustForwardedIP: func() bool {
			if req.APIKeyACLTrustForwardedIP != nil {
				return *req.APIKeyACLTrustForwardedIP
			}
			return previousSettings.APIKeyACLTrustForwardedIP
		}(),
		LinuxDoConnectEnabled:                  linuxDoConnectEnabled,
		LinuxDoConnectClientID:                 linuxDoConnectClientID,
		LinuxDoConnectClientSecret:             linuxDoConnectClientSecret,
		LinuxDoConnectRedirectURL:              linuxDoConnectRedirectURL,
		DingTalkConnectEnabled:                 req.DingTalkConnectEnabled,
		DingTalkConnectClientID:                req.DingTalkConnectClientID,
		DingTalkConnectClientSecret:            req.DingTalkConnectClientSecret,
		DingTalkConnectRedirectURL:             req.DingTalkConnectRedirectURL,
		DingTalkConnectCorpRestrictionPolicy:   req.DingTalkConnectCorpRestrictionPolicy,
		DingTalkConnectInternalCorpID:          req.DingTalkConnectInternalCorpID,
		DingTalkConnectBypassRegistration:      req.DingTalkConnectBypassRegistration,
		DingTalkConnectSyncCorpEmail:           req.DingTalkConnectSyncCorpEmail,
		DingTalkConnectSyncDisplayName:         req.DingTalkConnectSyncDisplayName,
		DingTalkConnectSyncDept:                req.DingTalkConnectSyncDept,
		DingTalkConnectSyncCorpEmailAttrKey:    req.DingTalkConnectSyncCorpEmailAttrKey,
		DingTalkConnectSyncDisplayNameAttrKey:  req.DingTalkConnectSyncDisplayNameAttrKey,
		DingTalkConnectSyncDeptAttrKey:         req.DingTalkConnectSyncDeptAttrKey,
		DingTalkConnectSyncCorpEmailAttrName:   req.DingTalkConnectSyncCorpEmailAttrName,
		DingTalkConnectSyncDisplayNameAttrName: req.DingTalkConnectSyncDisplayNameAttrName,
		DingTalkConnectSyncDeptAttrName:        req.DingTalkConnectSyncDeptAttrName,
		WeChatConnectEnabled:                   req.WeChatConnectEnabled,
		WeChatConnectAppID:                     req.WeChatConnectAppID,
		WeChatConnectAppSecret:                 req.WeChatConnectAppSecret,
		WeChatConnectOpenAppID:                 req.WeChatConnectOpenAppID,
		WeChatConnectOpenAppSecret:             req.WeChatConnectOpenAppSecret,
		WeChatConnectMPAppID:                   req.WeChatConnectMPAppID,
		WeChatConnectMPAppSecret:               req.WeChatConnectMPAppSecret,
		WeChatConnectMobileAppID:               req.WeChatConnectMobileAppID,
		WeChatConnectMobileAppSecret:           req.WeChatConnectMobileAppSecret,
		WeChatConnectOpenEnabled:               req.WeChatConnectOpenEnabled,
		WeChatConnectMPEnabled:                 req.WeChatConnectMPEnabled,
		WeChatConnectMobileEnabled:             req.WeChatConnectMobileEnabled,
		WeChatConnectMode:                      req.WeChatConnectMode,
		WeChatConnectScopes:                    req.WeChatConnectScopes,
		WeChatConnectRedirectURL:               req.WeChatConnectRedirectURL,
		WeChatConnectFrontendRedirectURL:       req.WeChatConnectFrontendRedirectURL,
		OIDCConnectEnabled:                     oidcConnectEnabled,
		OIDCConnectProviderName:                oidcConnectProviderName,
		OIDCConnectClientID:                    oidcConnectClientID,
		OIDCConnectClientSecret:                oidcConnectClientSecret,
		OIDCConnectIssuerURL:                   oidcConnectIssuerURL,
		OIDCConnectDiscoveryURL:                oidcConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:                oidcConnectAuthorizeURL,
		OIDCConnectTokenURL:                    oidcConnectTokenURL,
		OIDCConnectUserInfoURL:                 oidcConnectUserInfoURL,
		OIDCConnectJWKSURL:                     oidcConnectJWKSURL,
		OIDCConnectScopes:                      oidcConnectScopes,
		OIDCConnectRedirectURL:                 oidcConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:         oidcConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:             oidcConnectTokenAuthMethod,
		OIDCConnectUsePKCE:                     oidcUsePKCE,
		OIDCConnectValidateIDToken:             oidcValidateIDToken,
		OIDCConnectAllowedSigningAlgs:          oidcConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:            oidcConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:        oidcConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:           oidcConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:              oidcConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:        oidcConnectUserInfoUsernamePath,
		GitHubOAuthEnabled:                     req.GitHubOAuthEnabled,
		GitHubOAuthClientID:                    req.GitHubOAuthClientID,
		GitHubOAuthClientSecret:                req.GitHubOAuthClientSecret,
		GitHubOAuthRedirectURL:                 req.GitHubOAuthRedirectURL,
		GitHubOAuthFrontendRedirectURL:         req.GitHubOAuthFrontendRedirectURL,
		GoogleOAuthEnabled:                     req.GoogleOAuthEnabled,
		GoogleOAuthClientID:                    req.GoogleOAuthClientID,
		GoogleOAuthClientSecret:                req.GoogleOAuthClientSecret,
		GoogleOAuthRedirectURL:                 req.GoogleOAuthRedirectURL,
		GoogleOAuthFrontendRedirectURL:         req.GoogleOAuthFrontendRedirectURL,
		SiteName:                               siteName,
		SiteLogo: func() string {
			if hasSiteLogoField {
				return strings.TrimSpace(req.SiteLogo)
			}
			return previousSettings.SiteLogo
		}(),
		SiteSubtitle:                 siteSubtitle,
		APIBaseURL:                   req.APIBaseURL,
		ContactInfo:                  req.ContactInfo,
		SupportQRCodes:               supportQRCodes,
		DocURL:                       req.DocURL,
		HomeContent:                  req.HomeContent,
		HideCcsImportButton:          req.HideCcsImportButton,
		PurchaseSubscriptionEnabled:  purchaseEnabled,
		PurchaseSubscriptionURL:      purchaseURL,
		TableDefaultPageSize:         req.TableDefaultPageSize,
		TablePageSizeOptions:         req.TablePageSizeOptions,
		CustomMenuItems:              customMenuJSON,
		CustomEndpoints:              customEndpointsJSON,
		DefaultConcurrency:           req.DefaultConcurrency,
		DefaultBalance:               req.DefaultBalance,
		AffiliateEnabled:             boolValueOrDefault(req.AffiliateEnabled, previousSettings.AffiliateEnabled),
		AffiliateRebateRate:          affiliateRebateRate,
		AffiliateRebateCap:           affiliateRebateCap,
		AffiliateRebateInviteeLimit:  affiliateRebateInviteeLimit,
		AffiliateSignupBonus:         affiliateSignupBonus,
		TicketEnabled:                boolValueOrDefault(req.TicketEnabled, previousSettings.TicketEnabled),
		AffiliateRebateFreezeHours:   affiliateRebateFreezeHours,
		AffiliateRebateDurationDays:  affiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap: affiliateRebatePerInviteeCap,
		DefaultUserRPMLimit:          req.DefaultUserRPMLimit,
		DefaultSubscriptions:         defaultSubscriptions,
		EnableModelFallback:          enableModelFallback,
		FallbackModelAnthropic:       fallbackModelAnthropic,
		FallbackModelOpenAI:          fallbackModelOpenAI,
		FallbackModelGemini:          fallbackModelGemini,
		FallbackModelAntigravity:     fallbackModelAntigravity,
		PlatformDefaultAccountModelConfig: fromOptionalDTODefaultAccountModelConfig(
			req.PlatformDefaultAccountModelConfig,
			previousSettings.PlatformDefaultAccountModelConfig,
		),
		EnableIdentityPatch:         req.EnableIdentityPatch,
		IdentityPatchPrompt:         req.IdentityPatchPrompt,
		MinClaudeCodeVersion:        req.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:        req.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling: req.AllowUngroupedKeyScheduling,
		BackendModeEnabled:          req.BackendModeEnabled,
		OpsMonitoringEnabled: func() bool {
			if req.OpsMonitoringEnabled != nil {
				return *req.OpsMonitoringEnabled
			}
			return previousSettings.OpsMonitoringEnabled
		}(),
		OpsRealtimeMonitoringEnabled: func() bool {
			if req.OpsRealtimeMonitoringEnabled != nil {
				return *req.OpsRealtimeMonitoringEnabled
			}
			return previousSettings.OpsRealtimeMonitoringEnabled
		}(),
		OpsQueryModeDefault: func() string {
			if req.OpsQueryModeDefault != nil {
				return *req.OpsQueryModeDefault
			}
			return previousSettings.OpsQueryModeDefault
		}(),
		OpsMetricsIntervalSeconds: func() int {
			if req.OpsMetricsIntervalSeconds != nil {
				return *req.OpsMetricsIntervalSeconds
			}
			return previousSettings.OpsMetricsIntervalSeconds
		}(),
		EnableFingerprintUnification: func() bool {
			if req.EnableFingerprintUnification != nil {
				return *req.EnableFingerprintUnification
			}
			return previousSettings.EnableFingerprintUnification
		}(),
		EnableMetadataPassthrough: func() bool {
			if req.EnableMetadataPassthrough != nil {
				return *req.EnableMetadataPassthrough
			}
			return previousSettings.EnableMetadataPassthrough
		}(),
		ClaudeTelemetryMode: func() string {
			if req.ClaudeTelemetryMode != nil {
				return strings.TrimSpace(*req.ClaudeTelemetryMode)
			}
			return previousSettings.ClaudeTelemetryMode
		}(),
		EnableClaudeOAuthSystemPromptInjection: func() bool {
			if req.EnableClaudeOAuthSystemPromptInjection != nil {
				return *req.EnableClaudeOAuthSystemPromptInjection
			}
			return previousSettings.EnableClaudeOAuthSystemPromptInjection
		}(),
		ClaudeOAuthSystemPrompt: func() string {
			if req.ClaudeOAuthSystemPrompt != nil {
				return *req.ClaudeOAuthSystemPrompt
			}
			return previousSettings.ClaudeOAuthSystemPrompt
		}(),
		ClaudeOAuthSystemPromptBlocks: func() string {
			if req.ClaudeOAuthSystemPromptBlocks != nil {
				return *req.ClaudeOAuthSystemPromptBlocks
			}
			return previousSettings.ClaudeOAuthSystemPromptBlocks
		}(),
		GatewayDebugTimelineEnabled: func() bool {
			if req.GatewayDebugTimelineEnabled != nil {
				return *req.GatewayDebugTimelineEnabled
			}
			return previousSettings.GatewayDebugTimelineEnabled
		}(),
		GatewayDebugTimelineDirectory: func() string {
			if req.GatewayDebugTimelineDirectory != nil {
				return strings.TrimSpace(*req.GatewayDebugTimelineDirectory)
			}
			return previousSettings.GatewayDebugTimelineDirectory
		}(),
		GatewayDebugTimelineRetentionDays: func() int {
			if req.GatewayDebugTimelineRetentionDays != nil {
				return *req.GatewayDebugTimelineRetentionDays
			}
			return previousSettings.GatewayDebugTimelineRetentionDays
		}(),
		GatewayDebugTimelineMaxSizeMB: func() int64 {
			if req.GatewayDebugTimelineMaxSizeMB != nil {
				return *req.GatewayDebugTimelineMaxSizeMB
			}
			return previousSettings.GatewayDebugTimelineMaxSizeMB
		}(),
		GatewayDebugTimelineIncludeBody: func() bool {
			if req.GatewayDebugTimelineIncludeBody != nil {
				return *req.GatewayDebugTimelineIncludeBody
			}
			return previousSettings.GatewayDebugTimelineIncludeBody
		}(),
		GatewayDebugTimelineBodyMaxKB: func() int {
			if req.GatewayDebugTimelineBodyMaxKB != nil {
				return *req.GatewayDebugTimelineBodyMaxKB
			}
			return previousSettings.GatewayDebugTimelineBodyMaxKB
		}(),
		KiroDefaultVersion: func() string {
			if req.KiroDefaultVersion != nil {
				return strings.TrimSpace(*req.KiroDefaultVersion)
			}
			return previousSettings.KiroDefaultVersion
		}(),
		KiroDefaultCommit: func() string {
			if req.KiroDefaultCommit != nil {
				return strings.TrimSpace(*req.KiroDefaultCommit)
			}
			return previousSettings.KiroDefaultCommit
		}(),
		KiroDefaultSystemVersion: func() string {
			if req.KiroDefaultSystemVersion != nil {
				return strings.TrimSpace(*req.KiroDefaultSystemVersion)
			}
			return previousSettings.KiroDefaultSystemVersion
		}(),
		KiroDefaultNodeVersion: func() string {
			if req.KiroDefaultNodeVersion != nil {
				return strings.TrimSpace(*req.KiroDefaultNodeVersion)
			}
			return previousSettings.KiroDefaultNodeVersion
		}(),
		KiroCacheHitRateScale: func() int {
			if req.KiroCacheHitRateScale != nil {
				return *req.KiroCacheHitRateScale
			}
			return previousSettings.KiroCacheHitRateScale
		}(),
		KiroCacheMinBlockTokens: func() int {
			if req.KiroCacheMinBlockTokens != nil {
				return *req.KiroCacheMinBlockTokens
			}
			return previousSettings.KiroCacheMinBlockTokens
		}(),
		KiroCacheIndependentTTLSeconds: func() int {
			if req.KiroCacheIndependentTTLSeconds != nil {
				return *req.KiroCacheIndependentTTLSeconds
			}
			return previousSettings.KiroCacheIndependentTTLSeconds
		}(),
		KiroCachePrefixTTLSeconds: func() int {
			if req.KiroCachePrefixTTLSeconds != nil {
				return *req.KiroCachePrefixTTLSeconds
			}
			return previousSettings.KiroCachePrefixTTLSeconds
		}(),
		EnableAnthropicCacheTTL1hInjection: func() bool {
			if req.EnableAnthropicCacheTTL1hInjection != nil {
				return *req.EnableAnthropicCacheTTL1hInjection
			}
			return previousSettings.EnableAnthropicCacheTTL1hInjection
		}(),
		RewriteMessageCacheControl: func() bool {
			if req.RewriteMessageCacheControl != nil {
				return *req.RewriteMessageCacheControl
			}
			return previousSettings.RewriteMessageCacheControl
		}(),
		AntigravityUserAgentVersion: func() string {
			if req.AntigravityUserAgentVersion != nil {
				return *req.AntigravityUserAgentVersion
			}
			return previousSettings.AntigravityUserAgentVersion
		}(),
		OpenAICodexUserAgent: func() string {
			if req.OpenAICodexUserAgent != nil {
				return *req.OpenAICodexUserAgent
			}
			return previousSettings.OpenAICodexUserAgent
		}(),
		AntiBanPlatforms: func() map[string]bool {
			if req.AntiBanPlatforms != nil {
				return req.AntiBanPlatforms
			}
			return previousSettings.AntiBanPlatforms
		}(),
		MinCodexVersion:       strings.TrimSpace(req.MinCodexVersion),
		MaxCodexVersion:       strings.TrimSpace(req.MaxCodexVersion),
		CodexCLIOnlyBlacklist: strings.TrimSpace(req.CodexCLIOnlyBlacklist),
		CodexCLIOnlyWhitelist: strings.TrimSpace(req.CodexCLIOnlyWhitelist),
		CodexCLIOnlyAllowAppServerClients: func() bool {
			if req.CodexCLIOnlyAllowAppServerClients != nil {
				return *req.CodexCLIOnlyAllowAppServerClients
			}
			return previousSettings.CodexCLIOnlyAllowAppServerClients
		}(),
		CodexCLIOnlyEngineFingerprintSignals: strings.TrimSpace(req.CodexCLIOnlyEngineFingerprintSignals),
		PaymentVisibleMethodAlipaySource: func() string {
			if req.PaymentVisibleMethodAlipaySource != nil {
				return strings.TrimSpace(*req.PaymentVisibleMethodAlipaySource)
			}
			return previousSettings.PaymentVisibleMethodAlipaySource
		}(),
		PaymentVisibleMethodWxpaySource: func() string {
			if req.PaymentVisibleMethodWxpaySource != nil {
				return strings.TrimSpace(*req.PaymentVisibleMethodWxpaySource)
			}
			return previousSettings.PaymentVisibleMethodWxpaySource
		}(),
		PaymentVisibleMethodAlipayEnabled: func() bool {
			if req.PaymentVisibleMethodAlipayEnabled != nil {
				return *req.PaymentVisibleMethodAlipayEnabled
			}
			return previousSettings.PaymentVisibleMethodAlipayEnabled
		}(),
		PaymentVisibleMethodWxpayEnabled: func() bool {
			if req.PaymentVisibleMethodWxpayEnabled != nil {
				return *req.PaymentVisibleMethodWxpayEnabled
			}
			return previousSettings.PaymentVisibleMethodWxpayEnabled
		}(),
		OpenAIAdvancedSchedulerEnabled: func() bool {
			if req.OpenAIAdvancedSchedulerEnabled != nil {
				return *req.OpenAIAdvancedSchedulerEnabled
			}
			return previousSettings.OpenAIAdvancedSchedulerEnabled
		}(),
		OpenAIStickyReservePercent: func() int {
			if req.OpenAIStickyReservePercent != nil {
				value := *req.OpenAIStickyReservePercent
				if value < 0 {
					return 0
				}
				if value > 100 {
					return 100
				}
				return value
			}
			return previousSettings.OpenAIStickyReservePercent
		}(),
		OpenAIStickyWaitTimeoutSeconds: func() int {
			if req.OpenAIStickyWaitTimeoutSeconds != nil {
				value := *req.OpenAIStickyWaitTimeoutSeconds
				if value < 1 {
					return 1
				}
				if value > 300 {
					return 300
				}
				return value
			}
			return previousSettings.OpenAIStickyWaitTimeoutSeconds
		}(),
		OpenAIWSMinIdlePerAccount: func() int {
			if req.OpenAIWSMinIdlePerAccount != nil {
				value := *req.OpenAIWSMinIdlePerAccount
				if value < 0 {
					return 0
				}
				if value > 64 {
					return 64
				}
				return value
			}
			return previousSettings.OpenAIWSMinIdlePerAccount
		}(),
		OpenAIWSMaxIdlePerAccount: func() int {
			if req.OpenAIWSMaxIdlePerAccount != nil {
				value := *req.OpenAIWSMaxIdlePerAccount
				if value < 0 {
					return 0
				}
				if value > 64 {
					return 64
				}
				return value
			}
			return previousSettings.OpenAIWSMaxIdlePerAccount
		}(),
		OpenAIWSNeutralPrewarmPercent: func() int {
			if req.OpenAIWSNeutralPrewarmPercent != nil {
				value := *req.OpenAIWSNeutralPrewarmPercent
				if value < 0 {
					return 0
				}
				if value > 100 {
					return 100
				}
				return value
			}
			return previousSettings.OpenAIWSNeutralPrewarmPercent
		}(),
		OpenAIWSSessionIdleTTLSeconds: func() int {
			if req.OpenAIWSSessionIdleTTLSeconds != nil {
				value := *req.OpenAIWSSessionIdleTTLSeconds
				if value < 1 {
					return 1
				}
				if value > 3600 {
					return 3600
				}
				return value
			}
			return previousSettings.OpenAIWSSessionIdleTTLSeconds
		}(),
		OpenAIOAuthImageBridgeDisableKeepAlives: func() bool {
			if req.OpenAIOAuthImageBridgeDisableKeepAlives != nil {
				return *req.OpenAIOAuthImageBridgeDisableKeepAlives
			}
			return previousSettings.OpenAIOAuthImageBridgeDisableKeepAlives
		}(),
		OpenAIOAuthImageBridgeFreshUpstreamClient: func() bool {
			if req.OpenAIOAuthImageBridgeFreshUpstreamClient != nil {
				return *req.OpenAIOAuthImageBridgeFreshUpstreamClient
			}
			return previousSettings.OpenAIOAuthImageBridgeFreshUpstreamClient
		}(),
		BalanceLowNotifyEnabled: func() bool {
			if req.BalanceLowNotifyEnabled != nil {
				return *req.BalanceLowNotifyEnabled
			}
			return previousSettings.BalanceLowNotifyEnabled
		}(),
		BalanceLowNotifyThreshold: func() float64 {
			if req.BalanceLowNotifyThreshold != nil {
				return *req.BalanceLowNotifyThreshold
			}
			return previousSettings.BalanceLowNotifyThreshold
		}(),
		BalanceLowNotifyRechargeURL: func() string {
			if req.BalanceLowNotifyRechargeURL != nil {
				return *req.BalanceLowNotifyRechargeURL
			}
			return previousSettings.BalanceLowNotifyRechargeURL
		}(),
		SubscriptionExpiryNotifyEnabled: func() bool {
			if req.SubscriptionExpiryNotifyEnabled != nil {
				return *req.SubscriptionExpiryNotifyEnabled
			}
			return previousSettings.SubscriptionExpiryNotifyEnabled
		}(),
		AccountQuotaNotifyEnabled: func() bool {
			if req.AccountQuotaNotifyEnabled != nil {
				return *req.AccountQuotaNotifyEnabled
			}
			return previousSettings.AccountQuotaNotifyEnabled
		}(),
		AccountQuotaNotifyEmails: func() []service.NotifyEmailEntry {
			if req.AccountQuotaNotifyEmails != nil {
				return dto.NotifyEmailEntriesToService(*req.AccountQuotaNotifyEmails)
			}
			return previousSettings.AccountQuotaNotifyEmails
		}(),
		AIStudioEnabled: func() bool {
			if req.AIStudioEnabled != nil {
				return *req.AIStudioEnabled
			}
			return previousSettings.AIStudioEnabled
		}(),
		ChannelMonitorEnabled: func() bool {
			if req.ChannelMonitorEnabled != nil {
				return *req.ChannelMonitorEnabled
			}
			return previousSettings.ChannelMonitorEnabled
		}(),
		ChannelMonitorDefaultIntervalSeconds: func() int {
			if req.ChannelMonitorDefaultIntervalSeconds != nil {
				return *req.ChannelMonitorDefaultIntervalSeconds
			}
			return previousSettings.ChannelMonitorDefaultIntervalSeconds
		}(),
		AvailableChannelsEnabled: func() bool {
			if req.AvailableChannelsEnabled != nil {
				return *req.AvailableChannelsEnabled
			}
			return previousSettings.AvailableChannelsEnabled
		}(),
		RiskControlEnabled: func() bool {
			if req.RiskControlEnabled != nil {
				return *req.RiskControlEnabled
			}
			return previousSettings.RiskControlEnabled
		}(),
		CyberSessionBlockEnabled: func() bool {
			if req.CyberSessionBlockEnabled != nil {
				return *req.CyberSessionBlockEnabled
			}
			return previousSettings.CyberSessionBlockEnabled
		}(),
		CyberSessionBlockTTLSeconds: func() int {
			if req.CyberSessionBlockTTLSeconds != nil {
				return *req.CyberSessionBlockTTLSeconds
			}
			return previousSettings.CyberSessionBlockTTLSeconds
		}(),
	}

	// req.AuthSourceXxxPlatformQuotas 为 nil 表示本次请求未包含该 source 的 quota 配置（保留 previousAuthSourceDefaults 中的值）；
	// non-nil（含 empty map）表示整体覆盖：empty map = 清空该 source 的所有 quota 配置。
	authSourceDefaults := &service.AuthSourceDefaultSettings{
		Email: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultEmailBalance, previousAuthSourceDefaults.Email.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultEmailConcurrency, previousAuthSourceDefaults.Email.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultEmailSubscriptions, previousAuthSourceDefaults.Email.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultEmailGrantOnSignup, previousAuthSourceDefaults.Email.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultEmailGrantOnFirstBind, previousAuthSourceDefaults.Email.GrantOnFirstBind),
			PlatformQuotas:   platformQuotasValueOrDefault(req.AuthSourceEmailPlatformQuotas, previousAuthSourceDefaults.Email.PlatformQuotas),
		},
		LinuxDo: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultLinuxDoBalance, previousAuthSourceDefaults.LinuxDo.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultLinuxDoConcurrency, previousAuthSourceDefaults.LinuxDo.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultLinuxDoSubscriptions, previousAuthSourceDefaults.LinuxDo.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultLinuxDoGrantOnSignup, previousAuthSourceDefaults.LinuxDo.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultLinuxDoGrantOnFirstBind, previousAuthSourceDefaults.LinuxDo.GrantOnFirstBind),
			PlatformQuotas:   platformQuotasValueOrDefault(req.AuthSourceLinuxDoPlatformQuotas, previousAuthSourceDefaults.LinuxDo.PlatformQuotas),
		},
		OIDC: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultOIDCBalance, previousAuthSourceDefaults.OIDC.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultOIDCConcurrency, previousAuthSourceDefaults.OIDC.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultOIDCSubscriptions, previousAuthSourceDefaults.OIDC.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultOIDCGrantOnSignup, previousAuthSourceDefaults.OIDC.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultOIDCGrantOnFirstBind, previousAuthSourceDefaults.OIDC.GrantOnFirstBind),
			PlatformQuotas:   platformQuotasValueOrDefault(req.AuthSourceOIDCPlatformQuotas, previousAuthSourceDefaults.OIDC.PlatformQuotas),
		},
		WeChat: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultWeChatBalance, previousAuthSourceDefaults.WeChat.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultWeChatConcurrency, previousAuthSourceDefaults.WeChat.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultWeChatSubscriptions, previousAuthSourceDefaults.WeChat.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultWeChatGrantOnSignup, previousAuthSourceDefaults.WeChat.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultWeChatGrantOnFirstBind, previousAuthSourceDefaults.WeChat.GrantOnFirstBind),
			PlatformQuotas:   platformQuotasValueOrDefault(req.AuthSourceWeChatPlatformQuotas, previousAuthSourceDefaults.WeChat.PlatformQuotas),
		},
		GitHub: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultGitHubBalance, previousAuthSourceDefaults.GitHub.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultGitHubConcurrency, previousAuthSourceDefaults.GitHub.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultGitHubSubscriptions, previousAuthSourceDefaults.GitHub.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultGitHubGrantOnSignup, previousAuthSourceDefaults.GitHub.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultGitHubGrantOnFirstBind, previousAuthSourceDefaults.GitHub.GrantOnFirstBind),
			PlatformQuotas:   platformQuotasValueOrDefault(req.AuthSourceGitHubPlatformQuotas, previousAuthSourceDefaults.GitHub.PlatformQuotas),
		},
		Google: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultGoogleBalance, previousAuthSourceDefaults.Google.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultGoogleConcurrency, previousAuthSourceDefaults.Google.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultGoogleSubscriptions, previousAuthSourceDefaults.Google.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultGoogleGrantOnSignup, previousAuthSourceDefaults.Google.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultGoogleGrantOnFirstBind, previousAuthSourceDefaults.Google.GrantOnFirstBind),
			PlatformQuotas:   platformQuotasValueOrDefault(req.AuthSourceGooglePlatformQuotas, previousAuthSourceDefaults.Google.PlatformQuotas),
		},
		DingTalk: service.ProviderDefaultGrantSettings{
			Balance:          float64ValueOrDefault(req.AuthSourceDefaultDingTalkBalance, previousAuthSourceDefaults.DingTalk.Balance),
			Concurrency:      intValueOrDefault(req.AuthSourceDefaultDingTalkConcurrency, previousAuthSourceDefaults.DingTalk.Concurrency),
			Subscriptions:    defaultSubscriptionsValueOrDefault(req.AuthSourceDefaultDingTalkSubscriptions, previousAuthSourceDefaults.DingTalk.Subscriptions),
			GrantOnSignup:    boolValueOrDefault(req.AuthSourceDefaultDingTalkGrantOnSignup, previousAuthSourceDefaults.DingTalk.GrantOnSignup),
			GrantOnFirstBind: boolValueOrDefault(req.AuthSourceDefaultDingTalkGrantOnFirstBind, previousAuthSourceDefaults.DingTalk.GrantOnFirstBind),
			PlatformQuotas:   platformQuotasValueOrDefault(req.AuthSourceDingTalkPlatformQuotas, previousAuthSourceDefaults.DingTalk.PlatformQuotas),
		},
		ForceEmailOnThirdPartySignup: boolValueOrDefault(req.ForceEmailOnThirdPartySignup, previousAuthSourceDefaults.ForceEmailOnThirdPartySignup),
	}
	if err := h.settingService.UpdateSettingsWithAuthSourceDefaults(c.Request.Context(), settings, authSourceDefaults); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	rollbackMigratedMedia = false

	// Update OpenAI fast policy (stored under dedicated key, only when provided).
	if req.OpenAIFastPolicySettings != nil {
		if err := h.settingService.SetOpenAIFastPolicySettings(c.Request.Context(), openaiFastPolicySettingsFromDTO(req.OpenAIFastPolicySettings)); err != nil {
			rollbackErr := h.rollbackAdminSettingsUpdate(c.Request.Context(), previousSettings, previousAuthSourceDefaults, previousFastPolicy, nil)
			if rollbackErr != nil {
				h.cleanupIngestedSettingsMediaReferences(c.Request.Context(), createdMediaAssetIDs)
				response.ErrorFrom(c, fmt.Errorf("rollback admin settings update after fast policy save failure: %w", rollbackErr))
				return
			}
			h.cleanupIngestedSettingsMediaReferences(c.Request.Context(), createdMediaAssetIDs)
			response.BadRequest(c, err.Error())
			return
		}
	}

	// Update payment configuration (integrated into system settings).
	// Skip if no payment fields were provided (prevents accidental wipe).
	if h.paymentConfigService != nil && paymentReq != nil {
		if err := h.paymentConfigService.UpdatePaymentConfig(c.Request.Context(), *paymentReq); err != nil {
			var rollbackFastPolicy *service.OpenAIFastPolicySettings
			if req.OpenAIFastPolicySettings != nil {
				rollbackFastPolicy = previousFastPolicy
			}
			rollbackErr := h.rollbackAdminSettingsUpdate(c.Request.Context(), previousSettings, previousAuthSourceDefaults, rollbackFastPolicy, previousPaymentCfg)
			if rollbackErr != nil {
				h.cleanupIngestedSettingsMediaReferences(c.Request.Context(), createdMediaAssetIDs)
				response.ErrorFrom(c, fmt.Errorf("rollback admin settings update after payment config save failure: %w", rollbackErr))
				return
			}
			h.cleanupIngestedSettingsMediaReferences(c.Request.Context(), createdMediaAssetIDs)
			response.ErrorFrom(c, err)
			return
		}
		// Refresh in-memory provider registry so config changes take effect immediately
		if h.paymentService != nil {
			h.paymentService.RefreshProviders(c.Request.Context())
		}
	}

	updatedFastPolicy, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	h.auditSettingsUpdate(c, previousSettings, settings, previousAuthSourceDefaults, authSourceDefaults, previousFastPolicy, updatedFastPolicy, req)

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetAllSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	updatedAuthSourceDefaults, err := h.settingService.GetAuthSourceDefaultSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	updatedDefaultSubscriptions := make([]dto.DefaultSubscriptionSetting, 0, len(updatedSettings.DefaultSubscriptions))
	for _, sub := range updatedSettings.DefaultSubscriptions {
		updatedDefaultSubscriptions = append(updatedDefaultSubscriptions, dto.DefaultSubscriptionSetting{
			GroupID:      sub.GroupID,
			ValidityDays: sub.ValidityDays,
		})
	}

	// Reload payment config for response
	var updatedPaymentCfg *service.PaymentConfig
	if h.paymentConfigService != nil {
		updatedPaymentCfg, _ = h.paymentConfigService.GetPaymentConfig(c.Request.Context())
	}
	if updatedPaymentCfg == nil {
		updatedPaymentCfg = &service.PaymentConfig{}
	}

	payload := dto.SystemSettings{
		RegistrationEnabled:                       updatedSettings.RegistrationEnabled,
		EmailVerifyEnabled:                        updatedSettings.EmailVerifyEnabled,
		RegistrationEmailSuffixWhitelist:          updatedSettings.RegistrationEmailSuffixWhitelist,
		PromoCodeEnabled:                          updatedSettings.PromoCodeEnabled,
		PasswordResetEnabled:                      updatedSettings.PasswordResetEnabled,
		FrontendURL:                               updatedSettings.FrontendURL,
		InvitationCodeEnabled:                     updatedSettings.InvitationCodeEnabled,
		TotpEnabled:                               updatedSettings.TotpEnabled,
		TotpEncryptionKeyConfigured:               h.settingService.IsTotpEncryptionKeyConfigured(),
		LoginAgreementEnabled:                     updatedSettings.LoginAgreementEnabled,
		LoginAgreementMode:                        updatedSettings.LoginAgreementMode,
		LoginAgreementUpdatedAt:                   updatedSettings.LoginAgreementUpdatedAt,
		LoginAgreementDocuments:                   loginAgreementDocumentsToDTO(updatedSettings.LoginAgreementDocuments),
		SMTPHost:                                  updatedSettings.SMTPHost,
		SMTPPort:                                  updatedSettings.SMTPPort,
		SMTPUsername:                              updatedSettings.SMTPUsername,
		SMTPPasswordConfigured:                    updatedSettings.SMTPPasswordConfigured,
		SMTPFrom:                                  updatedSettings.SMTPFrom,
		SMTPFromName:                              updatedSettings.SMTPFromName,
		SMTPUseTLS:                                updatedSettings.SMTPUseTLS,
		TurnstileEnabled:                          updatedSettings.TurnstileEnabled,
		TurnstileSiteKey:                          updatedSettings.TurnstileSiteKey,
		TurnstileSecretKeyConfigured:              updatedSettings.TurnstileSecretKeyConfigured,
		APIKeyACLTrustForwardedIP:                 updatedSettings.APIKeyACLTrustForwardedIP,
		LinuxDoConnectEnabled:                     updatedSettings.LinuxDoConnectEnabled,
		LinuxDoConnectClientID:                    updatedSettings.LinuxDoConnectClientID,
		LinuxDoConnectClientSecretConfigured:      updatedSettings.LinuxDoConnectClientSecretConfigured,
		LinuxDoConnectRedirectURL:                 updatedSettings.LinuxDoConnectRedirectURL,
		DingTalkConnectEnabled:                    updatedSettings.DingTalkConnectEnabled,
		DingTalkConnectClientID:                   updatedSettings.DingTalkConnectClientID,
		DingTalkConnectClientSecretConfigured:     updatedSettings.DingTalkConnectClientSecretConfigured,
		DingTalkConnectRedirectURL:                updatedSettings.DingTalkConnectRedirectURL,
		DingTalkConnectCorpRestrictionPolicy:      updatedSettings.DingTalkConnectCorpRestrictionPolicy,
		DingTalkConnectInternalCorpID:             updatedSettings.DingTalkConnectInternalCorpID,
		DingTalkConnectBypassRegistration:         updatedSettings.DingTalkConnectBypassRegistration,
		DingTalkConnectSyncCorpEmail:              updatedSettings.DingTalkConnectSyncCorpEmail,
		DingTalkConnectSyncDisplayName:            updatedSettings.DingTalkConnectSyncDisplayName,
		DingTalkConnectSyncDept:                   updatedSettings.DingTalkConnectSyncDept,
		DingTalkConnectSyncCorpEmailAttrKey:       updatedSettings.DingTalkConnectSyncCorpEmailAttrKey,
		DingTalkConnectSyncDisplayNameAttrKey:     updatedSettings.DingTalkConnectSyncDisplayNameAttrKey,
		DingTalkConnectSyncDeptAttrKey:            updatedSettings.DingTalkConnectSyncDeptAttrKey,
		DingTalkConnectSyncCorpEmailAttrName:      updatedSettings.DingTalkConnectSyncCorpEmailAttrName,
		DingTalkConnectSyncDisplayNameAttrName:    updatedSettings.DingTalkConnectSyncDisplayNameAttrName,
		DingTalkConnectSyncDeptAttrName:           updatedSettings.DingTalkConnectSyncDeptAttrName,
		WeChatConnectEnabled:                      updatedSettings.WeChatConnectEnabled,
		WeChatConnectAppID:                        updatedSettings.WeChatConnectAppID,
		WeChatConnectAppSecretConfigured:          updatedSettings.WeChatConnectAppSecretConfigured,
		WeChatConnectOpenAppID:                    updatedSettings.WeChatConnectOpenAppID,
		WeChatConnectOpenAppSecretConfigured:      updatedSettings.WeChatConnectOpenAppSecretConfigured,
		WeChatConnectMPAppID:                      updatedSettings.WeChatConnectMPAppID,
		WeChatConnectMPAppSecretConfigured:        updatedSettings.WeChatConnectMPAppSecretConfigured,
		WeChatConnectMobileAppID:                  updatedSettings.WeChatConnectMobileAppID,
		WeChatConnectMobileAppSecretConfigured:    updatedSettings.WeChatConnectMobileAppSecretConfigured,
		WeChatConnectOpenEnabled:                  updatedSettings.WeChatConnectOpenEnabled,
		WeChatConnectMPEnabled:                    updatedSettings.WeChatConnectMPEnabled,
		WeChatConnectMobileEnabled:                updatedSettings.WeChatConnectMobileEnabled,
		WeChatConnectMode:                         updatedSettings.WeChatConnectMode,
		WeChatConnectScopes:                       updatedSettings.WeChatConnectScopes,
		WeChatConnectRedirectURL:                  updatedSettings.WeChatConnectRedirectURL,
		WeChatConnectFrontendRedirectURL:          updatedSettings.WeChatConnectFrontendRedirectURL,
		OIDCConnectEnabled:                        updatedSettings.OIDCConnectEnabled,
		OIDCConnectProviderName:                   updatedSettings.OIDCConnectProviderName,
		OIDCConnectClientID:                       updatedSettings.OIDCConnectClientID,
		OIDCConnectClientSecretConfigured:         updatedSettings.OIDCConnectClientSecretConfigured,
		OIDCConnectIssuerURL:                      updatedSettings.OIDCConnectIssuerURL,
		OIDCConnectDiscoveryURL:                   updatedSettings.OIDCConnectDiscoveryURL,
		OIDCConnectAuthorizeURL:                   updatedSettings.OIDCConnectAuthorizeURL,
		OIDCConnectTokenURL:                       updatedSettings.OIDCConnectTokenURL,
		OIDCConnectUserInfoURL:                    updatedSettings.OIDCConnectUserInfoURL,
		OIDCConnectJWKSURL:                        updatedSettings.OIDCConnectJWKSURL,
		OIDCConnectScopes:                         updatedSettings.OIDCConnectScopes,
		OIDCConnectRedirectURL:                    updatedSettings.OIDCConnectRedirectURL,
		OIDCConnectFrontendRedirectURL:            updatedSettings.OIDCConnectFrontendRedirectURL,
		OIDCConnectTokenAuthMethod:                updatedSettings.OIDCConnectTokenAuthMethod,
		OIDCConnectUsePKCE:                        updatedSettings.OIDCConnectUsePKCE,
		OIDCConnectValidateIDToken:                updatedSettings.OIDCConnectValidateIDToken,
		OIDCConnectAllowedSigningAlgs:             updatedSettings.OIDCConnectAllowedSigningAlgs,
		OIDCConnectClockSkewSeconds:               updatedSettings.OIDCConnectClockSkewSeconds,
		OIDCConnectRequireEmailVerified:           updatedSettings.OIDCConnectRequireEmailVerified,
		OIDCConnectUserInfoEmailPath:              updatedSettings.OIDCConnectUserInfoEmailPath,
		OIDCConnectUserInfoIDPath:                 updatedSettings.OIDCConnectUserInfoIDPath,
		OIDCConnectUserInfoUsernamePath:           updatedSettings.OIDCConnectUserInfoUsernamePath,
		GitHubOAuthEnabled:                        updatedSettings.GitHubOAuthEnabled,
		GitHubOAuthClientID:                       updatedSettings.GitHubOAuthClientID,
		GitHubOAuthClientSecretConfigured:         updatedSettings.GitHubOAuthClientSecretConfigured,
		GitHubOAuthRedirectURL:                    updatedSettings.GitHubOAuthRedirectURL,
		GitHubOAuthFrontendRedirectURL:            updatedSettings.GitHubOAuthFrontendRedirectURL,
		GoogleOAuthEnabled:                        updatedSettings.GoogleOAuthEnabled,
		GoogleOAuthClientID:                       updatedSettings.GoogleOAuthClientID,
		GoogleOAuthClientSecretConfigured:         updatedSettings.GoogleOAuthClientSecretConfigured,
		GoogleOAuthRedirectURL:                    updatedSettings.GoogleOAuthRedirectURL,
		GoogleOAuthFrontendRedirectURL:            updatedSettings.GoogleOAuthFrontendRedirectURL,
		SiteName:                                  updatedSettings.SiteName,
		SiteLogo:                                  updatedSettings.SiteLogo,
		SiteSubtitle:                              updatedSettings.SiteSubtitle,
		APIBaseURL:                                updatedSettings.APIBaseURL,
		ContactInfo:                               updatedSettings.ContactInfo,
		SupportQRCodes:                            dto.ParseSupportQRCodes(updatedSettings.SupportQRCodes),
		DocURL:                                    updatedSettings.DocURL,
		HomeContent:                               updatedSettings.HomeContent,
		HideCcsImportButton:                       updatedSettings.HideCcsImportButton,
		PurchaseSubscriptionEnabled:               updatedSettings.PurchaseSubscriptionEnabled,
		PurchaseSubscriptionURL:                   updatedSettings.PurchaseSubscriptionURL,
		TableDefaultPageSize:                      updatedSettings.TableDefaultPageSize,
		TablePageSizeOptions:                      updatedSettings.TablePageSizeOptions,
		CustomMenuItems:                           dto.ParseCustomMenuItems(updatedSettings.CustomMenuItems),
		CustomEndpoints:                           dto.ParseCustomEndpoints(updatedSettings.CustomEndpoints),
		DefaultConcurrency:                        updatedSettings.DefaultConcurrency,
		DefaultBalance:                            updatedSettings.DefaultBalance,
		AffiliateEnabled:                          updatedSettings.AffiliateEnabled,
		AffiliateRebateRate:                       updatedSettings.AffiliateRebateRate,
		AffiliateRebateCap:                        updatedSettings.AffiliateRebateCap,
		AffiliateRebateInviteeLimit:               updatedSettings.AffiliateRebateInviteeLimit,
		AffiliateSignupBonus:                      updatedSettings.AffiliateSignupBonus,
		TicketEnabled:                             updatedSettings.TicketEnabled,
		AIStudioEnabled:                           updatedSettings.AIStudioEnabled,
		AffiliateRebateFreezeHours:                updatedSettings.AffiliateRebateFreezeHours,
		AffiliateRebateDurationDays:               updatedSettings.AffiliateRebateDurationDays,
		AffiliateRebatePerInviteeCap:              updatedSettings.AffiliateRebatePerInviteeCap,
		DefaultUserRPMLimit:                       updatedSettings.DefaultUserRPMLimit,
		DefaultSubscriptions:                      updatedDefaultSubscriptions,
		EnableModelFallback:                       updatedSettings.EnableModelFallback,
		FallbackModelAnthropic:                    updatedSettings.FallbackModelAnthropic,
		FallbackModelOpenAI:                       updatedSettings.FallbackModelOpenAI,
		FallbackModelGemini:                       updatedSettings.FallbackModelGemini,
		FallbackModelAntigravity:                  updatedSettings.FallbackModelAntigravity,
		PlatformDefaultAccountModelConfig:         toDTODefaultAccountModelConfig(updatedSettings.PlatformDefaultAccountModelConfig),
		EnableIdentityPatch:                       updatedSettings.EnableIdentityPatch,
		IdentityPatchPrompt:                       updatedSettings.IdentityPatchPrompt,
		OpsMonitoringEnabled:                      updatedSettings.OpsMonitoringEnabled,
		OpsRealtimeMonitoringEnabled:              updatedSettings.OpsRealtimeMonitoringEnabled,
		OpsQueryModeDefault:                       updatedSettings.OpsQueryModeDefault,
		OpsMetricsIntervalSeconds:                 updatedSettings.OpsMetricsIntervalSeconds,
		MinClaudeCodeVersion:                      updatedSettings.MinClaudeCodeVersion,
		MaxClaudeCodeVersion:                      updatedSettings.MaxClaudeCodeVersion,
		AllowUngroupedKeyScheduling:               updatedSettings.AllowUngroupedKeyScheduling,
		BackendModeEnabled:                        updatedSettings.BackendModeEnabled,
		EnableFingerprintUnification:              updatedSettings.EnableFingerprintUnification,
		EnableMetadataPassthrough:                 updatedSettings.EnableMetadataPassthrough,
		EnableClaudeOAuthSystemPromptInjection:    updatedSettings.EnableClaudeOAuthSystemPromptInjection,
		ClaudeOAuthSystemPrompt:                   updatedSettings.ClaudeOAuthSystemPrompt,
		ClaudeOAuthSystemPromptBlocks:             updatedSettings.ClaudeOAuthSystemPromptBlocks,
		GatewayDebugTimelineEnabled:               updatedSettings.GatewayDebugTimelineEnabled,
		GatewayDebugTimelineDirectory:             updatedSettings.GatewayDebugTimelineDirectory,
		GatewayDebugTimelineRetentionDays:         updatedSettings.GatewayDebugTimelineRetentionDays,
		GatewayDebugTimelineMaxSizeMB:             updatedSettings.GatewayDebugTimelineMaxSizeMB,
		GatewayDebugTimelineIncludeBody:           updatedSettings.GatewayDebugTimelineIncludeBody,
		GatewayDebugTimelineBodyMaxKB:             updatedSettings.GatewayDebugTimelineBodyMaxKB,
		EnableAnthropicCacheTTL1hInjection:        updatedSettings.EnableAnthropicCacheTTL1hInjection,
		RewriteMessageCacheControl:                updatedSettings.RewriteMessageCacheControl,
		ClaudeTelemetryMode:                       updatedSettings.ClaudeTelemetryMode,
		AntigravityUserAgentVersion:               updatedSettings.AntigravityUserAgentVersion,
		OpenAICodexUserAgent:                      updatedSettings.OpenAICodexUserAgent,
		AntiBanPlatforms:                          updatedSettings.AntiBanPlatforms,
		MinCodexVersion:                           updatedSettings.MinCodexVersion,
		MaxCodexVersion:                           updatedSettings.MaxCodexVersion,
		CodexCLIOnlyBlacklist:                     updatedSettings.CodexCLIOnlyBlacklist,
		CodexCLIOnlyWhitelist:                     updatedSettings.CodexCLIOnlyWhitelist,
		CodexCLIOnlyAllowAppServerClients:         updatedSettings.CodexCLIOnlyAllowAppServerClients,
		CodexCLIOnlyEngineFingerprintSignals:      updatedSettings.CodexCLIOnlyEngineFingerprintSignals,
		KiroDefaultVersion:                        updatedSettings.KiroDefaultVersion,
		KiroDefaultCommit:                         updatedSettings.KiroDefaultCommit,
		KiroDefaultSystemVersion:                  updatedSettings.KiroDefaultSystemVersion,
		KiroDefaultNodeVersion:                    updatedSettings.KiroDefaultNodeVersion,
		KiroCacheHitRateScale:                     updatedSettings.KiroCacheHitRateScale,
		KiroCacheMinBlockTokens:                   updatedSettings.KiroCacheMinBlockTokens,
		KiroCacheIndependentTTLSeconds:            updatedSettings.KiroCacheIndependentTTLSeconds,
		KiroCachePrefixTTLSeconds:                 updatedSettings.KiroCachePrefixTTLSeconds,
		PaymentVisibleMethodAlipaySource:          updatedSettings.PaymentVisibleMethodAlipaySource,
		PaymentVisibleMethodWxpaySource:           updatedSettings.PaymentVisibleMethodWxpaySource,
		PaymentVisibleMethodAlipayEnabled:         updatedSettings.PaymentVisibleMethodAlipayEnabled,
		PaymentVisibleMethodWxpayEnabled:          updatedSettings.PaymentVisibleMethodWxpayEnabled,
		OpenAIAdvancedSchedulerEnabled:            updatedSettings.OpenAIAdvancedSchedulerEnabled,
		OpenAIStickyReservePercent:                updatedSettings.OpenAIStickyReservePercent,
		OpenAIStickyWaitTimeoutSeconds:            updatedSettings.OpenAIStickyWaitTimeoutSeconds,
		OpenAIWSMinIdlePerAccount:                 updatedSettings.OpenAIWSMinIdlePerAccount,
		OpenAIWSMaxIdlePerAccount:                 updatedSettings.OpenAIWSMaxIdlePerAccount,
		OpenAIWSNeutralPrewarmPercent:             updatedSettings.OpenAIWSNeutralPrewarmPercent,
		OpenAIWSSessionIdleTTLSeconds:             updatedSettings.OpenAIWSSessionIdleTTLSeconds,
		OpenAIOAuthImageBridgeDisableKeepAlives:   updatedSettings.OpenAIOAuthImageBridgeDisableKeepAlives,
		OpenAIOAuthImageBridgeFreshUpstreamClient: updatedSettings.OpenAIOAuthImageBridgeFreshUpstreamClient,
		BalanceLowNotifyEnabled:                   updatedSettings.BalanceLowNotifyEnabled,
		BalanceLowNotifyThreshold:                 updatedSettings.BalanceLowNotifyThreshold,
		BalanceLowNotifyRechargeURL:               updatedSettings.BalanceLowNotifyRechargeURL,
		SubscriptionExpiryNotifyEnabled:           updatedSettings.SubscriptionExpiryNotifyEnabled,
		AccountQuotaNotifyEnabled:                 updatedSettings.AccountQuotaNotifyEnabled,
		AccountQuotaNotifyEmails:                  dto.NotifyEmailEntriesFromService(updatedSettings.AccountQuotaNotifyEmails),
		PaymentEnabled:                            updatedPaymentCfg.Enabled,
		PaymentMinAmount:                          updatedPaymentCfg.MinAmount,
		PaymentMaxAmount:                          updatedPaymentCfg.MaxAmount,
		PaymentDailyLimit:                         updatedPaymentCfg.DailyLimit,
		PaymentOrderTimeoutMin:                    updatedPaymentCfg.OrderTimeoutMin,
		PaymentMaxPendingOrders:                   updatedPaymentCfg.MaxPendingOrders,
		PaymentEnabledTypes:                       updatedPaymentCfg.EnabledTypes,
		PaymentBalanceDisabled:                    updatedPaymentCfg.BalanceDisabled,
		PaymentBalanceRechargeMultiplier:          updatedPaymentCfg.BalanceRechargeMultiplier,
		PaymentRechargeFeeRate:                    updatedPaymentCfg.RechargeFeeRate,
		PaymentLoadBalanceStrat:                   updatedPaymentCfg.LoadBalanceStrategy,
		PaymentProductNamePrefix:                  updatedPaymentCfg.ProductNamePrefix,
		PaymentProductNameSuffix:                  updatedPaymentCfg.ProductNameSuffix,
		PaymentHelpImageURL:                       updatedPaymentCfg.HelpImageURL,
		PaymentHelpText:                           updatedPaymentCfg.HelpText,
		PaymentCancelRateLimitEnabled:             updatedPaymentCfg.CancelRateLimitEnabled,
		PaymentCancelRateLimitMax:                 updatedPaymentCfg.CancelRateLimitMax,
		PaymentCancelRateLimitWindow:              updatedPaymentCfg.CancelRateLimitWindow,
		PaymentCancelRateLimitUnit:                updatedPaymentCfg.CancelRateLimitUnit,
		PaymentCancelRateLimitMode:                updatedPaymentCfg.CancelRateLimitMode,
		PaymentAlipayForceQRCode:                  updatedPaymentCfg.AlipayForceQRCode,

		ChannelMonitorEnabled:                updatedSettings.ChannelMonitorEnabled,
		ChannelMonitorDefaultIntervalSeconds: updatedSettings.ChannelMonitorDefaultIntervalSeconds,

		AvailableChannelsEnabled:    updatedSettings.AvailableChannelsEnabled,
		RiskControlEnabled:          updatedSettings.RiskControlEnabled,
		CyberSessionBlockEnabled:    updatedSettings.CyberSessionBlockEnabled,
		CyberSessionBlockTTLSeconds: updatedSettings.CyberSessionBlockTTLSeconds,
		AccountSchedulingThresholds: updatedSettings.AccountSchedulingThresholds,
	}
	if fastPolicy, err := h.settingService.GetOpenAIFastPolicySettings(c.Request.Context()); err != nil {
		slog.Error("openai_fast_policy_settings_get_failed", "error", err)
	} else if fastPolicy != nil {
		payload.OpenAIFastPolicySettings = openaiFastPolicySettingsToDTO(fastPolicy)
	}

	// Default platform quotas（JSON map）—— 与 GetSettings 一致，避免保存后响应缺失该字段
	if platformQuotas, err := h.settingService.GetDefaultPlatformQuotas(c.Request.Context()); err != nil {
		slog.Error("default_platform_quotas_get_failed", "error", err)
	} else {
		payload.DefaultPlatformQuotas = platformQuotas
	}
	response.Success(c, systemSettingsResponseData(payload, updatedAuthSourceDefaults))
}

func mergePaymentConfigUpdate(req UpdateSettingsRequest, current *service.PaymentConfig) service.UpdatePaymentConfigRequest {
	if current == nil {
		current = &service.PaymentConfig{}
	}
	return service.UpdatePaymentConfigRequest{
		Enabled:                   boolPtrValueOrDefault(req.PaymentEnabled, current.Enabled),
		MinAmount:                 float64PtrValueOrDefault(req.PaymentMinAmount, current.MinAmount),
		MaxAmount:                 float64PtrValueOrDefault(req.PaymentMaxAmount, current.MaxAmount),
		DailyLimit:                float64PtrValueOrDefault(req.PaymentDailyLimit, current.DailyLimit),
		OrderTimeoutMin:           intPtrValueOrDefault(req.PaymentOrderTimeoutMin, current.OrderTimeoutMin),
		MaxPendingOrders:          intPtrValueOrDefault(req.PaymentMaxPendingOrders, current.MaxPendingOrders),
		EnabledTypes:              stringSliceValueOrDefault(req.PaymentEnabledTypes, current.EnabledTypes),
		BalanceDisabled:           boolPtrValueOrDefault(req.PaymentBalanceDisabled, current.BalanceDisabled),
		BalanceRechargeMultiplier: float64PtrValueOrDefault(req.PaymentBalanceRechargeMultiplier, current.BalanceRechargeMultiplier),
		RechargeFeeRate:           float64PtrValueOrDefault(req.PaymentRechargeFeeRate, current.RechargeFeeRate),
		LoadBalanceStrategy:       stringPtrValueOrDefault(req.PaymentLoadBalanceStrat, current.LoadBalanceStrategy),
		ProductNamePrefix:         stringPtrValueOrDefault(req.PaymentProductNamePrefix, current.ProductNamePrefix),
		ProductNameSuffix:         stringPtrValueOrDefault(req.PaymentProductNameSuffix, current.ProductNameSuffix),
		HelpImageURL:              stringPtrValueOrDefault(req.PaymentHelpImageURL, current.HelpImageURL),
		HelpText:                  stringPtrValueOrDefault(req.PaymentHelpText, current.HelpText),
		CancelRateLimitEnabled:    boolPtrValueOrDefault(req.PaymentCancelRateLimitEnabled, current.CancelRateLimitEnabled),
		CancelRateLimitMax:        intPtrValueOrDefault(req.PaymentCancelRateLimitMax, current.CancelRateLimitMax),
		CancelRateLimitWindow:     intPtrValueOrDefault(req.PaymentCancelRateLimitWindow, current.CancelRateLimitWindow),
		CancelRateLimitUnit:       stringPtrValueOrDefault(req.PaymentCancelRateLimitUnit, current.CancelRateLimitUnit),
		CancelRateLimitMode:       stringPtrValueOrDefault(req.PaymentCancelRateLimitMode, current.CancelRateLimitMode),
		AlipayForceQRCode:         boolPtrValueOrDefault(req.PaymentAlipayForceQRCode, current.AlipayForceQRCode),
	}
}

func boolPtrValueOrDefault(value *bool, fallback bool) *bool {
	if value != nil {
		return value
	}
	return &fallback
}

func float64PtrValueOrDefault(value *float64, fallback float64) *float64 {
	if value != nil {
		return value
	}
	return &fallback
}

func intPtrValueOrDefault(value *int, fallback int) *int {
	if value != nil {
		return value
	}
	return &fallback
}

func stringPtrValueOrDefault(value *string, fallback string) *string {
	if value != nil {
		return value
	}
	return &fallback
}

func stringSliceValueOrDefault(value []string, fallback []string) []string {
	if value != nil {
		return append([]string(nil), value...)
	}
	return append([]string(nil), fallback...)
}

// hasPaymentFields returns true if any payment-related field was explicitly provided.
func hasPaymentFields(req UpdateSettingsRequest) bool {
	return req.PaymentEnabled != nil || req.PaymentMinAmount != nil ||
		req.PaymentMaxAmount != nil || req.PaymentDailyLimit != nil ||
		req.PaymentOrderTimeoutMin != nil || req.PaymentMaxPendingOrders != nil ||
		req.PaymentEnabledTypes != nil || req.PaymentBalanceDisabled != nil ||
		req.PaymentBalanceRechargeMultiplier != nil || req.PaymentRechargeFeeRate != nil ||
		req.PaymentLoadBalanceStrat != nil || req.PaymentProductNamePrefix != nil ||
		req.PaymentProductNameSuffix != nil || req.PaymentHelpImageURL != nil ||
		req.PaymentHelpText != nil || req.PaymentCancelRateLimitEnabled != nil ||
		req.PaymentCancelRateLimitMax != nil || req.PaymentCancelRateLimitWindow != nil ||
		req.PaymentCancelRateLimitUnit != nil || req.PaymentCancelRateLimitMode != nil ||
		req.PaymentAlipayForceQRCode != nil
}

func (h *SettingHandler) auditSettingsUpdate(c *gin.Context, before *service.SystemSettings, after *service.SystemSettings, beforeAuthSourceDefaults *service.AuthSourceDefaultSettings, afterAuthSourceDefaults *service.AuthSourceDefaultSettings, beforeFastPolicy *service.OpenAIFastPolicySettings, afterFastPolicy *service.OpenAIFastPolicySettings, req UpdateSettingsRequest) {
	if before == nil || after == nil {
		return
	}

	changed := diffSettings(before, after, beforeAuthSourceDefaults, afterAuthSourceDefaults, beforeFastPolicy, afterFastPolicy, req)
	if len(changed) == 0 {
		return
	}

	subject, _ := middleware.GetAuthSubjectFromContext(c)
	role, _ := middleware.GetUserRoleFromContext(c)
	slog.Info("settings updated",
		"audit", true,
		"user_id", subject.UserID,
		"role", role,
		"changed", changed,
	)
}

func diffSettings(before *service.SystemSettings, after *service.SystemSettings, beforeAuthSourceDefaults *service.AuthSourceDefaultSettings, afterAuthSourceDefaults *service.AuthSourceDefaultSettings, beforeFastPolicy *service.OpenAIFastPolicySettings, afterFastPolicy *service.OpenAIFastPolicySettings, req UpdateSettingsRequest) []string {
	changed := make([]string, 0, 20)
	if before.RegistrationEnabled != after.RegistrationEnabled {
		changed = append(changed, "registration_enabled")
	}
	if before.EmailVerifyEnabled != after.EmailVerifyEnabled {
		changed = append(changed, "email_verify_enabled")
	}
	if !equalStringSlice(before.RegistrationEmailSuffixWhitelist, after.RegistrationEmailSuffixWhitelist) {
		changed = append(changed, "registration_email_suffix_whitelist")
	}
	if before.PromoCodeEnabled != after.PromoCodeEnabled {
		changed = append(changed, "promo_code_enabled")
	}
	if before.InvitationCodeEnabled != after.InvitationCodeEnabled {
		changed = append(changed, "invitation_code_enabled")
	}
	if before.PasswordResetEnabled != after.PasswordResetEnabled {
		changed = append(changed, "password_reset_enabled")
	}
	if before.FrontendURL != after.FrontendURL {
		changed = append(changed, "frontend_url")
	}
	if before.TotpEnabled != after.TotpEnabled {
		changed = append(changed, "totp_enabled")
	}
	if before.LoginAgreementEnabled != after.LoginAgreementEnabled {
		changed = append(changed, "login_agreement_enabled")
	}
	if before.LoginAgreementMode != after.LoginAgreementMode {
		changed = append(changed, "login_agreement_mode")
	}
	if before.LoginAgreementUpdatedAt != after.LoginAgreementUpdatedAt {
		changed = append(changed, "login_agreement_updated_at")
	}
	if !equalLoginAgreementDocuments(before.LoginAgreementDocuments, after.LoginAgreementDocuments) {
		changed = append(changed, "login_agreement_documents")
	}
	if before.SMTPHost != after.SMTPHost {
		changed = append(changed, "smtp_host")
	}
	if before.SMTPPort != after.SMTPPort {
		changed = append(changed, "smtp_port")
	}
	if before.SMTPUsername != after.SMTPUsername {
		changed = append(changed, "smtp_username")
	}
	if req.SMTPPassword != "" {
		changed = append(changed, "smtp_password")
	}
	if before.SMTPFrom != after.SMTPFrom {
		changed = append(changed, "smtp_from_email")
	}
	if before.SMTPFromName != after.SMTPFromName {
		changed = append(changed, "smtp_from_name")
	}
	if before.SMTPUseTLS != after.SMTPUseTLS {
		changed = append(changed, "smtp_use_tls")
	}
	if before.TurnstileEnabled != after.TurnstileEnabled {
		changed = append(changed, "turnstile_enabled")
	}
	if before.TurnstileSiteKey != after.TurnstileSiteKey {
		changed = append(changed, "turnstile_site_key")
	}
	if req.TurnstileSecretKey != "" {
		changed = append(changed, "turnstile_secret_key")
	}
	if before.LinuxDoConnectEnabled != after.LinuxDoConnectEnabled {
		changed = append(changed, "linuxdo_connect_enabled")
	}
	if before.LinuxDoConnectClientID != after.LinuxDoConnectClientID {
		changed = append(changed, "linuxdo_connect_client_id")
	}
	if req.LinuxDoConnectClientSecret != "" {
		changed = append(changed, "linuxdo_connect_client_secret")
	}
	if before.LinuxDoConnectRedirectURL != after.LinuxDoConnectRedirectURL {
		changed = append(changed, "linuxdo_connect_redirect_url")
	}
	if before.WeChatConnectEnabled != after.WeChatConnectEnabled {
		changed = append(changed, "wechat_connect_enabled")
	}
	if before.WeChatConnectAppID != after.WeChatConnectAppID {
		changed = append(changed, "wechat_connect_app_id")
	}
	if req.WeChatConnectAppSecret != "" {
		changed = append(changed, "wechat_connect_app_secret")
	}
	if before.WeChatConnectOpenAppID != after.WeChatConnectOpenAppID {
		changed = append(changed, "wechat_connect_open_app_id")
	}
	if req.WeChatConnectOpenAppSecret != "" {
		changed = append(changed, "wechat_connect_open_app_secret")
	}
	if before.WeChatConnectMPAppID != after.WeChatConnectMPAppID {
		changed = append(changed, "wechat_connect_mp_app_id")
	}
	if req.WeChatConnectMPAppSecret != "" {
		changed = append(changed, "wechat_connect_mp_app_secret")
	}
	if before.WeChatConnectMobileAppID != after.WeChatConnectMobileAppID {
		changed = append(changed, "wechat_connect_mobile_app_id")
	}
	if req.WeChatConnectMobileAppSecret != "" {
		changed = append(changed, "wechat_connect_mobile_app_secret")
	}
	if before.WeChatConnectOpenEnabled != after.WeChatConnectOpenEnabled {
		changed = append(changed, "wechat_connect_open_enabled")
	}
	if before.WeChatConnectMPEnabled != after.WeChatConnectMPEnabled {
		changed = append(changed, "wechat_connect_mp_enabled")
	}
	if before.WeChatConnectMobileEnabled != after.WeChatConnectMobileEnabled {
		changed = append(changed, "wechat_connect_mobile_enabled")
	}
	if before.WeChatConnectMode != after.WeChatConnectMode {
		changed = append(changed, "wechat_connect_mode")
	}
	if before.WeChatConnectScopes != after.WeChatConnectScopes {
		changed = append(changed, "wechat_connect_scopes")
	}
	if before.WeChatConnectRedirectURL != after.WeChatConnectRedirectURL {
		changed = append(changed, "wechat_connect_redirect_url")
	}
	if before.WeChatConnectFrontendRedirectURL != after.WeChatConnectFrontendRedirectURL {
		changed = append(changed, "wechat_connect_frontend_redirect_url")
	}
	if before.OIDCConnectEnabled != after.OIDCConnectEnabled {
		changed = append(changed, "oidc_connect_enabled")
	}
	if before.OIDCConnectProviderName != after.OIDCConnectProviderName {
		changed = append(changed, "oidc_connect_provider_name")
	}
	if before.OIDCConnectClientID != after.OIDCConnectClientID {
		changed = append(changed, "oidc_connect_client_id")
	}
	if req.OIDCConnectClientSecret != "" {
		changed = append(changed, "oidc_connect_client_secret")
	}
	if before.OIDCConnectIssuerURL != after.OIDCConnectIssuerURL {
		changed = append(changed, "oidc_connect_issuer_url")
	}
	if before.OIDCConnectDiscoveryURL != after.OIDCConnectDiscoveryURL {
		changed = append(changed, "oidc_connect_discovery_url")
	}
	if before.OIDCConnectAuthorizeURL != after.OIDCConnectAuthorizeURL {
		changed = append(changed, "oidc_connect_authorize_url")
	}
	if before.OIDCConnectTokenURL != after.OIDCConnectTokenURL {
		changed = append(changed, "oidc_connect_token_url")
	}
	if before.OIDCConnectUserInfoURL != after.OIDCConnectUserInfoURL {
		changed = append(changed, "oidc_connect_userinfo_url")
	}
	if before.OIDCConnectJWKSURL != after.OIDCConnectJWKSURL {
		changed = append(changed, "oidc_connect_jwks_url")
	}
	if before.OIDCConnectScopes != after.OIDCConnectScopes {
		changed = append(changed, "oidc_connect_scopes")
	}
	if before.OIDCConnectRedirectURL != after.OIDCConnectRedirectURL {
		changed = append(changed, "oidc_connect_redirect_url")
	}
	if before.OIDCConnectFrontendRedirectURL != after.OIDCConnectFrontendRedirectURL {
		changed = append(changed, "oidc_connect_frontend_redirect_url")
	}
	if before.OIDCConnectTokenAuthMethod != after.OIDCConnectTokenAuthMethod {
		changed = append(changed, "oidc_connect_token_auth_method")
	}
	if before.OIDCConnectUsePKCE != after.OIDCConnectUsePKCE {
		changed = append(changed, "oidc_connect_use_pkce")
	}
	if before.OIDCConnectValidateIDToken != after.OIDCConnectValidateIDToken {
		changed = append(changed, "oidc_connect_validate_id_token")
	}
	if before.OIDCConnectAllowedSigningAlgs != after.OIDCConnectAllowedSigningAlgs {
		changed = append(changed, "oidc_connect_allowed_signing_algs")
	}
	if before.OIDCConnectClockSkewSeconds != after.OIDCConnectClockSkewSeconds {
		changed = append(changed, "oidc_connect_clock_skew_seconds")
	}
	if before.OIDCConnectRequireEmailVerified != after.OIDCConnectRequireEmailVerified {
		changed = append(changed, "oidc_connect_require_email_verified")
	}
	if before.OIDCConnectUserInfoEmailPath != after.OIDCConnectUserInfoEmailPath {
		changed = append(changed, "oidc_connect_userinfo_email_path")
	}
	if before.OIDCConnectUserInfoIDPath != after.OIDCConnectUserInfoIDPath {
		changed = append(changed, "oidc_connect_userinfo_id_path")
	}
	if before.OIDCConnectUserInfoUsernamePath != after.OIDCConnectUserInfoUsernamePath {
		changed = append(changed, "oidc_connect_userinfo_username_path")
	}
	if before.SiteName != after.SiteName {
		changed = append(changed, "site_name")
	}
	if before.SiteLogo != after.SiteLogo {
		changed = append(changed, "site_logo")
	}
	if before.SiteSubtitle != after.SiteSubtitle {
		changed = append(changed, "site_subtitle")
	}
	if before.APIBaseURL != after.APIBaseURL {
		changed = append(changed, "api_base_url")
	}
	if before.ContactInfo != after.ContactInfo {
		changed = append(changed, "contact_info")
	}
	if before.SupportQRCodes != after.SupportQRCodes {
		changed = append(changed, "support_qr_codes")
	}
	if before.DocURL != after.DocURL {
		changed = append(changed, "doc_url")
	}
	if before.HomeContent != after.HomeContent {
		changed = append(changed, "home_content")
	}
	if before.HideCcsImportButton != after.HideCcsImportButton {
		changed = append(changed, "hide_ccs_import_button")
	}
	if before.DefaultConcurrency != after.DefaultConcurrency {
		changed = append(changed, "default_concurrency")
	}
	if before.DefaultBalance != after.DefaultBalance {
		changed = append(changed, "default_balance")
	}
	if before.AffiliateEnabled != after.AffiliateEnabled {
		changed = append(changed, "affiliate_enabled")
	}
	if before.AffiliateRebateRate != after.AffiliateRebateRate {
		changed = append(changed, "affiliate_rebate_rate")
	}
	if before.AffiliateRebateCap != after.AffiliateRebateCap {
		changed = append(changed, "affiliate_rebate_cap")
	}
	if before.AffiliateRebateInviteeLimit != after.AffiliateRebateInviteeLimit {
		changed = append(changed, "affiliate_rebate_invitee_limit")
	}
	if before.AffiliateSignupBonus != after.AffiliateSignupBonus {
		changed = append(changed, "affiliate_signup_bonus")
	}
	if before.TicketEnabled != after.TicketEnabled {
		changed = append(changed, "ticket_enabled")
	}
	if before.AIStudioEnabled != after.AIStudioEnabled {
		changed = append(changed, "ai_studio_enabled")
	}
	if before.AffiliateRebateFreezeHours != after.AffiliateRebateFreezeHours {
		changed = append(changed, "affiliate_rebate_freeze_hours")
	}
	if before.AffiliateRebateDurationDays != after.AffiliateRebateDurationDays {
		changed = append(changed, "affiliate_rebate_duration_days")
	}
	if before.AffiliateRebatePerInviteeCap != after.AffiliateRebatePerInviteeCap {
		changed = append(changed, "affiliate_rebate_per_invitee_cap")
	}
	if !equalDefaultSubscriptions(before.DefaultSubscriptions, after.DefaultSubscriptions) {
		changed = append(changed, "default_subscriptions")
	}
	if before.EnableModelFallback != after.EnableModelFallback {
		changed = append(changed, "enable_model_fallback")
	}
	if before.FallbackModelAnthropic != after.FallbackModelAnthropic {
		changed = append(changed, "fallback_model_anthropic")
	}
	if before.FallbackModelOpenAI != after.FallbackModelOpenAI {
		changed = append(changed, "fallback_model_openai")
	}
	if before.FallbackModelGemini != after.FallbackModelGemini {
		changed = append(changed, "fallback_model_gemini")
	}
	if before.FallbackModelAntigravity != after.FallbackModelAntigravity {
		changed = append(changed, "fallback_model_antigravity")
	}
	if !defaultAccountModelConfigEqual(before.PlatformDefaultAccountModelConfig, after.PlatformDefaultAccountModelConfig) {
		changed = append(changed, "platform_default_account_model_config")
	}
	if before.EnableIdentityPatch != after.EnableIdentityPatch {
		changed = append(changed, "enable_identity_patch")
	}
	if before.IdentityPatchPrompt != after.IdentityPatchPrompt {
		changed = append(changed, "identity_patch_prompt")
	}
	if before.OpsMonitoringEnabled != after.OpsMonitoringEnabled {
		changed = append(changed, "ops_monitoring_enabled")
	}
	if before.OpsRealtimeMonitoringEnabled != after.OpsRealtimeMonitoringEnabled {
		changed = append(changed, "ops_realtime_monitoring_enabled")
	}
	if before.OpsQueryModeDefault != after.OpsQueryModeDefault {
		changed = append(changed, "ops_query_mode_default")
	}
	if before.OpsMetricsIntervalSeconds != after.OpsMetricsIntervalSeconds {
		changed = append(changed, "ops_metrics_interval_seconds")
	}
	if before.MinClaudeCodeVersion != after.MinClaudeCodeVersion {
		changed = append(changed, "min_claude_code_version")
	}
	if before.MaxClaudeCodeVersion != after.MaxClaudeCodeVersion {
		changed = append(changed, "max_claude_code_version")
	}
	if before.MinCodexVersion != after.MinCodexVersion {
		changed = append(changed, "min_codex_version")
	}
	if before.MaxCodexVersion != after.MaxCodexVersion {
		changed = append(changed, "max_codex_version")
	}
	if before.CodexCLIOnlyAllowAppServerClients != after.CodexCLIOnlyAllowAppServerClients {
		changed = append(changed, "codex_cli_only_allow_app_server_clients")
	}
	if before.CodexCLIOnlyEngineFingerprintSignals != after.CodexCLIOnlyEngineFingerprintSignals {
		changed = append(changed, "codex_cli_only_engine_fingerprint_signals")
	}
	if before.CodexCLIOnlyBlacklist != after.CodexCLIOnlyBlacklist {
		changed = append(changed, "codex_cli_only_blacklist")
	}
	if before.CodexCLIOnlyWhitelist != after.CodexCLIOnlyWhitelist {
		changed = append(changed, "codex_cli_only_whitelist")
	}
	if before.AllowUngroupedKeyScheduling != after.AllowUngroupedKeyScheduling {
		changed = append(changed, "allow_ungrouped_key_scheduling")
	}
	if before.BackendModeEnabled != after.BackendModeEnabled {
		changed = append(changed, "backend_mode_enabled")
	}
	if before.PurchaseSubscriptionEnabled != after.PurchaseSubscriptionEnabled {
		changed = append(changed, "purchase_subscription_enabled")
	}
	if before.PurchaseSubscriptionURL != after.PurchaseSubscriptionURL {
		changed = append(changed, "purchase_subscription_url")
	}
	if before.TableDefaultPageSize != after.TableDefaultPageSize {
		changed = append(changed, "table_default_page_size")
	}
	if !equalIntSlice(before.TablePageSizeOptions, after.TablePageSizeOptions) {
		changed = append(changed, "table_page_size_options")
	}
	if before.CustomMenuItems != after.CustomMenuItems {
		changed = append(changed, "custom_menu_items")
	}
	if before.CustomEndpoints != after.CustomEndpoints {
		changed = append(changed, "custom_endpoints")
	}
	if before.EnableFingerprintUnification != after.EnableFingerprintUnification {
		changed = append(changed, "enable_fingerprint_unification")
	}
	if before.EnableMetadataPassthrough != after.EnableMetadataPassthrough {
		changed = append(changed, "enable_metadata_passthrough")
	}
	if before.ClaudeTelemetryMode != after.ClaudeTelemetryMode {
		changed = append(changed, "claude_telemetry_mode")
	}
	if before.EnableClaudeOAuthSystemPromptInjection != after.EnableClaudeOAuthSystemPromptInjection {
		changed = append(changed, "enable_claude_oauth_system_prompt_injection")
	}
	if before.ClaudeOAuthSystemPrompt != after.ClaudeOAuthSystemPrompt {
		changed = append(changed, "claude_oauth_system_prompt")
	}
	if before.ClaudeOAuthSystemPromptBlocks != after.ClaudeOAuthSystemPromptBlocks {
		changed = append(changed, "claude_oauth_system_prompt_blocks")
	}
	if before.GatewayDebugTimelineEnabled != after.GatewayDebugTimelineEnabled {
		changed = append(changed, "gateway_debug_timeline_enabled")
	}
	if before.GatewayDebugTimelineDirectory != after.GatewayDebugTimelineDirectory {
		changed = append(changed, "gateway_debug_timeline_directory")
	}
	if before.GatewayDebugTimelineRetentionDays != after.GatewayDebugTimelineRetentionDays {
		changed = append(changed, "gateway_debug_timeline_retention_days")
	}
	if before.GatewayDebugTimelineMaxSizeMB != after.GatewayDebugTimelineMaxSizeMB {
		changed = append(changed, "gateway_debug_timeline_max_size_mb")
	}
	if before.GatewayDebugTimelineIncludeBody != after.GatewayDebugTimelineIncludeBody {
		changed = append(changed, "gateway_debug_timeline_include_body")
	}
	if before.GatewayDebugTimelineBodyMaxKB != after.GatewayDebugTimelineBodyMaxKB {
		changed = append(changed, "gateway_debug_timeline_body_max_kb")
	}
	if before.EnableAnthropicCacheTTL1hInjection != after.EnableAnthropicCacheTTL1hInjection {
		changed = append(changed, "enable_anthropic_cache_ttl_1h_injection")
	}
	if before.RewriteMessageCacheControl != after.RewriteMessageCacheControl {
		changed = append(changed, "rewrite_message_cache_control")
	}
	if before.AntigravityUserAgentVersion != after.AntigravityUserAgentVersion {
		changed = append(changed, "antigravity_user_agent_version")
	}
	if before.OpenAICodexUserAgent != after.OpenAICodexUserAgent {
		changed = append(changed, "openai_codex_user_agent")
	}
	if before.PaymentVisibleMethodAlipaySource != after.PaymentVisibleMethodAlipaySource {
		changed = append(changed, "payment_visible_method_alipay_source")
	}
	if before.PaymentVisibleMethodWxpaySource != after.PaymentVisibleMethodWxpaySource {
		changed = append(changed, "payment_visible_method_wxpay_source")
	}
	if before.PaymentVisibleMethodAlipayEnabled != after.PaymentVisibleMethodAlipayEnabled {
		changed = append(changed, "payment_visible_method_alipay_enabled")
	}
	if before.PaymentVisibleMethodWxpayEnabled != after.PaymentVisibleMethodWxpayEnabled {
		changed = append(changed, "payment_visible_method_wxpay_enabled")
	}
	if before.OpenAIAdvancedSchedulerEnabled != after.OpenAIAdvancedSchedulerEnabled {
		changed = append(changed, "openai_advanced_scheduler_enabled")
	}
	if before.OpenAIStickyReservePercent != after.OpenAIStickyReservePercent {
		changed = append(changed, "openai_sticky_reserve_percent")
	}
	if before.OpenAIStickyWaitTimeoutSeconds != after.OpenAIStickyWaitTimeoutSeconds {
		changed = append(changed, "openai_sticky_wait_timeout_seconds")
	}
	if before.OpenAIWSMinIdlePerAccount != after.OpenAIWSMinIdlePerAccount {
		changed = append(changed, "openai_ws_min_idle_per_account")
	}
	if before.OpenAIWSMaxIdlePerAccount != after.OpenAIWSMaxIdlePerAccount {
		changed = append(changed, "openai_ws_max_idle_per_account")
	}
	if before.OpenAIWSNeutralPrewarmPercent != after.OpenAIWSNeutralPrewarmPercent {
		changed = append(changed, "openai_ws_neutral_prewarm_percent")
	}
	if before.OpenAIWSSessionIdleTTLSeconds != after.OpenAIWSSessionIdleTTLSeconds {
		changed = append(changed, "openai_ws_session_idle_ttl_seconds")
	}
	if before.OpenAIOAuthImageBridgeDisableKeepAlives != after.OpenAIOAuthImageBridgeDisableKeepAlives {
		changed = append(changed, "openai_oauth_image_bridge_disable_keepalives")
	}
	if before.OpenAIOAuthImageBridgeFreshUpstreamClient != after.OpenAIOAuthImageBridgeFreshUpstreamClient {
		changed = append(changed, "openai_oauth_image_bridge_fresh_upstream_client")
	}
	// Balance & quota notification
	if before.BalanceLowNotifyEnabled != after.BalanceLowNotifyEnabled {
		changed = append(changed, "balance_low_notify_enabled")
	}
	if before.BalanceLowNotifyThreshold != after.BalanceLowNotifyThreshold {
		changed = append(changed, "balance_low_notify_threshold")
	}
	if before.BalanceLowNotifyRechargeURL != after.BalanceLowNotifyRechargeURL {
		changed = append(changed, "balance_low_notify_recharge_url")
	}
	if before.SubscriptionExpiryNotifyEnabled != after.SubscriptionExpiryNotifyEnabled {
		changed = append(changed, "subscription_expiry_notify_enabled")
	}
	if before.AccountQuotaNotifyEnabled != after.AccountQuotaNotifyEnabled {
		changed = append(changed, "account_quota_notify_enabled")
	}
	if !equalNotifyEmailEntries(before.AccountQuotaNotifyEmails, after.AccountQuotaNotifyEmails) {
		changed = append(changed, "account_quota_notify_emails")
	}
	if before.ChannelMonitorEnabled != after.ChannelMonitorEnabled {
		changed = append(changed, "channel_monitor_enabled")
	}
	if before.ChannelMonitorDefaultIntervalSeconds != after.ChannelMonitorDefaultIntervalSeconds {
		changed = append(changed, "channel_monitor_default_interval_seconds")
	}
	if before.AvailableChannelsEnabled != after.AvailableChannelsEnabled {
		changed = append(changed, "available_channels_enabled")
	}
	if req.OpenAIFastPolicySettings != nil && !openAIFastPolicySettingsEqual(beforeFastPolicy, afterFastPolicy) {
		changed = append(changed, "openai_fast_policy_settings")
	}
	if before.RiskControlEnabled != after.RiskControlEnabled {
		changed = append(changed, "risk_control_enabled")
	}
	if before.CyberSessionBlockEnabled != after.CyberSessionBlockEnabled {
		changed = append(changed, "cyber_session_block_enabled")
	}
	if before.CyberSessionBlockTTLSeconds != after.CyberSessionBlockTTLSeconds {
		changed = append(changed, "cyber_session_block_ttl_seconds")
	}
	// Default platform quotas（JSON map，整体比较）
	if !equalPlatformQuotaSettings(before.DefaultPlatformQuotas, after.DefaultPlatformQuotas) {
		changed = append(changed, service.SettingKeyDefaultPlatformQuotas)
	}
	if !equalAccountSchedulingThresholds(before.AccountSchedulingThresholds, after.AccountSchedulingThresholds) {
		changed = append(changed, service.SettingKeyAccountSchedulingThresholds)
	}
	changed = appendAuthSourceDefaultChanges(changed, beforeAuthSourceDefaults, afterAuthSourceDefaults)
	return changed
}

func (h *SettingHandler) rollbackAdminSettingsUpdate(ctx context.Context, previousSettings *service.SystemSettings, previousAuthSourceDefaults *service.AuthSourceDefaultSettings, previousFastPolicy *service.OpenAIFastPolicySettings, previousPaymentCfg *service.PaymentConfig) error {
	var firstErr error

	if previousPaymentCfg != nil && h.paymentConfigService != nil {
		if err := h.paymentConfigService.UpdatePaymentConfig(ctx, paymentConfigUpdateRequestFromSnapshot(previousPaymentCfg)); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if previousFastPolicy != nil {
		if err := h.settingService.SetOpenAIFastPolicySettings(ctx, previousFastPolicy); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if previousSettings != nil && previousAuthSourceDefaults != nil {
		if err := h.settingService.UpdateSettingsWithAuthSourceDefaults(ctx, previousSettings, previousAuthSourceDefaults); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func paymentConfigUpdateRequestFromSnapshot(cfg *service.PaymentConfig) service.UpdatePaymentConfigRequest {
	if cfg == nil {
		cfg = &service.PaymentConfig{}
	}
	return service.UpdatePaymentConfigRequest{
		Enabled:                   paymentBoolPtr(cfg.Enabled),
		MinAmount:                 paymentFloat64Ptr(cfg.MinAmount),
		MaxAmount:                 paymentFloat64Ptr(cfg.MaxAmount),
		DailyLimit:                paymentFloat64Ptr(cfg.DailyLimit),
		OrderTimeoutMin:           paymentIntPtr(cfg.OrderTimeoutMin),
		MaxPendingOrders:          paymentIntPtr(cfg.MaxPendingOrders),
		EnabledTypes:              append([]string{}, cfg.EnabledTypes...),
		BalanceDisabled:           paymentBoolPtr(cfg.BalanceDisabled),
		BalanceRechargeMultiplier: paymentFloat64Ptr(cfg.BalanceRechargeMultiplier),
		RechargeFeeRate:           paymentFloat64Ptr(cfg.RechargeFeeRate),
		LoadBalanceStrategy:       paymentStringPtr(cfg.LoadBalanceStrategy),
		ProductNamePrefix:         paymentStringPtr(cfg.ProductNamePrefix),
		ProductNameSuffix:         paymentStringPtr(cfg.ProductNameSuffix),
		HelpImageURL:              paymentStringPtr(cfg.HelpImageURL),
		HelpText:                  paymentStringPtr(cfg.HelpText),
		CancelRateLimitEnabled:    paymentBoolPtr(cfg.CancelRateLimitEnabled),
		CancelRateLimitMax:        paymentIntPtr(cfg.CancelRateLimitMax),
		CancelRateLimitWindow:     paymentIntPtr(cfg.CancelRateLimitWindow),
		CancelRateLimitUnit:       paymentStringPtr(cfg.CancelRateLimitUnit),
		CancelRateLimitMode:       paymentStringPtr(cfg.CancelRateLimitMode),
		AlipayForceQRCode:         paymentBoolPtr(cfg.AlipayForceQRCode),
	}
}

func paymentBoolPtr(value bool) *bool {
	return &value
}

func paymentFloat64Ptr(value float64) *float64 {
	return &value
}

func paymentIntPtr(value int) *int {
	return &value
}

func paymentStringPtr(value string) *string {
	return &value
}

func openAIFastPolicySettingsEqual(before, after *service.OpenAIFastPolicySettings) bool {
	if before == nil || after == nil {
		return before == nil && after == nil
	}
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return false
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return false
	}
	return string(beforeJSON) == string(afterJSON)
}

func appendAuthSourceDefaultChanges(changed []string, before *service.AuthSourceDefaultSettings, after *service.AuthSourceDefaultSettings) []string {
	if before == nil {
		before = &service.AuthSourceDefaultSettings{}
	}
	if after == nil {
		after = &service.AuthSourceDefaultSettings{}
	}

	type providerDefaultGrantField struct {
		name   string
		before service.ProviderDefaultGrantSettings
		after  service.ProviderDefaultGrantSettings
	}

	fields := []providerDefaultGrantField{
		{name: "email", before: before.Email, after: after.Email},
		{name: "linuxdo", before: before.LinuxDo, after: after.LinuxDo},
		{name: "oidc", before: before.OIDC, after: after.OIDC},
		{name: "wechat", before: before.WeChat, after: after.WeChat},
		{name: "github", before: before.GitHub, after: after.GitHub},
		{name: "google", before: before.Google, after: after.Google},
		{name: "dingtalk", before: before.DingTalk, after: after.DingTalk},
	}
	for _, field := range fields {
		if field.before.Balance != field.after.Balance {
			changed = append(changed, "auth_source_default_"+field.name+"_balance")
		}
		if field.before.Concurrency != field.after.Concurrency {
			changed = append(changed, "auth_source_default_"+field.name+"_concurrency")
		}
		if !equalDefaultSubscriptions(field.before.Subscriptions, field.after.Subscriptions) {
			changed = append(changed, "auth_source_default_"+field.name+"_subscriptions")
		}
		if field.before.GrantOnSignup != field.after.GrantOnSignup {
			changed = append(changed, "auth_source_default_"+field.name+"_grant_on_signup")
		}
		if field.before.GrantOnFirstBind != field.after.GrantOnFirstBind {
			changed = append(changed, "auth_source_default_"+field.name+"_grant_on_first_bind")
		}
		// Platform quotas diff：整体替换语义，发单个 JSON key。
		if !equalPlatformQuotaSettings(field.before.PlatformQuotas, field.after.PlatformQuotas) {
			changed = append(changed, service.SettingKeyAuthSourcePlatformQuotas(field.name))
		}
	}
	if before.ForceEmailOnThirdPartySignup != after.ForceEmailOnThirdPartySignup {
		changed = append(changed, "force_email_on_third_party_signup")
	}
	return changed
}

func normalizeDefaultSubscriptions(input []dto.DefaultSubscriptionSetting) []dto.DefaultSubscriptionSetting {
	if len(input) == 0 {
		return nil
	}
	normalized := make([]dto.DefaultSubscriptionSetting, 0, len(input))
	for _, item := range input {
		if item.GroupID <= 0 || item.ValidityDays <= 0 {
			continue
		}
		if item.ValidityDays > service.MaxValidityDays {
			item.ValidityDays = service.MaxValidityDays
		}
		normalized = append(normalized, item)
	}
	return normalized
}

func normalizeOptionalDefaultSubscriptions(input *[]dto.DefaultSubscriptionSetting) *[]dto.DefaultSubscriptionSetting {
	if input == nil {
		return nil
	}
	normalized := normalizeDefaultSubscriptions(*input)
	return &normalized
}

func float64ValueOrDefault(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func intValueOrDefault(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

func boolValueOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func preserveOmittedSettingsFields(rawFields map[string]json.RawMessage, req *UpdateSettingsRequest, previous *service.SystemSettings) {
	if req == nil || previous == nil {
		return
	}
	reqValue := reflect.ValueOf(req)
	if reqValue.Kind() != reflect.Pointer || reqValue.IsNil() {
		return
	}
	reqValue = reqValue.Elem()
	previousValue := reflect.ValueOf(previous)
	if previousValue.Kind() != reflect.Pointer || previousValue.IsNil() {
		return
	}
	previousValue = previousValue.Elem()
	reqType := reqValue.Type()

	for i := 0; i < reqType.NumField(); i++ {
		field := reqType.Field(i)
		if field.PkgPath != "" {
			continue
		}
		jsonName := field.Tag.Get("json")
		if idx := strings.IndexByte(jsonName, ','); idx >= 0 {
			jsonName = jsonName[:idx]
		}
		jsonName = strings.TrimSpace(jsonName)
		if jsonName == "" || jsonName == "-" {
			continue
		}
		if jsonName == "site_logo" {
			continue
		}
		if _, ok := rawFields[jsonName]; ok {
			continue
		}
		if !preserveOmittedSettingsFieldType(field.Type) {
			continue
		}

		reqField := reqValue.Field(i)
		if !reqField.CanSet() {
			continue
		}
		previousField := previousValue.FieldByName(field.Name)
		if !previousField.IsValid() || !previousField.Type().AssignableTo(field.Type) {
			continue
		}
		reqField.Set(previousField)
	}
}

func preserveOmittedSettingsFieldType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool,
		reflect.String,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Slice,
		reflect.Map:
		return true
	default:
		return false
	}
}

func stringJSONFieldOrDefault(rawFields map[string]json.RawMessage, field, value, fallback string) string {
	if _, ok := rawFields[field]; !ok {
		return fallback
	}
	return value
}

func stringJSONFieldOrFirstNonEmpty(rawFields map[string]json.RawMessage, field, value string, fallbacks ...string) string {
	if _, ok := rawFields[field]; ok {
		return strings.TrimSpace(value)
	}
	values := make([]string, 0, len(fallbacks)+1)
	values = append(values, value)
	values = append(values, fallbacks...)
	return strings.TrimSpace(firstNonEmpty(values...))
}

func intJSONFieldOrDefault(rawFields map[string]json.RawMessage, field string, value, fallback int) int {
	if _, ok := rawFields[field]; !ok {
		return fallback
	}
	return value
}

func boolJSONFieldOrDefault(rawFields map[string]json.RawMessage, field string, value, fallback bool) bool {
	if _, ok := rawFields[field]; !ok {
		return fallback
	}
	return value
}

func defaultSubscriptionsValueOrDefault(input *[]dto.DefaultSubscriptionSetting, fallback []service.DefaultSubscriptionSetting) []service.DefaultSubscriptionSetting {
	if input == nil {
		return fallback
	}
	result := make([]service.DefaultSubscriptionSetting, 0, len(*input))
	for _, item := range *input {
		result = append(result, service.DefaultSubscriptionSetting{
			GroupID:      item.GroupID,
			ValidityDays: item.ValidityDays,
		})
	}
	return result
}

// platformQuotasValueOrDefault 处理 auth-source platform quota 的 nil 语义：
// nil = 请求未包含该字段（保留 fallback），non-nil（含 empty map）= 整体覆盖。
// 注意：JSON null 与字段省略等价——两者均反序列化为 nil map，因此都保留旧值；
// 若要清空某 source 的所有 quota 配置，须显式发空对象 {}。
func platformQuotasValueOrDefault(value, fallback map[string]*service.DefaultPlatformQuotaSetting) map[string]*service.DefaultPlatformQuotaSetting {
	if value == nil {
		return fallback
	}
	return value
}

func systemSettingsResponseData(settings dto.SystemSettings, authSourceDefaults *service.AuthSourceDefaultSettings) map[string]any {
	data := make(map[string]any)
	raw, err := json.Marshal(settings)
	if err == nil {
		_ = json.Unmarshal(raw, &data)
	}
	if authSourceDefaults == nil {
		authSourceDefaults = &service.AuthSourceDefaultSettings{}
	}

	data["auth_source_default_email_balance"] = authSourceDefaults.Email.Balance
	data["auth_source_default_email_concurrency"] = authSourceDefaults.Email.Concurrency
	data["auth_source_default_email_subscriptions"] = authSourceDefaults.Email.Subscriptions
	data["auth_source_default_email_grant_on_signup"] = authSourceDefaults.Email.GrantOnSignup
	data["auth_source_default_email_grant_on_first_bind"] = authSourceDefaults.Email.GrantOnFirstBind
	data["auth_source_default_linuxdo_balance"] = authSourceDefaults.LinuxDo.Balance
	data["auth_source_default_linuxdo_concurrency"] = authSourceDefaults.LinuxDo.Concurrency
	data["auth_source_default_linuxdo_subscriptions"] = authSourceDefaults.LinuxDo.Subscriptions
	data["auth_source_default_linuxdo_grant_on_signup"] = authSourceDefaults.LinuxDo.GrantOnSignup
	data["auth_source_default_linuxdo_grant_on_first_bind"] = authSourceDefaults.LinuxDo.GrantOnFirstBind
	data["auth_source_default_oidc_balance"] = authSourceDefaults.OIDC.Balance
	data["auth_source_default_oidc_concurrency"] = authSourceDefaults.OIDC.Concurrency
	data["auth_source_default_oidc_subscriptions"] = authSourceDefaults.OIDC.Subscriptions
	data["auth_source_default_oidc_grant_on_signup"] = authSourceDefaults.OIDC.GrantOnSignup
	data["auth_source_default_oidc_grant_on_first_bind"] = authSourceDefaults.OIDC.GrantOnFirstBind
	data["auth_source_default_wechat_balance"] = authSourceDefaults.WeChat.Balance
	data["auth_source_default_wechat_concurrency"] = authSourceDefaults.WeChat.Concurrency
	data["auth_source_default_wechat_subscriptions"] = authSourceDefaults.WeChat.Subscriptions
	data["auth_source_default_wechat_grant_on_signup"] = authSourceDefaults.WeChat.GrantOnSignup
	data["auth_source_default_wechat_grant_on_first_bind"] = authSourceDefaults.WeChat.GrantOnFirstBind
	data["auth_source_default_github_balance"] = authSourceDefaults.GitHub.Balance
	data["auth_source_default_github_concurrency"] = authSourceDefaults.GitHub.Concurrency
	data["auth_source_default_github_subscriptions"] = authSourceDefaults.GitHub.Subscriptions
	data["auth_source_default_github_grant_on_signup"] = authSourceDefaults.GitHub.GrantOnSignup
	data["auth_source_default_github_grant_on_first_bind"] = authSourceDefaults.GitHub.GrantOnFirstBind
	data["auth_source_default_google_balance"] = authSourceDefaults.Google.Balance
	data["auth_source_default_google_concurrency"] = authSourceDefaults.Google.Concurrency
	data["auth_source_default_google_subscriptions"] = authSourceDefaults.Google.Subscriptions
	data["auth_source_default_google_grant_on_signup"] = authSourceDefaults.Google.GrantOnSignup
	data["auth_source_default_google_grant_on_first_bind"] = authSourceDefaults.Google.GrantOnFirstBind
	data["auth_source_default_dingtalk_balance"] = authSourceDefaults.DingTalk.Balance
	data["auth_source_default_dingtalk_concurrency"] = authSourceDefaults.DingTalk.Concurrency
	data["auth_source_default_dingtalk_subscriptions"] = authSourceDefaults.DingTalk.Subscriptions
	data["auth_source_default_dingtalk_grant_on_signup"] = authSourceDefaults.DingTalk.GrantOnSignup
	data["auth_source_default_dingtalk_grant_on_first_bind"] = authSourceDefaults.DingTalk.GrantOnFirstBind
	data["auth_source_default_email_platform_quotas"] = authSourceDefaults.Email.PlatformQuotas
	data["auth_source_default_linuxdo_platform_quotas"] = authSourceDefaults.LinuxDo.PlatformQuotas
	data["auth_source_default_oidc_platform_quotas"] = authSourceDefaults.OIDC.PlatformQuotas
	data["auth_source_default_wechat_platform_quotas"] = authSourceDefaults.WeChat.PlatformQuotas
	data["auth_source_default_github_platform_quotas"] = authSourceDefaults.GitHub.PlatformQuotas
	data["auth_source_default_google_platform_quotas"] = authSourceDefaults.Google.PlatformQuotas
	data["auth_source_default_dingtalk_platform_quotas"] = authSourceDefaults.DingTalk.PlatformQuotas
	data["force_email_on_third_party_signup"] = authSourceDefaults.ForceEmailOnThirdPartySignup

	return data
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalDefaultSubscriptions(a, b []service.DefaultSubscriptionSetting) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].GroupID != b[i].GroupID || a[i].ValidityDays != b[i].ValidityDays {
			return false
		}
	}
	return true
}

func equalLoginAgreementDocuments(a, b []service.LoginAgreementDocument) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Title != b[i].Title || a[i].ContentMD != b[i].ContentMD {
			return false
		}
	}
	return true
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalNotifyEmailEntries(a, b []service.NotifyEmailEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Email != b[i].Email || a[i].Verified != b[i].Verified || a[i].Disabled != b[i].Disabled {
			return false
		}
	}
	return true
}

// TestSMTPRequest 测试SMTP连接请求
type TestSMTPRequest struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`
}

// TestSMTPConnection 测试SMTP连接
// POST /api/v1/admin/settings/test-smtp
func (h *SettingHandler) TestSMTPConnection(c *gin.Context) {
	var req TestSMTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)

	var savedConfig *service.SMTPConfig
	if cfg, err := h.emailService.GetSMTPConfig(c.Request.Context()); err == nil && cfg != nil {
		savedConfig = cfg
	}

	if req.SMTPHost == "" && savedConfig != nil {
		req.SMTPHost = savedConfig.Host
	}
	if req.SMTPPort <= 0 {
		if savedConfig != nil && savedConfig.Port > 0 {
			req.SMTPPort = savedConfig.Port
		} else {
			req.SMTPPort = 587
		}
	}
	if req.SMTPUsername == "" && savedConfig != nil {
		req.SMTPUsername = savedConfig.Username
	}
	password := strings.TrimSpace(req.SMTPPassword)
	if password == "" && savedConfig != nil {
		password = savedConfig.Password
	}
	if req.SMTPHost == "" {
		response.BadRequest(c, "SMTP host is required")
		return
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		UseTLS:   req.SMTPUseTLS,
	}

	err := h.emailService.TestSMTPConnectionWithConfig(config)
	if err != nil {
		response.BadRequest(c, "SMTP connection test failed: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "SMTP connection successful"})
}

// SendTestEmailRequest 发送测试邮件请求
type SendTestEmailRequest struct {
	Email        string `json:"email" binding:"required,email"`
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from_email"`
	SMTPFromName string `json:"smtp_from_name"`
	SMTPUseTLS   bool   `json:"smtp_use_tls"`
}

// SendTestEmail 发送测试邮件
// POST /api/v1/admin/settings/send-test-email
func (h *SettingHandler) SendTestEmail(c *gin.Context) {
	var req SendTestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	req.SMTPHost = strings.TrimSpace(req.SMTPHost)
	req.SMTPUsername = strings.TrimSpace(req.SMTPUsername)
	req.SMTPFrom = strings.TrimSpace(req.SMTPFrom)
	req.SMTPFromName = strings.TrimSpace(req.SMTPFromName)

	var savedConfig *service.SMTPConfig
	if cfg, err := h.emailService.GetSMTPConfig(c.Request.Context()); err == nil && cfg != nil {
		savedConfig = cfg
	}

	if req.SMTPHost == "" && savedConfig != nil {
		req.SMTPHost = savedConfig.Host
	}
	if req.SMTPPort <= 0 {
		if savedConfig != nil && savedConfig.Port > 0 {
			req.SMTPPort = savedConfig.Port
		} else {
			req.SMTPPort = 587
		}
	}
	if req.SMTPUsername == "" && savedConfig != nil {
		req.SMTPUsername = savedConfig.Username
	}
	password := strings.TrimSpace(req.SMTPPassword)
	if password == "" && savedConfig != nil {
		password = savedConfig.Password
	}
	if req.SMTPFrom == "" && savedConfig != nil {
		req.SMTPFrom = savedConfig.From
	}
	if req.SMTPFromName == "" && savedConfig != nil {
		req.SMTPFromName = savedConfig.FromName
	}
	if req.SMTPHost == "" {
		response.BadRequest(c, "SMTP host is required")
		return
	}

	config := &service.SMTPConfig{
		Host:     req.SMTPHost,
		Port:     req.SMTPPort,
		Username: req.SMTPUsername,
		Password: password,
		From:     req.SMTPFrom,
		FromName: req.SMTPFromName,
		UseTLS:   req.SMTPUseTLS,
	}

	siteName := h.settingService.GetSiteName(c.Request.Context())
	subject := "[" + siteName + "] Test Email"
	body := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background-color: #f5f5f5; margin: 0; padding: 20px; }
        .container { max-width: 600px; margin: 0 auto; background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; }
        .content { padding: 40px 30px; text-align: center; }
        .success { color: #10b981; font-size: 48px; margin-bottom: 20px; }
        .footer { background-color: #f8f9fa; padding: 20px; text-align: center; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>` + siteName + `</h1>
        </div>
        <div class="content">
            <div class="success">✓</div>
            <h2>Email Configuration Successful!</h2>
            <p>This is a test email to verify your SMTP settings are working correctly.</p>
        </div>
        <div class="footer">
            <p>This is an automated test message.</p>
        </div>
    </div>
</body>
</html>
`

	if err := h.emailService.SendEmailWithConfig(config, req.Email, subject, body); err != nil {
		response.BadRequest(c, "Failed to send test email: "+err.Error())
		return
	}

	response.Success(c, gin.H{"message": "Test email sent successfully"})
}

// GetAdminAPIKey 获取管理员 API Key 状态
// GET /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) GetAdminAPIKey(c *gin.Context) {
	maskedKey, exists, err := h.settingService.GetAdminAPIKeyStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"exists":     exists,
		"masked_key": maskedKey,
	})
}

// RegenerateAdminAPIKey 生成/重新生成管理员 API Key
// POST /api/v1/admin/settings/admin-api-key/regenerate
func (h *SettingHandler) RegenerateAdminAPIKey(c *gin.Context) {
	key, err := h.settingService.GenerateAdminAPIKey(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"key": key, // 完整 key 只在生成时返回一次
	})
}

// DeleteAdminAPIKey 删除管理员 API Key
// DELETE /api/v1/admin/settings/admin-api-key
func (h *SettingHandler) DeleteAdminAPIKey(c *gin.Context) {
	if err := h.settingService.DeleteAdminAPIKey(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Admin API key deleted"})
}

// GetOverloadCooldownSettings 获取529过载冷却配置
// GET /api/v1/admin/settings/overload-cooldown
func (h *SettingHandler) GetOverloadCooldownSettings(c *gin.Context) {
	settings, err := h.settingService.GetOverloadCooldownSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.OverloadCooldownSettings{
		Enabled:         settings.Enabled,
		CooldownMinutes: settings.CooldownMinutes,
	})
}

// UpdateOverloadCooldownSettingsRequest 更新529过载冷却配置请求
type UpdateOverloadCooldownSettingsRequest struct {
	Enabled         bool `json:"enabled"`
	CooldownMinutes int  `json:"cooldown_minutes"`
}

// UpdateOverloadCooldownSettings 更新529过载冷却配置
// PUT /api/v1/admin/settings/overload-cooldown
func (h *SettingHandler) UpdateOverloadCooldownSettings(c *gin.Context) {
	var req UpdateOverloadCooldownSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.OverloadCooldownSettings{
		Enabled:         req.Enabled,
		CooldownMinutes: req.CooldownMinutes,
	}

	if err := h.settingService.SetOverloadCooldownSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updatedSettings, err := h.settingService.GetOverloadCooldownSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.OverloadCooldownSettings{
		Enabled:         updatedSettings.Enabled,
		CooldownMinutes: updatedSettings.CooldownMinutes,
	})
}

// GetRateLimit429CooldownSettings 获取429默认回避配置
// GET /api/v1/admin/settings/rate-limit-429-cooldown
func (h *SettingHandler) GetRateLimit429CooldownSettings(c *gin.Context) {
	settings, err := h.settingService.GetRateLimit429CooldownSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.RateLimit429CooldownSettings{
		Enabled:         settings.Enabled,
		CooldownSeconds: settings.CooldownSeconds,
	})
}

// UpdateRateLimit429CooldownSettingsRequest 更新429默认回避配置请求
type UpdateRateLimit429CooldownSettingsRequest struct {
	Enabled         bool `json:"enabled"`
	CooldownSeconds int  `json:"cooldown_seconds"`
}

// UpdateRateLimit429CooldownSettings 更新429默认回避配置
// PUT /api/v1/admin/settings/rate-limit-429-cooldown
func (h *SettingHandler) UpdateRateLimit429CooldownSettings(c *gin.Context) {
	var req UpdateRateLimit429CooldownSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.RateLimit429CooldownSettings{
		Enabled:         req.Enabled,
		CooldownSeconds: req.CooldownSeconds,
	}

	if err := h.settingService.SetRateLimit429CooldownSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updatedSettings, err := h.settingService.GetRateLimit429CooldownSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.RateLimit429CooldownSettings{
		Enabled:         updatedSettings.Enabled,
		CooldownSeconds: updatedSettings.CooldownSeconds,
	})
}

// GetStreamTimeoutSettings 获取流超时处理配置
// GET /api/v1/admin/settings/stream-timeout
func (h *SettingHandler) GetStreamTimeoutSettings(c *gin.Context) {
	settings, err := h.settingService.GetStreamTimeoutSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.StreamTimeoutSettings{
		Enabled:                settings.Enabled,
		Action:                 settings.Action,
		TempUnschedMinutes:     settings.TempUnschedMinutes,
		ThresholdCount:         settings.ThresholdCount,
		ThresholdWindowMinutes: settings.ThresholdWindowMinutes,
	})
}

// GetRectifierSettings 获取请求整流器配置
// GET /api/v1/admin/settings/rectifier
func (h *SettingHandler) GetRectifierSettings(c *gin.Context) {
	settings, err := h.settingService.GetRectifierSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	patterns := settings.APIKeySignaturePatterns
	if patterns == nil {
		patterns = []string{}
	}
	response.Success(c, dto.RectifierSettings{
		Enabled:                  settings.Enabled,
		ThinkingSignatureEnabled: settings.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    settings.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   settings.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  patterns,
	})
}

// UpdateRectifierSettingsRequest 更新整流器配置请求
type UpdateRectifierSettingsRequest struct {
	Enabled                  bool     `json:"enabled"`
	ThinkingSignatureEnabled bool     `json:"thinking_signature_enabled"`
	ThinkingBudgetEnabled    bool     `json:"thinking_budget_enabled"`
	APIKeySignatureEnabled   bool     `json:"apikey_signature_enabled"`
	APIKeySignaturePatterns  []string `json:"apikey_signature_patterns"`
}

// UpdateRectifierSettings 更新请求整流器配置
// PUT /api/v1/admin/settings/rectifier
func (h *SettingHandler) UpdateRectifierSettings(c *gin.Context) {
	var req UpdateRectifierSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// 校验并清理自定义匹配关键词
	const maxPatterns = 50
	const maxPatternLen = 500
	if len(req.APIKeySignaturePatterns) > maxPatterns {
		response.BadRequest(c, "Too many signature patterns (max 50)")
		return
	}
	var cleanedPatterns []string
	for _, p := range req.APIKeySignaturePatterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(p) > maxPatternLen {
			response.BadRequest(c, "Signature pattern too long (max 500 characters)")
			return
		}
		cleanedPatterns = append(cleanedPatterns, p)
	}

	settings := &service.RectifierSettings{
		Enabled:                  req.Enabled,
		ThinkingSignatureEnabled: req.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    req.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   req.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  cleanedPatterns,
	}

	if err := h.settingService.SetRectifierSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetRectifierSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	updatedPatterns := updatedSettings.APIKeySignaturePatterns
	if updatedPatterns == nil {
		updatedPatterns = []string{}
	}
	response.Success(c, dto.RectifierSettings{
		Enabled:                  updatedSettings.Enabled,
		ThinkingSignatureEnabled: updatedSettings.ThinkingSignatureEnabled,
		ThinkingBudgetEnabled:    updatedSettings.ThinkingBudgetEnabled,
		APIKeySignatureEnabled:   updatedSettings.APIKeySignatureEnabled,
		APIKeySignaturePatterns:  updatedPatterns,
	})
}

// GetBetaPolicySettings 获取 Beta 策略配置
// GET /api/v1/admin/settings/beta-policy
func (h *SettingHandler) GetBetaPolicySettings(c *gin.Context) {
	settings, err := h.settingService.GetBetaPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	rules := make([]dto.BetaPolicyRule, len(settings.Rules))
	for i, r := range settings.Rules {
		rules[i] = dto.BetaPolicyRule(r)
	}
	response.Success(c, dto.BetaPolicySettings{Rules: rules})
}

// UpdateBetaPolicySettingsRequest 更新 Beta 策略配置请求
type UpdateBetaPolicySettingsRequest struct {
	Rules []dto.BetaPolicyRule `json:"rules"`
}

// UpdateBetaPolicySettings 更新 Beta 策略配置
// PUT /api/v1/admin/settings/beta-policy
func (h *SettingHandler) UpdateBetaPolicySettings(c *gin.Context) {
	var req UpdateBetaPolicySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	rules := make([]service.BetaPolicyRule, len(req.Rules))
	for i, r := range req.Rules {
		rules[i] = service.BetaPolicyRule(r)
	}

	settings := &service.BetaPolicySettings{Rules: rules}
	if err := h.settingService.SetBetaPolicySettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Re-fetch to return updated settings
	updated, err := h.settingService.GetBetaPolicySettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	outRules := make([]dto.BetaPolicyRule, len(updated.Rules))
	for i, r := range updated.Rules {
		outRules[i] = dto.BetaPolicyRule(r)
	}
	response.Success(c, dto.BetaPolicySettings{Rules: outRules})
}

// UpdateStreamTimeoutSettingsRequest 更新流超时配置请求
type UpdateStreamTimeoutSettingsRequest struct {
	Enabled                bool   `json:"enabled"`
	Action                 string `json:"action"`
	TempUnschedMinutes     int    `json:"temp_unsched_minutes"`
	ThresholdCount         int    `json:"threshold_count"`
	ThresholdWindowMinutes int    `json:"threshold_window_minutes"`
}

// UpdateStreamTimeoutSettings 更新流超时处理配置
// PUT /api/v1/admin/settings/stream-timeout
func (h *SettingHandler) UpdateStreamTimeoutSettings(c *gin.Context) {
	var req UpdateStreamTimeoutSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.StreamTimeoutSettings{
		Enabled:                req.Enabled,
		Action:                 req.Action,
		TempUnschedMinutes:     req.TempUnschedMinutes,
		ThresholdCount:         req.ThresholdCount,
		ThresholdWindowMinutes: req.ThresholdWindowMinutes,
	}

	if err := h.settingService.SetStreamTimeoutSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 重新获取设置返回
	updatedSettings, err := h.settingService.GetStreamTimeoutSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.StreamTimeoutSettings{
		Enabled:                updatedSettings.Enabled,
		Action:                 updatedSettings.Action,
		TempUnschedMinutes:     updatedSettings.TempUnschedMinutes,
		ThresholdCount:         updatedSettings.ThresholdCount,
		ThresholdWindowMinutes: updatedSettings.ThresholdWindowMinutes,
	})
}

// GetTempUnschedThresholdSettings 获取临时不可调度规则窗口阈值配置
// GET /api/v1/admin/settings/temp-unsched-threshold
func (h *SettingHandler) GetTempUnschedThresholdSettings(c *gin.Context) {
	settings, err := h.settingService.GetTempUnschedThresholdSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.TempUnschedThresholdSettings{
		Enabled:                settings.Enabled,
		ThresholdCount:         settings.ThresholdCount,
		ThresholdWindowMinutes: settings.ThresholdWindowMinutes,
	})
}

// UpdateTempUnschedThresholdSettingsRequest 更新临时不可调度窗口阈值配置请求
type UpdateTempUnschedThresholdSettingsRequest struct {
	Enabled                bool `json:"enabled"`
	ThresholdCount         int  `json:"threshold_count"`
	ThresholdWindowMinutes int  `json:"threshold_window_minutes"`
}

// UpdateTempUnschedThresholdSettings 更新临时不可调度规则窗口阈值配置
// PUT /api/v1/admin/settings/temp-unsched-threshold
func (h *SettingHandler) UpdateTempUnschedThresholdSettings(c *gin.Context) {
	var req UpdateTempUnschedThresholdSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.TempUnschedThresholdSettings{
		Enabled:                req.Enabled,
		ThresholdCount:         req.ThresholdCount,
		ThresholdWindowMinutes: req.ThresholdWindowMinutes,
	}

	if err := h.settingService.SetTempUnschedThresholdSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updatedSettings, err := h.settingService.GetTempUnschedThresholdSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.TempUnschedThresholdSettings{
		Enabled:                updatedSettings.Enabled,
		ThresholdCount:         updatedSettings.ThresholdCount,
		ThresholdWindowMinutes: updatedSettings.ThresholdWindowMinutes,
	})
}

// GetWebSearchEmulationConfig 获取 Web Search 模拟配置
// GET /api/v1/admin/settings/web-search-emulation
func (h *SettingHandler) GetWebSearchEmulationConfig(c *gin.Context) {
	cfg, err := h.settingService.GetWebSearchEmulationConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.PopulateWebSearchUsage(c.Request.Context(), cfg))
}

// UpdateWebSearchEmulationConfig 更新 Web Search 模拟配置
// PUT /api/v1/admin/settings/web-search-emulation
func (h *SettingHandler) UpdateWebSearchEmulationConfig(c *gin.Context) {
	var cfg service.WebSearchEmulationConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.settingService.SaveWebSearchEmulationConfig(c.Request.Context(), &cfg); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Re-read (with sanitized api keys) to return current state
	updated, err := h.settingService.GetWebSearchEmulationConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, service.PopulateWebSearchUsage(c.Request.Context(), updated))
}

// ResetWebSearchUsage 重置指定 provider 的配额用量
// POST /api/v1/admin/settings/web-search-emulation/reset-usage
func (h *SettingHandler) ResetWebSearchUsage(c *gin.Context) {
	var req struct {
		ProviderType string `json:"provider_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.ProviderType == "" {
		response.BadRequest(c, "provider_type is required")
		return
	}
	if err := service.ResetWebSearchUsage(c.Request.Context(), req.ProviderType); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

// TestWebSearchEmulation 测试 Web Search 搜索
// POST /api/v1/admin/settings/web-search-emulation/test
func (h *SettingHandler) TestWebSearchEmulation(c *gin.Context) {
	var req struct {
		Query string `json:"query"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		req.Query = "搜索今年世界大事件"
	}

	result, err := service.TestWebSearch(c.Request.Context(), req.Query)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// equalNullableFloat compares two *float64 values treating nil as a distinct case.
func equalNullableFloat(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// slotOf returns the *float64 for the given window from a DefaultPlatformQuotaSetting.
func slotOf(s *service.DefaultPlatformQuotaSetting, win string) *float64 {
	if s == nil {
		return nil
	}
	switch win {
	case "daily":
		return s.DailyLimitUSD
	case "weekly":
		return s.WeeklyLimitUSD
	case "monthly":
		return s.MonthlyLimitUSD
	}
	return nil
}

// equalPlatformQuotaSettings reports whether two platform-quota maps are identical across all 12 slots.
func equalPlatformQuotaSettings(before, after map[string]*service.DefaultPlatformQuotaSetting) bool {
	for _, platform := range service.AllowedQuotaPlatforms {
		b := before[platform]
		a := after[platform]
		if !equalNullableFloat(slotOf(b, "daily"), slotOf(a, "daily")) {
			return false
		}
		if !equalNullableFloat(slotOf(b, "weekly"), slotOf(a, "weekly")) {
			return false
		}
		if !equalNullableFloat(slotOf(b, "monthly"), slotOf(a, "monthly")) {
			return false
		}
	}
	return true
}

func equalAccountSchedulingThresholds(before, after map[string]int) bool {
	for _, platform := range service.AllowedSchedulingThresholdPlatforms {
		beforeValue := 100
		if before != nil {
			if value, ok := before[platform]; ok {
				beforeValue = value
			}
		}
		afterValue := 100
		if after != nil {
			if value, ok := after[platform]; ok {
				afterValue = value
			}
		}
		if beforeValue != afterValue {
			return false
		}
	}
	return true
}
