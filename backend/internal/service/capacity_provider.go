package service

import (
	"context"
	"sync"
	"time"
)

// QuotaWindow is a single rolling rate-limit window snapshot for an account,
// as understood by the generic capacity forecasting engine. Platforms expose
// whatever windows they track (e.g. OpenAI Codex: "5h"/"7d") through
// CapacityProvider.ExtractWindows.
type QuotaWindow struct {
	// Kind identifies the window, e.g. "5h", "7d", "30d", "day". It is
	// platform-defined and only needs to be stable within a platform.
	Kind string
	// UsedRatio is the fraction of the window consumed so far, in [0, 1]
	// (values slightly above 1 are possible from upstream rounding and are
	// not clamped here).
	UsedRatio float64
	// ResetAt is when this window's usage resets (the end of the current
	// rolling period).
	ResetAt time.Time
	// Duration is the window's rolling length (e.g. 5h, 7*24h).
	Duration time.Duration
}

// CapacityProvider adapts a single upstream platform's quota representation
// (however it stores it, typically inside Account.Extra) to the generic
// QuotaWindow shape the forecasting engine operates on, and knows how to
// refresh that data from upstream on demand ("probe" / 测活).
type CapacityProvider interface {
	// Platform returns the domain.Platform* constant this provider serves.
	Platform() string
	// ExtractWindows parses the account's currently known quota windows from
	// its cached state (typically Account.Extra). It returns nil if the
	// account has no quota snapshot yet (never probed, or upstream doesn't
	// expose quota for this account type).
	ExtractWindows(acc *Account, now time.Time) []QuotaWindow
	// Probe actively refreshes the account's quota snapshot from upstream and
	// persists it back (both onto acc.Extra / the account repository, and
	// into the account_quota_snapshots / account_quota_periods tables).
	Probe(ctx context.Context, acc *Account) error
}

// CapacityProviderRegistry maps platform -> CapacityProvider so the generic
// forecasting engine and the probe endpoint can stay platform-agnostic.
type CapacityProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]CapacityProvider
}

// NewCapacityProviderRegistry creates an empty registry. Providers register
// themselves via Register (typically from a Wire provider function).
func NewCapacityProviderRegistry() *CapacityProviderRegistry {
	return &CapacityProviderRegistry{providers: make(map[string]CapacityProvider)}
}

// Register adds (or replaces) the provider for its own Platform().
func (r *CapacityProviderRegistry) Register(p CapacityProvider) {
	if r == nil || p == nil {
		return
	}
	platform := p.Platform()
	if platform == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.providers == nil {
		r.providers = make(map[string]CapacityProvider)
	}
	r.providers[platform] = p
}

// Get returns the provider registered for platform, if any.
func (r *CapacityProviderRegistry) Get(platform string) (CapacityProvider, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[platform]
	return p, ok
}

// Platforms returns the list of platforms with a registered provider.
func (r *CapacityProviderRegistry) Platforms() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	platforms := make([]string, 0, len(r.providers))
	for platform := range r.providers {
		platforms = append(platforms, platform)
	}
	return platforms
}
