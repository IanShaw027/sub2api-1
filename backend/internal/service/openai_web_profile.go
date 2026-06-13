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

const (
	defaultOpenAIWebProfileClientVersion     = "prod-c9d58bd082f5fe5163759750852e4d690d489633"
	defaultOpenAIWebProfileClientBuildNumber = "6445842"
	defaultOpenAIWebProfileXOAIIS            = "ois1.initial"
)

type OpenAIWebProfile struct {
	Version                string
	Source                 string
	CapturedAt             string
	ProxyID                int64
	ProxyHash              string
	UserAgent              string
	Impersonate            string
	AcceptLanguage         string
	OAILanguage            string
	SecCHUA                string
	SecCHUAMobile          string
	SecCHUAPlatform        string
	SecCHUAArch            string
	SecCHUABitness         string
	SecCHUAFullVersion     string
	SecCHUAPlatformVersion string
	Timezone               string
	OAIClientVersion       string
	OAIClientBuildNumber   string
	XOAIIS                 string
	Viewport               OpenAIWebProfileViewport
	OAIDeviceID            string
	OAISessionID           string
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
		OAILanguage:            firstStringValue(profileMap, "oai_language", "oai_lang", "language"),
		SecCHUA:                stringValue(profileMap["sec_ch_ua"]),
		SecCHUAMobile:          stringValue(profileMap["sec_ch_ua_mobile"]),
		SecCHUAPlatform:        stringValue(profileMap["sec_ch_ua_platform"]),
		SecCHUAArch:            stringValue(profileMap["sec_ch_ua_arch"]),
		SecCHUABitness:         stringValue(profileMap["sec_ch_ua_bitness"]),
		SecCHUAFullVersion:     stringValue(profileMap["sec_ch_ua_full_version"]),
		SecCHUAPlatformVersion: stringValue(profileMap["sec_ch_ua_platform_version"]),
		Timezone:               stringValue(profileMap["timezone"]),
		OAIClientVersion:       firstStringValue(profileMap, "oai_client_version", "client_version"),
		OAIClientBuildNumber:   firstStringValue(profileMap, "oai_client_build_number", "client_build_number"),
		XOAIIS:                 firstStringValue(profileMap, "x_oai_is", "x-oai-is"),
		OAIDeviceID:            firstStringValue(profileMap, "oai_device_id", "openai_device_id", "oai_did"),
		OAISessionID:           firstStringValue(profileMap, "oai_session_id", "openai_session_id"),
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

// ResolveOpenAIImageWebProfile expands the raw nested web_profile with the
// legacy flat credential/extra fallbacks that the image bridge already knows
// how to normalize for persisted account data.
func ResolveOpenAIImageWebProfile(account *Account) *OpenAIWebProfile {
	if account == nil || !account.IsOpenAI() {
		return nil
	}
	return BuildOpenAIWebProfileFromAccountData(account.Credentials, account.Extra)
}

func (p *OpenAIWebProfile) HasOpenAIImageWeb2APIProfile() bool {
	if p == nil {
		return false
	}
	if strings.TrimSpace(p.UserAgent) == "" || strings.TrimSpace(p.AcceptLanguage) == "" {
		return false
	}
	if strings.TrimSpace(p.SecCHUA) == "" ||
		strings.TrimSpace(p.SecCHUAMobile) == "" ||
		strings.TrimSpace(p.SecCHUAPlatform) == "" ||
		strings.TrimSpace(p.SecCHUAArch) == "" ||
		strings.TrimSpace(p.SecCHUABitness) == "" ||
		strings.TrimSpace(p.SecCHUAFullVersion) == "" ||
		strings.TrimSpace(p.SecCHUAPlatformVersion) == "" {
		return false
	}
	hostname, requestPath := normalizeCookieTarget(openAIChatGPTConversationURL)
	for _, cookie := range p.Cookies {
		if strings.TrimSpace(cookie.Value) == "" {
			continue
		}
		if !cookieDomainMatchesHost(cookie.Domain, hostname) || !cookiePathMatches(cookie.Path, requestPath) {
			continue
		}
		return true
	}
	return false
}

func NormalizeOpenAIWebProfileExtra(platform string, accountType string, credentials map[string]any, extra map[string]any) map[string]any {
	if strings.ToLower(strings.TrimSpace(platform)) != PlatformOpenAI {
		return extra
	}
	if accountType != AccountTypeOAuth && accountType != AccountTypeAPIKey && accountType != "api_key" {
		return extra
	}
	extra = sanitizeMismatchedOpenAIWebProfileExtra(accountType, credentials, extra)
	profile := BuildOpenAIWebProfileFromAccountData(credentials, extra)
	if profile == nil {
		return extra
	}
	if extra == nil {
		extra = map[string]any{}
	}
	extra[openAIWebProfileExtraKey] = mergeOpenAIWebProfileExtraMap(extra[openAIWebProfileExtraKey], profile)
	return extra
}

func sanitizeMismatchedOpenAIWebProfileExtra(accountType string, credentials map[string]any, extra map[string]any) map[string]any {
	if accountType != AccountTypeOAuth || len(credentials) == 0 || len(extra) == 0 {
		return extra
	}
	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: credentials,
		Extra:       extra,
	}
	if openAICodexSnapshotIdentityTrusted(account) {
		return extra
	}

	for _, key := range []string{
		"email",
		"email_address",
		"name",
		"user_image",
		"user_picture",
		"workspace_id",
		"chatgpt_workspace_id",
		"organization_id",
		"org_id",
		"workspace_name",
		"organization_role",
		"subscription_type",
		"plan_type",
		"web_profile",
		"cookies",
		"browser_cookies",
		"storage_state",
		"session_token_present",
		"session_expires_at",
		"oai_device_id",
		"openai_device_id",
		"oai_session_id",
		"openai_session_id",
	} {
		delete(extra, key)
	}
	for key := range extra {
		if strings.HasPrefix(key, "codex_") {
			delete(extra, key)
		}
	}
	return extra
}

func mergeOpenAIWebProfileExtraMap(existing any, profile *OpenAIWebProfile) map[string]any {
	merged := map[string]any{}
	if existingMap, ok := asMap(existing); ok {
		for key, value := range existingMap {
			merged[key] = value
		}
	}

	delete(merged, "chatgpt_account_id")
	delete(merged, "account_id")
	for key, value := range profile.ToExtraMap() {
		if isEmptyOpenAIWebProfileExtraValue(value) {
			continue
		}
		merged[key] = value
	}
	delete(merged, "chatgpt_account_id")
	delete(merged, "account_id")
	return merged
}

func isEmptyOpenAIWebProfileExtraValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(typed) == ""
	case int:
		return typed == 0
	case int64:
		return typed == 0
	case float64:
		return typed == 0
	case map[string]any:
		return len(typed) == 0
	case []map[string]any:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	}
	return false
}

func BuildOpenAIWebProfileFromAccountData(credentials map[string]any, extra map[string]any) *OpenAIWebProfile {
	profile := ResolveOpenAIWebProfile(&Account{Extra: extra})
	if profile == nil {
		profile = &OpenAIWebProfile{}
	}

	if profile.Source == "" {
		profile.Source = firstStringValue(extra, "web_profile_source", "source")
	}
	if profile.Source == "" {
		profile.Source = "sub2api/account"
	}
	if profile.CapturedAt == "" {
		profile.CapturedAt = firstStringValue(extra, "web_profile_captured_at", "captured_at")
	}
	if profile.UserAgent == "" {
		profile.UserAgent = firstStringValue(credentials, "user_agent")
	}
	if profile.UserAgent == "" {
		profile.UserAgent = firstStringValue(extra, "user_agent")
	}
	if profile.Impersonate == "" {
		profile.Impersonate = firstStringValue(credentials, "impersonate")
	}
	if profile.Impersonate == "" {
		profile.Impersonate = firstStringValue(extra, "impersonate")
	}
	fillOpenAIWebProfileHeaderFields(profile, credentials)
	fillOpenAIWebProfileHeaderFields(profile, extra)
	if profile.OAILanguage == "" {
		profile.OAILanguage = deriveOpenAIWebProfileLanguage(profile.AcceptLanguage)
	}

	if profile.OAIDeviceID == "" {
		profile.OAIDeviceID = firstStringValue(extra, "oai_device_id", "openai_device_id", "oai_did")
	}
	if profile.OAISessionID == "" {
		profile.OAISessionID = firstStringValue(extra, "oai_session_id", "openai_session_id")
	}
	if profile.OAIClientVersion == "" {
		profile.OAIClientVersion = firstStringValue(extra, "oai_client_version", "client_version")
	}
	if profile.OAIClientBuildNumber == "" {
		profile.OAIClientBuildNumber = firstStringValue(extra, "oai_client_build_number", "client_build_number")
	}
	if profile.XOAIIS == "" {
		profile.XOAIIS = firstStringValue(extra, "x_oai_is", "x-oai-is")
	}
	if len(profile.Cookies) == 0 {
		profile.Cookies = firstOpenAIWebProfileCookies(extra, "cookies", "browser_cookies")
	}
	if len(profile.Cookies) == 0 {
		if storageState, ok := asMap(firstNonNil(mapValue(extra, "storage_state"), mapValue(extra, "browser_storage_state"))); ok {
			profile.Cookies = parseOpenAIWebProfileCookies(storageState["cookies"])
		}
	}
	if profile.OAIDeviceID == "" {
		profile.OAIDeviceID = oaiDeviceIDFromCookies(profile.Cookies)
	}

	if profile.UserAgent == "" && profile.AcceptLanguage == "" && profile.OAIDeviceID == "" && profile.OAISessionID == "" && len(profile.Cookies) == 0 {
		return nil
	}
	return profile
}

func fillOpenAIWebProfileHeaderFields(profile *OpenAIWebProfile, values map[string]any) {
	if profile == nil || values == nil {
		return
	}
	if profile.AcceptLanguage == "" {
		profile.AcceptLanguage = firstStringValue(values, "accept_language", "accept-language")
	}
	if profile.SecCHUA == "" {
		profile.SecCHUA = firstStringValue(values, "sec_ch_ua", "sec-ch-ua")
	}
	if profile.SecCHUAMobile == "" {
		profile.SecCHUAMobile = firstStringValue(values, "sec_ch_ua_mobile", "sec-ch-ua-mobile")
	}
	if profile.SecCHUAPlatform == "" {
		profile.SecCHUAPlatform = firstStringValue(values, "sec_ch_ua_platform", "sec-ch-ua-platform")
	}
	if profile.SecCHUAArch == "" {
		profile.SecCHUAArch = firstStringValue(values, "sec_ch_ua_arch", "sec-ch-ua-arch")
	}
	if profile.SecCHUABitness == "" {
		profile.SecCHUABitness = firstStringValue(values, "sec_ch_ua_bitness", "sec-ch-ua-bitness")
	}
	if profile.SecCHUAFullVersion == "" {
		profile.SecCHUAFullVersion = firstStringValue(values, "sec_ch_ua_full_version", "sec-ch-ua-full-version")
	}
	if profile.SecCHUAPlatformVersion == "" {
		profile.SecCHUAPlatformVersion = firstStringValue(values, "sec_ch_ua_platform_version", "sec-ch-ua-platform-version")
	}
	if profile.Timezone == "" {
		profile.Timezone = firstStringValue(values, "timezone", "timezone_id")
	}
	if profile.OAILanguage == "" {
		profile.OAILanguage = firstStringValue(values, "oai_language", "oai_lang", "language")
	}
	if profile.OAIClientVersion == "" {
		profile.OAIClientVersion = firstStringValue(values, "oai_client_version", "client_version")
	}
	if profile.OAIClientBuildNumber == "" {
		profile.OAIClientBuildNumber = firstStringValue(values, "oai_client_build_number", "client_build_number")
	}
	if profile.XOAIIS == "" {
		profile.XOAIIS = firstStringValue(values, "x_oai_is", "x-oai-is")
	}
}

func firstOpenAIWebProfileCookies(values map[string]any, keys ...string) []OpenAIWebProfileCookie {
	for _, key := range keys {
		cookies := parseOpenAIWebProfileCookies(mapValue(values, key))
		if len(cookies) > 0 {
			return cookies
		}
	}
	return nil
}

func oaiDeviceIDFromCookies(cookies []OpenAIWebProfileCookie) string {
	for _, cookie := range cookies {
		if strings.EqualFold(cookie.Name, "oai-did") && strings.TrimSpace(cookie.Value) != "" {
			return strings.TrimSpace(cookie.Value)
		}
	}
	return ""
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
	clientVersion := strings.TrimSpace(p.OAIClientVersion)
	if clientVersion == "" {
		clientVersion = defaultOpenAIWebProfileClientVersion
	}
	clientBuildNumber := strings.TrimSpace(p.OAIClientBuildNumber)
	if clientBuildNumber == "" {
		clientBuildNumber = defaultOpenAIWebProfileClientBuildNumber
	}
	xoaiis := strings.TrimSpace(p.XOAIIS)
	if xoaiis == "" {
		xoaiis = defaultOpenAIWebProfileXOAIIS
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
		"oai_language":               p.OAILanguage,
		"sec_ch_ua":                  p.SecCHUA,
		"sec_ch_ua_mobile":           p.SecCHUAMobile,
		"sec_ch_ua_platform":         p.SecCHUAPlatform,
		"sec_ch_ua_arch":             p.SecCHUAArch,
		"sec_ch_ua_bitness":          p.SecCHUABitness,
		"sec_ch_ua_full_version":     p.SecCHUAFullVersion,
		"sec_ch_ua_platform_version": p.SecCHUAPlatformVersion,
		"timezone":                   p.Timezone,
		"oai_client_version":         clientVersion,
		"oai_client_build_number":    clientBuildNumber,
		"x_oai_is":                   xoaiis,
		"oai_device_id":              p.OAIDeviceID,
		"oai_session_id":             p.OAISessionID,
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

func MergeOpenAIWebProfileResponseState(profile *OpenAIWebProfile, resp *http.Response) (*OpenAIWebProfile, bool) {
	merged, changed := MergeOpenAIWebProfileResponseCookies(profile, resp)
	if resp == nil || resp.Header == nil {
		return merged, changed
	}
	if update := strings.TrimSpace(resp.Header.Get("x-oai-is-update")); update != "" {
		if merged == nil {
			merged = cloneOpenAIWebProfile(profile)
		}
		if merged == nil {
			merged = &OpenAIWebProfile{Version: "1"}
		}
		if merged.XOAIIS != update {
			merged.XOAIIS = update
			changed = true
		}
	}
	if clientVersion := strings.TrimSpace(resp.Header.Get("oai-client-version")); clientVersion != "" {
		if merged == nil {
			merged = cloneOpenAIWebProfile(profile)
		}
		if merged == nil {
			merged = &OpenAIWebProfile{Version: "1"}
		}
		if merged.OAIClientVersion != clientVersion {
			merged.OAIClientVersion = clientVersion
			changed = true
		}
	}
	if clientBuildNumber := strings.TrimSpace(resp.Header.Get("oai-client-build-number")); clientBuildNumber != "" {
		if merged == nil {
			merged = cloneOpenAIWebProfile(profile)
		}
		if merged == nil {
			merged = &OpenAIWebProfile{Version: "1"}
		}
		if merged.OAIClientBuildNumber != clientBuildNumber {
			merged.OAIClientBuildNumber = clientBuildNumber
			changed = true
		}
	}
	return merged, changed
}

func ApplyOpenAIWebProfileHeaders(headers http.Header, profile *OpenAIWebProfile) {
	if headers == nil || profile == nil {
		return
	}
	setHeaderIfNotEmpty(headers, "User-Agent", profile.UserAgent)
	setHeaderIfNotEmpty(headers, "Accept-Language", profile.AcceptLanguage)
	setHeaderIfNotEmpty(headers, "OAI-Language", profileLanguageForHeader(profile))
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua", profile.SecCHUA)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Mobile", profile.SecCHUAMobile)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Platform", profile.SecCHUAPlatform)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Arch", profile.SecCHUAArch)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Bitness", profile.SecCHUABitness)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Full-Version", profile.SecCHUAFullVersion)
	setHeaderIfNotEmpty(headers, "Sec-Ch-Ua-Platform-Version", profile.SecCHUAPlatformVersion)
	setHeaderIfNotEmpty(headers, "OAI-Client-Version", profile.OAIClientVersion)
	setHeaderIfNotEmpty(headers, "OAI-Client-Build-Number", profile.OAIClientBuildNumber)
	setHeaderIfNotEmpty(headers, "X-OAI-IS", profile.XOAIIS)
}

func profileLanguageForHeader(profile *OpenAIWebProfile) string {
	if profile == nil {
		return ""
	}
	if trimmed := strings.TrimSpace(profile.OAILanguage); trimmed != "" {
		return trimmed
	}
	return deriveOpenAIWebProfileLanguage(profile.AcceptLanguage)
}

func deriveOpenAIWebProfileLanguage(acceptLanguage string) string {
	trimmed := strings.TrimSpace(acceptLanguage)
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(trimmed, ","); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	if idx := strings.Index(trimmed, ";"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	return strings.TrimSpace(trimmed)
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
	rawCookies, ok := asSlice(raw)
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

func asSlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []map[string]any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
		return items, true
	case json.RawMessage:
		var decoded []any
		if err := json.Unmarshal(typed, &decoded); err == nil {
			return decoded, true
		}
	case []byte:
		var decoded []any
		if err := json.Unmarshal(typed, &decoded); err == nil {
			return decoded, true
		}
	case string:
		var decoded []any
		if err := json.Unmarshal([]byte(typed), &decoded); err == nil {
			return decoded, true
		}
	}
	return nil, false
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

func mapValue(values map[string]any, key string) any {
	if values == nil {
		return nil
	}
	return values[key]
}

func firstStringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(stringValue(mapValue(values, key))); value != "" {
			return value
		}
	}
	return ""
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
