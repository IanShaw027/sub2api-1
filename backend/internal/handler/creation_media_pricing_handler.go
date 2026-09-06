package handler

import (
	"context"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type creationMediaPricing interface {
	Estimate(context.Context, service.CreationMediaPriceInput) (*service.CreationMediaPrice, error)
	Receipt(context.Context, int64, int64, string, string) (*service.CreationMediaPrice, error)
}

type CreationMediaPricingHandler struct {
	pricing creationMediaPricing
}

func NewCreationMediaPricingHandler(pricing *service.CreationMediaPricingService) *CreationMediaPricingHandler {
	return &CreationMediaPricingHandler{pricing: pricing}
}

func (h *CreationMediaPricingHandler) Estimate(c *gin.Context) {
	userID, groupID, ok := creationMediaPricingIdentity(c)
	if !ok {
		return
	}
	duration := 0
	if raw := c.Query("duration"); raw != "" {
		var err error
		duration, err = strconv.Atoi(raw)
		if err != nil || duration < 0 || duration > 3600 {
			response.BadRequest(c, "Invalid duration")
			return
		}
	}
	price, err := h.pricing.Estimate(c.Request.Context(), service.CreationMediaPriceInput{
		UserID: userID, GroupID: groupID, Kind: c.Query("kind"), Model: strings.TrimSpace(c.Query("model")),
		Size: c.Query("size"), Duration: duration, Resolution: c.Query("resolution"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, price)
}

func (h *CreationMediaPricingHandler) Receipt(c *gin.Context) {
	userID, groupID, ok := creationMediaPricingIdentity(c)
	if !ok {
		return
	}
	price, err := h.pricing.Receipt(c.Request.Context(), userID, groupID, c.Query("kind"), c.Query("task_id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, price)
}

func creationMediaPricingIdentity(c *gin.Context) (int64, int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, 0, false
	}
	groupID, err := strconv.ParseInt(c.Query("group_id"), 10, 64)
	if err != nil || groupID <= 0 {
		response.BadRequest(c, "Invalid group_id")
		return 0, 0, false
	}
	return subject.UserID, groupID, true
}
