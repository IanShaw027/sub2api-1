package middleware

import (
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// NewJWTAuthMiddleware 创建 JWT 认证中间件
func NewJWTAuthMiddleware(authService *service.AuthService, userService *service.UserService, opsService *service.OpsService) JWTAuthMiddleware {
	return JWTAuthMiddleware(jwtAuth(authService, userService, opsService))
}

// jwtAuth JWT认证中间件实现
func jwtAuth(authService *service.AuthService, userService *service.UserService, opsService *service.OpsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Authorization header中提取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			recordOpsJWTAuthError(c, opsService, nil, 401, "UNAUTHORIZED", "Authorization header is required")
			AbortWithError(c, 401, "UNAUTHORIZED", "Authorization header is required")
			return
		}

		// 验证Bearer scheme
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			recordOpsJWTAuthError(c, opsService, nil, 401, "INVALID_AUTH_HEADER", "Authorization header format must be 'Bearer {token}'")
			AbortWithError(c, 401, "INVALID_AUTH_HEADER", "Authorization header format must be 'Bearer {token}'")
			return
		}

		tokenString := parts[1]
		if tokenString == "" {
			recordOpsJWTAuthError(c, opsService, nil, 401, "EMPTY_TOKEN", "Token cannot be empty")
			AbortWithError(c, 401, "EMPTY_TOKEN", "Token cannot be empty")
			return
		}

		// 验证token
		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			if errors.Is(err, service.ErrTokenExpired) {
				recordOpsJWTAuthError(c, opsService, nil, 401, "TOKEN_EXPIRED", "Token has expired")
				AbortWithError(c, 401, "TOKEN_EXPIRED", "Token has expired")
				return
			}
			recordOpsJWTAuthError(c, opsService, nil, 401, "INVALID_TOKEN", "Invalid token")
			AbortWithError(c, 401, "INVALID_TOKEN", "Invalid token")
			return
		}

		// 从数据库获取最新的用户信息
		user, err := userService.GetByID(c.Request.Context(), claims.UserID)
		if err != nil {
			recordOpsJWTAuthError(c, opsService, &claims.UserID, 401, "USER_NOT_FOUND", "User lookup failed: "+err.Error())
			AbortWithError(c, 401, "USER_NOT_FOUND", "User not found")
			return
		}

		// 检查用户状态
		if !user.IsActive() {
			recordOpsJWTAuthError(c, opsService, &claims.UserID, 401, "USER_INACTIVE", "User account is not active")
			AbortWithError(c, 401, "USER_INACTIVE", "User account is not active")
			return
		}

		// Security: Validate TokenVersion to ensure token hasn't been invalidated
		// This check ensures tokens issued before a password change are rejected
		if claims.TokenVersion != user.TokenVersion {
			recordOpsJWTAuthError(c, opsService, &claims.UserID, 401, "TOKEN_REVOKED", "Token has been revoked (password changed)")
			AbortWithError(c, 401, "TOKEN_REVOKED", "Token has been revoked (password changed)")
			return
		}

		c.Set(string(ContextKeyUser), AuthSubject{
			UserID:      user.ID,
			Concurrency: user.Concurrency,
		})
		c.Set(string(ContextKeyUserRole), user.Role)

		c.Next()
	}
}

// Deprecated: prefer GetAuthSubjectFromContext in auth_subject.go.

func recordOpsJWTAuthError(c *gin.Context, opsService *service.OpsService, userID *int64, status int, reason, message string) {
	if c == nil || opsService == nil {
		return
	}

	requestID := c.GetHeader("x-request-id")
	if requestID == "" {
		requestID = c.Writer.Header().Get("x-request-id")
	}

	logEntry := &service.OpsErrorLog{
		Phase:      "auth",
		Type:       "authentication_error",
		Severity:   "P2",
		StatusCode: status,
		RequestID:  requestID,
		Message:    reason + ": " + message,
		ClientIP:   c.ClientIP(),
		RequestPath: func() string {
			if c.Request != nil && c.Request.URL != nil {
				return c.Request.URL.Path
			}
			return ""
		}(),
	}

	if userID != nil {
		logEntry.UserID = userID
	}

	// 异步记录错误日志，避免阻塞请求
	go func() {
		_ = opsService.RecordOpsError(c.Request.Context(), logEntry, nil)
	}()
}
