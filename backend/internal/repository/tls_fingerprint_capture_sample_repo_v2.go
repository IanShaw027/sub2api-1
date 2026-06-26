package repository

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/tlsfingerprintcapturesample"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type tlsFingerprintCaptureSampleRepoV2 struct {
	client *ent.Client
}

func NewTLSFingerprintCaptureSampleRepoV2(client *ent.Client) *tlsFingerprintCaptureSampleRepoV2 {
	return &tlsFingerprintCaptureSampleRepoV2{client: client}
}

func (r *tlsFingerprintCaptureSampleRepoV2) DeleteSamplesByTask(ctx context.Context, taskID int64) error {
	_, err := clientFromContext(ctx, r.client).TLSFingerprintCaptureSample.Delete().
		Where(tlsfingerprintcapturesample.TaskID(taskID)).
		Exec(ctx)
	return err
}

func (r *tlsFingerprintCaptureSampleRepoV2) CreateSampleIfAbsent(ctx context.Context, sample *service.TLSFingerprintCaptureSample) (*service.TLSFingerprintCaptureSample, bool, error) {
	profile := sample.Profile
	if profile == nil {
		profile = &model.TLSFingerprintProfile{}
	}

	create := clientFromContext(ctx, r.client).TLSFingerprintCaptureSample.Create().
		SetTaskID(sample.TaskID).
		SetSessionID(strings.TrimSpace(sample.SessionID)).
		SetPlatform(strings.TrimSpace(sample.Platform)).
		SetTransport(strings.TrimSpace(sample.Transport)).
		SetUserAgent(strings.TrimSpace(sample.UserAgent)).
		SetOriginator(strings.TrimSpace(sample.Originator)).
		SetFingerprintHash(strings.TrimSpace(sample.FingerprintHash)).
		SetReplayHash(normalizeTLSCaptureStoredReplayHash(sample.ReplayHash, sample.FingerprintHash)).
		SetJa3Raw(strings.TrimSpace(sample.JA3Raw)).
		SetJa3Hash(strings.TrimSpace(sample.JA3Hash)).
		SetJa4(strings.TrimSpace(sample.JA4)).
		SetRequestPath(sample.RequestPath).
		SetHTTPMethod(sample.HTTPMethod).
		SetIsWebsocket(sample.IsWebsocket).
		SetWebsocketProtocol(strings.TrimSpace(sample.WebsocketProtocol)).
		SetClientType(strings.TrimSpace(sample.ClientType)).
		SetModel(sample.Model).
		SetRequestKind(strings.TrimSpace(sample.RequestKind)).
		SetStreaming(sample.Streaming).
		SetResponseMode(strings.TrimSpace(sample.ResponseMode)).
		SetHttp2Fingerprint(sample.HTTP2Fingerprint).
		SetStainlessMetadata(copyStringAnyMapOrEmpty(sample.StainlessMetadata)).
		SetReplayProfile(profile).
		SetRawPayload(sample.RawPayload).
		SetCapturedAt(coalesceTime(sample.CapturedAt, sample.CreatedAt))
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

func (r *tlsFingerprintCaptureSampleRepoV2) GetSampleByTaskHash(ctx context.Context, taskID int64, fingerprintHash string) (*service.TLSFingerprintCaptureSample, error) {
	sample, err := clientFromContext(ctx, r.client).TLSFingerprintCaptureSample.Query().
		Where(
			tlsfingerprintcapturesample.TaskID(taskID),
			tlsfingerprintcapturesample.FingerprintHash(strings.TrimSpace(fingerprintHash)),
		).
		Only(ctx)
	if err == nil {
		return tlsCaptureSampleToService(sample), nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	return nil, nil
}

func (r *tlsFingerprintCaptureSampleRepoV2) ListSamplesByTask(ctx context.Context, taskID int64) ([]*service.TLSFingerprintCaptureSample, error) {
	samples, err := clientFromContext(ctx, r.client).TLSFingerprintCaptureSample.Query().
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
