package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/gin-gonic/gin"
)

type openAILegacySessionHashContextKey struct{}

var openAILegacySessionHashKey = openAILegacySessionHashContextKey{}

var (
	openAIStickyLegacyReadFallbackTotal atomic.Int64
	openAIStickyLegacyReadFallbackHit   atomic.Int64
	openAIStickyLegacyDualWriteTotal    atomic.Int64
)

const openAIStickySessionTTLNotCheckedMS int64 = -3

func openAIStickyCompatStats() (legacyReadFallbackTotal, legacyReadFallbackHit, legacyDualWriteTotal int64) {
	return openAIStickyLegacyReadFallbackTotal.Load(),
		openAIStickyLegacyReadFallbackHit.Load(),
		openAIStickyLegacyDualWriteTotal.Load()
}

// DeriveSessionHashFromSeed computes the current-format sticky-session hash
// from an arbitrary seed string.
func DeriveSessionHashFromSeed(seed string) string {
	currentHash, _ := deriveOpenAISessionHashes(seed)
	return currentHash
}

func deriveOpenAISessionHashes(sessionID string) (currentHash string, legacyHash string) {
	normalized := strings.TrimSpace(sessionID)
	if normalized == "" {
		return "", ""
	}

	currentHash = fmt.Sprintf("%016x", xxhash.Sum64String(normalized))
	sum := sha256.Sum256([]byte(normalized))
	legacyHash = hex.EncodeToString(sum[:])
	return currentHash, legacyHash
}

func withOpenAILegacySessionHash(ctx context.Context, legacyHash string) context.Context {
	if ctx == nil {
		return nil
	}
	trimmed := strings.TrimSpace(legacyHash)
	if trimmed == "" {
		return ctx
	}
	return context.WithValue(ctx, openAILegacySessionHashKey, trimmed)
}

func openAILegacySessionHashFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(openAILegacySessionHashKey).(string)
	return strings.TrimSpace(value)
}

func attachOpenAILegacySessionHashToGin(c *gin.Context, legacyHash string) {
	if c == nil || c.Request == nil {
		return
	}
	c.Request = c.Request.WithContext(withOpenAILegacySessionHash(c.Request.Context(), legacyHash))
}

func (s *OpenAIGatewayService) openAISessionHashReadOldFallbackEnabled() bool {
	if s == nil || s.cfg == nil {
		return true
	}
	return s.cfg.Gateway.OpenAIWS.SessionHashReadOldFallback
}

func (s *OpenAIGatewayService) openAISessionHashDualWriteOldEnabled() bool {
	if s == nil || s.cfg == nil {
		return true
	}
	return s.cfg.Gateway.OpenAIWS.SessionHashDualWriteOld
}

func (s *OpenAIGatewayService) openAISessionCacheKey(sessionHash string) string {
	normalized := strings.TrimSpace(sessionHash)
	if normalized == "" {
		return ""
	}
	return "openai:" + normalized
}

func (s *OpenAIGatewayService) openAILegacySessionCacheKey(ctx context.Context, sessionHash string) string {
	legacyHash := openAILegacySessionHashFromContext(ctx)
	if legacyHash == "" {
		return ""
	}
	legacyKey := "openai:" + legacyHash
	if legacyKey == s.openAISessionCacheKey(sessionHash) {
		return ""
	}
	return legacyKey
}

func (s *OpenAIGatewayService) openAIStickyLegacyTTL(ttl time.Duration) time.Duration {
	legacyTTL := ttl
	if legacyTTL <= 0 {
		legacyTTL = openaiStickySessionTTL
	}
	if legacyTTL > 10*time.Minute {
		return 10 * time.Minute
	}
	return legacyTTL
}

type openAIStickySessionLookupResult struct {
	AccountID               int64
	Source                  string
	PrimaryKey              string
	PrimaryTTLMS            int64
	PrimaryTTLError         string
	LegacyKey               string
	LegacyTTLMS             int64
	LegacyTTLError          string
	PrimaryHit              bool
	PrimaryError            string
	LegacyFallbackEnabled   bool
	LegacyFallbackAttempted bool
	LegacyFallbackHit       bool
	LegacyError             string
	Err                     error
}

type openAIStickySessionTTLReader interface {
	GetSessionAccountTTL(ctx context.Context, groupID int64, sessionHash string) (time.Duration, error)
}

func newOpenAIStickySessionLookupResult(source string) openAIStickySessionLookupResult {
	return openAIStickySessionLookupResult{
		Source:       source,
		PrimaryTTLMS: openAIStickySessionTTLNotCheckedMS,
		LegacyTTLMS:  openAIStickySessionTTLNotCheckedMS,
	}
}

func openAIStickySessionLookupErrorValue(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrGatewayCacheMiss) {
		return "redis_nil"
	}
	return compactOpenAIWSLogValue(err.Error(), openAIWSLogValueMaxLen)
}

func openAIStickySessionMissSource(err error) string {
	if err == nil || errors.Is(err, ErrGatewayCacheMiss) {
		return "redis_miss"
	}
	return "redis_error"
}

func (s *OpenAIGatewayService) getStickySessionAccountIDWithSource(ctx context.Context, groupID *int64, sessionHash string) openAIStickySessionLookupResult {
	result := newOpenAIStickySessionLookupResult("no_cache")
	if s == nil || s.cache == nil {
		return result
	}

	primaryKey := s.openAISessionCacheKey(sessionHash)
	result.PrimaryKey = primaryKey
	if primaryKey == "" {
		result.Source = "no_session_hash"
		return result
	}

	accountID, err := s.cache.GetSessionAccountID(ctx, derefGroupID(groupID), primaryKey)
	if err == nil && accountID > 0 {
		result.AccountID = accountID
		result.Source = "redis_current"
		result.PrimaryHit = true
		return result
	}
	result.AccountID = accountID
	result.Err = err
	result.PrimaryError = openAIStickySessionLookupErrorValue(err)
	result.Source = openAIStickySessionMissSource(err)
	if !s.openAISessionHashReadOldFallbackEnabled() {
		return result
	}
	result.LegacyFallbackEnabled = true

	legacyKey := s.openAILegacySessionCacheKey(ctx, sessionHash)
	result.LegacyKey = legacyKey
	if legacyKey == "" {
		return result
	}
	result.LegacyFallbackAttempted = true

	openAIStickyLegacyReadFallbackTotal.Add(1)
	legacyAccountID, legacyErr := s.cache.GetSessionAccountID(ctx, derefGroupID(groupID), legacyKey)
	if legacyErr == nil && legacyAccountID > 0 {
		openAIStickyLegacyReadFallbackHit.Add(1)
		result.AccountID = legacyAccountID
		result.Source = "redis_legacy_fallback"
		result.LegacyFallbackHit = true
		result.Err = nil
		return result
	}
	result.LegacyError = openAIStickySessionLookupErrorValue(legacyErr)
	return result
}

func (s *OpenAIGatewayService) populateOpenAIStickySessionLookupTTLs(ctx context.Context, groupID *int64, lookup *openAIStickySessionLookupResult) {
	if s == nil || lookup == nil {
		return
	}
	if lookup.PrimaryTTLMS == 0 {
		lookup.PrimaryTTLMS = openAIStickySessionTTLNotCheckedMS
	}
	if lookup.LegacyTTLMS == 0 {
		lookup.LegacyTTLMS = openAIStickySessionTTLNotCheckedMS
	}
	reader, ok := s.cache.(openAIStickySessionTTLReader)
	if !ok || reader == nil {
		return
	}
	readTTL := func(sessionKey string) (int64, string) {
		key := strings.TrimSpace(sessionKey)
		if key == "" {
			return openAIStickySessionTTLNotCheckedMS, ""
		}
		ttl, err := reader.GetSessionAccountTTL(ctx, derefGroupID(groupID), key)
		if err != nil {
			return openAIStickySessionTTLNotCheckedMS, openAIStickySessionLookupErrorValue(err)
		}
		return ttl.Milliseconds(), ""
	}
	if lookup.PrimaryKey != "" {
		lookup.PrimaryTTLMS, lookup.PrimaryTTLError = readTTL(lookup.PrimaryKey)
	}
	if lookup.LegacyKey != "" {
		lookup.LegacyTTLMS, lookup.LegacyTTLError = readTTL(lookup.LegacyKey)
	}
}

func (s *OpenAIGatewayService) getStickySessionAccountID(ctx context.Context, groupID *int64, sessionHash string) (int64, error) {
	lookup := s.getStickySessionAccountIDWithSource(ctx, groupID, sessionHash)
	return lookup.AccountID, lookup.Err
}

func (s *OpenAIGatewayService) setStickySessionAccountID(ctx context.Context, groupID *int64, sessionHash string, accountID int64, ttl time.Duration) error {
	if s == nil || s.cache == nil || accountID <= 0 {
		return nil
	}
	primaryKey := s.openAISessionCacheKey(sessionHash)
	if primaryKey == "" {
		return nil
	}

	if err := s.cache.SetSessionAccountID(ctx, derefGroupID(groupID), primaryKey, accountID, ttl); err != nil {
		return err
	}

	if !s.openAISessionHashDualWriteOldEnabled() {
		return nil
	}
	legacyKey := s.openAILegacySessionCacheKey(ctx, sessionHash)
	if legacyKey == "" {
		return nil
	}
	if err := s.cache.SetSessionAccountID(ctx, derefGroupID(groupID), legacyKey, accountID, s.openAIStickyLegacyTTL(ttl)); err != nil {
		return err
	}
	openAIStickyLegacyDualWriteTotal.Add(1)
	return nil
}

func (s *OpenAIGatewayService) refreshStickySessionTTL(ctx context.Context, groupID *int64, sessionHash string, ttl time.Duration) error {
	if s == nil || s.cache == nil {
		return nil
	}
	primaryKey := s.openAISessionCacheKey(sessionHash)
	if primaryKey == "" {
		return nil
	}

	err := s.cache.RefreshSessionTTL(ctx, derefGroupID(groupID), primaryKey, ttl)
	if !s.openAISessionHashReadOldFallbackEnabled() && !s.openAISessionHashDualWriteOldEnabled() {
		return err
	}

	legacyKey := s.openAILegacySessionCacheKey(ctx, sessionHash)
	if legacyKey != "" {
		_ = s.cache.RefreshSessionTTL(ctx, derefGroupID(groupID), legacyKey, s.openAIStickyLegacyTTL(ttl))
	}
	return err
}

func (s *OpenAIGatewayService) deleteStickySessionAccountID(ctx context.Context, groupID *int64, sessionHash string) error {
	if s == nil || s.cache == nil {
		return nil
	}
	primaryKey := s.openAISessionCacheKey(sessionHash)
	if primaryKey == "" {
		return nil
	}

	err := s.cache.DeleteSessionAccountID(ctx, derefGroupID(groupID), primaryKey)
	if !s.openAISessionHashReadOldFallbackEnabled() && !s.openAISessionHashDualWriteOldEnabled() {
		return err
	}

	legacyKey := s.openAILegacySessionCacheKey(ctx, sessionHash)
	if legacyKey != "" {
		_ = s.cache.DeleteSessionAccountID(ctx, derefGroupID(groupID), legacyKey)
	}
	return err
}
