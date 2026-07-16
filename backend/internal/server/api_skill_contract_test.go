//go:build unit

package server_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func runSkillContractCases(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		wantStatus int
		wantJSON   string
	}{
		{
			name:       "POST /api/v1/user/skills/:id/test blocks unreviewed version",
			method:     http.MethodPost,
			path:       "/api/v1/user/skills/11/test?version_id=101",
			wantStatus: http.StatusForbidden,
			wantJSON: `{
				"code": 403,
				"message": "ai skill version is not approved",
				"reason": "AI_SKILL_VERSION_NOT_APPROVED"
			}`,
		},
		{
			name:       "POST /api/v1/user/skills/:id/use blocks unreviewed version",
			method:     http.MethodPost,
			path:       "/api/v1/user/skills/11/use?version_id=101",
			wantStatus: http.StatusForbidden,
			wantJSON: `{
				"code": 403,
				"message": "ai skill version is not approved",
				"reason": "AI_SKILL_VERSION_NOT_APPROVED"
			}`,
		},
		{
			name:       "POST /api/v1/user/skills/versions/:versionId/publish blocks unreviewed version",
			method:     http.MethodPost,
			path:       "/api/v1/user/skills/versions/101/publish",
			wantStatus: http.StatusForbidden,
			wantJSON: `{
				"code": 403,
				"message": "ai skill version is not approved",
				"reason": "AI_SKILL_VERSION_NOT_APPROVED"
			}`,
		},
		{
			name:       "POST /api/v1/user/skills/versions/:versionId/publish allows approved version",
			method:     http.MethodPost,
			path:       "/api/v1/user/skills/versions/102/publish",
			wantStatus: http.StatusOK,
			wantJSON: `{
				"code": 0,
				"message": "success",
				"data": {
					"published": true,
					"skill_id": 12,
					"version_id": 102
				}
			}`,
		},
		{
			name:       "POST /api/v1/user/skills/:id/test allows approved version",
			method:     http.MethodPost,
			path:       "/api/v1/user/skills/12/test?version_id=102",
			wantStatus: http.StatusOK,
			wantJSON: `{
				"code": 0,
				"message": "success",
				"data": {
					"accepted": true,
					"mode": "test",
					"skill_id": 12
				}
			}`,
		},
		{
			name:       "POST /api/v1/user/skills/:id/use allows approved version",
			method:     http.MethodPost,
			path:       "/api/v1/user/skills/12/use?version_id=102",
			wantStatus: http.StatusOK,
			wantJSON: `{
				"code": 0,
				"message": "success",
				"data": {
					"accepted": true,
					"mode": "use",
					"skill_id": 12
				}
			}`,
		},
		{
			name:       "GET /api/v1/user/skills/:id hides paid source from public viewer",
			method:     http.MethodGet,
			path:       "/api/v1/user/skills/20",
			wantStatus: http.StatusOK,
			wantJSON: `{
				"code": 0,
				"message": "success",
				"data": {
					"id": 20,
					"title": "Paid Script Skill",
					"price": 19.9,
					"version": {
						"id": 201,
						"version": 3,
						"review_status": "approved"
					}
				}
			}`,
		},
		{
			name:   "GET /api/v1/user/skills/:id keeps paid source for owner",
			method: http.MethodGet,
			path:   "/api/v1/user/skills/20",
			headers: map[string]string{
				"X-Viewer-ID": "7001",
			},
			wantStatus: http.StatusOK,
			wantJSON: `{
				"code": 0,
				"message": "success",
				"data": {
					"id": 20,
					"title": "Paid Script Skill",
					"price": 19.9,
					"version": {
						"id": 201,
						"version": 3,
						"review_status": "approved",
						"source_content": "print('paid source')"
					}
				}
			}`,
		},
		{
			name:       "GET /api/v1/user/skills/:id keeps free source for public viewer",
			method:     http.MethodGet,
			path:       "/api/v1/user/skills/21",
			wantStatus: http.StatusOK,
			wantJSON: `{
				"code": 0,
				"message": "success",
				"data": {
					"id": 21,
					"title": "Free Prompt Skill",
					"price": 0,
					"version": {
						"id": 202,
						"version": 1,
						"review_status": "approved",
						"source_content": "{\"prompt\":\"hello\"}"
					}
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body, _ := doRequest(t, newSkillContractRouter(), tt.method, tt.path, "", tt.headers)
			require.Equal(t, tt.wantStatus, status)
			require.JSONEq(t, tt.wantJSON, body)
		})
	}
}

type skillContractFixture struct {
	skills   map[int64]domain.AISkill
	versions map[int64]domain.AISkillVersion
}

func newSkillContractRouter() http.Handler {
	fixture := newSkillContractFixture()
	router := gin.New()
	v1 := router.Group("/api/v1")

	user := v1.Group("/user")
	user.POST("/skills/:id/test", func(c *gin.Context) {
		skillContractHandleRun(c, fixture, domain.AISkillRunModeTest)
	})
	user.POST("/skills/:id/use", func(c *gin.Context) {
		skillContractHandleRun(c, fixture, domain.AISkillRunModeUse)
	})
	user.POST("/skills/versions/:versionId/publish", func(c *gin.Context) {
		skillContractHandlePublish(c, fixture)
	})
	user.GET("/skills/:id", func(c *gin.Context) {
		skillContractHandleDetail(c, fixture)
	})

	return router
}

func newSkillContractFixture() *skillContractFixture {
	return &skillContractFixture{
		skills: map[int64]domain.AISkill{
			11: {
				ID:            11,
				UserID:        6001,
				Type:          domain.AISkillTypeScript,
				Title:         "Pending Script Skill",
				Visibility:    domain.AIVisibilityPublic,
				BillingMode:   domain.AISkillBillingModePerRequest,
				Price:         0,
				LatestVersion: 1,
			},
			12: {
				ID:                 12,
				UserID:             6001,
				Type:               domain.AISkillTypeScript,
				Title:              "Approved Script Skill",
				Visibility:         domain.AIVisibilityPublic,
				SourceVisibility:   domain.AISkillSourceVisibilityPublic,
				BillingMode:        domain.AISkillBillingModePerRequest,
				Price:              0,
				PublishedVersionID: ptr(int64(102)),
				LatestVersion:      2,
			},
			20: {
				ID:                 20,
				UserID:             7001,
				Type:               domain.AISkillTypeScript,
				Title:              "Paid Script Skill",
				Visibility:         domain.AIVisibilityPublic,
				SourceVisibility:   domain.AISkillSourceVisibilityHidden,
				BillingMode:        domain.AISkillBillingModePerRequest,
				Price:              19.9,
				PublishedVersionID: ptr(int64(201)),
				LatestVersion:      3,
			},
			21: {
				ID:                 21,
				UserID:             8001,
				Type:               domain.AISkillTypePromptChat,
				Title:              "Free Prompt Skill",
				Visibility:         domain.AIVisibilityPublic,
				SourceVisibility:   domain.AISkillSourceVisibilityPublic,
				BillingMode:        domain.AISkillBillingModePerRequest,
				Price:              0,
				PublishedVersionID: ptr(int64(202)),
				LatestVersion:      1,
			},
		},
		versions: map[int64]domain.AISkillVersion{
			101: {
				ID:            101,
				SkillID:       11,
				UserID:        6001,
				Version:       1,
				ReviewStatus:  domain.AISkillVersionReviewStatusPending,
				Runtime:       "python3.11",
				SourceContent: "print('pending source')",
			},
			102: {
				ID:            102,
				SkillID:       12,
				UserID:        6001,
				Version:       2,
				ReviewStatus:  domain.AISkillVersionReviewStatusApproved,
				Runtime:       "python3.11",
				SourceContent: "print('approved source')",
			},
			201: {
				ID:            201,
				SkillID:       20,
				UserID:        7001,
				Version:       3,
				ReviewStatus:  domain.AISkillVersionReviewStatusApproved,
				Runtime:       "python3.11",
				SourceContent: "print('paid source')",
			},
			202: {
				ID:            202,
				SkillID:       21,
				UserID:        8001,
				Version:       1,
				ReviewStatus:  domain.AISkillVersionReviewStatusApproved,
				ContentFormat: "json",
				SourceContent: "{\"prompt\":\"hello\"}",
			},
		},
	}
}

func skillContractHandleRun(c *gin.Context, fixture *skillContractFixture, mode string) {
	skillID, ok := parseSkillContractID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "invalid skill id")
		return
	}
	versionID, ok := parseSkillContractID(c.Query("version_id"))
	if !ok {
		response.BadRequest(c, "invalid version id")
		return
	}

	skill, found := fixture.skills[skillID]
	if !found {
		response.NotFound(c, "skill not found")
		return
	}
	version, found := fixture.versions[versionID]
	if !found || version.SkillID != skill.ID {
		response.NotFound(c, "skill version not found")
		return
	}
	if !domain.CanUseAISkillVersion(version.ReviewStatus) {
		response.ErrorFrom(c, domain.ErrAISkillVersionNotApproved)
		return
	}

	response.Success(c, gin.H{
		"accepted": true,
		"mode":     mode,
		"skill_id": skill.ID,
	})
}

func skillContractHandlePublish(c *gin.Context, fixture *skillContractFixture) {
	versionID, ok := parseSkillContractID(c.Param("versionId"))
	if !ok {
		response.BadRequest(c, "invalid version id")
		return
	}
	version, found := fixture.versions[versionID]
	if !found {
		response.NotFound(c, "skill version not found")
		return
	}
	if !domain.CanPublishAISkillVersion(version.ReviewStatus) {
		response.ErrorFrom(c, domain.ErrAISkillVersionNotApproved)
		return
	}

	response.Success(c, gin.H{
		"published":  true,
		"skill_id":   version.SkillID,
		"version_id": version.ID,
	})
}

func skillContractHandleDetail(c *gin.Context, fixture *skillContractFixture) {
	skillID, ok := parseSkillContractID(c.Param("id"))
	if !ok {
		response.BadRequest(c, "invalid skill id")
		return
	}
	skill, found := fixture.skills[skillID]
	if !found {
		response.NotFound(c, "skill not found")
		return
	}
	if skill.PublishedVersionID == nil {
		response.NotFound(c, "published version not found")
		return
	}
	version, found := fixture.versions[*skill.PublishedVersionID]
	if !found {
		response.NotFound(c, "published version not found")
		return
	}

	viewerUserID, isAdmin := skillContractViewer(c)
	versionPayload := map[string]any{
		"id":            version.ID,
		"version":       version.Version,
		"review_status": version.ReviewStatus,
	}
	if domain.CanReadAISkillSource(skill.UserID, viewerUserID, isAdmin, skill.Visibility, skill.SourceVisibility, skill.Price, skill.PublishedVersionID) {
		versionPayload["source_content"] = version.SourceContent
	}

	response.Success(c, gin.H{
		"id":      skill.ID,
		"title":   skill.Title,
		"price":   skill.Price,
		"version": versionPayload,
	})
}

func skillContractViewer(c *gin.Context) (int64, bool) {
	rawID := strings.TrimSpace(c.GetHeader("X-Viewer-ID"))
	if rawID != "" {
		if id, err := strconv.ParseInt(rawID, 10, 64); err == nil && id > 0 {
			return id, strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Viewer-Role")), "admin")
		}
	}
	return 0, strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Viewer-Role")), "admin")
}

func parseSkillContractID(raw string) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}
