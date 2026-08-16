package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// AccountDeviceProfileInspect is the admin inspect DTO for a stored device profile.
// It omits identity secrets and the raw payload.
type AccountDeviceProfileInspect struct {
	AccountID       int64     `json:"account_id"`
	Platform        string    `json:"platform"`
	ClientFamily    string    `json:"client_family"`
	ClientVersion   string    `json:"client_version"`
	UserAgent       string    `json:"user_agent"`
	OSFamily        string    `json:"os_family"`
	Arch            string    `json:"arch"`
	Revision        int64     `json:"revision"`
	LearnedFrom     string    `json:"learned_from"`
	LearningEnabled bool      `json:"learning_enabled"`
	TransportFamily string    `json:"transport_family"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func AccountDeviceProfileInspectFromService(p *service.AccountDeviceProfile) *AccountDeviceProfileInspect {
	if p == nil {
		return nil
	}
	return &AccountDeviceProfileInspect{
		AccountID:       p.AccountID,
		Platform:        p.Platform,
		ClientFamily:    p.ClientFamily,
		ClientVersion:   p.ClientVersion,
		UserAgent:       deviceProfileUserAgent(p),
		OSFamily:        p.OSFamily,
		Arch:            p.Arch,
		Revision:        p.Revision,
		LearnedFrom:     p.LearnedFrom,
		LearningEnabled: p.LearningEnabled,
		TransportFamily: p.TransportFamily,
		UpdatedAt:       p.UpdatedAt,
	}
}

func deviceProfileUserAgent(p *service.AccountDeviceProfile) string {
	if p == nil || p.ProfilePayload == nil {
		return ""
	}
	ua, _ := p.ProfilePayload["user_agent"].(string)
	return ua
}
