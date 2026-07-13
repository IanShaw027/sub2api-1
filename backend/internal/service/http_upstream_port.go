package service

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// OpsUpstreamFailure describes one physical upstream attempt that failed.
// It is intentionally transport-oriented so shared HTTP/WS clients can report
// failures without depending on a specific gateway implementation.
type OpsUpstreamFailure struct {
	Platform     string
	AccountID    int64
	Method       string
	URL          string
	Kind         string
	StatusCode   int
	ResponseBody string
	Err          error
}

// OpsUpstreamFailureSink persists individual failed upstream attempts. The
// implementation must not block the gateway hot path.
type OpsUpstreamFailureSink interface {
	EnqueueOpsUpstreamFailure(ctx context.Context, failure OpsUpstreamFailure)
}

// HTTPUpstreamFailureSinkSetter is implemented by the shared HTTP upstream
// transport. It stays separate from HTTPUpstream so existing test transports
// do not need no-op setter methods.
type HTTPUpstreamFailureSinkSetter interface {
	SetOpsUpstreamFailureSink(sink OpsUpstreamFailureSink)
}

// HTTPUpstream 上游 HTTP 请求接口
// 用于向上游 API（Claude、OpenAI、Gemini 等）发送请求
type HTTPUpstream interface {
	// Do 执行 HTTP 请求（不启用 TLS 指纹）
	Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error)

	// DoWithTLS 执行带 TLS 指纹伪装的 HTTP 请求
	//
	// profile 参数:
	//   - nil: 不启用 TLS 指纹，行为与 Do 方法相同
	//   - non-nil: 使用指定的 Profile 进行 TLS 指纹伪装
	//
	// Profile 由调用方通过 TLSFingerprintProfileService 解析后传入，
	// 支持按账号绑定的数据库 profile 或内置默认 profile。
	DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error)
}
