package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

const CreationVoiceTicketTTL = 60 * time.Second

var (
	ErrCreationVoiceTicketInvalid     = errors.New("voice ticket is invalid or expired")
	ErrCreationVoiceTicketUnavailable = errors.New("voice ticket storage is unavailable")
)

// CreationVoiceTicketRecord contains no bearer JWT, API key, or upstream credential.
type CreationVoiceTicketRecord struct {
	UserID       int64  `json:"user_id"`
	GroupID      int64  `json:"group_id"`
	TokenVersion int64  `json:"token_version"`
	AuthEpoch    string `json:"auth_epoch"`
	SessionID    string `json:"session_id"`
	Fingerprint  string `json:"fingerprint"`
	ExpiresAt    int64  `json:"expires_at"`
}

// CreationVoiceTicketStore is optionally implemented by the existing GatewayCache.
type CreationVoiceTicketStore interface {
	SaveCreationVoiceTicket(context.Context, string, *CreationVoiceTicketRecord, time.Duration) error
	ConsumeCreationVoiceTicket(context.Context, string) (*CreationVoiceTicketRecord, error)
}

type creationVoiceTicketAuth interface {
	ValidateToken(string) (*JWTClaims, error)
	ValidateAccessTokenState(context.Context, *JWTClaims) error
}

type creationVoiceTicketUserReader interface {
	GetByID(context.Context, int64) (*User, error)
}

type CreationVoiceTicketService struct {
	auth  creationVoiceTicketAuth
	users creationVoiceTicketUserReader
	store CreationVoiceTicketStore
	now   func() time.Time
}

func NewCreationVoiceTicketService(auth *AuthService, users UserRepository, cache GatewayCache) *CreationVoiceTicketService {
	store, _ := cache.(CreationVoiceTicketStore)
	var validator creationVoiceTicketAuth
	if auth != nil {
		validator = auth
	}
	return &CreationVoiceTicketService{auth: validator, users: users, store: store, now: time.Now}
}

func (s *CreationVoiceTicketService) Issue(ctx context.Context, bearer string, groupID int64, fingerprint string) (string, time.Time, error) {
	if s == nil || s.auth == nil || s.users == nil || s.store == nil {
		return "", time.Time{}, ErrCreationVoiceTicketUnavailable
	}
	claims, err := s.auth.ValidateToken(bearer)
	if err != nil || claims == nil || claims.ExpiresAt == nil || groupID <= 0 {
		return "", time.Time{}, ErrCreationVoiceTicketInvalid
	}
	if _, err := s.validateIdentity(ctx, claims); err != nil {
		return "", time.Time{}, err
	}
	now := s.now()
	expires := now.Add(CreationVoiceTicketTTL)
	if claims.ExpiresAt.Time.Before(expires) {
		expires = claims.ExpiresAt.Time
	}
	if !expires.After(now) {
		return "", time.Time{}, ErrCreationVoiceTicketInvalid
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", time.Time{}, ErrCreationVoiceTicketUnavailable
	}
	ticket := base64.RawURLEncoding.EncodeToString(random)
	record := &CreationVoiceTicketRecord{
		UserID: claims.UserID, GroupID: groupID, TokenVersion: claims.TokenVersion,
		AuthEpoch: claims.AuthEpoch, SessionID: claims.SessionID,
		Fingerprint: fingerprint, ExpiresAt: expires.Unix(),
	}
	if err := s.store.SaveCreationVoiceTicket(ctx, hashCreationVoiceTicket(ticket), record, expires.Sub(now)); err != nil {
		return "", time.Time{}, ErrCreationVoiceTicketUnavailable
	}
	return ticket, expires, nil
}

func (s *CreationVoiceTicketService) Consume(ctx context.Context, ticket, fingerprint string) (*User, *CreationVoiceTicketRecord, error) {
	if s == nil || s.auth == nil || s.users == nil || s.store == nil {
		return nil, nil, ErrCreationVoiceTicketUnavailable
	}
	decoded, err := base64.RawURLEncoding.DecodeString(ticket)
	if err != nil || len(decoded) != 32 {
		return nil, nil, ErrCreationVoiceTicketInvalid
	}
	record, err := s.store.ConsumeCreationVoiceTicket(ctx, hashCreationVoiceTicket(ticket))
	if err != nil {
		return nil, nil, err
	}
	if record == nil || record.ExpiresAt <= s.now().Unix() || record.Fingerprint != fingerprint || record.GroupID <= 0 {
		return nil, nil, ErrCreationVoiceTicketInvalid
	}
	user, err := s.validateIdentity(ctx, &JWTClaims{
		UserID: record.UserID, TokenVersion: record.TokenVersion,
		AuthEpoch: record.AuthEpoch, SessionID: record.SessionID,
	})
	if err != nil {
		return nil, nil, err
	}
	return user, record, nil
}

func (s *CreationVoiceTicketService) validateIdentity(ctx context.Context, claims *JWTClaims) (*User, error) {
	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil || user == nil || !user.IsActive() || resolvedTokenVersion(user) != claims.TokenVersion {
		return nil, ErrCreationVoiceTicketInvalid
	}
	if err := s.auth.ValidateAccessTokenState(ctx, claims); err != nil {
		if errors.Is(err, ErrTokenRevoked) || errors.Is(err, ErrInvalidToken) {
			return nil, ErrCreationVoiceTicketInvalid
		}
		return nil, ErrCreationVoiceTicketUnavailable
	}
	return user, nil
}

func hashCreationVoiceTicket(ticket string) string {
	hash := sha256.Sum256([]byte(ticket))
	return hex.EncodeToString(hash[:])
}
