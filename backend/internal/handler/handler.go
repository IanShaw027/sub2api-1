package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
)

// AdminHandlers contains all admin-related HTTP handlers
type AdminHandlers struct {
	AI                     *admin.AIHandler
	Dashboard              *admin.DashboardHandler
	User                   *admin.UserHandler
	Group                  *admin.GroupHandler
	Account                *admin.AccountHandler
	Announcement           *admin.AnnouncementHandler
	DataManagement         *admin.DataManagementHandler
	Backup                 *admin.BackupHandler
	OAuth                  *admin.OAuthHandler
	OpenAIOAuth            *admin.OpenAIOAuthHandler
	GeminiOAuth            *admin.GeminiOAuthHandler
	AntigravityOAuth       *admin.AntigravityOAuthHandler
	KiroOAuth              *admin.KiroOAuthHandler
	GrokOAuth              *admin.GrokOAuthHandler
	Proxy                  *admin.ProxyHandler
	Redeem                 *admin.RedeemHandler
	Promo                  *admin.PromoHandler
	Setting                *admin.SettingHandler
	Compliance             *admin.ComplianceHandler
	Ops                    *admin.OpsHandler
	System                 *admin.SystemHandler
	Subscription           *admin.SubscriptionHandler
	Affiliate              *admin.AffiliateHandler
	Ticket                 *admin.TicketHandler
	Media                  *admin.MediaHandler
	Usage                  *admin.UsageHandler
	UserAttribute          *admin.UserAttributeHandler
	ErrorPassthrough       *admin.ErrorPassthroughHandler
	TLSFingerprintProfile  *admin.TLSFingerprintProfileHandler
	TLSFingerprintRouter   *admin.TLSFingerprintRouterHandler
	APIKey                 *admin.AdminAPIKeyHandler
	ScheduledTest          *admin.ScheduledTestHandler
	Channel                *admin.ChannelHandler
	ChannelMonitor         *admin.ChannelMonitorHandler
	ChannelMonitorTemplate *admin.ChannelMonitorRequestTemplateHandler
	ContentModeration      *admin.ContentModerationHandler
	IPSecurity             *admin.IPSecurityHandler
	Payment                *admin.PaymentHandler
	CodexInviteReset       *admin.CodexInviteResetHandler
}

// Handlers contains all HTTP handlers
type Handlers struct {
	Auth             *AuthHandler
	AI               *AIHandler
	User             *UserHandler
	APIKey           *APIKeyHandler
	Usage            *UsageHandler
	Redeem           *RedeemHandler
	Subscription     *SubscriptionHandler
	Ticket           *TicketHandler
	Media            *MediaHandler
	Announcement     *AnnouncementHandler
	ChannelMonitor   *ChannelMonitorUserHandler
	Admin            *AdminHandlers
	Gateway          *GatewayHandler
	OpenAIGateway    *OpenAIGatewayHandler
	Setting          *SettingHandler
	Totp             *TotpHandler
	Payment          *PaymentHandler
	PaymentWebhook   *PaymentWebhookHandler
	AvailableChannel *AvailableChannelHandler
	BatchImage       *BatchImageHandler
}

// BuildInfo contains build-time information
type BuildInfo struct {
	Version   string
	BuildType string // "source" for manual builds, "release" for CI builds
}
