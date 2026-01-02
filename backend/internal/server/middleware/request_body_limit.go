package middleware

import (
	"fmt"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// RequestBodyLimit 使用 MaxBytesReader 限制请求体大小。
func RequestBodyLimit(maxBytes int64) gin.HandlerFunc {
	return RequestBodyLimitWithOps(maxBytes, nil)
}

const (
	requestBodyReadBytesKey       = "__request_body_read_bytes"
	opsBodyTooLargeLoggedKey      = "__ops_body_too_large_logged"
	opsBodyTooLargeErrType        = "invalid_request_error"
	opsBodyTooLargePhase          = "request"
	opsBodyTooLargeSeverity       = "P3"
	opsBodyTooLargeMessageFormat  = "Request body too large: %d bytes exceeds limit %d"
)

type countingReadCloser struct {
	rc        io.ReadCloser
	bytesRead *atomic.Int64
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.rc.Read(p)
	if c.bytesRead != nil && n > 0 {
		c.bytesRead.Add(int64(n))
	}
	return n, err
}

func (c *countingReadCloser) Close() error {
	return c.rc.Close()
}

// RequestBodyLimitWithOps limits request body size and records an ops error when a 413 is returned.
func RequestBodyLimitWithOps(maxBytes int64, opsService *service.OpsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(string(ContextKeyMaxBodySize), maxBytes)

		var readBytes atomic.Int64
		if c.Request != nil && c.Request.Body != nil {
			limited := http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
			c.Request.Body = &countingReadCloser{rc: limited, bytesRead: &readBytes}
			c.Set(requestBodyReadBytesKey, &readBytes)
		}

		c.Next()

		if opsService == nil {
			return
		}
		if c.Writer.Status() != http.StatusRequestEntityTooLarge {
			return
		}
		if v, ok := c.Get(opsBodyTooLargeLoggedKey); ok {
			if logged, _ := v.(bool); logged {
				return
			}
		}

		size := requestBodySizeForOpsLog(c, &readBytes, maxBytes)
		if size <= 0 || maxBytes <= 0 {
			return
		}

		recordOpsRequestBodyTooLarge(c, opsService, size, maxBytes)
		c.Set(opsBodyTooLargeLoggedKey, true)
	}
}

func GetMaxBodySizeFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(string(ContextKeyMaxBodySize))
	if !exists {
		return 0, false
	}
	maxBytes, ok := value.(int64)
	return maxBytes, ok
}

func requestBodySizeForOpsLog(c *gin.Context, readBytes *atomic.Int64, limit int64) int64 {
	var size int64

	if c != nil && c.Request != nil && c.Request.ContentLength > 0 {
		size = c.Request.ContentLength
	}
	if readBytes != nil {
		if v := readBytes.Load(); v > size {
			size = v
		}
	}

	// If size can't be determined reliably (e.g. chunked), log the minimum size that triggers 413.
	if limit > 0 && size <= limit {
		size = limit + 1
	}
	return size
}

func recordOpsRequestBodyTooLarge(c *gin.Context, opsService *service.OpsService, size, limit int64) {
	if c == nil || opsService == nil {
		return
	}

	apiKey, _ := GetAPIKeyFromContext(c)

	logEntry := &service.OpsErrorLog{
		Phase:      opsBodyTooLargePhase,
		Type:       opsBodyTooLargeErrType,
		Severity:   opsBodyTooLargeSeverity,
		StatusCode: http.StatusRequestEntityTooLarge,
		RequestID:  c.Writer.Header().Get("x-request-id"),
		Message:    fmt.Sprintf(opsBodyTooLargeMessageFormat, size, limit),
		ClientIP:   c.ClientIP(),
		RequestPath: func() string {
			if c.Request != nil && c.Request.URL != nil {
				return c.Request.URL.Path
			}
			return ""
		}(),
	}

	if apiKey != nil {
		logEntry.APIKeyID = &apiKey.ID
		if apiKey.User != nil {
			logEntry.UserID = &apiKey.User.ID
		}
		if apiKey.GroupID != nil {
			logEntry.GroupID = apiKey.GroupID
		}
		if apiKey.Group != nil {
			logEntry.Platform = apiKey.Group.Platform
		}
	}

	RecordOpsError(opsService, logEntry)
}
