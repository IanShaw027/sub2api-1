package repository

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/Wei-Shaw/sub2api/internal/pkg/reqclientpool"
	"github.com/Wei-Shaw/sub2api/internal/pkg/servertiming"
	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"
)

func forceHTTPVersion(t *testing.T, client *req.Client) string {
	t.Helper()
	transport := client.GetTransport()
	field := reflect.ValueOf(transport).Elem().FieldByName("forceHttpVersion")
	require.True(t, field.IsValid(), "forceHttpVersion field not found")
	require.True(t, field.CanAddr(), "forceHttpVersion field not addressable")
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().String()
}

func TestGetSharedReqClient_ForceHTTP2SeparatesCache(t *testing.T) {
	reqclientpool.ResetForTest()
	base := ReqClientOptions{
		ProxyURL: "http://proxy.local:8080",
		Timeout:  time.Second,
	}
	clientDefault, err := getSharedReqClient(base)
	require.NoError(t, err)

	force := base
	force.ForceHTTP2 = true
	clientForce, err := getSharedReqClient(force)
	require.NoError(t, err)

	require.NotSame(t, clientDefault, clientForce)
	require.NotEqual(t, buildReqClientKey(base), buildReqClientKey(force))
}

func TestGetSharedReqClient_ReuseCachedClient(t *testing.T) {
	reqclientpool.ResetForTest()
	opts := ReqClientOptions{
		ProxyURL: "http://proxy.local:8080",
		Timeout:  2 * time.Second,
	}
	first, err := getSharedReqClient(opts)
	require.NoError(t, err)
	second, err := getSharedReqClient(opts)
	require.NoError(t, err)
	require.Same(t, first, second)
}

func TestGetSharedReqClient_ImpersonateAndProxy(t *testing.T) {
	reqclientpool.ResetForTest()
	opts := ReqClientOptions{
		ProxyURL:    "  http://proxy.local:8080  ",
		Timeout:     4 * time.Second,
		Impersonate: true,
	}
	client, err := getSharedReqClient(opts)
	require.NoError(t, err)

	require.NotNil(t, client)
	require.Equal(t, "http://proxy.local:8080|4s|true|false|false", buildReqClientKey(opts))
}

func TestGetSharedReqClient_DisableCookiesSeparatesCacheAndDisablesJar(t *testing.T) {
	reqclientpool.ResetForTest()
	base := ReqClientOptions{
		ProxyURL: "http://proxy.local:8080",
		Timeout:  2 * time.Second,
	}
	withCookies, err := getSharedReqClient(base)
	require.NoError(t, err)
	require.NotNil(t, withCookies.GetClient().Jar)

	noCookies := base
	noCookies.DisableCookies = true
	withoutCookies, err := getSharedReqClient(noCookies)
	require.NoError(t, err)

	require.NotSame(t, withCookies, withoutCookies)
	require.NotEqual(t, buildReqClientKey(base), buildReqClientKey(noCookies))
	require.Nil(t, withoutCookies.GetClient().Jar)
}

func TestGetSharedReqClient_InvalidProxyURL(t *testing.T) {
	reqclientpool.ResetForTest()
	opts := ReqClientOptions{
		ProxyURL: "://missing-scheme",
		Timeout:  time.Second,
	}
	_, err := getSharedReqClient(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid proxy URL")
}

func TestGetSharedReqClient_ProxyURLMissingHost(t *testing.T) {
	reqclientpool.ResetForTest()
	opts := ReqClientOptions{
		ProxyURL: "http://",
		Timeout:  time.Second,
	}
	_, err := getSharedReqClient(opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "proxy URL missing host")
}

func TestCreateOpenAIReqClient_Timeout120Seconds(t *testing.T) {
	reqclientpool.ResetForTest()
	client, err := createOpenAIReqClient("http://proxy.local:8080")
	require.NoError(t, err)
	require.Equal(t, 120*time.Second, client.GetClient().Timeout)
}

func TestCreateGeminiReqClient_ForceHTTP2Disabled(t *testing.T) {
	reqclientpool.ResetForTest()
	client, err := createGeminiReqClient("http://proxy.local:8080")
	require.NoError(t, err)
	require.Equal(t, "", forceHTTPVersion(t, client))
}

func TestGetSharedReqClientRecordsDependency(t *testing.T) {
	reqclientpool.ResetForTest()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	collector := servertiming.New(time.Now())
	ctx := servertiming.WithCollector(context.Background(), collector)
	client, err := getSharedReqClient(ReqClientOptions{Timeout: time.Second})
	require.NoError(t, err)
	response, err := client.R().SetContext(ctx).Get(server.URL)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, response.StatusCode)

	header := collector.HeaderValue(time.Now(), "bypass")
	require.True(t, strings.Contains(header, "dep_http;dur="), header)
}
