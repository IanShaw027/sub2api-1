package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/tlsfingerprintcapturesample"
	"github.com/Wei-Shaw/sub2api/ent/tlsfingerprintcapturetask"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type tlsFingerprintCaptureRepository struct {
	client *ent.Client
}

func NewTLSFingerprintCaptureRepository(client *ent.Client) service.TLSFingerprintCaptureRepository {
	return &tlsFingerprintCaptureRepository{client: client}
}

func (r *tlsFingerprintCaptureRepository) CreateTask(ctx context.Context, task *service.TLSFingerprintCaptureTask) (*service.TLSFingerprintCaptureTask, error) {
	created, err := r.client.TLSFingerprintCaptureTask.Create().
		SetName(task.Name).
		SetStatus(task.Status).
		SetToken(task.Token).
		SetTargets(task.Targets).
		SetCounts(task.Counts).
		SetUaKeywords(task.UAKeywords).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return tlsCaptureTaskToService(created), nil
}

func (r *tlsFingerprintCaptureRepository) ListTasks(ctx context.Context) ([]*service.TLSFingerprintCaptureTask, error) {
	tasks, err := r.client.TLSFingerprintCaptureTask.Query().
		Order(ent.Desc(tlsfingerprintcapturetask.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*service.TLSFingerprintCaptureTask, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, tlsCaptureTaskToService(task))
	}
	return out, nil
}

func (r *tlsFingerprintCaptureRepository) GetTaskByID(ctx context.Context, id int64) (*service.TLSFingerprintCaptureTask, error) {
	task, err := r.client.TLSFingerprintCaptureTask.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return tlsCaptureTaskToService(task), nil
}

func (r *tlsFingerprintCaptureRepository) GetRunningTaskByToken(ctx context.Context, token string) (*service.TLSFingerprintCaptureTask, error) {
	task, err := r.client.TLSFingerprintCaptureTask.Query().
		Where(
			tlsfingerprintcapturetask.Token(token),
			tlsfingerprintcapturetask.Status(service.TLSFingerprintCaptureStatusRunning),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return tlsCaptureTaskToService(task), nil
}

func (r *tlsFingerprintCaptureRepository) UpdateTask(ctx context.Context, task *service.TLSFingerprintCaptureTask) (*service.TLSFingerprintCaptureTask, error) {
	builder := r.client.TLSFingerprintCaptureTask.UpdateOneID(task.ID).
		SetName(task.Name).
		SetStatus(task.Status).
		SetToken(task.Token).
		SetTargets(task.Targets).
		SetCounts(task.Counts).
		SetUaKeywords(task.UAKeywords)
	if task.CompletedAt != nil {
		builder.SetCompletedAt(*task.CompletedAt)
	} else {
		builder.ClearCompletedAt()
	}
	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return tlsCaptureTaskToService(updated), nil
}

func (r *tlsFingerprintCaptureRepository) DeleteTask(ctx context.Context, id int64) error {
	return r.client.TLSFingerprintCaptureTask.DeleteOneID(id).Exec(ctx)
}

func (r *tlsFingerprintCaptureRepository) DeleteSamplesByTask(ctx context.Context, taskID int64) error {
	_, err := r.client.TLSFingerprintCaptureSample.Delete().
		Where(tlsfingerprintcapturesample.TaskID(taskID)).
		Exec(ctx)
	return err
}

func (r *tlsFingerprintCaptureRepository) CreateSampleIfAbsent(ctx context.Context, sample *service.TLSFingerprintCaptureSample) (*service.TLSFingerprintCaptureSample, bool, error) {
	profile := sample.Profile
	if profile == nil {
		profile = &model.TLSFingerprintProfile{}
	}
	create := r.client.TLSFingerprintCaptureSample.Create().
		SetTaskID(sample.TaskID).
		SetPlatform(sample.Platform).
		SetUserAgent(sample.UserAgent).
		SetOriginator(sample.Originator).
		SetFingerprintHash(sample.FingerprintHash).
		SetProfile(profile).
		SetRawPayload(sample.RawPayload)
	if len(sample.RawClientHello) > 0 {
		create.SetRawClientHello(sample.RawClientHello)
	}
	saved, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			existing, getErr := r.GetSampleByTaskHash(ctx, sample.TaskID, sample.FingerprintHash)
			if getErr != nil {
				return nil, false, getErr
			}
			if existing != nil {
				return existing, false, nil
			}
		}
		return nil, false, err
	}
	return tlsCaptureSampleToService(saved), true, nil
}

func (r *tlsFingerprintCaptureRepository) GetSampleByTaskHash(ctx context.Context, taskID int64, fingerprintHash string) (*service.TLSFingerprintCaptureSample, error) {
	sample, err := r.client.TLSFingerprintCaptureSample.Query().
		Where(
			tlsfingerprintcapturesample.TaskID(taskID),
			tlsfingerprintcapturesample.FingerprintHash(fingerprintHash),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return tlsCaptureSampleToService(sample), nil
}

func (r *tlsFingerprintCaptureRepository) ListSamplesByTask(ctx context.Context, taskID int64) ([]*service.TLSFingerprintCaptureSample, error) {
	samples, err := r.client.TLSFingerprintCaptureSample.Query().
		Where(tlsfingerprintcapturesample.TaskID(taskID)).
		Order(ent.Asc(tlsfingerprintcapturesample.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*service.TLSFingerprintCaptureSample, 0, len(samples))
	for _, sample := range samples {
		out = append(out, tlsCaptureSampleToService(sample))
	}
	return out, nil
}

func tlsCaptureTaskToService(task *ent.TLSFingerprintCaptureTask) *service.TLSFingerprintCaptureTask {
	if task == nil {
		return nil
	}
	out := &service.TLSFingerprintCaptureTask{
		ID:          task.ID,
		Name:        task.Name,
		Status:      task.Status,
		Token:       task.Token,
		Targets:     task.Targets,
		Counts:      task.Counts,
		UAKeywords:  task.UaKeywords,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
		CompletedAt: task.CompletedAt,
	}
	if out.Targets == nil {
		out.Targets = map[string]int{}
	}
	if out.Counts == nil {
		out.Counts = map[string]int{}
	}
	if out.UAKeywords == nil {
		out.UAKeywords = []string{}
	}
	return out
}

func tlsCaptureSampleToService(sample *ent.TLSFingerprintCaptureSample) *service.TLSFingerprintCaptureSample {
	if sample == nil {
		return nil
	}
	profile := sample.Profile
	if profile == nil {
		profile = &model.TLSFingerprintProfile{}
	}
	return &service.TLSFingerprintCaptureSample{
		ID:              sample.ID,
		TaskID:          sample.TaskID,
		Platform:        sample.Platform,
		UserAgent:       sample.UserAgent,
		Originator:      sample.Originator,
		FingerprintHash: sample.FingerprintHash,
		Profile:         profile,
		RawPayload:      sample.RawPayload,
		RawClientHello:  cloneBytesPtr(sample.RawClientHello),
		CreatedAt:       sample.CreatedAt,
	}
}

func cloneBytesPtr(in *[]byte) []byte {
	if in == nil {
		return nil
	}
	return append([]byte(nil), (*in)...)
}
