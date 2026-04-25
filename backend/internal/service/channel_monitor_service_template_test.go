//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type channelMonitorRepoStub struct {
	createFn        func(ctx context.Context, m *ChannelMonitor) error
	getByIDFn       func(ctx context.Context, id int64) (*ChannelMonitor, error)
	updateFn        func(ctx context.Context, m *ChannelMonitor) error
	getTemplateByID func(ctx context.Context, id int64) (*ChannelMonitorRequestTemplate, error)
}

func (s *channelMonitorRepoStub) Create(ctx context.Context, m *ChannelMonitor) error {
	if s.createFn != nil {
		return s.createFn(ctx, m)
	}
	return nil
}
func (s *channelMonitorRepoStub) GetByID(ctx context.Context, id int64) (*ChannelMonitor, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return nil, ErrChannelMonitorNotFound
}
func (s *channelMonitorRepoStub) Update(ctx context.Context, m *ChannelMonitor) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, m)
	}
	return nil
}
func (s *channelMonitorRepoStub) Delete(context.Context, int64) error { return nil }
func (s *channelMonitorRepoStub) List(context.Context, ChannelMonitorListParams) ([]*ChannelMonitor, int64, error) {
	return nil, 0, nil
}
func (s *channelMonitorRepoStub) ListEnabled(context.Context) ([]*ChannelMonitor, error) {
	return nil, nil
}
func (s *channelMonitorRepoStub) MarkChecked(context.Context, int64, time.Time) error { return nil }
func (s *channelMonitorRepoStub) InsertHistoryBatch(context.Context, []*ChannelMonitorHistoryRow) error {
	return nil
}
func (s *channelMonitorRepoStub) DeleteHistoryBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (s *channelMonitorRepoStub) ListHistory(context.Context, int64, string, int) ([]*ChannelMonitorHistoryEntry, error) {
	return nil, nil
}
func (s *channelMonitorRepoStub) ListLatestPerModel(context.Context, int64) ([]*ChannelMonitorLatest, error) {
	return nil, nil
}
func (s *channelMonitorRepoStub) ComputeAvailability(context.Context, int64, int) ([]*ChannelMonitorAvailability, error) {
	return nil, nil
}
func (s *channelMonitorRepoStub) ListLatestForMonitorIDs(context.Context, []int64) (map[int64][]*ChannelMonitorLatest, error) {
	return nil, nil
}
func (s *channelMonitorRepoStub) ComputeAvailabilityForMonitors(context.Context, []int64, int) (map[int64][]*ChannelMonitorAvailability, error) {
	return nil, nil
}
func (s *channelMonitorRepoStub) ListRecentHistoryForMonitors(context.Context, []int64, map[int64]string, int) (map[int64][]*ChannelMonitorHistoryEntry, error) {
	return nil, nil
}
func (s *channelMonitorRepoStub) UpsertDailyRollupsFor(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (s *channelMonitorRepoStub) DeleteRollupsBefore(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (s *channelMonitorRepoStub) LoadAggregationWatermark(context.Context) (*time.Time, error) {
	return nil, nil
}
func (s *channelMonitorRepoStub) UpdateAggregationWatermark(context.Context, time.Time) error {
	return nil
}
func (s *channelMonitorRepoStub) GetTemplateByID(ctx context.Context, id int64) (*ChannelMonitorRequestTemplate, error) {
	if s.getTemplateByID != nil {
		return s.getTemplateByID(ctx, id)
	}
	return nil, ErrChannelMonitorTemplateNotFound
}

type channelMonitorEncryptorStub struct{}

func (channelMonitorEncryptorStub) Encrypt(plaintext string) (string, error) {
	return "enc:" + plaintext, nil
}
func (channelMonitorEncryptorStub) Decrypt(ciphertext string) (string, error) { return ciphertext, nil }

func TestChannelMonitorCreate_TemplateNotFound(t *testing.T) {
	called := false
	repo := &channelMonitorRepoStub{
		createFn: func(context.Context, *ChannelMonitor) error {
			called = true
			return nil
		},
		getTemplateByID: func(context.Context, int64) (*ChannelMonitorRequestTemplate, error) {
			return nil, ErrChannelMonitorTemplateNotFound
		},
	}
	svc := NewChannelMonitorService(repo, channelMonitorEncryptorStub{})
	templateID := int64(999)
	_, err := svc.Create(context.Background(), ChannelMonitorCreateParams{
		Name:            "m1",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "https://api.openai.com",
		APIKey:          "sk",
		PrimaryModel:    "gpt-4.1",
		Enabled:         true,
		IntervalSeconds: 60,
		TemplateID:      &templateID,
	})
	if !errors.Is(err, ErrChannelMonitorTemplateNotFound) {
		t.Fatalf("expected ErrChannelMonitorTemplateNotFound, got %v", err)
	}
	if called {
		t.Fatal("repo.Create should not be called when template does not exist")
	}
}

func TestChannelMonitorCreate_TemplateProviderMismatch(t *testing.T) {
	called := false
	repo := &channelMonitorRepoStub{
		createFn: func(context.Context, *ChannelMonitor) error {
			called = true
			return nil
		},
		getTemplateByID: func(context.Context, int64) (*ChannelMonitorRequestTemplate, error) {
			return &ChannelMonitorRequestTemplate{ID: 1, Provider: MonitorProviderAnthropic}, nil
		},
	}
	svc := NewChannelMonitorService(repo, channelMonitorEncryptorStub{})
	templateID := int64(1)
	_, err := svc.Create(context.Background(), ChannelMonitorCreateParams{
		Name:            "m1",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "https://api.openai.com",
		APIKey:          "sk",
		PrimaryModel:    "gpt-4.1",
		Enabled:         true,
		IntervalSeconds: 60,
		TemplateID:      &templateID,
	})
	if !errors.Is(err, ErrChannelMonitorTemplateProviderMismatch) {
		t.Fatalf("expected ErrChannelMonitorTemplateProviderMismatch, got %v", err)
	}
	if called {
		t.Fatal("repo.Create should not be called on provider mismatch")
	}
}

func TestChannelMonitorUpdate_ProviderChangeViolatesTemplateProvider(t *testing.T) {
	updated := false
	repo := &channelMonitorRepoStub{
		getByIDFn: func(context.Context, int64) (*ChannelMonitor, error) {
			return &ChannelMonitor{
				ID:              1,
				Name:            "m1",
				Provider:        MonitorProviderOpenAI,
				Endpoint:        "https://api.openai.com",
				APIKey:          "enc:sk",
				PrimaryModel:    "gpt-4.1",
				Enabled:         true,
				IntervalSeconds: 60,
				TemplateID:      ptrInt64CM(1),
			}, nil
		},
		updateFn: func(context.Context, *ChannelMonitor) error {
			updated = true
			return nil
		},
		getTemplateByID: func(context.Context, int64) (*ChannelMonitorRequestTemplate, error) {
			return &ChannelMonitorRequestTemplate{ID: 1, Provider: MonitorProviderOpenAI}, nil
		},
	}
	svc := NewChannelMonitorService(repo, channelMonitorEncryptorStub{})
	newProvider := MonitorProviderAnthropic
	_, err := svc.Update(context.Background(), 1, ChannelMonitorUpdateParams{
		Provider: &newProvider,
	})
	if !errors.Is(err, ErrChannelMonitorTemplateProviderMismatch) {
		t.Fatalf("expected ErrChannelMonitorTemplateProviderMismatch, got %v", err)
	}
	if updated {
		t.Fatal("repo.Update should not be called on provider mismatch")
	}
}

func ptrInt64CM(v int64) *int64 { return &v }
