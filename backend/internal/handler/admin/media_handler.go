package admin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
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

func (h *MediaHandler) Upload(c *gin.Context) {
	subject, ok := servermiddleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ownerUserID := &subject.UserID
	if raw := strings.TrimSpace(c.PostForm("owner_user_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "Invalid owner_user_id")
			return
		}
		ownerUserID = &parsed
	}
	input, err := parseAdminMediaUploadInput(c, ownerUserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item, err := h.mediaService.Upload(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.MediaAssetFromService(
		item,
		h.mediaService.PublicURL(item),
		h.mediaService.ThumbnailPublicURL(item),
	))
}

func (h *MediaHandler) GetByID(c *gin.Context) {
	mediaID, err := parseAdminMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	item, err := h.mediaService.GetForAdmin(c.Request.Context(), mediaID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MediaAssetFromService(
		item,
		h.mediaService.PublicURL(item),
		h.mediaService.ThumbnailPublicURL(item),
	))
}

func (h *MediaHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	var ownerUserID *int64
	if raw := strings.TrimSpace(c.Query("owner_user_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "Invalid owner_user_id")
			return
		}
		ownerUserID = &parsed
	}
	items, result, err := h.mediaService.ListForAdmin(c.Request.Context(), pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}, service.MediaListFilters{
		BizType:     c.Query("biz_type"),
		BizID:       c.Query("biz_id"),
		Visibility:  c.Query("visibility"),
		Status:      c.Query("status"),
		Search:      c.Query("search"),
		OwnerUserID: ownerUserID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.MediaAsset, 0, len(items))
	for i := range items {
		item := items[i]
		out = append(out, *dto.MediaAssetFromService(
			&item,
			h.mediaService.PublicURL(&item),
			h.mediaService.ThumbnailPublicURL(&item),
		))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

func (h *MediaHandler) Delete(c *gin.Context) {
	mediaID, err := parseAdminMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := h.mediaService.DeleteForAdmin(c.Request.Context(), mediaID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *MediaHandler) UpdateVisibility(c *gin.Context) {
	mediaID, err := parseAdminMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req updateMediaVisibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.mediaService.UpdateVisibilityForAdmin(c.Request.Context(), mediaID, service.UpdateMediaVisibilityInput{
		Visibility: req.Visibility,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MediaAssetFromService(
		item,
		h.mediaService.PublicURL(item),
		h.mediaService.ThumbnailPublicURL(item),
	))
}

func (h *MediaHandler) PresignDownload(c *gin.Context) {
	mediaID, err := parseAdminMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.mediaService.CreateDownloadURLForAdmin(service.WithRequestBaseURL(c.Request.Context(), requestBaseURL(c)), mediaID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MediaDownloadURLFromService(result))
}

func (h *MediaHandler) PresignThumbnailDownload(c *gin.Context) {
	mediaID, err := parseAdminMediaID(c.Param("id"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.mediaService.CreateThumbnailDownloadURLForAdmin(service.WithRequestBaseURL(c.Request.Context(), requestBaseURL(c)), mediaID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.MediaDownloadURLFromService(result))
}

type updateMediaVisibilityRequest struct {
	Visibility string `json:"visibility" binding:"required"`
}

func parseAdminMediaID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, service.ErrMediaInvalidID
	}
	return id, nil
}

func parseAdminMediaUploadInput(c *gin.Context, ownerUserID *int64) (service.UploadMediaInput, error) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return service.UploadMediaInput{}, service.ErrMediaFileRequired
	}
	fileBytes, contentType, width, height, shaValue, err := readAdminUploadedMedia(fileHeader, 64<<20)
	if err != nil {
		return service.UploadMediaInput{}, err
	}

	var thumbnailBytes []byte
	thumbnailName := ""
	if thumbnailHeader, thumbnailErr := c.FormFile("thumbnail"); thumbnailErr == nil && thumbnailHeader != nil {
		thumbnailBytes, _, _, _, _, err = readAdminUploadedMedia(thumbnailHeader, 64<<20)
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

func readAdminUploadedMedia(fileHeader *multipart.FileHeader, maxSizeBytes int64) ([]byte, string, *int, *int, string, error) {
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
