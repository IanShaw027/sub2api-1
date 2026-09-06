package handler

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CreationPublicationHandler struct {
	publications *service.CreationPublicationService
}

func NewCreationPublicationHandler(publications *service.CreationPublicationService) *CreationPublicationHandler {
	return &CreationPublicationHandler{publications: publications}
}

func (h *CreationPublicationHandler) Upload(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxCreationPublicationBytes+(1<<20))
	if err := c.Request.ParseMultipartForm(8 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			response.ErrorFrom(c, service.ErrMediaTooLarge)
		} else {
			response.BadRequest(c, "invalid multipart upload")
		}
		return
	}
	defer func() { _ = c.Request.MultipartForm.RemoveAll() }()
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, service.MaxCreationPublicationBytes+1))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	p, err := h.publications.Publish(c.Request.Context(), service.PublishCreationInput{
		OwnerUserID: subject.UserID, RequestID: c.PostForm("request_id"), Title: c.PostForm("title"),
		Prompt: c.PostForm("prompt"), Model: c.PostForm("model"), Kind: c.PostForm("kind"),
		Visibility: c.PostForm("visibility"), Data: data,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}

func (h *CreationPublicationHandler) Status(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	p, err := h.publications.Status(c.Request.Context(), subject.UserID, c.Param("request_id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, p)
}

func (h *CreationPublicationHandler) List(c *gin.Context) {
	page, pageErr := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, sizeErr := strconv.Atoi(c.DefaultQuery("page_size", "24"))
	if pageErr != nil || sizeErr != nil {
		response.ErrorFrom(c, service.ErrCreationPublicationInvalid)
		return
	}
	result, err := h.publications.List(c.Request.Context(), service.CreationPublicationFilter{
		Page: page, PageSize: pageSize, Kind: c.Query("kind"), Search: c.Query("search"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, result)
}

func (h *CreationPublicationHandler) Media(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid publication ID")
		return
	}
	p, data, err := h.publications.Open(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Content-Type", p.MIME)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")
	// ServeContent provides byte ranges needed by video players without exposing
	// presigned URLs that would continue working after the owner withdraws.
	http.ServeContent(c.Writer, c.Request, "creation", p.CreatedAt, bytes.NewReader(data))
}

func (h *CreationPublicationHandler) Delete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid publication ID")
		return
	}
	if err := h.publications.Delete(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}
