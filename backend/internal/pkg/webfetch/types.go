package webfetch

// FetchRequest describes one outbound web fetch.
type FetchRequest struct {
	URL               string
	ProxyURL          string
	AllowedHosts      []string
	BlockedHosts      []string
	AllowPrivate      bool
	AllowInsecureHTTP bool
	MaxRedirects      int
	MaxContentBytes   int
}

// FetchResult holds either a successful fetch payload or a structured error.
type FetchResult struct {
	RequestedURL string      `json:"requested_url,omitempty"`
	FinalURL     string      `json:"final_url,omitempty"`
	StatusCode   int         `json:"status_code,omitempty"`
	ContentType  string      `json:"content_type,omitempty"`
	Title        string      `json:"title,omitempty"`
	Text         string      `json:"text,omitempty"`
	Truncated    bool        `json:"truncated,omitempty"`
	Error        *FetchError `json:"error,omitempty"`
}

// FetchError describes bridge-friendly fetch failures without losing detail.
type FetchError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"status_code,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Retryable  bool   `json:"retryable,omitempty"`
}

func (e *FetchError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

const (
	ErrorCodeInvalidURL       = "invalid_url"
	ErrorCodeDomainBlocked    = "domain_blocked"
	ErrorCodeClientConfig     = "client_config_error"
	ErrorCodeTooManyRedirects = "too_many_redirects"
	ErrorCodeRequestFailed    = "request_failed"
	ErrorCodeHTTPStatus       = "http_status_error"
	ErrorCodeReadFailed       = "read_failed"
)
