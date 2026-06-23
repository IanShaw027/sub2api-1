package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/tlsfingerprintcapturetask"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type tlsFingerprintCaptureTaskRepoV2 struct {
	client *ent.Client
}

func NewTLSFingerprintCaptureTaskRepoV2(client *ent.Client) *tlsFingerprintCaptureTaskRepoV2 {
	return &tlsFingerprintCaptureTaskRepoV2{client: client}
}

func (r *tlsFingerprintCaptureTaskRepoV2) CreateTask(ctx context.Context, task *service.TLSFingerprintCaptureTask) (*service.TLSFingerprintCaptureTask, error) {
	created, err := clientFromContext(ctx, r.client).TLSFingerprintCaptureTask.Create().
		SetName(task.Name).
		SetStatus(task.Status).
		SetToken(task.Token).
		SetTargets(copyStringIntMapOrEmpty(task.Targets)).
		SetCounts(copyStringIntMapOrEmpty(task.Counts)).
		SetTransportTargets(copyStringIntMapOrEmpty(task.TransportTargets)).
		SetTransportCounts(copyStringIntMapOrEmpty(task.TransportCounts)).
		SetCaptureFilters(copyStringAnyMapOrEmpty(task.CaptureFilters)).
		SetSampleSchemaVersion(defaultInt(task.SampleSchemaVersion, 2)).
		SetTaskStats(copyStringAnyMapOrEmpty(task.TaskStats)).
		SetUaKeywords(copyStringSliceOrEmpty(task.UAKeywords)).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return tlsCaptureTaskToService(created), nil
}

func (r *tlsFingerprintCaptureTaskRepoV2) ListTasks(ctx context.Context) ([]*service.TLSFingerprintCaptureTask, error) {
	tasks, err := clientFromContext(ctx, r.client).TLSFingerprintCaptureTask.Query().
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

func (r *tlsFingerprintCaptureTaskRepoV2) GetTaskByID(ctx context.Context, id int64) (*service.TLSFingerprintCaptureTask, error) {
	task, err := clientFromContext(ctx, r.client).TLSFingerprintCaptureTask.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return tlsCaptureTaskToService(task), nil
}

func (r *tlsFingerprintCaptureTaskRepoV2) GetRunningTaskByToken(ctx context.Context, token string) (*service.TLSFingerprintCaptureTask, error) {
	task, err := clientFromContext(ctx, r.client).TLSFingerprintCaptureTask.Query().
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

func (r *tlsFingerprintCaptureTaskRepoV2) UpdateTask(ctx context.Context, task *service.TLSFingerprintCaptureTask) (*service.TLSFingerprintCaptureTask, error) {
	builder := clientFromContext(ctx, r.client).TLSFingerprintCaptureTask.UpdateOneID(task.ID).
		SetName(task.Name).
		SetStatus(task.Status).
		SetToken(task.Token).
		SetTargets(copyStringIntMapOrEmpty(task.Targets)).
		SetCounts(copyStringIntMapOrEmpty(task.Counts)).
		SetTransportTargets(copyStringIntMapOrEmpty(task.TransportTargets)).
		SetTransportCounts(copyStringIntMapOrEmpty(task.TransportCounts)).
		SetCaptureFilters(copyStringAnyMapOrEmpty(task.CaptureFilters)).
		SetSampleSchemaVersion(defaultInt(task.SampleSchemaVersion, 2)).
		SetTaskStats(copyStringAnyMapOrEmpty(task.TaskStats)).
		SetUaKeywords(copyStringSliceOrEmpty(task.UAKeywords))
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

func (r *tlsFingerprintCaptureTaskRepoV2) DeleteTask(ctx context.Context, id int64) error {
	return clientFromContext(ctx, r.client).TLSFingerprintCaptureTask.DeleteOneID(id).Exec(ctx)
}
