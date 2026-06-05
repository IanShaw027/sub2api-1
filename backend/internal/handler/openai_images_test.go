package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIImages_SelectionFailure_PreservesCompatibleAccountsMessage(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/images/generations", `{"model":"gpt-image-1","prompt":"cat"}`)
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Images(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"message":"No available compatible accounts"`)
}
