//go:build unit

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type mediaPricingHandlerStub struct {
	estimate *service.CreationMediaPriceInput
	userID   int64
	groupID  int64
	kind     string
	taskID   string
}

func (s *mediaPricingHandlerStub) Estimate(_ context.Context, input service.CreationMediaPriceInput) (*service.CreationMediaPrice, error) {
	s.estimate = &input
	return &service.CreationMediaPrice{Status: "unavailable", Currency: "USD"}, nil
}

func (s *mediaPricingHandlerStub) Receipt(_ context.Context, userID, groupID int64, kind, taskID string) (*service.CreationMediaPrice, error) {
	s.userID, s.groupID, s.kind, s.taskID = userID, groupID, kind, taskID
	return &service.CreationMediaPrice{Status: "pending", Currency: "USD"}, nil
}

func TestCreationMediaPricingHandlerContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name string
		url  string
		auth bool
		code int
	}{
		{"unauthenticated", "/pricing?group_id=3", false, http.StatusUnauthorized},
		{"bad group", "/pricing?group_id=wrong", true, http.StatusBadRequest},
		{"bad duration", "/pricing?group_id=3&duration=oops", true, http.StatusBadRequest},
		{"negative duration", "/pricing?group_id=3&duration=-1", true, http.StatusBadRequest},
		{"quote", "/pricing?group_id=3&kind=video&model=grok-imagine-video&duration=6&resolution=720p", true, http.StatusOK},
		{"receipt", "/billing?group_id=3&kind=image&task_id=imgtask_owned&request_id=arbitrary&user_id=999", true, http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stub := &mediaPricingHandlerStub{}
			h := &CreationMediaPricingHandler{pricing: stub}
			r := gin.New()
			if tt.auth {
				r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7}) })
			}
			r.GET("/pricing", h.Estimate)
			r.GET("/billing", h.Receipt)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.url, nil))
			require.Equal(t, tt.code, w.Code)
			if tt.code != http.StatusOK {
				require.Nil(t, stub.estimate)
				require.Empty(t, stub.taskID)
				return
			}
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			require.Contains(t, w.Body.String(), `"amount":null`)
			if tt.name == "quote" {
				require.Equal(t, int64(7), stub.estimate.UserID)
				require.Equal(t, 6, stub.estimate.Duration)
				require.Equal(t, "720p", stub.estimate.Resolution)
			} else {
				require.Equal(t, int64(7), stub.userID)
				require.Equal(t, int64(3), stub.groupID)
				require.Equal(t, "image", stub.kind)
				require.Equal(t, "imgtask_owned", stub.taskID)
			}
		})
	}
}
