package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// codexFingerprintIDsContextKey 是暂存在 gin context 的收敛 ID 集合键。
// 由 Forward（非透传）或 forwardOpenAIPassthrough（透传）解析后写入，请求
// 构造器读取用于出站头改写——请求体与出站头必须共享同一份 IDs，保证
// turn_id 等随机字段一致。
const codexFingerprintIDsContextKey = "codex_fingerprint_ids"

// stageCodexFingerprintIDs 将本 attempt 解析出的收敛 ID 暂存到 gin context。
// 必须无条件覆写（含 nil）：failover 从收敛账号切到 off 账号时，上一账号的
// IDs 不得残留并被误应用到新账号的出站头（typed-nil 由应用侧 nil 守卫吸收）。
func stageCodexFingerprintIDs(c *gin.Context, ids *codexFingerprintIDs) {
	if c != nil {
		c.Set(codexFingerprintIDsContextKey, ids)
	}
}

func stagedCodexFingerprintIDs(c *gin.Context, account *Account) *codexFingerprintIDs {
	if c == nil || account == nil || account.Type != AccountTypeOAuth {
		return nil
	}
	value, ok := c.Get(codexFingerprintIDsContextKey)
	if !ok {
		return nil
	}
	ids, ok := value.(*codexFingerprintIDs)
	if !ok || ids == nil || ids.accountID != account.ID {
		return nil
	}
	return ids
}

// applyStagedCodexFingerprintHeaders 读取 context 暂存的收敛 ID 并改写出站头。
// 非透传与透传两个请求构造器共用本函数，防止应用语义漂移。仅解析该
// snapshot 的 OAuth 账号可读取，避免 stale context 跨账号 failover 泄漏。
func applyStagedCodexFingerprintHeaders(c *gin.Context, account *Account, h http.Header) {
	applyCodexFingerprintHeaders(h, stagedCodexFingerprintIDs(c, account))
}

func applyStagedCodexFingerprintClientMetadata(c *gin.Context, account *Account, reqBody map[string]any) bool {
	return applyCodexFingerprintClientMetadata(reqBody, stagedCodexFingerprintIDs(c, account))
}

// codexFingerprintMode 控制 OAuth 账号出站请求的设备指纹收敛强度。
// 多人共享同一 OAuth 账号时，每个用户的 Codex 客户端会携带各自不同的
// installation_id / session_id / thread_id，上游据此判定设备数和会话数。
// 收敛模式将这些标识改写为账号级恒定值，减少上游可见的设备/会话指纹。
type codexFingerprintMode string

const (
	// codexFingerprintOff 不做任何收敛，原样透传客户端标识。
	// 这是默认值：收敛是显式 opt-in 的（见 GetCodexFingerprintMode）。
	codexFingerprintOff codexFingerprintMode = "off"
	// codexFingerprintDevice 仅收敛 installation_id 为账号级恒定值。
	// 上游看到 1 台设备 + 多会话（每用户各自的 session）。
	codexFingerprintDevice codexFingerprintMode = "device"
	// codexFingerprintSession 收敛 installation_id + session_id，
	// thread_id 按客户端原始 session-id 确定性派生（每个真实 Codex 会话一个独立线程）。
	// 上游看到 1 台设备 + 1 会话 + N 线程，最接近正常用户 spawn 子代理的模式。
	codexFingerprintSession codexFingerprintMode = "session"
	// codexFingerprintFull 收敛所有标识：installation_id + session_id + thread_id。
	// 上游看到 1 台设备 + 1 会话 + 1 线程，最激进。
	codexFingerprintFull codexFingerprintMode = "full"
)

const (
	codexFingerprintModeExtraKey     = "codex_fingerprint_mode"
	codexFingerprintSeedExtraKey     = "codex_fingerprint_seed"
	outboundDeviceProfileLoadTimeout = 2 * time.Second
	codexIdentityRejectLogInterval   = 30 * time.Second
)

var (
	codexIdentityRejectLogMu   sync.Mutex
	codexIdentityRejectLastLog = map[int64]time.Time{}
)

func logCodexIdentityReject(accountID int64, err error) {
	now := time.Now()
	codexIdentityRejectLogMu.Lock()
	if last, ok := codexIdentityRejectLastLog[accountID]; ok && now.Sub(last) < codexIdentityRejectLogInterval {
		codexIdentityRejectLogMu.Unlock()
		return
	}
	codexIdentityRejectLastLog[accountID] = now
	codexIdentityRejectLogMu.Unlock()
	slog.Warn("identity_reject: failed to load outbound device profile", "account_id", accountID, "error", err)
}

func canonicalCodexFingerprintSeed(value any) (string, bool) {
	raw, ok := value.(string)
	if !ok {
		return "", false
	}
	trimmed := strings.TrimSpace(raw)
	parsed, err := uuid.Parse(trimmed)
	if err != nil || parsed == uuid.Nil || trimmed != parsed.String() {
		return "", false
	}
	return trimmed, true
}

func newCodexFingerprintSeed() string {
	return uuid.NewString()
}

func stripCodexFingerprintSeed(extra map[string]any) map[string]any {
	if extra == nil {
		return nil
	}
	stripped := maps.Clone(extra)
	delete(stripped, codexFingerprintSeedExtraKey)
	return stripped
}

func codexFingerprintModeFromExtra(extra map[string]any) codexFingerprintMode {
	if extra == nil {
		return codexFingerprintOff
	}
	raw, _ := extra[codexFingerprintModeExtraKey].(string)
	switch codexFingerprintMode(strings.TrimSpace(raw)) {
	case codexFingerprintOff, codexFingerprintDevice, codexFingerprintSession, codexFingerprintFull:
		return codexFingerprintMode(strings.TrimSpace(raw))
	default:
		return codexFingerprintOff
	}
}

func codexFingerprintModeRequiresSeed(mode codexFingerprintMode) bool {
	switch mode {
	case codexFingerprintDevice, codexFingerprintSession, codexFingerprintFull:
		return true
	default:
		return false
	}
}

func codexFingerprintSeed(extra map[string]any) (string, bool) {
	if extra == nil {
		return "", false
	}
	return canonicalCodexFingerprintSeed(extra[codexFingerprintSeedExtraKey])
}

func prepareCodexFingerprintExtraForCreate(platform, accountType string, extra map[string]any) map[string]any {
	prepared := stripCodexFingerprintSeed(extra)
	if platform != PlatformOpenAI || accountType != AccountTypeOAuth || !codexFingerprintModeRequiresSeed(codexFingerprintModeFromExtra(prepared)) {
		return prepared
	}
	if prepared == nil {
		prepared = make(map[string]any, 1)
	}
	prepared[codexFingerprintSeedExtraKey] = newCodexFingerprintSeed()
	return prepared
}

func prepareCodexFingerprintExtraForUpdate(account *Account, extra map[string]any) map[string]any {
	prepared := stripCodexFingerprintSeed(extra)
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return prepared
	}
	if seed, ok := codexFingerprintSeed(account.Extra); ok {
		if prepared == nil {
			prepared = make(map[string]any, 1)
		}
		prepared[codexFingerprintSeedExtraKey] = seed
		return prepared
	}
	if codexFingerprintModeRequiresSeed(codexFingerprintModeFromExtra(prepared)) {
		if prepared == nil {
			prepared = make(map[string]any, 1)
		}
		prepared[codexFingerprintSeedExtraKey] = newCodexFingerprintSeed()
	}
	return prepared
}

func sanitizedCodexFingerprintExtraUpdates(updates map[string]any) map[string]any {
	if updates == nil {
		return nil
	}
	sanitized := maps.Clone(updates)
	delete(sanitized, codexFingerprintSeedExtraKey)
	return sanitized
}

// ShouldEnsureCodexFingerprintSeedForExtraUpdates reports whether a JSONB key-level
// extra update is enabling Codex fingerprint convergence and therefore must atomically
// preserve or create the system-managed per-account seed in the repository update.
func ShouldEnsureCodexFingerprintSeedForExtraUpdates(updates map[string]any) bool {
	if updates == nil {
		return false
	}
	return codexFingerprintModeRequiresSeed(codexFingerprintModeFromExtra(updates))
}

// GetCodexFingerprintMode 从账号 extra JSON 读取指纹收敛模式。
//
// **收敛是显式 opt-in**：未设置、空值或非法值一律按 off 处理，只有管理员
// 明确配置 device / session / full 才收敛。
//
// 历史：v0.1.175（#5553）把缺省值当作 session，导致升级后存量 OAuth 账号
// （普遍没有这个 extra 键）的每个非透传请求都被静默改写 installation /
// session / thread / turn / window 五类标识；#5555、#5556、#5582 报告的额度
// 缩水都卡在该版本边界，并有"回退 v0.1.173 即恢复"与"新账号开收敛后降额"
// 的 A/B 实测。上游的配额判定策略不可观测，因此这里取兼容安全的一侧：
// 不显式 opt-in 就保持 v0.1.175 之前的客户端身份（#5610）。
func (a *Account) GetCodexFingerprintMode() codexFingerprintMode {
	if a == nil || !a.IsOpenAIOAuth() {
		return codexFingerprintOff
	}
	return codexFingerprintModeFromExtra(a.Extra)
}

// deriveStableUUIDv4 从种子确定性派生一个 UUIDv4 格式的字符串。
// 同一种子永远返回同一值。
func deriveStableUUIDv4(seed string) string {
	h := sha256.Sum256([]byte(seed))
	b := h[:16]
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 1
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]),
		binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16])
}

func loadOutboundCodexProfile(ctx context.Context, account *Account) *AccountDeviceProfile {
	if account == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	loadCtx, cancel := context.WithTimeout(ctx, outboundDeviceProfileLoadTimeout)
	defer cancel()
	profile, err := LoadOutboundDeviceProfile(loadCtx, account)
	if err != nil || profile == nil {
		logCodexIdentityReject(account.ID, err)
		return nil
	}
	return profile
}

// loadExistingOutboundCodexProfile returns the pinned outbound profile for
// fingerprint stamping. When the device service is configured and no row
// exists it GetOrCreates once so body client_metadata, session headers, and
// enforceCodexIdentityFromAccount share the same IDs on request 1 and stay
// stable on request 2. Unconfigured service returns (nil, nil) so the caller
// can seed-fallback. Load/mint errors fail closed (no seed fallback).
func loadExistingOutboundCodexProfile(ctx context.Context, account *Account) (*AccountDeviceProfile, error) {
	if account == nil {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	svc := OutboundDeviceProfileService()
	if svc == nil {
		return nil, nil
	}
	loadCtx, cancel := context.WithTimeout(ctx, outboundDeviceProfileLoadTimeout)
	defer cancel()
	profile, err := LoadOutboundDeviceProfile(loadCtx, account)
	if err != nil {
		if errors.Is(err, ErrDeviceProfileServiceUnconfigured) {
			return nil, nil
		}
		logCodexIdentityReject(account.ID, err)
		return nil, err
	}
	return profile, nil
}

// resolveConvergedInstallationIDFromProfile 只读已校验的设备档案；
// 加载或校验失败时返回空串，由调用方跳过收敛，不造半包。
func resolveConvergedInstallationIDFromProfile(ctx context.Context, account *Account) string {
	profile := loadOutboundCodexProfile(ctx, account)
	if profile == nil {
		return ""
	}
	return strings.TrimSpace(profile.InstallationID)
}

// resolveConvergedInstallationID 优先使用管理员配置的真实 device_id，
// 无则从系统管理的账号随机种子确定性派生。
func resolveConvergedInstallationID(account *Account, seed string) string {
	if account == nil {
		return ""
	}
	if deviceID := account.GetOpenAIDeviceID(); deviceID != "" {
		return deviceID
	}
	if seed == "" {
		return ""
	}
	return deriveStableUUIDv4("sub2api:codex-install-id:v2:" + seed)
}

// resolveConvergedSessionID 返回账号级恒定的 session_id。
// 合法 fingerprint seed（UUID）走 v2 种子派生；否则按设备档案
// session_namespace 用 DeriveSessionIDs 派生（identity pinning）。
// fingerprintSeedSessionNamespace is the 32-hex namespace minted for an
// OpenAI baseline that adopted a fingerprint seed. finishCodexFingerprintIDs
// recognizes it and stamps historical seed-v2 session/thread IDs so a
// mint-once profile does not flip away from the seed identity.
func fingerprintSeedSessionNamespace(seed string) string {
	h := sha256.Sum256([]byte("sub2api:codex-session-ns:v1:" + seed))
	return hex.EncodeToString(h[:16])
}

func codexFingerprintFinishKey(account *Account, profile *AccountDeviceProfile) string {
	if profile == nil {
		return ""
	}
	if account != nil {
		if seed, ok := codexFingerprintSeed(account.Extra); ok && profile.SessionNamespace == fingerprintSeedSessionNamespace(seed) {
			return seed
		}
	}
	return profile.SessionNamespace
}

func resolveConvergedSessionID(key string) string {
	if key == "" {
		return ""
	}
	if seed, ok := canonicalCodexFingerprintSeed(key); ok {
		return deriveStableUUIDv4("sub2api:codex-session-id:v2:" + seed)
	}
	sessionID, _, _, err := DeriveSessionIDs(key, "")
	if err != nil {
		return ""
	}
	return sessionID
}

// resolveConvergedThreadID 按客户端原始 session-id 派生 thread_id。
// 每个真实 Codex 会话（不同客户端启动实例）获得一个独立线程，
// 模拟正常用户 spawn 子代理或开多窗口的模式。
//
// 合法 fingerprint seed 走 v2 种子派生。namespace 路径使用
// DeriveSessionIDs(ns, clientSessionID).sessionID, not the returned
// threadID. Design §7's thread_id is HMAC(ns, "thread:"+session_id) and would
// collapse session mode to one thread per account-stable session.
func resolveConvergedThreadID(key, clientSessionID string) string {
	if key == "" || clientSessionID == "" {
		return ""
	}
	if seed, ok := canonicalCodexFingerprintSeed(key); ok {
		return deriveStableUUIDv4("sub2api:codex-thread-id:v2:" + seed + ":" + clientSessionID)
	}
	sessionID, _, _, err := DeriveSessionIDs(key, clientSessionID)
	if err != nil {
		return ""
	}
	return sessionID
}

// codexFingerprintIDs 收敛后的完整 ID 集合。
// 由 resolveCodexFingerprintIDs 一次性生成，同一个实例在头改写和体改写之间共享，
// 确保所有载体中的 turn_id 等随机字段一致。体改写时还会补记原始
// client_metadata.session_id，用于识别 root prompt_cache_key 的默认值。
type codexFingerprintIDs struct {
	accountID                     int64
	mode                          codexFingerprintMode
	profile                       *AccountDeviceProfile
	installationID                string
	sessionID                     string
	threadID                      string
	turnID                        string
	windowID                      string
	turnStartedAtUnixMs           int64
	originalBodySessionID         string
	originalBodySessionIDCaptured bool
}

const outboundDeviceProfileGinKey = "outbound_device_profile"

type outboundDeviceProfileGinValue struct {
	accountID int64
	profile   *AccountDeviceProfile
}

func outboundDeviceAccountID(account *Account) int64 {
	if account == nil {
		return 0
	}
	return canonicalDeviceAccountID(account)
}

func outboundDeviceProfileFromGin(c *gin.Context, account *Account) *AccountDeviceProfile {
	profile, ok := outboundDeviceProfileFromGinForAccount(c, outboundDeviceAccountID(account))
	if !ok {
		return nil
	}
	return profile
}

func outboundDeviceProfileFromGinForAccount(c *gin.Context, accountID int64) (*AccountDeviceProfile, bool) {
	if c == nil || accountID == 0 {
		return nil, false
	}
	v, ok := c.Get(outboundDeviceProfileGinKey)
	if !ok {
		return nil, false
	}
	stashed, ok := v.(*outboundDeviceProfileGinValue)
	if !ok || stashed == nil || stashed.accountID != accountID {
		return nil, false
	}
	return stashed.profile, true
}

func stashOutboundDeviceProfile(c *gin.Context, accountID int64, profile *AccountDeviceProfile) {
	if c == nil || accountID == 0 {
		return
	}
	c.Set(outboundDeviceProfileGinKey, &outboundDeviceProfileGinValue{
		accountID: accountID,
		profile:   profile,
	})
}

func resolveOpenAIOutboundDeviceProfile(ctx context.Context, c *gin.Context, account *Account) *AccountDeviceProfile {
	accountID := outboundDeviceAccountID(account)
	if accountID == 0 {
		return nil
	}
	if profile, ok := outboundDeviceProfileFromGinForAccount(c, accountID); ok {
		return profile
	}
	profile := loadOpenAIOutboundSessionProfile(ctx, account)
	stashOutboundDeviceProfile(c, accountID, profile)
	return profile
}

func fingerprintIDsBelongToAccount(ids *codexFingerprintIDs, account *Account) bool {
	if ids == nil || ids.profile == nil {
		return false
	}
	return ids.profile.AccountID == outboundDeviceAccountID(account)
}

func clearCodexFingerprintIDsForAccount(c *gin.Context, account *Account) {
	if c == nil {
		return
	}
	v, ok := c.Get("codex_fingerprint_ids")
	if !ok {
		return
	}
	ids, ok := v.(*codexFingerprintIDs)
	if !ok || !fingerprintIDsBelongToAccount(ids, account) {
		return
	}
	c.Set("codex_fingerprint_ids", (*codexFingerprintIDs)(nil))
}

// resolveCodexFingerprintIDs 按收敛模式计算出站 ID 集合。
// clientSessionID 是客户端原始的 session-id 头值（连字符形式），用于 session 模式下
// 的 thread_id 派生——每个真实 Codex 会话得到一个独立线程。
// 返回 nil 表示 off 模式，不需要改写。
// 注意：包含随机生成的 turn_id，调用方必须只调用一次并共享结果给头改写和体改写。
// 优先使用已校验设备档案（identity pinning）。服务已配置但档案不存在时
// GetOrCreate 一次（mint-once），避免同请求后续 GetOrCreate 与种子 ID 分裂。
// 服务未配置时回退账号种子派生；加载/铸造失败 fail-closed（不回退种子）。
func resolveCodexFingerprintIDs(account *Account, clientSessionID string, mode codexFingerprintMode) *codexFingerprintIDs {
	return resolveCodexFingerprintIDsWithContext(context.Background(), account, clientSessionID, mode)
}

func resolveCodexFingerprintIDsWithContext(ctx context.Context, account *Account, clientSessionID string, mode codexFingerprintMode) *codexFingerprintIDs {
	if account == nil || mode == codexFingerprintOff {
		return nil
	}

	ids := &codexFingerprintIDs{
		accountID:           account.ID,
		mode:                mode,
		turnStartedAtUnixMs: time.Now().UnixMilli(),
	}

	if profile, err := loadExistingOutboundCodexProfile(ctx, account); err != nil {
		return nil
	} else if profile != nil {
		ids.profile = profile
		ids.installationID = strings.TrimSpace(profile.InstallationID)
		if ids.installationID == "" {
			return nil
		}
		return finishCodexFingerprintIDs(ids, account, codexFingerprintFinishKey(account, profile), clientSessionID, mode)
	}

	seed, ok := codexFingerprintSeed(account.Extra)
	if !ok {
		return nil
	}
	ids.installationID = resolveConvergedInstallationID(account, seed)
	if ids.installationID == "" {
		return nil
	}
	return finishCodexFingerprintIDs(ids, account, seed, clientSessionID, mode)
}

func finishCodexFingerprintIDs(ids *codexFingerprintIDs, account *Account, key, clientSessionID string, mode codexFingerprintMode) *codexFingerprintIDs {
	switch mode {
	case codexFingerprintDevice:
		return ids
	case codexFingerprintSession:
		ids.sessionID = resolveConvergedSessionID(key)
		if ids.sessionID == "" {
			if account != nil && ids.profile != nil {
				logCodexIdentityReject(account.ID, fmt.Errorf("identity_reject: derive session_id from session_namespace"))
			}
			return nil
		}
		ids.threadID = resolveConvergedThreadID(key, clientSessionID)
		if ids.threadID == "" {
			ids.threadID = ids.sessionID
		}
		ids.turnID = uuid.Must(uuid.NewV7()).String()
		ids.windowID = ids.threadID + ":0"
		return ids
	case codexFingerprintFull:
		ids.sessionID = resolveConvergedSessionID(key)
		if ids.sessionID == "" {
			if account != nil && ids.profile != nil {
				logCodexIdentityReject(account.ID, fmt.Errorf("identity_reject: derive session_id from session_namespace"))
			}
			return nil
		}
		ids.threadID = ids.sessionID
		ids.turnID = uuid.Must(uuid.NewV7()).String()
		ids.windowID = ids.threadID + ":0"
		return ids
	default:
		return nil
	}
}

// extractClientSessionID 从请求头中提取客户端原始的会话标识。
// 优先取 session-id（连字符形式，Codex CLI 标准），回退到 session_id（下划线形式）。
// 返回的值尚未被 isolateOpenAISessionID 改写，是客户端的真实标识。
func extractClientSessionID(h http.Header) string {
	if v := strings.TrimSpace(h.Get("session-id")); v != "" {
		return v
	}
	return strings.TrimSpace(h.Get("session_id"))
}

// applyCodexSharedRequestIdentity loads the outbound profile once and stamps
// request-body client_metadata from the same IDs used for outbound headers.
// When no shared header identity will be applied (fpIDs == nil), the body is
// left unchanged — including skipping a standalone InstallationID stamp.
func applyCodexSharedRequestIdentity(ctx context.Context, reqBody map[string]any, account *Account, clientHeaders http.Header) *codexFingerprintIDs {
	fpIDs := resolveCodexFingerprintIDsFromRequestContext(ctx, account, clientHeaders)
	if fpIDs == nil {
		return nil
	}
	applyCodexFingerprintClientMetadata(reqBody, fpIDs)
	return fpIDs
}

// applyCodexForwardRequestIdentity is the OpenAI Forward identity block:
// one profile load, stamp body from those IDs, and store them for outbound headers.
func applyCodexForwardRequestIdentity(ctx context.Context, c *gin.Context, reqBody map[string]any, account *Account, clientHeaders http.Header) *codexFingerprintIDs {
	fpIDs := applyCodexSharedRequestIdentity(ctx, reqBody, account, clientHeaders)
	if c == nil {
		return fpIDs
	}
	stageCodexFingerprintIDs(c, fpIDs)
	if fpIDs != nil {
		stashOutboundDeviceProfile(c, outboundDeviceAccountID(account), fpIDs.profile)
		return fpIDs
	}
	if account != nil && account.GetCodexFingerprintMode() != codexFingerprintOff {
		stashOutboundDeviceProfile(c, outboundDeviceAccountID(account), nil)
		clearCodexFingerprintIDsForAccount(c, account)
	}
	return nil
}

// resolveCodexFingerprintIDsFromRequest 从客户端原始请求头中提取 session-id，
// 结合账号配置一次性解析收敛 ID 集合。调用方应将返回的 ids 同时传给
// applyCodexFingerprintHeaders 和 applyCodexFingerprintClientMetadata。
func resolveCodexFingerprintIDsFromRequest(account *Account, clientHeaders http.Header) *codexFingerprintIDs {
	return resolveCodexFingerprintIDsFromRequestContext(context.Background(), account, clientHeaders)
}

func resolveCodexFingerprintIDsFromRequestContext(ctx context.Context, account *Account, clientHeaders http.Header) *codexFingerprintIDs {
	if account == nil {
		return nil
	}
	mode := account.GetCodexFingerprintMode()
	if mode == codexFingerprintOff {
		return nil
	}
	clientSessionID := ""
	if clientHeaders != nil {
		clientSessionID = extractClientSessionID(clientHeaders)
	}
	return resolveCodexFingerprintIDsWithContext(ctx, account, clientSessionID, mode)
}

// applyCodexFingerprintHeaders 按预计算的收敛 ID 改写出站 HTTP 头中的设备指纹。
// 在 buildUpstreamRequest 的白名单透传之后、enforceCodexIdentityHeaders 之前调用。
func applyCodexFingerprintHeaders(h http.Header, ids *codexFingerprintIDs) {
	if h == nil || ids == nil {
		return
	}

	// 所有非 off 模式都收敛 installation_id
	h.Set("x-codex-installation-id", ids.installationID)

	if ids.mode == codexFingerprintDevice {
		rewriteCodexTurnMetadataFields(h, map[string]any{
			"installation_id": ids.installationID,
		})
		return
	}

	// session / full 模式：改写所有相关头
	h.Set("x-codex-window-id", ids.windowID)
	h.Set("x-client-request-id", ids.threadID)
	// 连字符形式和下划线形式都改写，保证一致
	h.Set("session-id", ids.sessionID)
	h.Set("session_id", ids.sessionID)
	h.Set("thread-id", ids.threadID)

	rewriteCodexTurnMetadataFields(h, map[string]any{
		"installation_id":         ids.installationID,
		"session_id":              ids.sessionID,
		"thread_id":               ids.threadID,
		"turn_id":                 ids.turnID,
		"window_id":               ids.windowID,
		"turn_started_at_unix_ms": ids.turnStartedAtUnixMs,
	})
}

// rewriteCodexTurnMetadataFields 解析 x-codex-turn-metadata 头中的 JSON，
// 替换指定字段后回写。合法对象保留未指定字段（如 sandbox、thread_source）；
// 非法/非对象值重建为最小合法 metadata，避免 flat 与 embedded identity 分裂。
func rewriteCodexTurnMetadataFields(h http.Header, fields map[string]any) {
	raw := strings.TrimSpace(h.Get("x-codex-turn-metadata"))
	if raw == "" {
		return
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil || metadata == nil {
		metadata = make(map[string]any, len(fields))
	}
	for k, v := range fields {
		metadata[k] = v
	}
	rebuilt, err := json.Marshal(metadata)
	if err != nil {
		return
	}
	h.Set("x-codex-turn-metadata", string(rebuilt))
}

// applyCodexFingerprintClientMetadata 按预计算的收敛 ID 改写请求体中的 client_metadata。
// 使用与头改写相同的 ids 实例，确保 turn_id 等随机字段一致。
func applyCodexFingerprintClientMetadata(reqBody map[string]any, ids *codexFingerprintIDs) bool {
	if reqBody == nil || ids == nil {
		return false
	}

	captureCodexFingerprintOriginalBodySessionID(ids, reqBody["client_metadata"])
	existing, _ := reqBody["client_metadata"].(map[string]any)
	if existing == nil {
		existing = make(map[string]any)
	}

	modified := false
	if applyCodexFingerprintToClientMetadataMap(existing, ids) {
		reqBody["client_metadata"] = existing
		modified = true
	}
	if applyCodexFingerprintPromptCacheKey(reqBody, ids) {
		modified = true
	}
	return modified
}

// applyCodexFingerprintToClientMetadataMap 是 client_metadata 改写的共享核心，
// map 版（非透传，body 已解码）与 raw 字节版（透传热路径）都经由它，保证两条
// 路径的收敛语义永不漂移。
func applyCodexFingerprintToClientMetadataMap(existing map[string]any, ids *codexFingerprintIDs) bool {
	if existing == nil || ids == nil {
		return false
	}

	modified := false

	if ids.installationID != "" {
		existing["x-codex-installation-id"] = ids.installationID
		modified = true
	}

	if ids.mode == codexFingerprintDevice {
		rewriteClientMetadataEmbeddedTurnMetadata(existing, map[string]any{
			"installation_id": ids.installationID,
		})
		return modified
	}

	// session / full 模式
	existing["session_id"] = ids.sessionID
	existing["thread_id"] = ids.threadID
	existing["turn_id"] = ids.turnID
	existing["x-codex-window-id"] = ids.windowID

	rewriteClientMetadataEmbeddedTurnMetadata(existing, map[string]any{
		"installation_id":         ids.installationID,
		"session_id":              ids.sessionID,
		"thread_id":               ids.threadID,
		"turn_id":                 ids.turnID,
		"window_id":               ids.windowID,
		"turn_started_at_unix_ms": ids.turnStartedAtUnixMs,
	})
	return true
}

func captureCodexFingerprintOriginalBodySessionID(ids *codexFingerprintIDs, clientMetadata any) {
	if ids == nil || ids.originalBodySessionIDCaptured {
		return
	}
	ids.originalBodySessionIDCaptured = true
	if clientMetadata == nil {
		return
	}
	switch metadata := clientMetadata.(type) {
	case map[string]any:
		if sessionID, ok := metadata["session_id"].(string); ok {
			ids.originalBodySessionID = strings.TrimSpace(sessionID)
		}
	case map[string]string:
		ids.originalBodySessionID = strings.TrimSpace(metadata["session_id"])
	}
}

func captureCodexFingerprintOriginalBodySessionIDRaw(ids *codexFingerprintIDs, value gjson.Result) {
	if ids == nil || ids.originalBodySessionIDCaptured {
		return
	}
	ids.originalBodySessionIDCaptured = true
	if value.Exists() && value.Type == gjson.String {
		ids.originalBodySessionID = strings.TrimSpace(value.String())
	}
}

func shouldRewriteCodexFingerprintPromptCacheKey(ids *codexFingerprintIDs, promptCacheKey string) bool {
	if ids == nil || !ids.originalBodySessionIDCaptured || ids.originalBodySessionID == "" || ids.sessionID == "" {
		return false
	}
	if ids.mode != codexFingerprintSession && ids.mode != codexFingerprintFull {
		return false
	}
	return promptCacheKey == ids.originalBodySessionID
}

func applyCodexFingerprintPromptCacheKey(reqBody map[string]any, ids *codexFingerprintIDs) bool {
	if reqBody == nil {
		return false
	}
	promptCacheKey, ok := reqBody["prompt_cache_key"].(string)
	if !ok || strings.TrimSpace(promptCacheKey) == "" || !shouldRewriteCodexFingerprintPromptCacheKey(ids, promptCacheKey) {
		return false
	}
	if promptCacheKey == ids.sessionID {
		return false
	}
	reqBody["prompt_cache_key"] = ids.sessionID
	return true
}

// applyCodexFingerprintClientMetadataRaw 在原始 JSON 字节上改写 client_metadata，
// 供透传路径使用——透传是热路径，禁止对可能高达数十 MB 的 body 做全量
// Unmarshal（见 forwardOpenAIPassthrough 的轻量提取注释）。实现为：gjson 提取
// client_metadata 小对象单独解码，经共享核心改写后 sjson 一次性拼回，body
// 其余字节原样保留；root prompt_cache_key 仅在可证明是 body session 默认值时
// 做标量改写。语义与 applyCodexFingerprintClientMetadata 逐点一致（含
// "非对象值整体替换为收敛集合"的行为）。
func applyCodexFingerprintClientMetadataRaw(body []byte, ids *codexFingerprintIDs) ([]byte, bool, error) {
	if len(body) == 0 || ids == nil {
		return body, false, nil
	}
	// 非 JSON 对象的 body（数组/标量/畸形）没有 client_metadata 语义，
	// sjson 在这类根上写字段会改写整体结构，直接放行保持原样。
	root := gjson.ParseBytes(body)
	if !root.IsObject() {
		captureCodexFingerprintOriginalBodySessionIDRaw(ids, gjson.Result{})
		return body, false, nil
	}

	existing := map[string]any{}
	if cm := gjson.GetBytes(body, "client_metadata"); cm.IsObject() {
		captureCodexFingerprintOriginalBodySessionIDRaw(ids, gjson.GetBytes(body, "client_metadata.session_id"))
		if err := json.Unmarshal([]byte(cm.Raw), &existing); err != nil {
			return body, false, fmt.Errorf("decode client_metadata for fingerprint: %w", err)
		}
	} else {
		captureCodexFingerprintOriginalBodySessionIDRaw(ids, gjson.Result{})
	}

	next := body
	modified := false
	if applyCodexFingerprintToClientMetadataMap(existing, ids) {
		raw, err := json.Marshal(existing)
		if err != nil {
			return body, false, fmt.Errorf("encode converged client_metadata: %w", err)
		}
		var setErr error
		next, setErr = sjson.SetRawBytes(body, "client_metadata", raw)
		if setErr != nil {
			return body, false, fmt.Errorf("splice converged client_metadata: %w", setErr)
		}
		modified = true
	}
	promptCacheKey := gjson.GetBytes(body, "prompt_cache_key")
	if promptCacheKey.Exists() && promptCacheKey.Type == gjson.String && strings.TrimSpace(promptCacheKey.String()) != "" && shouldRewriteCodexFingerprintPromptCacheKey(ids, promptCacheKey.String()) {
		rewritten, err := sjson.SetBytes(next, "prompt_cache_key", ids.sessionID)
		if err != nil {
			return body, false, fmt.Errorf("splice converged prompt_cache_key: %w", err)
		}
		next = rewritten
		modified = true
	}
	return next, modified, nil
}

// rewriteClientMetadataEmbeddedTurnMetadata 改写 client_metadata 中内嵌的
// x-codex-turn-metadata JSON 字符串里的指定字段。非法/非对象值会重建，
// 避免 flat client_metadata 与 embedded metadata 暴露两套身份。
func rewriteClientMetadataEmbeddedTurnMetadata(clientMetadata map[string]any, fields map[string]any) {
	raw, ok := clientMetadata["x-codex-turn-metadata"].(string)
	if !ok || raw == "" {
		return
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil || metadata == nil {
		metadata = make(map[string]any, len(fields))
	}
	for k, v := range fields {
		metadata[k] = v
	}
	if rebuilt, err := json.Marshal(metadata); err == nil {
		clientMetadata["x-codex-turn-metadata"] = string(rebuilt)
	}
}
