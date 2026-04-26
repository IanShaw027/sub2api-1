//go:build unit

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type channelMonitorRepoStub struct {
	createFn        func(ctx context.Context, m *ChannelMonitor) error
	getByIDFn       func(ctx context.Context, id int64) (*ChannelMonitor, error)
	updateFn        func(ctx context.Context, m *ChannelMonitor) error
	insertHistoryFn func(ctx context.Context, rows []*ChannelMonitorHistoryRow) error
	markCheckedFn   func(ctx context.Context, id int64, checkedAt time.Time) error
	lockFn          func(ctx context.Context, monitorID int64) (func(), bool, error)
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
func (s *channelMonitorRepoStub) MarkChecked(ctx context.Context, id int64, checkedAt time.Time) error {
	if s.markCheckedFn != nil {
		return s.markCheckedFn(ctx, id, checkedAt)
	}
	return nil
}
func (s *channelMonitorRepoStub) InsertHistoryBatch(ctx context.Context, rows []*ChannelMonitorHistoryRow) error {
	if s.insertHistoryFn != nil {
		return s.insertHistoryFn(ctx, rows)
	}
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

func (s *channelMonitorRepoStub) AcquireChannelMonitorRunLock(ctx context.Context, monitorID int64) (func(), bool, error) {
	if s.lockFn != nil {
		return s.lockFn(ctx, monitorID)
	}
	return nil, true, nil
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

func TestChannelMonitorCreate_RejectsTooManyExtraModels(t *testing.T) {
	created := false
	repo := &channelMonitorRepoStub{createFn: func(context.Context, *ChannelMonitor) error {
		created = true
		return nil
	}}
	svc := NewChannelMonitorService(repo, channelMonitorEncryptorStub{})
	extras := make([]string, monitorExtraModelsMaxCount+1)
	for i := range extras {
		extras[i] = "model"
	}

	_, err := svc.Create(context.Background(), ChannelMonitorCreateParams{
		Name:            "m1",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "https://api.openai.com",
		APIKey:          "sk",
		PrimaryModel:    "gpt-4.1",
		ExtraModels:     extras,
		Enabled:         true,
		IntervalSeconds: 60,
	})
	if !errors.Is(err, ErrChannelMonitorExtraModelsTooMany) {
		t.Fatalf("expected ErrChannelMonitorExtraModelsTooMany, got %v", err)
	}
	if created {
		t.Fatal("repo.Create should not be called for too many extra models")
	}
}

func TestChannelMonitorCreate_RejectsTooLongExtraModel(t *testing.T) {
	repo := &channelMonitorRepoStub{createFn: func(context.Context, *ChannelMonitor) error {
		t.Fatal("repo.Create should not be called for too long extra model")
		return nil
	}}
	svc := NewChannelMonitorService(repo, channelMonitorEncryptorStub{})
	_, err := svc.Create(context.Background(), ChannelMonitorCreateParams{
		Name:            "m1",
		Provider:        MonitorProviderOpenAI,
		Endpoint:        "https://api.openai.com",
		APIKey:          "sk",
		PrimaryModel:    "gpt-4.1",
		ExtraModels:     []string{strings.Repeat("x", monitorExtraModelMaxLength+1)},
		Enabled:         true,
		IntervalSeconds: 60,
	})
	if !errors.Is(err, ErrChannelMonitorExtraModelTooLong) {
		t.Fatalf("expected ErrChannelMonitorExtraModelTooLong, got %v", err)
	}
}

func TestChannelMonitorUpdate_RejectsTooManyExtraModels(t *testing.T) {
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
			}, nil
		},
		updateFn: func(context.Context, *ChannelMonitor) error {
			updated = true
			return nil
		},
	}
	svc := NewChannelMonitorService(repo, channelMonitorEncryptorStub{})
	extras := make([]string, monitorExtraModelsMaxCount+1)
	for i := range extras {
		extras[i] = "model"
	}

	_, err := svc.Update(context.Background(), 1, ChannelMonitorUpdateParams{ExtraModels: &extras})
	if !errors.Is(err, ErrChannelMonitorExtraModelsTooMany) {
		t.Fatalf("expected ErrChannelMonitorExtraModelsTooMany, got %v", err)
	}
	if updated {
		t.Fatal("repo.Update should not be called for too many extra models")
	}
}

func TestChannelMonitorManualRun_UsesSchedulerPolicy(t *testing.T) {
	directRunCalled := false
	schedulerRunCalled := false
	repo := &channelMonitorRepoStub{
		getByIDFn: func(context.Context, int64) (*ChannelMonitor, error) {
			directRunCalled = true
			return nil, ErrChannelMonitorNotFound
		},
	}
	svc := NewChannelMonitorService(repo, channelMonitorEncryptorStub{})
	svc.SetScheduler(manualRunSchedulerStub{runFn: func(ctx context.Context, id int64) ([]*CheckResult, error) {
		schedulerRunCalled = true
		if id != 9 {
			t.Fatalf("expected monitor id=9, got %d", id)
		}
		return []*CheckResult{{Model: "gpt-4.1", Status: MonitorStatusOperational}}, nil
	}})

	results, err := svc.RunManual(context.Background(), 9)
	if err != nil {
		t.Fatalf("RunManual returned error: %v", err)
	}
	if len(results) != 1 || results[0].Model != "gpt-4.1" {
		t.Fatalf("unexpected manual run results: %#v", results)
	}
	if !schedulerRunCalled {
		t.Fatal("scheduler policy was not used")
	}
	if directRunCalled {
		t.Fatal("RunManual should not bypass scheduler policy with direct RunCheck")
	}
}

func TestChannelMonitorRunCheck_RejectsDisabledMonitor(t *testing.T) {
	insertHistoryCalled := false
	markCheckedCalled := false
	repo := &channelMonitorRepoStub{
		getByIDFn: func(context.Context, int64) (*ChannelMonitor, error) {
			return &ChannelMonitor{
				ID:              17,
				Name:            "disabled",
				Provider:        MonitorProviderOpenAI,
				Endpoint:        "https://api.openai.com",
				APIKey:          "enc:sk",
				PrimaryModel:    "gpt-4.1",
				Enabled:         false,
				IntervalSeconds: 60,
			}, nil
		},
		insertHistoryFn: func(context.Context, []*ChannelMonitorHistoryRow) error {
			insertHistoryCalled = true
			return nil
		},
		markCheckedFn: func(context.Context, int64, time.Time) error {
			markCheckedCalled = true
			return nil
		},
	}
	svc := NewChannelMonitorService(repo, channelMonitorEncryptorStub{})

	results, err := svc.RunCheck(context.Background(), 17)
	if !errors.Is(err, ErrChannelMonitorDisabled) {
		t.Fatalf("expected ErrChannelMonitorDisabled, got %v", err)
	}
	if results != nil {
		t.Fatalf("expected no results for disabled monitor, got %#v", results)
	}
	if insertHistoryCalled || markCheckedCalled {
		t.Fatal("disabled monitor should not persist check results")
	}
}

type manualRunSchedulerStub struct {
	runFn func(context.Context, int64) ([]*CheckResult, error)
}

func (s manualRunSchedulerStub) Schedule(*ChannelMonitor) {}
func (s manualRunSchedulerStub) Unschedule(int64)         {}
func (s manualRunSchedulerStub) RunManual(ctx context.Context, id int64) ([]*CheckResult, error) {
	return s.runFn(ctx, id)
}

func ptrInt64CM(v int64) *int64 { return &v }
