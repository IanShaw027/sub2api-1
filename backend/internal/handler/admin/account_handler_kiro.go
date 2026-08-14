package admin

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

var fetchKiroAvailableModels = func(ctx context.Context, account *service.Account, provider *service.KiroTokenProvider, usageSvc *service.AccountUsageService) ([]claude.Model, error) {
	if account == nil || provider == nil || usageSvc == nil {
		return nil, fmt.Errorf("kiro token provider is not configured")
	}
	accessToken, err := provider.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	models, err := usageSvc.NewKiroUsageService().FetchAvailableModels(ctx, account, accessToken)
	if err != nil {
		return nil, err
	}
	out := make([]claude.Model, 0, len(models))
	for _, model := range models {
		id := strings.TrimSpace(model.ModelID)
		if id == "" {
			continue
		}
		out = append(out, claude.Model{
			ID:          id,
			Type:        "model",
			DisplayName: id,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("kiro available models empty")
	}
	return out, nil
}

var fetchKiroDiscoveredProfiles = func(ctx context.Context, account *service.Account, provider *service.KiroTokenProvider, usageSvc *service.AccountUsageService) ([]service.KiroAvailableProfile, error) {
	if account == nil || provider == nil || usageSvc == nil {
		return nil, fmt.Errorf("kiro token provider is not configured")
	}
	accessToken, err := provider.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}
	profiles, err := usageSvc.NewKiroUsageService().ResolveAvailableProfiles(ctx, account, accessToken)
	if err != nil {
		return nil, err
	}
	return profiles, nil
}

var setKiroOveragePreference = func(ctx context.Context, account *service.Account, provider *service.KiroTokenProvider, usageSvc *service.AccountUsageService, enabled bool) error {
	if account == nil || provider == nil || usageSvc == nil {
		return fmt.Errorf("kiro token provider is not configured")
	}
	accessToken, err := provider.GetAccessToken(ctx, account)
	if err != nil {
		return err
	}
	return usageSvc.NewKiroUsageService().SetOveragePreference(ctx, account, accessToken, enabled)
}

var fetchAdminKiroUsage = func(ctx context.Context, usageSvc *service.AccountUsageService, accountID int64) (*service.UsageInfo, error) {
	if usageSvc == nil {
		return nil, fmt.Errorf("account usage service is not configured")
	}
	return usageSvc.GetUsage(ctx, accountID)
}

type KiroReauthorizeAccountRequest struct {
	Name        string         `json:"name"`
	Credentials map[string]any `json:"credentials" binding:"required"`
	Extra       map[string]any `json:"extra"`
}

func (h *AccountHandler) SetKiroTokenProvider(provider *service.KiroTokenProvider) {
	if h != nil {
		h.kiroTokenProvider = provider
	}
}

func (h *AccountHandler) ReauthorizeKiroOAuth(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	var req KiroReauthorizeAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if account.Platform != service.PlatformKiro || account.Type != service.AccountTypeOAuth {
		response.BadRequest(c, "account is not a Kiro OAuth account")
		return
	}
	if err := service.ValidateAccountExtraWrites(req.Extra); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if _, err := h.adminService.UpdateAccount(c.Request.Context(), accountID, &service.UpdateAccountInput{
		Name:                      req.Name,
		Type:                      service.AccountTypeOAuth,
		Credentials:               req.Credentials,
		Extra:                     req.Extra,
		AllowSensitiveCredentials: true,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	updated, err := h.adminService.ClearAccountError(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.tokenCacheInvalidator != nil {
		if err := h.tokenCacheInvalidator.InvalidateToken(c.Request.Context(), updated); err != nil {
			log.Printf("[WARN] Failed to invalidate token cache for account %d after Kiro reauthorize: %v", updated.ID, err)
		}
	}
	h.invalidateKiroUsageCache(updated)
	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), updated))
}

func (h *AccountHandler) invalidateKiroUsageCache(account *service.Account) {
	if h == nil || h.accountUsageService == nil || account == nil || account.Platform != service.PlatformKiro {
		return
	}
	h.accountUsageService.InvalidateKiroUsageCache(account.ID)
}

func (h *AccountHandler) GetKiroProfiles(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}
	if account == nil || account.Platform != service.PlatformKiro || account.Type != service.AccountTypeOAuth {
		response.BadRequest(c, "Account does not support Kiro profile discovery")
		return
	}
	profiles, err := fetchKiroDiscoveredProfiles(c.Request.Context(), account, h.kiroTokenProvider, h.accountUsageService)
	if err != nil {
		response.BadRequest(c, "Failed to discover Kiro profiles: "+err.Error())
		return
	}
	response.Success(c, gin.H{"profiles": profiles})
}

type setKiroOverageRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *AccountHandler) SetKiroOverage(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req setKiroOverageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}
	if account == nil || account.Platform != service.PlatformKiro || account.Type != service.AccountTypeOAuth {
		response.BadRequest(c, "Account does not support Kiro overage")
		return
	}
	if err := setKiroOveragePreference(c.Request.Context(), account, h.kiroTokenProvider, h.accountUsageService, req.Enabled); err != nil {
		response.BadRequest(c, "Failed to update Kiro overage: "+err.Error())
		return
	}
	if h.accountUsageService != nil {
		h.accountUsageService.InvalidateKiroUsageCache(accountID)
	}
	response.Success(c, gin.H{"id": accountID, "enabled": req.Enabled})
}

func (h *AccountHandler) EnableAllKiroOverage(c *gin.Context) {
	const pageSize = 500
	type item struct {
		ID     int64  `json:"id"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	results := make([]item, 0)
	enabledCount := 0
	for page, processed, total := 1, int64(0), int64(1); processed < total; page++ {
		accounts, listedTotal, err := h.adminService.ListAccounts(c.Request.Context(), page, pageSize, service.PlatformKiro, service.AccountTypeOAuth, "", "", 0, "", "", "")
		if err != nil {
			response.BadRequest(c, "Failed to list Kiro accounts: "+err.Error())
			return
		}
		total = listedTotal
		if len(accounts) == 0 {
			break
		}
		processed += int64(len(accounts))
		for i := range accounts {
			account := &accounts[i]
			usage, usageErr := fetchAdminKiroUsage(c.Request.Context(), h.accountUsageService, account.ID)
			if usageErr != nil {
				results = append(results, item{ID: account.ID, Status: "usage_error", Error: usageErr.Error()})
				continue
			}
			if usage == nil || !kiroOverageCapabilitySupported(usage.KiroOverageCapability) {
				results = append(results, item{ID: account.ID, Status: "unsupported"})
				continue
			}
			if usage.KiroOverageEnabled != nil && *usage.KiroOverageEnabled {
				results = append(results, item{ID: account.ID, Status: "already_enabled"})
				continue
			}
			if err := setKiroOveragePreference(c.Request.Context(), account, h.kiroTokenProvider, h.accountUsageService, true); err != nil {
				results = append(results, item{ID: account.ID, Status: "set_error", Error: err.Error()})
				continue
			}
			enabledCount++
			if h.accountUsageService != nil {
				h.accountUsageService.InvalidateKiroUsageCache(account.ID)
			}
			results = append(results, item{ID: account.ID, Status: "enabled"})
		}
	}
	response.Success(c, gin.H{
		"enabled_count": enabledCount,
		"results":       results,
	})
}

func kiroOverageCapabilitySupported(capability string) bool {
	switch strings.ToUpper(strings.TrimSpace(capability)) {
	case "SUPPORTED", "ENABLED", "AVAILABLE", "CAPABLE":
		return true
	default:
		return false
	}
}

func respondWithKiroAvailableModels(c *gin.Context, account *service.Account, provider *service.KiroTokenProvider, usageSvc *service.AccountUsageService) bool {
	if account == nil || account.Platform != service.PlatformKiro {
		return false
	}
	mapping := account.GetModelMapping()
	if len(mapping) == 0 {
		if models, err := fetchKiroAvailableModels(c.Request.Context(), account, provider, usageSvc); err == nil && len(models) > 0 {
			response.Success(c, models)
			return true
		}
		response.Success(c, kiro.DefaultModels)
		return true
	}

	var models []claude.Model
	for requestedModel := range mapping {
		var found bool
		for _, dm := range kiro.DefaultModels {
			if dm.ID == requestedModel {
				models = append(models, dm)
				found = true
				break
			}
		}
		if !found {
			models = append(models, claude.Model{
				ID:          requestedModel,
				Type:        "model",
				DisplayName: requestedModel,
			})
		}
	}
	response.Success(c, models)
	return true
}
