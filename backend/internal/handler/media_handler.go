package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/webp"
)

type MediaHandler struct {
	mediaService *service.MediaService
}

func NewMediaHandler(mediaService *service.MediaService) *MediaHandler {
	return &MediaHandler{mediaService: mediaService}
}

type UpdateMediaVisibilityRequest struct {
	Visibility string `json:"visibility" binding:"required"`
}

func (h *MediaHandler) Upload(c *gin.Context) {
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	input, err := parseMediaUploadInput(c, &subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item, err := h.mediaService.Upload(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.MediaAssetPublicFromService(
		item,
		h.mediaService.PublicURL(item),
		h.mediaService.ThumbnailPublicURL(item),
	))
}

func (h *MediaHandler) GetByID(c *gin.Context) {
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	mediaID, err := parseMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item, err := h.mediaService.GetForUser(c.Request.Context(), subject.UserID, mediaID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MediaAssetPublicFromService(
		item,
		h.mediaService.PublicURL(item),
		h.mediaService.ThumbnailPublicURL(item),
	))
}

func (h *MediaHandler) Delete(c *gin.Context) {
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	mediaID, err := parseMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := h.mediaService.DeleteForUser(c.Request.Context(), subject.UserID, mediaID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *MediaHandler) UpdateVisibility(c *gin.Context) {
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	mediaID, err := parseMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req UpdateMediaVisibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.mediaService.UpdateVisibilityForUser(c.Request.Context(), subject.UserID, mediaID, service.UpdateMediaVisibilityInput{
		Visibility: req.Visibility,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MediaAssetPublicFromService(
		item,
		h.mediaService.PublicURL(item),
		h.mediaService.ThumbnailPublicURL(item),
	))
}

func (h *MediaHandler) PresignDownload(c *gin.Context) {
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	mediaID, err := parseMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.mediaService.CreateDownloadURLForUser(service.WithRequestBaseURL(c.Request.Context(), requestBaseURL(c)), subject.UserID, mediaID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MediaDownloadURLFromService(result))
}

func (h *MediaHandler) PresignThumbnailDownload(c *gin.Context) {
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	mediaID, err := parseMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.mediaService.CreateThumbnailDownloadURLForUser(service.WithRequestBaseURL(c.Request.Context(), requestBaseURL(c)), subject.UserID, mediaID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MediaDownloadURLFromService(result))
}

func (h *MediaHandler) ServePublic(c *gin.Context) {
	mediaID, err := parseMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stream, _, err := h.mediaService.OpenPublic(c.Request.Context(), mediaID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = stream.Body.Close() }()
	setMediaCacheHeaders(c, true)
	c.DataFromReader(http.StatusOK, stream.SizeBytes, stream.ContentType, stream.Body, nil)
}

func (h *MediaHandler) ServePublicThumbnail(c *gin.Context) {
	mediaID, err := parseMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stream, _, err := h.mediaService.OpenPublic(c.Request.Context(), mediaID, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = stream.Body.Close() }()
	setMediaCacheHeaders(c, true)
	c.DataFromReader(http.StatusOK, stream.SizeBytes, stream.ContentType, stream.Body, nil)
}

func (h *MediaHandler) ServeSignedDownload(c *gin.Context) {
	h.serveSignedDownload(c, false)
}

func (h *MediaHandler) ServeSignedThumbnailDownload(c *gin.Context) {
	h.serveSignedDownload(c, true)
}

func (h *MediaHandler) serveSignedDownload(c *gin.Context, thumbnail bool) {
	mediaID, err := parseMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	expiresAt, err := strconv.ParseInt(strings.TrimSpace(c.Query("expires")), 10, 64)
	if err != nil {
		response.ErrorFrom(c, service.ErrMediaSignatureInvalid)
		return
	}
	stream, asset, err := h.mediaService.OpenSignedDownload(c.Request.Context(), mediaID, expiresAt, strings.TrimSpace(c.Query("sig")), thumbnail)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = stream.Body.Close() }()
	fileName := sanitizeDownloadFileName(asset.OriginalFileName)
	if thumbnail {
		fileName = "thumbnail-" + fileName
	}
	setMediaCacheHeaders(c, false)
	headers := map[string]string{
		"Content-Disposition": buildDownloadContentDisposition(fileName),
	}
	c.DataFromReader(http.StatusOK, stream.SizeBytes, stream.ContentType, stream.Body, headers)
}

func setMediaCacheHeaders(c *gin.Context, public bool) {
	if public {
		c.Header("Cache-Control", "public, max-age=300")
		return
	}
	c.Header("Cache-Control", "private, no-store, max-age=0, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
}

func parseMediaID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, service.ErrMediaInvalidID
	}
	return id, nil
}

func parseMediaUploadInput(c *gin.Context, ownerUserID *int64) (service.UploadMediaInput, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return service.UploadMediaInput{}, service.ErrMediaFileRequired
	}
	fileBytes, contentType, width, height, shaValue, err := readUploadedMedia(fileHeader, 64<<20)
	if err != nil {
		return service.UploadMediaInput{}, err
	}

	var thumbnailBytes []byte
	thumbnailName := ""
	if thumbnailHeader, thumbnailErr := c.FormFile("thumbnail"); thumbnailErr == nil && thumbnailHeader != nil {
		thumbnailBytes, _, _, _, _, err = readUploadedMedia(thumbnailHeader, 64<<20)
		if err != nil {
			return service.UploadMediaInput{}, err
		}
		thumbnailName = thumbnailHeader.Filename
	}

	return service.UploadMediaInput{
		BizType:           c.PostForm("biz_type"),
		BizID:             c.PostForm("biz_id"),
		Visibility:        c.DefaultPostForm("visibility", service.MediaVisibilityPrivate),
		OwnerUserID:       ownerUserID,
		FileName:          fileHeader.Filename,
		ContentType:       contentType,
		SizeBytes:         int64(len(fileBytes)),
		SHA256:            shaValue,
		Width:             width,
		Height:            height,
		File:              fileBytes,
		ThumbnailFileName: thumbnailName,
		ThumbnailFile:     thumbnailBytes,
	}, nil
}

func readUploadedMedia(fileHeader *multipart.FileHeader, maxSizeBytes int64) ([]byte, string, *int, *int, string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, "", nil, nil, "", err
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, maxSizeBytes+1))
	if err != nil {
		return nil, "", nil, nil, "", err
	}
	if int64(len(data)) > maxSizeBytes {
		return nil, "", nil, nil, "", service.ErrMediaTooLarge
	}

	contentType := http.DetectContentType(data)
	var width, height *int
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil && cfg.Width > 0 && cfg.Height > 0 {
		widthValue := cfg.Width
		heightValue := cfg.Height
		width = &widthValue
		height = &heightValue
	}
	sum := sha256.Sum256(data)
	return data, contentType, width, height, hex.EncodeToString(sum[:]), nil
}

func sanitizeDownloadFileName(fileName string) string {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return "download"
	}
	fileName = strings.ReplaceAll(fileName, `"`, "")
	fileName = strings.ReplaceAll(fileName, "\n", "")
	fileName = strings.ReplaceAll(fileName, "\r", "")
	return fileName
}

func buildDownloadContentDisposition(fileName string) string {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = "download"
	}
	if value := mime.FormatMediaType("attachment", map[string]string{"filename": fileName}); value != "" {
		return value
	}
	fileName = sanitizeDownloadFileName(fileName)
	return `attachment; filename="` + fileName + `"`
}
