package service

import (
	"context"
	"time"
)

// GroupCapacitySummary holds aggregated capacity for a single group.
type GroupCapacitySummary struct {
	GroupID         int64 `json:"group_id"`
	ConcurrencyUsed int   `json:"concurrency_used"`
	ConcurrencyMax  int   `json:"concurrency_max"`
	SessionsUsed    int   `json:"sessions_used"`
	SessionsMax     int   `json:"sessions_max"`
	RPMUsed         int   `json:"rpm_used"`
	RPMMax          int   `json:"rpm_max"`
}

// GroupCapacityService aggregates per-group capacity from runtime data.
type GroupCapacityService struct {
	accountRepo        AccountRepository
	groupRepo          GroupRepository
	concurrencyService *ConcurrencyService
	sessionLimitCache  SessionLimitCache
	rpmCache           RPMCache
}

// NewGroupCapacityService creates a new GroupCapacityService.
func NewGroupCapacityService(
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	concurrencyService *ConcurrencyService,
	sessionLimitCache SessionLimitCache,
	rpmCache RPMCache,
) *GroupCapacityService {
	return &GroupCapacityService{
		accountRepo:        accountRepo,
		groupRepo:          groupRepo,
		concurrencyService: concurrencyService,
		sessionLimitCache:  sessionLimitCache,
		rpmCache:           rpmCache,
	}
}

// GetAllGroupCapacity returns capacity summary for all active groups.
func (s *GroupCapacityService) GetAllGroupCapacity(ctx context.Context) ([]GroupCapacitySummary, error) {
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]GroupCapacitySummary, 0, len(groups))
	for i := range groups {
		cap, err := s.getGroupCapacity(ctx, groups[i].ID)
		if err != nil {
			// Skip groups with errors, return partial results
			continue
		}
		cap.GroupID = groups[i].ID
		results = append(results, cap)
	}
	return results, nil
}

func (s *GroupCapacityService) getGroupCapacity(ctx context.Context, groupID int64) (GroupCapacitySummary, error) {
	var concurrencyUsed int
	if s.concurrencyService != nil {
		concurrencyUsed, _ = s.concurrencyService.GetGroupConcurrency(ctx, groupID)
	}

	accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, groupID)
	if err != nil {
		return GroupCapacitySummary{}, err
	}
	if len(accounts) == 0 {
		return GroupCapacitySummary{ConcurrencyUsed: concurrencyUsed}, nil
	}

	// Collect account IDs and config values
	accountIDs := make([]int64, 0, len(accounts))
	sessionTimeouts := make(map[int64]time.Duration)
	var sessionsMax, rpmMax int

	for i := range accounts {
		acc := &accounts[i]
		accountIDs = append(accountIDs, acc.ID)

		if ms := acc.GetMaxSessions(); ms > 0 {
			sessionsMax += ms
			timeout := time.Duration(acc.GetSessionIdleTimeoutMinutes()) * time.Minute
			if timeout <= 0 {
				timeout = 5 * time.Minute
			}
			sessionTimeouts[acc.ID] = timeout
		}

		if rpm := acc.GetBaseRPM(); rpm > 0 {
			rpmMax += rpm
		}
	}

	var sessionsMap map[int64]int
	if sessionsMax > 0 && s.sessionLimitCache != nil {
		sessionsMap, _ = s.sessionLimitCache.GetActiveSessionCountBatch(ctx, accountIDs, sessionTimeouts)
	}

	var rpmMap map[int64]int
	if rpmMax > 0 && s.rpmCache != nil {
		rpmMap, _ = s.rpmCache.GetRPMBatch(ctx, accountIDs)
	}

	// Aggregate
	var sessionsUsed, rpmUsed int
	for _, id := range accountIDs {
		if sessionsMap != nil {
			sessionsUsed += sessionsMap[id]
		}
		if rpmMap != nil {
			rpmUsed += rpmMap[id]
		}
	}

	return GroupCapacitySummary{
		ConcurrencyUsed: concurrencyUsed,
		ConcurrencyMax:  0,
		SessionsUsed:    sessionsUsed,
		SessionsMax:     sessionsMax,
		RPMUsed:         rpmUsed,
		RPMMax:          rpmMax,
	}, nil
}
