package middleware

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// InjectGatewayContextFromAPIKey mirrors the successful API-key auth path for
// internal gateway delegation (e.g. creation center hidden keys).
func InjectGatewayContextFromAPIKey(c *gin.Context, apiKey *service.APIKey, subscription *service.UserSubscription) {
	if c == nil || apiKey == nil || apiKey.User == nil {
		return
	}
	ctx := context.WithValue(c.Request.Context(), ctxkey.UserID, apiKey.User.ID)
	c.Request = c.Request.WithContext(ctx)

	if subscription != nil {
		c.Set(string(ContextKeySubscription), subscription)
	}
	c.Set(string(ContextKeyAPIKey), apiKey)
	c.Set(string(ContextKeyUser), AuthSubject{
		UserID:      apiKey.User.ID,
		Concurrency: apiKey.User.Concurrency,
	})
	c.Set(string(ContextKeyUserRole), apiKey.User.Role)
	setGroupContext(c, apiKey.Group)
}
