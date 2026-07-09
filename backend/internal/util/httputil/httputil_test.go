package httputil

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsCloudflareChallengeResponse_DetectsHTML503Challenge(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "text/html; charset=utf-8")

	body := []byte(`<!DOCTYPE html><html><head><title>Just a moment...</title></head><body><script>window._cf_chl_opt={};</script></body></html>`)

	require.True(t, IsCloudflareChallengeResponse(http.StatusServiceUnavailable, headers, body))
}
