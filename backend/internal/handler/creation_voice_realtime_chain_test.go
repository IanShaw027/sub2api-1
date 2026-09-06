//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type voiceChainUserRepo struct {
	service.UserRepository
	user *service.User
}

func (r *voiceChainUserRepo) GetByID(context.Context, int64) (*service.User, error) {
	copy := *r.user
	return &copy, nil
}

func (r *voiceChainUserRepo) GetUserAvatar(context.Context, int64) (*service.UserAvatar, error) {
	return nil, nil
}

type voiceChainTicketCache struct {
	service.GatewayCache
	mu      sync.Mutex
	records map[string]*service.CreationVoiceTicketRecord
}

func (s *voiceChainTicketCache) SaveCreationVoiceTicket(_ context.Context, hash string, record *service.CreationVoiceTicketRecord, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[hash] = record
	return nil
}
func (s *voiceChainTicketCache) ConsumeCreationVoiceTicket(_ context.Context, hash string) (*service.CreationVoiceTicketRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.records[hash]
	delete(s.records, hash)
	if record == nil {
		return nil, service.ErrCreationVoiceTicketInvalid
	}
	return record, nil
}

func TestCreationVoiceJWTToTicketWebSocketHandshake(t *testing.T) {
	now := time.Now()
	userRepo := &voiceChainUserRepo{user: &service.User{ID: 7, Status: service.StatusActive, TokenVersion: 1, TokenVersionResolved: true, LastActiveAt: &now}}
	cfg := &config.Config{}
	cfg.JWT.Secret = "voice-chain-test-secret"
	auth := service.NewAuthService(nil, userRepo, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
	users := service.NewUserService(userRepo, nil, nil, nil)
	tokens := &voiceChainTicketCache{records: make(map[string]*service.CreationVoiceTicketRecord)}
	group := &service.Group{ID: 3, Platform: service.PlatformGrok, Status: service.StatusActive}
	groups := &creationSubscriptionGroupRepo{group: group}
	keyRepo := &handlerCreationAPIKeyRepo{key: &service.APIKey{
		ID: 9, UserID: 7, GroupID: &group.ID, Group: group, User: userRepo.user,
		Key: "sk-internal-never-exposed", Status: service.StatusAPIKeyActive, Purpose: service.APIKeyPurposeCreation,
	}}
	keys := service.NewAPIKeyService(keyRepo, nil, nil, nil, nil, nil, cfg)
	h := NewCreationHandler(service.NewCreationService(nil, nil, nil, groups, userRepo, &creationSubscriptionRepo{}), service.NewCreationKeyResolver(keyRepo, keys), nil, nil, nil, nil, cfg)
	h.voiceTickets = service.NewCreationVoiceTicketService(auth, userRepo, tokens)
	claims := service.JWTClaims{UserID: 7, TokenVersion: 1, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}}
	jwtToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWT.Secret))
	require.NoError(t, err)
	router := gin.New()
	router.POST("/api/v1/creation/audio/realtime-ticket", gin.HandlerFunc(middleware2.NewJWTAuthMiddleware(auth, users, nil, nil)), h.GatewayContext, h.RealtimeTicket)
	// Exercise the real ticket auth and gateway context, then the exact Accept
	// function used by GrokRealtime without requiring a paid upstream account.
	router.GET("/api/v1/creation/audio/realtime", h.RealtimeVoiceAuth, h.GatewayContext, func(c *gin.Context) {
		key, ok := middleware2.GetAPIKeyFromContext(c)
		if !ok || key.Group.ID != 3 || c.GetHeader("Authorization") != "" || c.GetHeader("Sec-WebSocket-Protocol") != creationVoiceProtocol {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		c.Header("Sec-WebSocket-Protocol", creationVoiceProtocol)
		conn, err := acceptVoiceRealtime(c)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_ = conn.Write(c.Request.Context(), coderws.MessageText, []byte(`{"type":"session.created"}`))
		_, _, _ = conn.Read(c.Request.Context())
	})
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	issue := func() string {
		request, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/creation/audio/realtime-ticket", strings.NewReader("{}"))
		require.NoError(t, err)
		request.Header.Set("Authorization", "Bearer "+jwtToken)
		request.Header.Set("X-Group-Id", "3")
		request.Header.Set("User-Agent", "voice-test")
		response, err := server.Client().Do(request)
		require.NoError(t, err)
		defer response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
		var payload struct {
			Ticket string `json:"ticket"`
		}
		require.NoError(t, json.NewDecoder(response.Body).Decode(&payload))
		require.Len(t, payload.Ticket, 43)
		return payload.Ticket
	}
	dial := func(ticket string) (*coderws.Conn, *http.Response, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/api/v1/creation/audio/realtime?group_id=999", &coderws.DialOptions{
			HTTPHeader:   http.Header{"Origin": []string{server.URL}, "User-Agent": []string{"voice-test"}},
			Subprotocols: []string{creationVoiceProtocol, "ticket." + ticket},
		})
	}
	ticket := issue()
	conn, response, err := dial(ticket)
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	require.Equal(t, creationVoiceProtocol, conn.Subprotocol())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, message, err := conn.Read(ctx)
	cancel()
	require.NoError(t, err)
	require.Contains(t, string(message), "session.created")
	_ = conn.Close(coderws.StatusNormalClosure, "done")
	_, response, err = dial(ticket)
	require.Error(t, err)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)

	ticket = issue()
	tokens.mu.Lock()
	for _, record := range tokens.records {
		record.ExpiresAt = time.Now().Add(-time.Second).Unix()
	}
	tokens.mu.Unlock()
	_, response, err = dial(ticket)
	require.Error(t, err)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)

	ticket = issue()
	userRepo.user.TokenVersion++
	_, response, err = dial(ticket)
	require.Error(t, err)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
}
