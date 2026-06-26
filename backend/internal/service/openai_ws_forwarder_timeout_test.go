package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayService_OpenAIWSTransportTimeouts_Defaults(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	timeouts := svc.openAIWSTransportTimeouts()
	require.Equal(t, defaultOpenAIWSDialTimeout, timeouts.Dial)
	require.Equal(t, defaultOpenAIWSDialTimeout+openAIWSAcquireTimeoutExtra, timeouts.Acquire)
	require.Equal(t, defaultOpenAIWSReadTimeout, timeouts.Read)
	require.Equal(t, defaultOpenAIWSWriteTimeout, timeouts.Write)
	require.Equal(t, defaultOpenAIWSReadTimeout, timeouts.PassthroughIdle)
}

func TestOpenAIGatewayService_OpenAIWSTransportTimeouts_Configured(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIWS.DialTimeoutSeconds = 7
	cfg.Gateway.OpenAIWS.ReadTimeoutSeconds = 11
	cfg.Gateway.OpenAIWS.WriteTimeoutSeconds = 13
	cfg.Gateway.OpenAIWS.AcquireTimeoutExtraMS = 900
	svc := &OpenAIGatewayService{cfg: cfg}

	timeouts := svc.openAIWSTransportTimeouts()
	require.Equal(t, 7*time.Second, timeouts.Dial)
	require.Equal(t, 7900*time.Millisecond, timeouts.Acquire)
	require.Equal(t, 11*time.Second, timeouts.Read)
	require.Equal(t, 13*time.Second, timeouts.Write)
	require.Equal(t, 11*time.Second, timeouts.PassthroughIdle)
}

func TestOpenAIGatewayService_OpenAIWSIngressPreflightPingIdle(t *testing.T) {
	svc := &OpenAIGatewayService{}
	require.Equal(t, defaultOpenAIWSIngressPreflightPingIdle, svc.openAIWSIngressPreflightPingIdle())

	svc.cfg = &config.Config{}
	svc.cfg.Gateway.OpenAIWS.IngressPreflightPingIdleSeconds = 0
	require.Equal(t, time.Duration(0), svc.openAIWSIngressPreflightPingIdle())

	svc.cfg.Gateway.OpenAIWS.IngressPreflightPingIdleSeconds = 9
	require.Equal(t, 9*time.Second, svc.openAIWSIngressPreflightPingIdle())
}
