package service

import (
	"context"
	"fmt"
)

// Get returns the stored device profile for accountID without minting a baseline.
// A missing row is (nil, nil).
func (s *AccountDeviceService) Get(ctx context.Context, accountID int64) (*AccountDeviceProfile, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("identity_reject: account device service is not configured")
	}
	return s.repo.GetByAccountID(ctx, accountID)
}
