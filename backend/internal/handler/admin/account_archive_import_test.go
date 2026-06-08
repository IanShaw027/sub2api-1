package admin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestParseImportArchiveRejectsAggregateJSONPayloadTooLarge(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entryPayload := archiveImportJSONFixture(archiveImportMaxJSONSize - 512)
	entryCount := archiveImportMaxSize/int64(len(entryPayload)) + 2
	for i := int64(0); i < entryCount; i++ {
		w, err := zw.Create(fmt.Sprintf("backup-%02d.json", i))
		require.NoError(t, err)
		_, err = w.Write(entryPayload)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	_, _, err := parseImportArchive(buf.Bytes())

	require.Error(t, err)
	require.Contains(t, err.Error(), "JSON 总大小")
}

func TestImportArchiveAcceptsAccountOnlySubAPIBackup(t *testing.T) {
	router, adminSvc := setupAccountDataRouter()

	archive := subAPIArchiveFixture(t, map[string]any{
		"type":    dataType,
		"version": dataVersion,
		"proxies": []map[string]any{},
		"accounts": []map[string]any{
			{
				"name":        "acc",
				"platform":    service.PlatformOpenAI,
				"type":        service.AccountTypeOAuth,
				"credentials": map[string]any{"token": "x"},
				"concurrency": 3,
				"priority":    50,
			},
		},
	})

	body, contentType := archiveMultipartBody(t, "accounts.zip", archive)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/import/archive", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, adminSvc.createdProxies, 0)
	require.Len(t, adminSvc.createdAccounts, 1)
}

func archiveImportJSONFixture(targetSize int64) []byte {
	const prefix = `{"type":"sub2api","version":"1.0","accounts":[],"padding":"`
	const suffix = `"}`
	paddingLen := int(targetSize) - len(prefix) - len(suffix)
	if paddingLen < 0 {
		paddingLen = 0
	}
	return []byte(prefix + strings.Repeat("a", paddingLen) + suffix)
}

func subAPIArchiveFixture(t *testing.T, payload map[string]any) []byte {
	t.Helper()

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("backup.json")
	require.NoError(t, err)
	_, err = w.Write(raw)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func archiveMultipartBody(t *testing.T, filename string, archive []byte) ([]byte, string) {
	t.Helper()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = fw.Write(archive)
	require.NoError(t, err)
	require.NoError(t, mw.WriteField("dedup_mode", dataImportDedupModeNone))
	require.NoError(t, mw.WriteField("skip_default_group_bind", "true"))
	require.NoError(t, mw.WriteField("update_existing", "true"))
	require.NoError(t, mw.Close())
	return body.Bytes(), mw.FormDataContentType()
}
