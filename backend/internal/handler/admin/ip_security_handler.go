package admin

import (
	"database/sql"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// IPSecurityHandler exposes the configurable multi-account IP security controls.
type IPSecurityHandler struct {
	service *service.IPSecurityService
}

func NewIPSecurityHandler(svc *service.IPSecurityService) *IPSecurityHandler {
	return &IPSecurityHandler{service: svc}
}

func (h *IPSecurityHandler) GetConfig(c *gin.Context) {
	response.Success(c, h.service.GetConfig(c.Request.Context()))
}

func (h *IPSecurityHandler) ListBans(c *gin.Context) {
	status := strings.TrimSpace(c.DefaultQuery("status", "active"))
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	items, total, err := h.service.ListBans(c.Request.Context(), status, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *IPSecurityHandler) GetBan(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid ban id")
		return
	}
	ban, err := h.service.GetBan(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(c, "ban not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	since, until := banActivityWindow(ban, c.Query("days"))
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	activities, total, err := h.service.ListActivityPage(c.Request.Context(), ban.IPAddress, since, until, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	pages := int64(0)
	if total > 0 {
		pages = (total + int64(pageSize) - 1) / int64(pageSize)
	}
	response.Success(c, gin.H{"ban": ban, "activities": activities, "page": page, "page_size": pageSize, "total": total, "pages": pages})
}

type createIPBanRequest struct {
	IPAddress string `json:"ip_address"`
	Reason    string `json:"reason"`
}

func (h *IPSecurityHandler) CreateBan(c *gin.Context) {
	var req createIPBanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	ban, err := h.service.ManualBan(c.Request.Context(), req.IPAddress, req.Reason)
	if err != nil {
		respondIPSecurityMutationError(c, err)
		return
	}
	response.Success(c, ban)
}

func (h *IPSecurityHandler) ReleaseBanByIP(c *gin.Context) {
	ip, ok := pathIP(c)
	if !ok {
		return
	}
	if err := h.service.WhitelistBanByIP(c.Request.Context(), ip, getAdminIDFromContext(c)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(c, "ban not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "IP ban released and address whitelisted"})
}

func (h *IPSecurityHandler) SetUserIPPin(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "invalid user id")
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	state, err := h.service.SetPinKnownIPs(c.Request.Context(), userID, req.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, state)
}

func (h *IPSecurityHandler) AppendUserAllowedIP(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "invalid user id")
		return
	}
	var req struct {
		IP string `json:"ip"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if err := h.service.AppendAllowedIP(c.Request.Context(), userID, req.IP); err != nil {
		respondIPSecurityMutationError(c, err)
		return
	}
	response.Success(c, gin.H{"message": "allowed ip added"})
}

func (h *IPSecurityHandler) GetUserIPSummary(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "invalid user id")
		return
	}
	days, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("days", "30")))
	summary, err := h.service.GetUserIPSummary(c.Request.Context(), userID, days)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(c, "user not found")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

func (h *IPSecurityHandler) ActivateBanByIP(c *gin.Context) {
	ip, ok := pathIP(c)
	if !ok {
		return
	}
	var req createIPBanRequest
	_ = c.ShouldBindJSON(&req)
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "manual: admin re-activate"
	}
	ban, err := h.service.ActivateBanByIP(c.Request.Context(), ip, reason)
	if err != nil {
		respondIPSecurityMutationError(c, err)
		return
	}
	response.Success(c, ban)
}

func respondIPSecurityMutationError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(err.Error())), "invalid public ip") {
		response.BadRequest(c, err.Error())
		return
	}
	response.ErrorFrom(c, err)
}

func pathIP(c *gin.Context) (string, bool) {
	raw := strings.TrimSpace(c.Param("ip"))
	if raw == "" {
		response.BadRequest(c, "invalid ip")
		return "", false
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		decoded = raw
	}
	return decoded, true
}

func banActivityWindow(ban *service.IPSecurityBan, daysRaw string) (time.Time, time.Time) {
	until := time.Now().Add(time.Minute)
	days := 30
	if parsed, err := strconv.Atoi(strings.TrimSpace(daysRaw)); err == nil && parsed > 0 {
		if parsed > 90 {
			parsed = 90
		}
		days = parsed
	}
	since := until.Add(-time.Duration(days) * 24 * time.Hour)
	// Keep auto-detect windows as a floor when they are longer than the default lookback.
	if ban != nil && ban.WindowMinutes > 0 {
		windowSince := ban.LastSeenAt.Add(-time.Duration(ban.WindowMinutes) * time.Minute)
		if windowSince.Before(since) {
			since = windowSince
		}
	}
	return since, until
}

func (h *IPSecurityHandler) ReleaseBan(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid ban id")
		return
	}
	if err := h.service.WhitelistBan(c.Request.Context(), id, getAdminIDFromContext(c)); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "IP ban released and address whitelisted"})
}

func (h *IPSecurityHandler) RemoveWhitelist(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid ban id")
		return
	}
	if err := h.service.RemoveWhitelist(c.Request.Context(), id, getAdminIDFromContext(c)); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "IP address removed from whitelist"})
}
