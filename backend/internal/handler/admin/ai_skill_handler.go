package admin

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type skillAdminActionRequest struct {
	Note   string         `json:"note"`
	Reason string         `json:"reason"`
	Trace  aiTraceRequest `json:"trace"`
}

func (h *AIHandler) ListSkillReviews(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	params := adminSkillPagination(c)
	items, result, err := module.Queries.ListReviews(c.Request.Context(), params, c.Query("review_status"), c.Query("visibility"), c.Query("risk_level"), c.Query("search"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.SkillReviewItem, 0, len(items))
	for _, item := range items {
		out = append(out, dto.SkillReviewItemFromQuery(item))
	}
	response.Success(c, gin.H{
		"items":     out,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
		"pages":     result.Pages,
		"summary":   dto.SkillReviewSummaryFromItems(out),
	})
}

func (h *AIHandler) ApproveSkillReview(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	reviewID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid review ID")
		return
	}
	var req skillAdminActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeAdminIdempotentJSON(c, "skills:reviews:approve", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		review, err := module.DomainService.GetSkillReviewByID(ctx, reviewID)
		if err != nil {
			return nil, err
		}
		version, err := module.ReviewService.ApproveVersion(ctx, subject.UserID, review.VersionID, &service.AIReviewSkillVersionInput{
			Comment: stringPtr(adminActionComment(req)),
			Trace:   buildAITrace(req.Trace),
		})
		if err != nil {
			return nil, err
		}
		return gin.H{
			"message":     "ok",
			"status":      version.Status,
			"operated_at": time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
}

func (h *AIHandler) RejectSkillReview(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	reviewID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid review ID")
		return
	}
	var req skillAdminActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeAdminIdempotentJSON(c, "skills:reviews:reject", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		review, err := module.DomainService.GetSkillReviewByID(ctx, reviewID)
		if err != nil {
			return nil, err
		}
		version, err := module.ReviewService.RejectVersion(ctx, subject.UserID, review.VersionID, &service.AIReviewSkillVersionInput{
			Comment: stringPtr(adminActionComment(req)),
			Trace:   buildAITrace(req.Trace),
		})
		if err != nil {
			return nil, err
		}
		return gin.H{
			"message":     "ok",
			"status":      version.Status,
			"operated_at": time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
}

func (h *AIHandler) ListSkillGovernance(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	params := adminSkillPagination(c)
	items, result, err := module.Queries.ListGovernance(c.Request.Context(), params, c.Query("governance_status"), c.Query("review_status"), c.Query("visibility"), c.Query("search"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.SkillGovernanceItem, 0, len(items))
	for _, item := range items {
		out = append(out, dto.SkillGovernanceItemFromQuery(item))
	}
	response.Success(c, gin.H{
		"items":     out,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
		"pages":     result.Pages,
		"summary":   dto.SkillGovernanceSummaryFromItems(out),
	})
}

func (h *AIHandler) DisableSkill(c *gin.Context) {
	subject, ok := requireAdminAIAuth(c)
	if !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	var req skillAdminActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeAdminIdempotentJSON(c, "skills:disable", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		skill, err := module.DomainService.GetSkillByID(ctx, skillID)
		if err != nil {
			return nil, err
		}
		versionID := pickVersionForAdminAction(skill)
		if versionID != nil {
			if _, err := module.ReviewService.DisableVersion(ctx, subject.UserID, *versionID, &service.AIReviewSkillVersionInput{
				Comment: stringPtr(adminActionComment(req)),
				Trace:   buildAITrace(req.Trace),
			}); err != nil && !errors.Is(err, service.ErrAISkillVersionTransitionInvalid) && !errors.Is(err, service.ErrAISkillVersionNotApproved) {
				return nil, err
			}
		}
		meta := cloneAdminSkillMeta(skill.Metadata)
		meta["governance_status"] = "disabled"
		skill.Visibility = domain.AIVisibilityPrivate
		skill.Metadata = meta
		if err := module.DomainService.UpdateSkill(ctx, skill); err != nil {
			return nil, err
		}
		return gin.H{
			"message":     "ok",
			"status":      "disabled",
			"operated_at": time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
}

func (h *AIHandler) ForcePrivateSkill(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	skillID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid skill ID")
		return
	}
	var req skillAdminActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeAdminIdempotentJSON(c, "skills:force-private", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		skill, err := module.DomainService.GetSkillByID(ctx, skillID)
		if err != nil {
			return nil, err
		}
		meta := cloneAdminSkillMeta(skill.Metadata)
		meta["governance_status"] = "force_private"
		skill.Visibility = domain.AIVisibilityPrivate
		skill.Metadata = meta
		if err := module.DomainService.UpdateSkill(ctx, skill); err != nil {
			return nil, err
		}
		return gin.H{
			"message":     "ok",
			"status":      "force_private",
			"operated_at": time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
}

func (h *AIHandler) ListSkillRuntime(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	params := adminSkillPagination(c)
	items, result, err := module.Queries.ListRuntime(c.Request.Context(), params, c.Query("health_status"), c.Query("search"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	events, err := module.Queries.ListRuntimeEvents(c.Request.Context(), 10)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.SkillRuntimeItem, 0, len(items))
	for _, item := range items {
		out = append(out, dto.SkillRuntimeItemFromQuery(item))
	}
	eventOut := make([]dto.SkillRuntimeEvent, 0, len(events))
	for _, item := range events {
		eventOut = append(eventOut, dto.SkillRuntimeEventFromQuery(item))
	}
	response.Success(c, gin.H{
		"items":     out,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
		"pages":     result.Pages,
		"summary":   dto.SkillRuntimeSummaryFromItems(out),
		"events":    eventOut,
	})
}

func (h *AIHandler) ListSkillSettlements(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	params := adminSkillPagination(c)
	items, result, err := module.Queries.ListSettlements(c.Request.Context(), params, c.Query("settlement_status"), c.Query("search"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.SkillSettlementItem, 0, len(items))
	for _, item := range items {
		out = append(out, dto.SkillSettlementItemFromQuery(item))
	}
	response.Success(c, gin.H{
		"items":     out,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
		"pages":     result.Pages,
		"summary":   dto.SkillSettlementSummaryFromItems(out),
	})
}

func (h *AIHandler) ReplaySkillSettlement(c *gin.Context) {
	if _, ok := requireAdminAIAuth(c); !ok {
		return
	}
	module, err := h.skillModuleOrErr()
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	settlementID, ok := parseAIID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "Invalid settlement ID")
		return
	}
	var req skillAdminActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	executeAdminIdempotentJSON(c, "skills:settlements:replay", gin.H{
		"id":     settlementID,
		"reason": strings.TrimSpace(req.Reason),
		"note":   strings.TrimSpace(req.Note),
		"trace":  req.Trace,
	}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		settlement, err := module.DomainService.GetSkillSettlementByID(ctx, settlementID)
		if err != nil {
			return nil, err
		}
		replayed, err := module.SettlementService.ReplaySettlement(ctx, settlement.RunID)
		if err != nil {
			return nil, err
		}
		return gin.H{
			"message":       "ok",
			"status":        replayed.Status,
			"settlement_id": replayed.ID,
			"run_id":        replayed.RunID,
			"operated_at":   time.Now().UTC().Format(time.RFC3339),
		}, nil
	})
}

func adminSkillPagination(c *gin.Context) pagination.PaginationParams {
	page, pageSize := response.ParsePagination(c)
	return pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "updated_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
}

func adminActionComment(req skillAdminActionRequest) string {
	if strings.TrimSpace(req.Note) != "" {
		return strings.TrimSpace(req.Note)
	}
	return strings.TrimSpace(req.Reason)
}

func pickVersionForAdminAction(skill *domain.AISkill) *int64 {
	if skill == nil {
		return nil
	}
	if skill.CurrentVersionID != nil {
		return skill.CurrentVersionID
	}
	if skill.PublishedVersionID != nil {
		return skill.PublishedVersionID
	}
	return skill.LatestApprovedVersionID
}

func cloneAdminSkillMeta(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func stringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
