package admin

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestImportData_EnrichesOpenAICredentialsFromIDTokenWithOrganizationRole(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	dataPayload := map[string]any{
		"data": map[string]any{
			"type":    dataType,
			"version": dataVersion,
			"proxies": []map[string]any{},
			"accounts": []map[string]any{
				{
					"name":     "imported",
					"platform": service.PlatformOpenAI,
					"type":     service.AccountTypeOAuth,
					"credentials": map[string]any{
						"id_token": buildOpenAIIDTokenForAccountImportTest(t, openai.IDTokenClaims{
							Email: "user@example.com",
							OpenAIAuth: &openai.OpenAIAuthClaims{
								ChatGPTAccountID: "acct-123",
								ChatGPTPlanType:  "team",
								Organizations: []openai.OrganizationClaim{
									{
										ID:        "org-default",
										Role:      "owner",
										Title:     "Default Org",
										IsDefault: true,
									},
								},
							},
						}),
					},
					"concurrency": 3,
					"priority":    50,
				},
			},
		},
		"skip_default_group_bind": true,
	}

	body, err := json.Marshal(dataPayload)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/data", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Len(t, adminSvc.createdAccounts, 1)
	creds := adminSvc.createdAccounts[0].Credentials
	require.Equal(t, "user@example.com", creds["email"])
	require.Equal(t, "acct-123", creds["chatgpt_account_id"])
	require.Equal(t, "team", creds["plan_type"])
	require.Equal(t, "org-default", creds["organization_id"])
	require.Equal(t, "owner", creds["organization_role"])
}

func buildOpenAIIDTokenForAccountImportTest(t *testing.T, claims openai.IDTokenClaims) string {
	t.Helper()

	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	body := base64.RawURLEncoding.EncodeToString(payload)
	return header + "." + body + ".signature"
}
