package service

import (
	"context"
	"errors"
	"time"
)

// ErrRefreshTokenNotFound is returned when a refresh token is not found in cache.
// This is used to abstract away the underlying cache implementation (e.g., redis.Nil).
var ErrRefreshTokenNotFound = errors.New("refresh token not found")

// RefreshTokenConcurrentReuseGrace is the short period in which a duplicate
// request may be a legitimate concurrent loser rather than a confirmed replay.
const RefreshTokenConcurrentReuseGrace = 5 * time.Second

// RefreshTokenData 存储在Redis中的Refresh Token数据
type RefreshTokenData struct {
	UserID       int64     `json:"user_id"`
	TokenVersion int64     `json:"token_version"` // 用于检测密码更改后的Token失效
	FamilyID     string    `json:"family_id"`     // Token家族ID，用于防重放攻击
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// UsedRefreshTokenMarker records when a refresh token was atomically consumed.
// The timestamp lets the service distinguish an ambiguous concurrent retry from
// a replay that occurs after the normal request race window.
type UsedRefreshTokenMarker struct {
	FamilyID   string    `json:"family_id"`
	ConsumedAt time.Time `json:"consumed_at"`
}

// RefreshTokenReuseMarkerCache is an optional extension implemented by caches
// that persist detailed rotation markers. Keeping it separate preserves
// compatibility with lightweight test and alternate cache implementations.
type RefreshTokenReuseMarkerCache interface {
	GetUsedRefreshTokenMarker(ctx context.Context, tokenHash string) (*UsedRefreshTokenMarker, error)
	IsRefreshTokenReuseGraceActive(ctx context.Context, tokenHash string) (bool, error)
}

// RefreshTokenCache 管理Refresh Token的Redis缓存
// 用于JWT Token刷新机制，支持Token轮转和防重放攻击
//
// Key 格式:
//   - refresh_token:{token_hash}     -> RefreshTokenData (JSON)
//   - refresh_token_used:{token_hash} -> family_id
//   - user_refresh_tokens:{user_id}  -> Set<token_hash>
//   - token_family:{family_id}       -> Set<token_hash>
type RefreshTokenCache interface {
	// StoreRefreshToken 存储Refresh Token
	// tokenHash: Token的SHA256哈希值（不存储原始Token）
	// data: Token关联的数据
	// ttl: Token过期时间
	StoreRefreshToken(ctx context.Context, tokenHash string, data *RefreshTokenData, ttl time.Duration) error

	// GetRefreshToken 获取Refresh Token数据
	// 返回 (data, nil) 如果Token存在
	// 返回 (nil, ErrRefreshTokenNotFound) 如果Token不存在
	// 返回 (nil, err) 如果发生其他错误
	GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshTokenData, error)

	// ConsumeRefreshToken atomically reads and deletes an active token and writes
	// its used marker. Exactly one concurrent caller can consume a token.
	// Returns ErrRefreshTokenNotFound if another caller already consumed it.
	ConsumeRefreshToken(ctx context.Context, tokenHash, familyID string, usedTTL time.Duration) (*RefreshTokenData, error)

	// DeleteRefreshToken 删除单个Refresh Token
	// 用于Token轮转时使旧Token失效
	DeleteRefreshToken(ctx context.Context, tokenHash string) error

	// GetUsedRefreshTokenFamily returns the family ID if this hash was previously rotated.
	// Returns ("", ErrRefreshTokenNotFound) when no used marker exists.
	GetUsedRefreshTokenFamily(ctx context.Context, tokenHash string) (familyID string, err error)

	// DeleteUserRefreshTokens 删除用户的所有Refresh Token
	// 用于密码更改或用户主动登出所有设备
	DeleteUserRefreshTokens(ctx context.Context, userID int64) error

	// DeleteTokenFamily 删除整个Token家族
	// 用于检测到Token重放攻击时，撤销整个会话链
	DeleteTokenFamily(ctx context.Context, familyID string) error

	// AddToUserTokenSet 将Token添加到用户的Token集合
	// 用于跟踪用户的所有活跃Refresh Token
	AddToUserTokenSet(ctx context.Context, userID int64, tokenHash string, ttl time.Duration) error

	// AddToFamilyTokenSet 将Token添加到家族Token集合
	// 用于跟踪同一登录会话的所有Token
	AddToFamilyTokenSet(ctx context.Context, familyID string, tokenHash string, ttl time.Duration) error

	// GetUserTokenHashes 获取用户的所有Token哈希
	// 用于批量删除用户Token
	GetUserTokenHashes(ctx context.Context, userID int64) ([]string, error)

	// GetFamilyTokenHashes 获取家族的所有Token哈希
	// 用于批量删除家族Token
	GetFamilyTokenHashes(ctx context.Context, familyID string) ([]string, error)

	// IsTokenInFamily 检查Token是否属于指定家族
	// 用于验证Token家族关系
	IsTokenInFamily(ctx context.Context, familyID string, tokenHash string) (bool, error)
}
