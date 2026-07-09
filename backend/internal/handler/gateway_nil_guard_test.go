package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForwardGeminiAIStudioGETWithFailover_RequiresPathBuilder(t *testing.T) {
	result, account, err := (&GatewayHandler{}).forwardGeminiAIStudioGETWithFailover(nil, nil, nil)
	require.Nil(t, result)
	require.Nil(t, account)
	require.EqualError(t, err, "path builder is required")
}

func TestFetchWeChatUserInfo_RequiresTokenResponse(t *testing.T) {
	userInfo, err := fetchWeChatUserInfo(context.Background(), nil)
	require.Nil(t, userInfo)
	require.EqualError(t, err, "wechat token response is required")
}
