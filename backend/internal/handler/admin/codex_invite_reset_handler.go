package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// CodexInviteResetHandler 处理 Codex 邀请重置管理接口。
type CodexInviteResetHandler struct {
	service *service.CodexInviteResetService
}

// NewCodexInviteResetHandler 创建 Codex 邀请重置管理处理器。
func NewCodexInviteResetHandler(service *service.CodexInviteResetService) *CodexInviteResetHandler {
	return &CodexInviteResetHandler{service: service}
}

type codexInviteResetInviteRequest struct {
	Emails []string `json:"emails" binding:"required"`
}

type codexInviteResetConsumeRequest struct {
	CreditID string `json:"credit_id" binding:"required"`
}

// GetStatus 查询当前账号的邀请资格和可用重置次数。
func (h *CodexInviteResetHandler) GetStatus(c *gin.Context) {
	accountID, ok := parseCodexInviteResetAccountID(c)
	if !ok {
		return
	}
	result, err := h.service.GetStatus(c.Request.Context(), accountID)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// SendInvite 发送 Codex 邀请邮件。
func (h *CodexInviteResetHandler) SendInvite(c *gin.Context) {
	accountID, ok := parseCodexInviteResetAccountID(c)
	if !ok {
		return
	}
	var req codexInviteResetInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.SendInvite(c.Request.Context(), accountID, req.Emails, codexInviteResetOperatorID(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// Consume 使用一次 Codex 重置机会。
func (h *CodexInviteResetHandler) Consume(c *gin.Context) {
	accountID, ok := parseCodexInviteResetAccountID(c)
	if !ok {
		return
	}
	var req codexInviteResetConsumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.Consume(c.Request.Context(), accountID, req.CreditID, codexInviteResetOperatorID(c))
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, result)
}

// ListHistory 分页查询当前账号的邀请/重置操作流水。
func (h *CodexInviteResetHandler) ListHistory(c *gin.Context) {
	accountID, ok := parseCodexInviteResetAccountID(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.ListHistory(c.Request.Context(), accountID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if response.ErrorFrom(c, err) {
		return
	}
	response.Paginated(c, items, result.Total, page, pageSize)
}

func parseCodexInviteResetAccountID(c *gin.Context) (int64, bool) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return 0, false
	}
	return accountID, true
}

// codexInviteResetOperatorID 取当前操作管理员 ID；取不到返回 nil（视为系统触发）。
func codexInviteResetOperatorID(c *gin.Context) *int64 {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		return nil
	}
	id := subject.UserID
	return &id
}
