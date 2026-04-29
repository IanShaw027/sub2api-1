package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const openAIWebProfileExtraKey = "web_profile"

type OpenAIWebProfile struct {
	Version                string
	Source                 string
	CapturedAt             string
	ProxyID                int64
	ProxyHash              string
	UserAgent              string
	Impersonate            string
	AcceptLanguage         string
	SecCHUA                string
	SecCHUAMobile          string
	SecCHUAPlatform        string
	SecCHUAArch            string
	SecCHUABitness         string
	SecCHUAFullVersion     string
	SecCHUAPlatformVersion string
	Timezone               string
	Viewport               OpenAIWebProfileViewport
	OAIDeviceID            string
	OAISessionID           string
	ChatGPTAccountID       string
	Cookies                []OpenAIWebProfileCookie
}

type OpenAIWebProfileViewport struct {
	Width             int
	Height            int
	DeviceScaleFactor float64
}

type OpenAIWebProfileCookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Secure   bool
	Expires  int64
	HTTPOnly bool
	SameSite string
}

func ResolveOpenAIWebProfile(account *Account) *OpenAIWebProfile {
	if account == nil || account.Extra == nil {
		return nil
	}
	raw, ok := account.Extra[openAIWebProfileExtraKey]
	if !ok || raw == nil {
		return nil
	}
	profileMap, ok := asMap(raw)
	if !ok {
		return nil
	}

	profile := &OpenAIWebProfile{
		Version:                stringValue(profileMap["version"]),
		Source:                 stringValue(profileMap["source"]),
		CapturedAt:             stringValue(profileMap["captured_at"]),
		ProxyID:                int64Value(profileMap["proxy_id"]),
		ProxyHash:              stringValue(profileMap["proxy_hash"]),
		UserAgent:              stringValue(profileMap["user_agent"]),
		Impersonate:            stringValue(profileMap["impersonate"]),
		AcceptLanguage:         stringValue(profileMap["accept_language"]),
		SecCHUA:                stringValue(profileMap["sec_ch_ua"]),
		SecCHUAMobile:          stringValue(profileMap["sec_ch_ua_mobile"]),
		SecCHUAPlatform:        stringValue(profileMap["sec_ch_ua_platform"]),
		SecCHUAArch:            stringValue(profileMap["sec_ch_ua_arch"]),
		SecCHUABitness:         stringValue(profileMap["sec_ch_ua_bitness"]),
		SecCHUAFullVersion:     stringValue(profileMap["sec_ch_ua_full_version"]),
		SecCHUAPlatformVersion: stringValue(profileMap["sec_ch_ua_platform_version"]),
		Timezone:               stringValue(profileMap["timezone"]),
		OAIDeviceID:            stringValue(profileMap["oai_device_id"]),
		OAISessionID:           stringValue(profileMap["oai_session_id"]),
		ChatGPTAccountID:       stringValue(profileMap["chatgpt_account_id"]),
	}
	if viewport, ok := asMap(profileMap["viewport"]); ok {
		profile.Viewport = OpenAIWebProfileViewport{
			Width:             intValue(viewport["width"]),
			Height:            intValue(viewport["height"]),
			DeviceScaleFactor: float64Value(viewport["device_scale_factor"]),
		}
	}
	profile.Cookies = parseOpenAIWebProfileCookies(profileMap["cookies"])
	return profile
}

func (p *OpenAIWebProfile) HeaderValues() http.Header {
	headers := make(http.Header)
	ApplyOpenAIWebProfileHeaders(headers, p)
	return headers
}

func (p *OpenAIWebProfile) ToExtraMap() map[string]any {
	if p == nil {
		return nil
	}
	result := map[string]any{
		"version":                    p.Version,
		"source":                     p.Source,
		"captured_at":                p.CapturedAt,
		"proxy_id":                   p.ProxyID,
		"proxy_hash":                 p.ProxyHash,
		"user_agent":                 p.UserAgent,
		"impersonate":                p.Impersonate,
		"accept_language":            p.AcceptLanguage,
		"sec_ch_ua":                  p.SecCHUA,
		"sec_ch_ua_mobile":           p.SecCHUAMobile,
		"sec_ch_ua_platform":         p.SecCHUAPlatform,
		"sec_ch_ua_arch":             p.SecCHUAArch,
		"sec_ch_ua_bitness":          p.SecCHUABitness,
		"sec_ch_ua_full_version":     p.SecCHUAFullVersion,
		"sec_ch_ua_platform_version": p.SecCHUAPlatformVersion,
		"timezone":                   p.Timezone,
		"oai_device_id":              p.OAIDeviceID,
		"oai_session_id":             p.OAISessionID,
		"chatgpt_account_id":         p.ChatGPTAccountID,
	}
	if p.Viewport.Width > 0 || p.Viewport.Height > 0 || p.Viewport.DeviceScaleFactor > 0 {
		result["viewport"] = map[string]any{
			"width":               p.Viewport.Width,
			"height":              p.Viewport.Height,
			"device_scale_factor": p.Viewport.DeviceScaleFactor,
		}
	}
	if len(p.Cookies) > 0 {
		cookies := make([]map[string]any, 0, len(p.Cookies))
		for _, cookie := range p.Cookies {
			cookies = append(cookies, map[string]any{
				"name":      cookie.Name,
				"value":     cookie.Value,
				"domain":    cookie.Domain,
				"path":      cookie.Path,
				"secure":    cookie.Secure,
				"expires":   cookie.Expires,
				"httponly":  cookie.HTTPOnly,
				"same_site": cookie.SameSite,
			})
		}
		result["cookies"] = cookies
	}
	return result
}

func MergeOpenAIWebProfileResponseCookies(profile *OpenAIWebProfile, resp *http.Response) (*OpenAIWebProfile, bool) {
	if resp == nil || resp.Header == nil {
		return profile, false
	}
	setCookies := resp.Cookies()
	if len(setCookies) == 0 {
		return profile, false
	}
	requestHost := ""
	requestPath := "/"
	if resp.Request != nil && resp.Request.URL != nil {
		if host := strings.TrimSpace(resp.Request.URL.Hostname()); host != "" {
			requestHost = strings.ToLower(host)
		}
		if path := strings.TrimSpace(resp.Request.URL.Path); path != "" {
			requestPath = path
		}
	}
	if !isChatGPTWebProfileHost(requestHost) {
		return profile, false
	}
	validCookies := make([]OpenAIWebProfileCookie, 0, len(setCookies))
	for _, cookie := range setCookies {
		if cookie == nil || isExpiredOpenAIWebProfileResponseCookie(cookie) {
			continue
		}
		item := OpenAIWebProfileCookie{
			Name:     strings.TrimSpace(cookie.Name),
			Value:    cookie.Value,
			Domain:   strings.TrimSpace(cookie.Domain),
			Path:     strings.TrimSpace(cookie.Path),
			Secure:   cookie.Secure,
			Expires:  cookie.Expires.Unix(),
			HTTPOnly: cookie.HttpOnly,
			SameSite: sameSiteString(cookie.SameSite),
		}
		if item.Domain == "" {
			item.Domain = requestHost
		}
		if !isValidOpenAIWebProfileCookie(item) || !cookieDomainMatchesHost(item.Domain, requestHost) {
			continue
		}
		if item.Path == "" {
			item.Path = defaultCookiePath(requestPath)
		}
		validCookies = append(validCookies, item)
	}
	if len(validCookies) == 0 {
		return profile, false
	}
	merged := cloneOpenAIWebProfile(profile)
	if merged == nil {
		merged = &OpenAIWebProfile{Version: "1"}
	}
	for _, cookie := range validCookies {
		merged.Cookies = upsertOpenAIWebProfileCookie(merged.Cookies, cookie)
	}
	return merged, true
}

func ApplyOpenAIWebProfileHeaders(headers http.Header, profile *OpenAIWebProfile) {
	if headers == nil || profile == nil {
		return
	}
	setHeaderIfNotEmpty(headers, "User-Agent", profile.UserAgent)
	setHeaderIfNotEmpty(headers, "Accept-Language", profile.AcceptLanguage)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua", profile.SecCHUA)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Mobile", profile.SecCHUAMobile)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Platform", profile.SecCHUAPlatform)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Arch", profile.SecCHUAArch)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Bitness", profile.SecCHUABitness)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Full-Version", profile.SecCHUAFullVersion)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Platform-Version", profile.SecCHUAPlatformVersion)
}

func (p *OpenAIWebProfile) CookieHeaderForHost(host string) string {
	if p == nil || len(p.Cookies) == 0 {
		return ""
	}
	hostname, requestPath := normalizeCookieTarget(host)
	if hostname == "" {
		return ""
	}

	values := make([]string, 0, len(p.Cookies))
	for _, cookie := range p.Cookies {
		if cookie.Name == "" || !cookieDomainMatchesHost(cookie.Domain, hostname) || !cookiePathMatches(cookie.Path, requestPath) {
			continue
		}
		values = append(values, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(values, "; ")
}

func parseOpenAIWebProfileCookies(raw any) []OpenAIWebProfileCookie {
	rawCookies, ok := raw.([]any)
	if !ok || len(rawCookies) == 0 {
		return nil
	}
	cookies := make([]OpenAIWebProfileCookie, 0, len(rawCookies))
	for _, rawCookie := range rawCookies {
		cookieMap, ok := asMap(rawCookie)
		if !ok {
			continue
		}
		cookie := OpenAIWebProfileCookie{
			Name:     stringValue(cookieMap["name"]),
			Value:    stringValue(cookieMap["value"]),
			Domain:   stringValue(cookieMap["domain"]),
			Path:     stringValue(cookieMap["path"]),
			Secure:   boolValue(cookieMap["secure"]),
			Expires:  int64Value(cookieMap["expires"]),
			HTTPOnly: boolValue(firstNonNil(cookieMap["httponly"], cookieMap["http_only"])),
			SameSite: stringValue(cookieMap["same_site"]),
		}
		if cookie.Path == "" {
			cookie.Path = "/"
		}
		if !isValidOpenAIWebProfileCookie(cookie) {
			continue
		}
		cookies = append(cookies, cookie)
	}
	return cookies
}

func isValidOpenAIWebProfileCookie(cookie OpenAIWebProfileCookie) bool {
	if cookie.Name == "" || hasCookieCTLOrSemicolon(cookie.Name) || hasCookieCTLOrSemicolon(cookie.Value) {
		return false
	}
	if cookie.Expires > 0 && cookie.Expires <= time.Now().Unix() {
		return false
	}
	return isOpenAIWebProfileCookieDomain(cookie.Domain)
}

func hasCookieCTLOrSemicolon(value string) bool {
	for _, r := range value {
		if r == ';' || r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func isExpiredOpenAIWebProfileResponseCookie(cookie *http.Cookie) bool {
	if cookie.MaxAge < 0 {
		return true
	}
	return !cookie.Expires.IsZero() && !cookie.Expires.After(time.Now())
}

func isOpenAIWebProfileCookieDomain(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(domain, ".")))
	return domain == "chatgpt.com" || strings.HasSuffix(domain, ".chatgpt.com") || domain == "openai.com" || strings.HasSuffix(domain, ".openai.com")
}

func isChatGPTWebProfileHost(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	return hostname == "chatgpt.com" || strings.HasSuffix(hostname, ".chatgpt.com")
}

func cloneOpenAIWebProfile(profile *OpenAIWebProfile) *OpenAIWebProfile {
	if profile == nil {
		return nil
	}
	cloned := *profile
	if len(profile.Cookies) > 0 {
		cloned.Cookies = append([]OpenAIWebProfileCookie(nil), profile.Cookies...)
	}
	return &cloned
}

func upsertOpenAIWebProfileCookie(cookies []OpenAIWebProfileCookie, next OpenAIWebProfileCookie) []OpenAIWebProfileCookie {
	for i := range cookies {
		if strings.EqualFold(cookies[i].Name, next.Name) && strings.EqualFold(strings.TrimPrefix(cookies[i].Domain, "."), strings.TrimPrefix(next.Domain, ".")) && cookies[i].Path == next.Path {
			cookies[i] = next
			return cookies
		}
	}
	return append(cookies, next)
}

func defaultCookiePath(requestPath string) string {
	requestPath = strings.TrimSpace(requestPath)
	if requestPath == "" || requestPath[0] != '/' {
		return "/"
	}
	idx := strings.LastIndex(requestPath, "/")
	if idx <= 0 {
		return "/"
	}
	return requestPath[:idx]
}

func sameSiteString(value http.SameSite) string {
	switch value {
	case http.SameSiteDefaultMode:
		return "Default"
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteNoneMode:
		return "None"
	default:
		return ""
	}
}

func normalizeCookieTarget(rawHost string) (string, string) {
	requestPath := "/"
	target := strings.TrimSpace(rawHost)
	if target == "" {
		return "", requestPath
	}
	if !strings.Contains(target, "://") {
		target = "https://" + target
	}
	parsed, err := url.Parse(target)
	if err == nil && parsed.Hostname() != "" {
		if parsed.Path != "" {
			requestPath = parsed.Path
		}
		return strings.ToLower(parsed.Hostname()), requestPath
	}
	return strings.ToLower(strings.Trim(rawHost, "[]")), requestPath
}

func cookieDomainMatchesHost(domain string, hostname string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if domain == "" || hostname == "" {
		return false
	}
	domain = strings.TrimPrefix(domain, ".")
	if hostname == domain || strings.HasSuffix(hostname, "."+domain) {
		return true
	}
	if domain == "chatgpt.com" && hostname == "chat.openai.com" {
		return true
	}
	return false
}

func cookiePathMatches(cookiePath string, requestPath string) bool {
	if cookiePath == "" || cookiePath == "/" {
		return true
	}
	if requestPath == "" {
		requestPath = "/"
	}
	return requestPath == cookiePath || strings.HasPrefix(requestPath, strings.TrimRight(cookiePath, "/")+"/")
}

func setHeaderIfNotEmpty(headers http.Header, key string, value string) {
	if value != "" {
		headers.Set(key, value)
	}
}

func asMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case json.RawMessage:
		var decoded map[string]any
		if err := json.Unmarshal(typed, &decoded); err == nil {
			return decoded, true
		}
	case []byte:
		var decoded map[string]any
		if err := json.Unmarshal(typed, &decoded); err == nil {
			return decoded, true
		}
	case string:
		var decoded map[string]any
		if err := json.Unmarshal([]byte(typed), &decoded); err == nil {
			return decoded, true
		}
	}
	return nil, false
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func intValue(value any) int {
	return int(int64Value(value))
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	default:
		return 0
	}
}

func float64Value(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		parsed, _ := typed.Float64()
		return parsed
	case string:
		parsed, _ := strconv.ParseFloat(typed, 64)
		return parsed
	default:
		return 0
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(typed)
		return parsed
	default:
		return false
	}
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
