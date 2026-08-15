package service

import (
	"context"
	"fmt"
	"sync"
)

// Outbound device-profile loader for Tasks 9–13.
// Task 16 wires SetOutboundDeviceProfileService. Until then tests call it directly.

var outboundDeviceProfileMu sync.RWMutex
var outboundDeviceProfileSvc *AccountDeviceService

func SetOutboundDeviceProfileService(s *AccountDeviceService) {
	outboundDeviceProfileMu.Lock()
	outboundDeviceProfileSvc = s
	outboundDeviceProfileMu.Unlock()
}

func OutboundDeviceProfileService() *AccountDeviceService {
	outboundDeviceProfileMu.RLock()
	defer outboundDeviceProfileMu.RUnlock()
	return outboundDeviceProfileSvc
}

// LoadOutboundDeviceProfile returns the validated profile for outbound mimic.
// On miss it GetOrCreates. On ValidateOutboundBundle failure it returns the
// error and does not invent a half bundle.
func LoadOutboundDeviceProfile(ctx context.Context, account *Account) (*AccountDeviceProfile, error) {
	svc := OutboundDeviceProfileService()
	if svc == nil {
		return nil, fmt.Errorf("identity_reject: device profile service is not configured")
	}
	p, err := svc.GetOrCreate(ctx, account)
	if err != nil {
		return nil, err
	}
	if err := ValidateOutboundBundle(p); err != nil {
		return nil, err
	}
	if deviceProfilePlatformMismatch(p, account) {
		return nil, deviceProfilePlatformMismatchError(p, account)
	}
	return p, nil
}

func deviceProfilePlatformMismatch(p *AccountDeviceProfile, account *Account) bool {
	if p == nil || account == nil || account.Platform == "" {
		return false
	}
	return p.Platform != account.Platform
}

func deviceProfilePlatformMismatchError(p *AccountDeviceProfile, account *Account) error {
	platform := ""
	accountPlatform := ""
	var accountID int64
	if p != nil {
		platform = p.Platform
	}
	if account != nil {
		accountPlatform = account.Platform
		accountID = account.ID
	}
	return fmt.Errorf("identity_reject: device profile platform %s does not match account %s (account_id=%d)", platform, accountPlatform, accountID)
}
