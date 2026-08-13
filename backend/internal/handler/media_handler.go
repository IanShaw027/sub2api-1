package handler

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// MediaHandler serves the shared media/S3 gateway (public GET vs HMAC private download).
type MediaHandler struct {
	mediaService *service.MediaService
}

func NewMediaHandler(mediaService *service.MediaService) *MediaHandler {
	return &MediaHandler{mediaService: mediaService}
}

type mediaPresignRequest struct {
	TTLMinutes int `json:"ttl_minutes"`
}

// AdminUpload stores a file for an explicit owner (invoice issuance, ticket attachments).
// POST /api/v1/admin/media/upload
func (h *MediaHandler) AdminUpload(c *gin.Context) {
	h.uploadFromMultipart(c, 0, true)
}

func (h *MediaHandler) Upload(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	h.uploadFromMultipart(c, subject.UserID, false)
}

func (h *MediaHandler) uploadFromMultipart(c *gin.Context, ownerUserID int64, admin bool) {
	const multipartOverhead = 1 << 20
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.MaxMediaUploadBytes+multipartOverhead)
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			response.ErrorFrom(c, service.ErrMediaTooLarge)
			return
		}
		response.BadRequest(c, "invalid multipart upload")
		return
	}
	if admin {
		parsed, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("owner_user_id")), 10, 64)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "owner_user_id is required")
			return
		}
		ownerUserID = parsed
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, service.MaxMediaUploadBytes+1))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if int64(len(data)) > service.MaxMediaUploadBytes {
		response.ErrorFrom(c, service.ErrMediaTooLarge)
		return
	}

	filename := c.PostForm("filename")
	if filename == "" && header != nil {
		filename = header.Filename
	}
	asset, err := h.mediaService.Upload(c.Request.Context(), service.UploadMediaInput{
		OwnerUserID:  ownerUserID,
		BizType:      c.PostForm("biz_type"),
		BizID:        c.PostForm("biz_id"),
		Filename:     filename,
		Visibility:   c.PostForm("visibility"),
		ActorIsAdmin: admin,
		Data:         data,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, asset)
}

// PresignDownload issues a 15-minute (1–60) HMAC download URL. Never returns the S3 endpoint.
// POST /api/v1/media/:id/presign-download
func (h *MediaHandler) PresignDownload(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	h.presignForActor(c, subject.UserID, false)
}

// AdminPresignDownload is the admin variant; ownership checks are skipped.
// POST /api/v1/admin/media/:id/presign-download
func (h *MediaHandler) AdminPresignDownload(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	h.presignForActor(c, subject.UserID, true)
}

func (h *MediaHandler) presignForActor(c *gin.Context, actorUserID int64, isAdmin bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid media ID")
		return
	}
	var req mediaPresignRequest
	_ = c.ShouldBindJSON(&req)
	grant, err := h.mediaService.CreateDownloadGrant(c.Request.Context(), service.CreateDownloadGrantInput{
		AssetID:      id,
		ActorUserID:  actorUserID,
		ActorIsAdmin: isAdmin,
		TTLMinutes:   req.TTLMinutes,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, grant)
}

// PublicGet streams a public object through the gateway. Bucket ACL stays private.
// GET /api/v1/media/public/:id
func (h *MediaHandler) PublicGet(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid media ID")
		return
	}
	asset, rc, err := h.mediaService.OpenPublicStream(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = rc.Close() }()
	c.Header("Cache-Control", "public, max-age=300")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "sandbox")
	c.Header("Content-Disposition", service.MediaContentDisposition(mediaDownloadFilename(asset)))
	c.DataFromReader(http.StatusOK, asset.Size, asset.MIME, rc, nil)
}

// SignedDownload streams a private object after verifying the app HMAC.
// GET /api/v1/media/download/:id
func (h *MediaHandler) SignedDownload(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid media ID")
		return
	}
	expires, _ := strconv.ParseInt(c.Query("expires"), 10, 64)
	asset, rc, err := h.mediaService.OpenSignedDownloadStream(c.Request.Context(), service.OpenSignedDownloadInput{
		AssetID: id,
		Expires: expires,
		Sig:     c.Query("sig"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = rc.Close() }()
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", service.MediaContentDisposition(mediaDownloadFilename(asset)))
	c.DataFromReader(http.StatusOK, asset.Size, asset.MIME, rc, nil)
}

type mediaVisibilityRequest struct {
	Visibility string `json:"visibility"`
}

func (h *MediaHandler) Get(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid media ID")
		return
	}
	asset, err := h.mediaService.GetForUser(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, asset)
}

func (h *MediaHandler) AdminGet(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid media ID")
		return
	}
	asset, err := h.mediaService.GetForAdmin(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, asset)
}

func (h *MediaHandler) Delete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid media ID")
		return
	}
	if err := h.mediaService.Delete(c.Request.Context(), id, subject.UserID, false); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *MediaHandler) AdminDelete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid media ID")
		return
	}
	if err := h.mediaService.Delete(c.Request.Context(), id, subject.UserID, true); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *MediaHandler) UpdateVisibility(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	h.updateVisibilityForActor(c, subject.UserID, false)
}

func (h *MediaHandler) AdminUpdateVisibility(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	h.updateVisibilityForActor(c, subject.UserID, true)
}

func (h *MediaHandler) updateVisibilityForActor(c *gin.Context, actorUserID int64, isAdmin bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid media ID")
		return
	}
	var req mediaVisibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	asset, err := h.mediaService.UpdateVisibility(c.Request.Context(), id, actorUserID, isAdmin, req.Visibility)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, asset)
}

func mediaDownloadFilename(asset *service.MediaAsset) string {
	if asset != nil && strings.TrimSpace(asset.Filename) != "" {
		return asset.Filename
	}
	if asset != nil {
		return service.MediaFilenameFromKey(asset.StorageKey)
	}
	return "file"
}
