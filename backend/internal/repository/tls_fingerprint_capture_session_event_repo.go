package repository

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/tlsfingerprintcapturesessionevent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type tlsFingerprintCaptureSessionEventRepo struct {
	client *ent.Client
}

func NewTLSFingerprintCaptureSessionEventRepo(client *ent.Client) *tlsFingerprintCaptureSessionEventRepo {
	return &tlsFingerprintCaptureSessionEventRepo{client: client}
}

func (r *tlsFingerprintCaptureSessionEventRepo) DeleteSessionEventsByTask(ctx context.Context, taskID int64) error {
	_, err := clientFromContext(ctx, r.client).TLSFingerprintCaptureSessionEvent.Delete().
		Where(tlsfingerprintcapturesessionevent.TaskID(taskID)).
		Exec(ctx)
	return err
}

func (r *tlsFingerprintCaptureSessionEventRepo) CreateSessionEvent(ctx context.Context, event *service.TLSFingerprintCaptureSessionEvent) (*service.TLSFingerprintCaptureSessionEvent, error) {
	create := clientFromContext(ctx, r.client).TLSFingerprintCaptureSessionEvent.Create().
		SetTaskID(event.TaskID).
		SetSessionID(strings.TrimSpace(event.SessionID)).
		SetEventID(strings.TrimSpace(event.EventID)).
		SetPlatform(strings.TrimSpace(event.Platform)).
		SetTransport(strings.TrimSpace(event.Transport)).
		SetEventType(strings.TrimSpace(event.EventType)).
		SetRequestSequence(event.RequestSequence).
		SetStreamID(strings.TrimSpace(event.StreamID)).
		SetRequestPath(event.RequestPath).
		SetHTTPMethod(event.HTTPMethod).
		SetIsWebsocket(event.IsWebsocket).
		SetWebsocketProtocol(strings.TrimSpace(event.WebsocketProtocol)).
		SetClientType(strings.TrimSpace(event.ClientType)).
		SetModel(event.Model).
		SetRequestKind(strings.TrimSpace(event.RequestKind)).
		SetStreaming(event.Streaming).
		SetResponseMode(strings.TrimSpace(event.ResponseMode)).
		SetUserAgent(event.UserAgent).
		SetOriginator(strings.TrimSpace(event.Originator)).
		SetStainlessMetadata(copyStringAnyMapOrEmpty(event.StainlessMetadata)).
		SetHeadersSnapshot(copyStringAnyMapOrEmpty(event.HeadersSnapshot)).
		SetBodySummary(event.BodySummary).
		SetRawPayload(event.RawPayload).
		SetEventStatus(defaultString(strings.TrimSpace(event.EventStatus), "recorded")).
		SetEventError(strings.TrimSpace(event.Error)).
		SetReplayable(event.Replayable).
		SetReplayHash(strings.TrimSpace(event.ReplayHash))
	if event.SessionRef > 0 {
		create.SetSessionRef(event.SessionRef)
	}
	if event.SampleID != nil && *event.SampleID > 0 {
		create.SetSampleID(*event.SampleID)
	}
	created, err := create.Save(ctx)
	if err != nil {
		return nil, err
	}
	return tlsCaptureSessionEventToService(created), nil
}
