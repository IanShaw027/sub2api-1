package admin

import (
	"encoding/json"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// TLSFingerprintRouterHandler 处理 TLS 指纹路由的 HTTP 请求。
type TLSFingerprintRouterHandler struct {
	service *service.TLSFingerprintRouterService
}

// NewTLSFingerprintRouterHandler 创建 TLS 指纹路由处理器。
func NewTLSFingerprintRouterHandler(service *service.TLSFingerprintRouterService) *TLSFingerprintRouterHandler {
	return &TLSFingerprintRouterHandler{service: service}
}

type CreateTLSFingerprintRouterRequest struct {
	Name        string                           `json:"name" binding:"required"`
	Description *string                          `json:"description"`
	Enabled     *bool                            `json:"enabled"`
	Rules       []model.TLSFingerprintRouterRule `json:"rules"`
}

type UpdateTLSFingerprintRouterRequest struct {
	Name        *string                          `json:"name"`
	Description *string                          `json:"description"`
	Enabled     *bool                            `json:"enabled"`
	Rules       []model.TLSFingerprintRouterRule `json:"rules"`
}

func (h *TLSFingerprintRouterHandler) List(c *gin.Context) {
	routers, err := h.service.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, routers)
}

func (h *TLSFingerprintRouterHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid router ID")
		return
	}
	router, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if router == nil {
		response.NotFound(c, "Router not found")
		return
	}
	response.Success(c, router)
}

func (h *TLSFingerprintRouterHandler) Create(c *gin.Context) {
	var req CreateTLSFingerprintRouterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	created, err := h.service.Create(c.Request.Context(), &model.TLSFingerprintRouter{
		Name:        req.Name,
		Description: req.Description,
		Enabled:     enabled,
		Rules:       req.Rules,
	})
	if err != nil {
		if _, ok := err.(*model.ValidationError); ok {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, created)
}

func (h *TLSFingerprintRouterHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid router ID")
		return
	}
	rawBody, err := c.GetRawData()
	if err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var req UpdateTLSFingerprintRouterRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	existing, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if existing == nil {
		response.NotFound(c, "Router not found")
		return
	}
	router := &model.TLSFingerprintRouter{
		ID:          id,
		Name:        existing.Name,
		Description: existing.Description,
		Enabled:     existing.Enabled,
		Rules:       existing.Rules,
	}
	if req.Name != nil {
		router.Name = *req.Name
	}
	if _, ok := raw["description"]; ok {
		router.Description = req.Description
	}
	if req.Enabled != nil {
		router.Enabled = *req.Enabled
	}
	if req.Rules != nil {
		router.Rules = req.Rules
	}
	updated, err := h.service.Update(c.Request.Context(), router)
	if err != nil {
		if _, ok := err.(*model.ValidationError); ok {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *TLSFingerprintRouterHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid router ID")
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Router deleted successfully"})
}
