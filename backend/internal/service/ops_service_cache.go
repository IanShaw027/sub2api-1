package service

import (
	"strings"
	"time"
)

type dashboardOverviewCacheEntry struct {
	expiresAt time.Time
	data      *DashboardOverviewData
}

type providerHealthCacheEntry struct {
	expiresAt time.Time
	data      []*ProviderHealthData
}

type latencyHistogramCacheEntry struct {
	expiresAt time.Time
	data      []*LatencyHistogramItem
}

type errorDistributionCacheEntry struct {
	expiresAt time.Time
	data      []*ErrorDistributionItem
}

func (s *OpsQueryService) getDashboardOverviewFromLocalCache(timeRange string) (*DashboardOverviewData, bool) {
	if s == nil {
		return nil, false
	}
	key := strings.TrimSpace(timeRange)
	if key == "" {
		key = "1h"
	}
	now := time.Now()

	s.dashboardOverviewCacheMu.Lock()
	defer s.dashboardOverviewCacheMu.Unlock()
	entry, ok := s.dashboardOverviewCache[key]
	if !ok || entry.data == nil || now.After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func (s *OpsQueryService) setDashboardOverviewLocalCache(timeRange string, data *DashboardOverviewData, ttl time.Duration) {
	if s == nil || data == nil {
		return
	}
	if ttl <= 0 {
		ttl = opsDashboardLocalCacheTTL
	}
	key := strings.TrimSpace(timeRange)
	if key == "" {
		key = "1h"
	}
	s.dashboardOverviewCacheMu.Lock()
	s.dashboardOverviewCache[key] = dashboardOverviewCacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
	s.dashboardOverviewCacheMu.Unlock()
}

func (s *OpsQueryService) getProviderHealthFromLocalCache(timeRange string) ([]*ProviderHealthData, bool) {
	if s == nil {
		return nil, false
	}
	key := strings.TrimSpace(timeRange)
	if key == "" {
		key = "1h"
	}
	now := time.Now()

	s.providerHealthCacheMu.Lock()
	defer s.providerHealthCacheMu.Unlock()
	entry, ok := s.providerHealthCache[key]
	if !ok || entry.data == nil || now.After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func (s *OpsQueryService) setProviderHealthLocalCache(timeRange string, data []*ProviderHealthData, ttl time.Duration) {
	if s == nil || data == nil {
		return
	}
	if ttl <= 0 {
		ttl = opsDashboardLocalCacheTTL
	}
	key := strings.TrimSpace(timeRange)
	if key == "" {
		key = "1h"
	}
	s.providerHealthCacheMu.Lock()
	s.providerHealthCache[key] = providerHealthCacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
	s.providerHealthCacheMu.Unlock()
}

func (s *OpsQueryService) getLatencyHistogramFromLocalCache(timeRange string) ([]*LatencyHistogramItem, bool) {
	if s == nil {
		return nil, false
	}
	key := strings.TrimSpace(timeRange)
	if key == "" {
		key = "1h"
	}
	now := time.Now()

	s.latencyHistogramCacheMu.Lock()
	defer s.latencyHistogramCacheMu.Unlock()
	entry, ok := s.latencyHistogramCache[key]
	if !ok || entry.data == nil || now.After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func (s *OpsQueryService) setLatencyHistogramLocalCache(timeRange string, data []*LatencyHistogramItem, ttl time.Duration) {
	if s == nil || data == nil {
		return
	}
	if ttl <= 0 {
		ttl = opsDashboardLocalCacheTTL
	}
	key := strings.TrimSpace(timeRange)
	if key == "" {
		key = "1h"
	}
	s.latencyHistogramCacheMu.Lock()
	s.latencyHistogramCache[key] = latencyHistogramCacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
	s.latencyHistogramCacheMu.Unlock()
}

func (s *OpsQueryService) getErrorDistributionFromLocalCache(timeRange string) ([]*ErrorDistributionItem, bool) {
	if s == nil {
		return nil, false
	}
	key := strings.TrimSpace(timeRange)
	if key == "" {
		key = "1h"
	}
	now := time.Now()

	s.errorDistributionCacheMu.Lock()
	defer s.errorDistributionCacheMu.Unlock()
	entry, ok := s.errorDistributionCache[key]
	if !ok || entry.data == nil || now.After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func (s *OpsQueryService) setErrorDistributionLocalCache(timeRange string, data []*ErrorDistributionItem, ttl time.Duration) {
	if s == nil || data == nil {
		return
	}
	if ttl <= 0 {
		ttl = opsDashboardLocalCacheTTL
	}
	key := strings.TrimSpace(timeRange)
	if key == "" {
		key = "1h"
	}
	s.errorDistributionCacheMu.Lock()
	s.errorDistributionCache[key] = errorDistributionCacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
	s.errorDistributionCacheMu.Unlock()
}
