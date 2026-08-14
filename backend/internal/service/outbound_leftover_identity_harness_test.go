//go:build unit

package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
)

type leftoverSharedDeviceRepo struct {
	mu      sync.RWMutex
	byID    map[int64]*AccountDeviceProfile
	errByID map[int64]error
}

func (r *leftoverSharedDeviceRepo) GetByAccountID(_ context.Context, accountID int64) (*AccountDeviceProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if err, ok := r.errByID[accountID]; ok {
		return nil, err
	}
	if p, ok := r.byID[accountID]; ok {
		return p, nil
	}
	return nil, nil
}

func (r *leftoverSharedDeviceRepo) InsertBaseline(_ context.Context, p *AccountDeviceProfile) (*AccountDeviceProfile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byID == nil {
		r.byID = map[int64]*AccountDeviceProfile{}
	}
	r.byID[p.AccountID] = p
	return p, nil
}

func (r *leftoverSharedDeviceRepo) UpdateCAS(context.Context, int64, int64, *AccountDeviceProfile) (bool, error) {
	return false, nil
}

func (r *leftoverSharedDeviceRepo) put(p *AccountDeviceProfile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byID == nil {
		r.byID = map[int64]*AccountDeviceProfile{}
	}
	r.byID[p.AccountID] = p
}

func (r *leftoverSharedDeviceRepo) putErr(accountID int64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.errByID == nil {
		r.errByID = map[int64]error{}
	}
	r.errByID[accountID] = err
}

func (r *leftoverSharedDeviceRepo) clear(accountID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, accountID)
	delete(r.errByID, accountID)
}

var leftoverSharedRepo = &leftoverSharedDeviceRepo{
	byID:    map[int64]*AccountDeviceProfile{},
	errByID: map[int64]error{},
}

func TestMain(m *testing.M) {
	SetOutboundDeviceProfileService(NewAccountDeviceService(leftoverSharedRepo))
	os.Exit(m.Run())
}

func leftoverValidProfile(accountID int64, platform, family, ua, machineID string) *AccountDeviceProfile {
	payload := map[string]any{}
	if ua != "" {
		payload["user_agent"] = ua
	}
	return &AccountDeviceProfile{
		AccountID:          accountID,
		Revision:           1,
		SchemaVersion:      1,
		Platform:           platform,
		ClientFamily:       family,
		InstallationID:     "11111111-1111-4111-8111-111111111111",
		DeviceID:           "22222222-2222-4222-8222-222222222222",
		MachineID:          machineID,
		GatewayAccountUUID: "55555555-5555-4555-8555-555555555555",
		SessionNamespace:   "0123456789abcdef0123456789abcdef",
		OSFamily:           "windows",
		Arch:               "x64",
		Runtime:            "node",
		RuntimeVersion:     "v20.0.0",
		ClientVersion:      "1.2.3",
		TransportFamily:    TransportH1,
		ProfilePayload:     payload,
		LearnedFrom:        LearnedFromBaseline,
	}
}

type failAfterDeviceRepo struct {
	leftoverSharedDeviceRepo
	failAfter int
	gets      int
}

func (r *failAfterDeviceRepo) GetByAccountID(ctx context.Context, accountID int64) (*AccountDeviceProfile, error) {
	r.mu.Lock()
	r.gets++
	n := r.gets
	r.mu.Unlock()
	if r.failAfter > 0 && n > r.failAfter {
		return nil, fmt.Errorf("identity_reject: second profile load failed")
	}
	return r.leftoverSharedDeviceRepo.GetByAccountID(ctx, accountID)
}

func ensureLeftoverOutboundProfileService(t *testing.T) {
	t.Helper()
	prev := OutboundDeviceProfileService()
	SetOutboundDeviceProfileService(NewAccountDeviceService(leftoverSharedRepo))
	t.Cleanup(func() { SetOutboundDeviceProfileService(prev) })
}

func installLeftoverOutboundProfile(t *testing.T, p *AccountDeviceProfile) {
	t.Helper()
	ensureLeftoverOutboundProfileService(t)
	leftoverSharedRepo.put(p)
	t.Cleanup(func() { leftoverSharedRepo.clear(p.AccountID) })
}

func installLeftoverOutboundProfileError(t *testing.T, accountID int64, err error) {
	t.Helper()
	ensureLeftoverOutboundProfileService(t)
	leftoverSharedRepo.putErr(accountID, err)
	t.Cleanup(func() { leftoverSharedRepo.clear(accountID) })
}
