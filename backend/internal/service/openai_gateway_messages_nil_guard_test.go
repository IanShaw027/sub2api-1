package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadOpenAICompatBufferedTerminal_RequiresResponseBody(t *testing.T) {
	result, usage, acc, err := (&OpenAIGatewayService{}).readOpenAICompatBufferedTerminal(context.Background(), nil, "", "", nil)
	require.Nil(t, result)
	require.Equal(t, OpenAIUsage{}, usage)
	require.NotNil(t, acc)
	require.EqualError(t, err, "upstream response body is required")
}
