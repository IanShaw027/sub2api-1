package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloneGatewayRequestBody_DoesNotAliasInput(t *testing.T) {
	original := []byte(`{"messages":[{"role":"user","content":"keep me"}]}`)

	cloned := cloneGatewayRequestBody(original)
	require.Equal(t, original, cloned)

	cloned[1] = 'X'

	require.Equal(t, byte('"'), original[1])
	require.NotEqual(t, original[1], cloned[1])
	require.Equal(t, `{"messages":[{"role":"user","content":"keep me"}]}`, string(original))
}
