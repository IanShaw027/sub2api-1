package service

import "context"

type apiKeyLookupHashCacheInvalidator interface {
	InvalidateAuthCacheByLookupHash(ctx context.Context, lookupHash string)
}

func invalidateAuthCacheByLookupHash(invalidator APIKeyAuthCacheInvalidator, ctx context.Context, lookupHash string) {
	if invalidator == nil || lookupHash == "" {
		return
	}
	if hashInvalidator, ok := invalidator.(apiKeyLookupHashCacheInvalidator); ok {
		hashInvalidator.InvalidateAuthCacheByLookupHash(ctx, lookupHash)
	}
	// A lookup hash must never be passed to InvalidateAuthCacheByKey: that
	// method hashes raw keys and would therefore delete a double-hashed entry.
}

func invalidateAuthCacheForAPIKey(invalidator APIKeyAuthCacheInvalidator, ctx context.Context, key *APIKey) {
	if key == nil {
		return
	}
	if key.LookupHash != "" {
		invalidateAuthCacheByLookupHash(invalidator, ctx, key.LookupHash)
		return
	}
	if key.Key != "" && invalidator != nil {
		invalidator.InvalidateAuthCacheByKey(ctx, key.Key)
	}
}

// InvalidateAuthCacheByKey 清除指定 API Key 的认证缓存
func (s *APIKeyService) InvalidateAuthCacheByKey(ctx context.Context, key string) {
	if key == "" {
		return
	}
	cacheKey := s.authCacheKey(key)
	s.deleteAuthCache(ctx, cacheKey)
}

// InvalidateAuthCacheByLookupHash clears an entry when the caller already has
// the persisted lookup fingerprint and no raw key material.
func (s *APIKeyService) InvalidateAuthCacheByLookupHash(ctx context.Context, lookupHash string) {
	if lookupHash == "" {
		return
	}
	s.deleteAuthCache(ctx, lookupHash)
}

// InvalidateAuthCacheByUserID 清除用户相关的 API Key 认证缓存
func (s *APIKeyService) InvalidateAuthCacheByUserID(ctx context.Context, userID int64) {
	if s == nil || s.apiKeyRepo == nil || userID <= 0 {
		return
	}
	hashes, err := s.apiKeyRepo.ListKeysByUserID(ctx, userID)
	if err != nil {
		return
	}
	s.deleteAuthCacheByLookupHashes(ctx, hashes)
}

// InvalidateAuthCacheByGroupID 清除分组相关的 API Key 认证缓存
func (s *APIKeyService) InvalidateAuthCacheByGroupID(ctx context.Context, groupID int64) {
	if s == nil || s.apiKeyRepo == nil || groupID <= 0 {
		return
	}
	hashes, err := s.apiKeyRepo.ListKeysByGroupID(ctx, groupID)
	if err != nil {
		return
	}
	s.deleteAuthCacheByLookupHashes(ctx, hashes)
}

func (s *APIKeyService) deleteAuthCacheByLookupHashes(ctx context.Context, hashes []string) {
	if len(hashes) == 0 {
		return
	}
	for _, lookupHash := range hashes {
		if lookupHash == "" {
			continue
		}
		s.deleteAuthCache(ctx, lookupHash)
	}
}
