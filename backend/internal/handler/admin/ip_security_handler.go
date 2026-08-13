package admin

import (
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
		response.ErrorFrom(c, err)
		return
	}
	since := ban.FirstSeenAt.Add(-1 * time.Minute)
	if ban.WindowMinutes > 0 {
		since = ban.LastSeenAt.Add(-time.Duration(ban.WindowMinutes) * time.Minute)
	}
	until := ban.LastSeenAt.Add(time.Minute)
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
