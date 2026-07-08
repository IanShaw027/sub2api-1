package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestTypeCyberBlocked(t *testing.T) {
	require.True(t, RequestTypeCyberBlocked.IsValid())
	require.Equal(t, "cyber", RequestTypeCyberBlocked.String())

	rt, err := ParseUsageRequestType("cyber")
	require.NoError(t, err)
	require.Equal(t, RequestTypeCyberBlocked, rt)

	// 显式 cyber 被 EffectiveRequestType 保留（不被 legacy 推导覆盖）
	u := &UsageLog{RequestType: RequestTypeCyberBlocked, Stream: true}
	require.Equal(t, RequestTypeCyberBlocked, u.EffectiveRequestType())

	// Sync 保留 cyber 且不覆盖真实 stream
	u.SyncRequestTypeAndLegacyFields()
	require.Equal(t, RequestTypeCyberBlocked, u.RequestType)
	require.True(t, u.Stream, "cyber 不应覆盖真实 stream 字段")
}

func TestRequestTypeNumericAssignmentsKeepHistoricalCyberValue(t *testing.T) {
	require.Equal(t, int16(4), int16(RequestTypeCyberBlocked), "request_type=4 is already persisted as cyber in historical rows")
	require.NotEqual(t, RequestTypeCyberBlocked, RequestTypeImage, "image must not reuse the historical cyber enum value")
}

func TestUsageLogEffectiveRequestTypeDisambiguatesLegacyImageRows(t *testing.T) {
	billingMode := string(BillingModeImage)
	inbound := "/v1/images/generations"
	u := &UsageLog{
		RequestType:       RequestTypeCyberBlocked, // raw historical value 4, reused by image during the collision window
		BillingMode:       &billingMode,
		InboundEndpoint:   &inbound,
		ImageCount:        1,
		ImageOutputTokens: 42,
	}
	require.Equal(t, RequestTypeImage, u.EffectiveRequestType())
}
