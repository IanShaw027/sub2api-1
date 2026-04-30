package admin

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type AffiliateHandler struct {
	affiliateService *service.AffiliateService
}

func NewAffiliateHandler(affiliateService *service.AffiliateService) *AffiliateHandler {
	return &AffiliateHandler{affiliateService: affiliateService}
}

func (h *AffiliateHandler) List(c *gin.Context) {
	if h == nil || h.affiliateService == nil {
		response.InternalError(c, "Affiliate service is not configured")
		return
	}

	page, pageSize := response.ParsePagination(c)
	userTZ := strings.TrimSpace(c.Query("timezone"))
	startAt, ok := parseAffiliateDateQuery(c, "start_date", userTZ)
	if !ok {
		return
	}
	endAt, ok := parseAffiliateDateQuery(c, "end_date", userTZ)
	if !ok {
		return
	}
	if endAt != nil {
		endExclusive := endAt.AddDate(0, 0, 1)
		endAt = &endExclusive
	}
	if startAt != nil && endAt != nil && !startAt.Before(*endAt) {
		response.BadRequest(c, "start_date must be before or equal to end_date")
		return
	}

	items, total, err := h.affiliateService.ListAdminAffiliateStats(c.Request.Context(), service.AdminAffiliateListParams{
		Page:     page,
		PageSize: pageSize,
		Search:   strings.TrimSpace(c.Query("search")),
		StartAt:  startAt,
		EndAt:    endAt,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

func (h *AffiliateHandler) ListInvitees(c *gin.Context) {
	if h == nil || h.affiliateService == nil {
		response.InternalError(c, "Affiliate service is not configured")
		return
	}

	inviterID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	items, err := h.affiliateService.ListAdminInvitees(c.Request.Context(), inviterID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func parseAffiliateDateQuery(c *gin.Context, key string, userTZ string) (*time.Time, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, true
	}
	t, err := timezone.ParseInUserLocation("2006-01-02", raw, userTZ)
	if err != nil {
		response.BadRequest(c, key+" must use YYYY-MM-DD")
		return nil, false
	}
	return &t, true
}
