//go:build unit

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type creationSubscriptionGroupRepo struct {
	service.GroupRepository
	group *service.Group
}

func (r *creationSubscriptionGroupRepo) GetByID(context.Context, int64) (*service.Group, error) {
	return r.group, nil
}

type creationSubscriptionRepo struct {
	service.UserSubscriptionRepository
	sub         service.UserSubscription
	resets      int
	activeReads int
	resetErr    error
}

func (r *creationSubscriptionRepo) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*service.UserSubscription, error) {
	r.activeReads++
	if r.sub.ExpiresAt.Before(time.Now()) {
		return nil, service.ErrSubscriptionNotFound
	}
	copy := r.sub
	return &copy, nil
}

func (r *creationSubscriptionRepo) GetByID(context.Context, int64) (*service.UserSubscription, error) {
	copy := r.sub
	return &copy, nil
}

func (r *creationSubscriptionRepo) ResetDailyUsage(_ context.Context, _ int64, _ *time.Time, start time.Time) error {
	r.resets++
	if r.resetErr != nil {
		return r.resetErr
	}
	r.sub.DailyWindowStart, r.sub.DailyUsageUSD = &start, 0
	return nil
}

func (r *creationSubscriptionRepo) ResetWeeklyUsage(_ context.Context, _ int64, _ *time.Time, start time.Time) error {
	r.resets++
	r.sub.WeeklyWindowStart, r.sub.WeeklyUsageUSD = &start, 0
	return nil
}

func (r *creationSubscriptionRepo) ResetMonthlyUsage(_ context.Context, _ int64, _ *time.Time, start time.Time) error {
	r.resets++
	r.sub.MonthlyWindowStart, r.sub.MonthlyUsageUSD = &start, 0
	return nil
}

func newCreationSubscriptionHandler(t *testing.T, repo *creationSubscriptionRepo) *CreationHandler {
	t.Helper()
	limit := 10.0
	group := &service.Group{
		ID: 3, Status: service.StatusActive, Platform: service.PlatformOpenAI,
		SubscriptionType: service.SubscriptionTypeSubscription,
		DailyLimitUSD:    &limit, WeeklyLimitUSD: &limit, MonthlyLimitUSD: &limit,
	}
	groups := &creationSubscriptionGroupRepo{group: group}
	creation := service.NewCreationService(nil, nil, nil, groups, &handlerCreationUserRepo{}, repo)
	keyRepo := &handlerCreationAPIKeyRepo{key: &service.APIKey{
		ID: 9, UserID: 7, GroupID: &group.ID, Group: group,
		Key: "sk-creation-test", Status: service.StatusAPIKeyActive, Purpose: service.APIKeyPurposeCreation,
		User: &service.User{ID: 7, Status: service.StatusActive},
	}}
	keys := service.NewAPIKeyService(keyRepo, nil, nil, nil, nil, nil, &config.Config{})
	subscriptions := service.NewSubscriptionService(groups, repo, nil, nil, nil)
	t.Cleanup(subscriptions.Stop)
	return NewCreationHandler(creation, service.NewCreationKeyResolver(keyRepo, keys), subscriptions, nil, nil, nil, nil)
}

func newCreationSubscription() service.UserSubscription {
	now := time.Now()
	return service.UserSubscription{
		ID: 1, UserID: 7, GroupID: 3, Status: service.SubscriptionStatusActive,
		StartsAt: now.Add(-60 * 24 * time.Hour), ExpiresAt: now.Add(60 * 24 * time.Hour),
		DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now,
	}
}

func TestCreationGatewayContextMaintainsExpiredSubscriptionWindows(t *testing.T) {
	for _, window := range []string{"daily", "weekly", "monthly"} {
		t.Run(window, func(t *testing.T) {
			repo := &creationSubscriptionRepo{sub: newCreationSubscription()}
			past := time.Now().Add(-40 * 24 * time.Hour)
			switch window {
			case "daily":
				repo.sub.DailyWindowStart, repo.sub.DailyUsageUSD = &past, 10
			case "weekly":
				repo.sub.WeeklyWindowStart, repo.sub.WeeklyUsageUSD = &past, 10
			case "monthly":
				repo.sub.MonthlyWindowStart, repo.sub.MonthlyUsageUSD = &past, 10
			}
			h := newCreationSubscriptionHandler(t, repo)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/creation/chat/completions?group_id=3", nil)
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
			called := false
			h.withGatewayContext(c, func(c *gin.Context) {
				called = true
				sub, ok := middleware2.GetSubscriptionFromContext(c)
				require.True(t, ok)
				require.Zero(t, sub.DailyUsageUSD+sub.WeeklyUsageUSD+sub.MonthlyUsageUSD)
			})
			require.True(t, called, w.Body.String())
			require.Equal(t, 1, repo.resets)
		})
	}
}

func TestCreationGatewayContextRejectsCurrentLimitAndMaintenanceFailure(t *testing.T) {
	for _, maintenanceFailure := range []bool{false, true} {
		repo := &creationSubscriptionRepo{sub: newCreationSubscription()}
		repo.sub.DailyUsageUSD = 11
		if maintenanceFailure {
			past := time.Now().Add(-48 * time.Hour)
			repo.sub.DailyWindowStart = &past
			repo.resetErr = errors.New("reset failed")
		}
		h := newCreationSubscriptionHandler(t, repo)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/creation/messages?group_id=3", nil)
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		h.withGatewayContext(c, func(*gin.Context) { t.Fatal("rejected request reached gateway") })
		if maintenanceFailure {
			require.Equal(t, http.StatusInternalServerError, w.Code)
		} else {
			require.Equal(t, http.StatusTooManyRequests, w.Code)
		}
	}
}

func TestCreationGatewayContextAllowsTaskReadAfterSubscriptionExpires(t *testing.T) {
	repo := &creationSubscriptionRepo{sub: newCreationSubscription()}
	repo.sub.ExpiresAt = time.Now().Add(-time.Hour)
	h := newCreationSubscriptionHandler(t, repo)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/creation/images/tasks/imgtask_owned?group_id=3", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	called := false
	h.withGatewayContext(c, func(*gin.Context) { called = true })
	require.True(t, called, w.Body.String())
	require.Zero(t, repo.activeReads)
	require.Zero(t, repo.resets)
}

func TestCreationGatewayContextSimpleModeSkipsSubscriptionLimits(t *testing.T) {
	repo := &creationSubscriptionRepo{sub: newCreationSubscription()}
	repo.sub.DailyUsageUSD = 11
	h := newCreationSubscriptionHandler(t, repo)
	h.cfg = &config.Config{RunMode: config.RunModeSimple}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/creation/messages?group_id=3", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
	called := false
	h.withGatewayContext(c, func(*gin.Context) { called = true })
	require.True(t, called, w.Body.String())
	require.Zero(t, repo.resets)
}

func TestCreationGatewayContextAbortsFailedPreparation(t *testing.T) {
	router := gin.New()
	h := NewCreationHandler(nil, nil, nil, nil, nil, nil, nil)
	router.Use(h.GatewayContext)
	router.POST("/", func(*gin.Context) { t.Fatal("failed authentication continued to routing") })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", nil))
	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreationGatewayDispatchUsesResolvedCompositePlatform(t *testing.T) {
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformGrok, service.PlatformKimi, service.PlatformAnthropic, service.PlatformGemini} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(service.WithResolvedTargetPlatform(context.Background(), platform))
		key := &service.APIKey{Group: &service.Group{Platform: service.PlatformComposite}}
		effective := effectiveAPIKeyPlatform(c, key)
		require.Equal(t, platform, effective)
		require.Equal(t, platform == service.PlatformOpenAI || platform == service.PlatformGrok || platform == service.PlatformKimi, creationUsesOpenAIGateway(effective))
	}
}
