package repository

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/tlsfingerprintcapturesession"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type tlsFingerprintCaptureSessionRepo struct {
	client *ent.Client
}

func NewTLSFingerprintCaptureSessionRepo(client *ent.Client) *tlsFingerprintCaptureSessionRepo {
	return &tlsFingerprintCaptureSessionRepo{client: client}
}

func (r *tlsFingerprintCaptureSessionRepo) CreateSessionIfAbsent(ctx context.Context, session *service.TLSFingerprintCaptureSession) (*service.TLSFingerprintCaptureSession, bool, error) {
	sessionID := strings.TrimSpace(session.SessionID)
	existing, err := clientFromContext(ctx, r.client).TLSFingerprintCaptureSession.Query().
		Where(
			tlsfingerprintcapturesession.TaskID(session.TaskID),
			tlsfingerprintcapturesession.SessionID(sessionID),
		).
		Only(ctx)
	if err == nil {
		return tlsCaptureSessionToService(existing), false, nil
	}
	if !ent.IsNotFound(err) {
		return nil, false, err
	}

	create := clientFromContext(ctx, r.client).TLSFingerprintCaptureSession.Create().
		SetTaskID(session.TaskID).
		SetSessionID(sessionID).
		SetClientIP(strings.TrimSpace(session.ClientIP)).
		SetPlatform(strings.TrimSpace(session.Platform)).
		SetUserAgent(strings.TrimSpace(session.UserAgent)).
		SetOriginator(strings.TrimSpace(session.Originator)).
		SetAlpnNegotiated(strings.TrimSpace(session.ALPNNegotiated)).
		SetObservedClientHello(copyStringAnyMapOrEmpty(session.ObservedClientHello)).
		SetReplayProfile(copyStringAnyMapOrEmpty(session.ReplayProfile)).
		SetDerivedFingerprint(copyStringAnyMapOrEmpty(session.DerivedFingerprint)).
		SetSessionStatus(defaultString(strings.TrimSpace(session.SessionStatus), "observed")).
		SetErrorSummary(session.ErrorSummary)
	if len(session.RawClientHello) > 0 {
		create.SetRawClientHello(session.RawClientHello)
	}
	create.SetOpenedAt(coalesceTime(session.OpenedAt, session.CreatedAt, session.UpdatedAt, time.Now().UTC()))
	if session.ClosedAt != nil && !session.ClosedAt.IsZero() {
		create.SetClosedAt(*session.ClosedAt)
	}
	created, err := create.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			existing, getErr := clientFromContext(ctx, r.client).TLSFingerprintCaptureSession.Query().
				Where(
					tlsfingerprintcapturesession.TaskID(session.TaskID),
					tlsfingerprintcapturesession.SessionID(sessionID),
				).
				Only(ctx)
			if getErr != nil {
				return nil, false, getErr
			}
			return tlsCaptureSessionToService(existing), false, nil
		}
		return nil, false, err
	}
	return tlsCaptureSessionToService(created), true, nil
}
