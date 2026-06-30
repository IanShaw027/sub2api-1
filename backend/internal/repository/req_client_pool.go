package repository

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/reqclientpool"

	"github.com/imroc/req/v3"
)

// ReqClientOptions 定义 req 客户端的构建参数（导出供 service 层使用）
type ReqClientOptions struct {
	ProxyURL       string        // 代理 URL（支持 http/https/socks5）
	Timeout        time.Duration // 请求超时时间
	Impersonate    bool          // 是否模拟 Chrome 浏览器指纹
	ForceHTTP2     bool          // 是否强制使用 HTTP/2
	DisableCookies bool          // 是否禁用 client-level CookieJar，避免共享客户端串 cookie
}

// GetSharedReqClient 获取共享的 req 客户端实例（导出，供 service 层使用）。
// 相同配置（代理+超时+模拟设置）复用同一客户端，复用底层连接池。
func GetSharedReqClient(opts ReqClientOptions) (*req.Client, error) {
	return reqclientpool.Get(reqclientpool.Options(opts))
}

// getSharedReqClient 获取共享的 req 客户端实例
// 性能优化：相同配置复用同一客户端，避免重复创建
func getSharedReqClient(opts ReqClientOptions) (*req.Client, error) {
	return reqclientpool.Get(reqclientpool.Options(opts))
}

func buildReqClientKey(opts ReqClientOptions) string {
	return reqclientpool.Key(reqclientpool.Options(opts))
}

// CreatePrivacyReqClient creates an HTTP client for OpenAI privacy settings API
// This is exported for use by OpenAIPrivacyService
// Uses Chrome TLS fingerprint impersonation to bypass Cloudflare checks
func CreatePrivacyReqClient(proxyURL string) (*req.Client, error) {
	return GetSharedReqClient(ReqClientOptions{
		ProxyURL:    proxyURL,
		Timeout:     30 * time.Second,
		Impersonate: true, // Enable Chrome TLS fingerprint impersonation
	})
}
