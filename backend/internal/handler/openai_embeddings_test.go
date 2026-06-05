package handler

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIEmbeddings_SelectionFailure_ReturnsSupportingModelMessage(t *testing.T) {
	c, rec := newOpenAISelectionErrorTestContext("/v1/embeddings", `{"model":"text-embedding-3-large","input":"hello"}`)
	h := newOpenAISelectionErrorTestHandler(t, nil)

	h.Embeddings(c)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), `"message":"No available accounts supporting model: text-embedding-3-large"`)
}
