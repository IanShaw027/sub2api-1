package handler

import (
	"net/http"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const creationLocalVideoReadKey = "creation.local_video_read"

func (h *CreationHandler) localImageHandler(c *gin.Context) *AsyncImageHandler {
	if h.asyncImage == nil || h.asyncImage.tasks == nil {
		imageTaskError(c, service.ErrImageTaskUnavailable)
		return nil
	}
	local := *h.asyncImage
	local.tasks = h.asyncImage.tasks.ForLocalResults()
	local.gemini = h.gateway
	local.execute = h.localImageExecutor(&local)
	return &local
}

func (h *CreationHandler) LocalImagesAsync(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		if local := h.localImageHandler(c); local != nil {
			local.Submit(c)
		}
	})
}

func (h *CreationHandler) LocalImageTask(c *gin.Context) {
	h.withGatewayContext(c, func(c *gin.Context) {
		if local := h.localImageHandler(c); local != nil {
			local.Get(c)
		}
	})
}

func (h *CreationHandler) withLocalVideoGateway(c *gin.Context, next gin.HandlerFunc) {
	h.withGatewayContext(c, func(c *gin.Context) {
		apiKey, _ := middleware2.GetAPIKeyFromContext(c)
		platform := effectiveAPIKeyPlatform(c, apiKey)
		if platform != service.PlatformGrok && platform != service.PlatformComposite {
			imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "Videos API is not supported for this platform")
			return
		}
		if h.openAI == nil {
			imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "video gateway is unavailable")
			return
		}
		if c.Request.Method == http.MethodGet {
			c.Set(creationLocalVideoReadKey, true)
		}
		next(c)
	})
}

func isLocalCreationVideoRead(c *gin.Context, endpoint service.GrokMediaEndpoint) bool {
	return c.GetBool(creationLocalVideoReadKey) && c.Request.Method == http.MethodGet && endpoint.IsVideoLookupRequest()
}

func (h *CreationHandler) LocalVideoGeneration(c *gin.Context) {
	h.submitLocalVideo(c, func(c *gin.Context) { h.openAI.GrokVideoGeneration(c) })
}

func (h *CreationHandler) LocalVideoEdit(c *gin.Context) {
	h.submitLocalVideo(c, func(c *gin.Context) { h.openAI.GrokVideoEdit(c) })
}

func (h *CreationHandler) LocalVideoExtension(c *gin.Context) {
	h.submitLocalVideo(c, func(c *gin.Context) { h.openAI.GrokVideoExtension(c) })
}

func (h *CreationHandler) LocalVideoStatus(c *gin.Context) {
	h.withLocalVideoGateway(c, func(c *gin.Context) { h.openAI.GrokVideoStatus(c) })
}

func (h *CreationHandler) LocalVideoContent(c *gin.Context) {
	h.withLocalVideoGateway(c, func(c *gin.Context) { h.openAI.GrokVideoContent(c) })
}
