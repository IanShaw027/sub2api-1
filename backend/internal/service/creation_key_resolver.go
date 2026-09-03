package service

import (
	"context"
	"fmt"
	"sync"
)

// CreationKeyResolver finds or creates hidden internal API keys for the
// creation center. Gateway context injection is handled by middleware.
type CreationKeyResolver struct {
	apiKeyRepo    APIKeyRepository
	apiKeyService *APIKeyService
	ensureMu      sync.Map // keyed by "userID:groupID"
}

func NewCreationKeyResolver(
	apiKeyRepo APIKeyRepository,
	apiKeyService *APIKeyService,
) *CreationKeyResolver {
	return &CreationKeyResolver{
		apiKeyRepo:    apiKeyRepo,
		apiKeyService: apiKeyService,
	}
}

func ProvideCreationKeyResolver(
	apiKeyRepo APIKeyRepository,
	apiKeyService *APIKeyService,
) *CreationKeyResolver {
	return NewCreationKeyResolver(apiKeyRepo, apiKeyService)
}

// ResolveAuthKey returns the internal creation API key with auth edges loaded.
func (r *CreationKeyResolver) ResolveAuthKey(ctx context.Context, userID, groupID int64) (*APIKey, error) {
	if r == nil || r.apiKeyRepo == nil || r.apiKeyService == nil {
		return nil, fmt.Errorf("creation key resolver is not configured")
	}
	if userID <= 0 || groupID <= 0 {
		return nil, ErrCreationGroupRequired
	}

	lockKey := fmt.Sprintf("%d:%d", userID, groupID)
	muIface, _ := r.ensureMu.LoadOrStore(lockKey, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)
	mu.Lock()
	defer func() {
		mu.Unlock()
		r.ensureMu.Delete(lockKey)
	}()

	apiKey, err := r.apiKeyRepo.GetByUserGroupAndPurpose(ctx, userID, groupID, APIKeyPurposeCreation)
	if err != nil {
		return nil, err
	}
	if apiKey == nil {
		apiKey, err = r.createInternalKey(ctx, userID, groupID)
		if err != nil {
			return nil, err
		}
	}

	fullKey, err := r.apiKeyRepo.GetByKeyForAuth(ctx, apiKey.Key)
	if err != nil {
		return nil, err
	}
	if fullKey.Status == StatusAPIKeyDisabled ||
		fullKey.Status == StatusAPIKeyExpired ||
		fullKey.Status == StatusAPIKeyQuotaExhausted {
		fullKey.Status = StatusAPIKeyActive
		if err := r.apiKeyRepo.Update(ctx, fullKey, APIKeyUpdateFields{Status: true}); err != nil {
			return nil, err
		}
	}
	r.apiKeyService.compileAPIKeyIPRules(fullKey)
	return fullKey, nil
}

func (r *CreationKeyResolver) createInternalKey(ctx context.Context, userID, groupID int64) (*APIKey, error) {
	key, err := r.apiKeyService.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("generate creation key: %w", err)
	}
	row := &APIKey{
		UserID:  userID,
		Key:     key,
		Name:    creationInternalKeyName,
		GroupID: &groupID,
		Status:  StatusAPIKeyActive,
		Purpose: APIKeyPurposeCreation,
	}
	if err := r.apiKeyRepo.Create(ctx, row); err != nil {
		existing, lookupErr := r.apiKeyRepo.GetByUserGroupAndPurpose(ctx, userID, groupID, APIKeyPurposeCreation)
		if lookupErr == nil && existing != nil {
			return existing, nil
		}
		return nil, err
	}
	return row, nil
}
