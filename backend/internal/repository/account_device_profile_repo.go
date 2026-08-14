package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/accountdeviceprofile"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type accountDeviceProfileRepository struct {
	client *ent.Client
}

func NewAccountDeviceProfileRepository(client *ent.Client) service.AccountDeviceProfileRepository {
	return &accountDeviceProfileRepository{client: client}
}

func (r *accountDeviceProfileRepository) GetByAccountID(ctx context.Context, accountID int64) (*service.AccountDeviceProfile, error) {
	row, err := r.client.AccountDeviceProfile.Query().
		Where(accountdeviceprofile.AccountID(accountID)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return accountDeviceProfileToService(row), nil
}

func (r *accountDeviceProfileRepository) InsertBaseline(ctx context.Context, p *service.AccountDeviceProfile) (*service.AccountDeviceProfile, error) {
	if p == nil {
		return nil, fmt.Errorf("account device profile is required")
	}
	builder := r.client.AccountDeviceProfile.Create().
		SetAccountID(p.AccountID).
		SetRevision(p.Revision).
		SetSchemaVersion(p.SchemaVersion).
		SetPlatform(p.Platform).
		SetClientFamily(p.ClientFamily).
		SetInstallationID(p.InstallationID).
		SetDeviceID(p.DeviceID).
		SetClientID(p.ClientID).
		SetMachineID(p.MachineID).
		SetGatewayAccountUUID(p.GatewayAccountUUID).
		SetSessionNamespace(p.SessionNamespace).
		SetOsFamily(p.OSFamily).
		SetArch(p.Arch).
		SetRuntime(p.Runtime).
		SetRuntimeVersion(p.RuntimeVersion).
		SetClientVersion(p.ClientVersion).
		SetTransportFamily(p.TransportFamily).
		SetProfilePayload(cloneProfilePayload(p.ProfilePayload)).
		SetLearnedFrom(p.LearnedFrom).
		SetLearningEnabled(p.LearningEnabled)
	if p.TLSProfileID != nil {
		builder.SetTLSProfileID(*p.TLSProfileID)
	}
	if p.VersionUpgradedAt != nil {
		builder.SetVersionUpgradedAt(*p.VersionUpgradedAt)
	}

	row, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return accountDeviceProfileToService(row), nil
}

func (r *accountDeviceProfileRepository) UpdateCAS(ctx context.Context, accountID, expectedRevision int64, next *service.AccountDeviceProfile) (bool, error) {
	if next == nil {
		return false, fmt.Errorf("account device profile is required")
	}
	builder := r.client.AccountDeviceProfile.Update().
		Where(
			accountdeviceprofile.AccountID(accountID),
			accountdeviceprofile.RevisionEQ(expectedRevision),
		).
		SetRevision(expectedRevision + 1).
		SetSchemaVersion(next.SchemaVersion).
		SetPlatform(next.Platform).
		SetClientFamily(next.ClientFamily).
		SetInstallationID(next.InstallationID).
		SetDeviceID(next.DeviceID).
		SetClientID(next.ClientID).
		SetMachineID(next.MachineID).
		SetGatewayAccountUUID(next.GatewayAccountUUID).
		SetOsFamily(next.OSFamily).
		SetArch(next.Arch).
		SetRuntime(next.Runtime).
		SetRuntimeVersion(next.RuntimeVersion).
		SetClientVersion(next.ClientVersion).
		SetTransportFamily(next.TransportFamily).
		SetProfilePayload(cloneProfilePayload(next.ProfilePayload)).
		SetLearnedFrom(next.LearnedFrom).
		SetLearningEnabled(next.LearningEnabled)
	if next.TLSProfileID != nil {
		builder.SetTLSProfileID(*next.TLSProfileID)
	} else {
		builder.ClearTLSProfileID()
	}
	if next.VersionUpgradedAt != nil {
		builder.SetVersionUpgradedAt(*next.VersionUpgradedAt)
	} else {
		builder.ClearVersionUpgradedAt()
	}

	n, err := builder.Save(ctx)
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func accountDeviceProfileToService(row *ent.AccountDeviceProfile) *service.AccountDeviceProfile {
	if row == nil {
		return nil
	}
	return &service.AccountDeviceProfile{
		ID:                 row.ID,
		AccountID:          row.AccountID,
		Revision:           row.Revision,
		SchemaVersion:      row.SchemaVersion,
		Platform:           row.Platform,
		ClientFamily:       row.ClientFamily,
		InstallationID:     row.InstallationID,
		DeviceID:           row.DeviceID,
		ClientID:           row.ClientID,
		MachineID:          row.MachineID,
		GatewayAccountUUID: row.GatewayAccountUUID,
		SessionNamespace:   row.SessionNamespace,
		OSFamily:           row.OsFamily,
		Arch:               row.Arch,
		Runtime:            row.Runtime,
		RuntimeVersion:     row.RuntimeVersion,
		ClientVersion:      row.ClientVersion,
		TLSProfileID:       row.TLSProfileID,
		TransportFamily:    row.TransportFamily,
		ProfilePayload:     cloneProfilePayload(row.ProfilePayload),
		LearnedFrom:        row.LearnedFrom,
		LearningEnabled:    row.LearningEnabled,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
		VersionUpgradedAt:  row.VersionUpgradedAt,
	}
}

func cloneProfilePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(payload))
	for key, value := range payload {
		out[key] = value
	}
	return out
}
