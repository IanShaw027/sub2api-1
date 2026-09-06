package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type voiceTicketAuthStub struct {
	claims *JWTClaims
	err    error
}

func (a *voiceTicketAuthStub) ValidateToken(string) (*JWTClaims, error) { return a.claims, nil }
func (a *voiceTicketAuthStub) ValidateAccessTokenState(context.Context, *JWTClaims) error {
	return a.err
}

type voiceTicketUsersStub struct{ user *User }

func (u *voiceTicketUsersStub) GetByID(context.Context, int64) (*User, error) { return u.user, nil }

type voiceTicketStoreStub struct {
	mu      sync.Mutex
	records map[string]*CreationVoiceTicketRecord
}

func (s *voiceTicketStoreStub) SaveCreationVoiceTicket(_ context.Context, hash string, record *CreationVoiceTicketRecord, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[hash] = record
	return nil
}
func (s *voiceTicketStoreStub) ConsumeCreationVoiceTicket(_ context.Context, hash string) (*CreationVoiceTicketRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[hash]
	delete(s.records, hash)
	if !ok {
		return nil, ErrCreationVoiceTicketInvalid
	}
	return record, nil
}

func newVoiceTicketTestService() (*CreationVoiceTicketService, *voiceTicketAuthStub, *voiceTicketUsersStub) {
	now := time.Now()
	user := &User{ID: 10, Status: StatusActive, TokenVersion: 7, TokenVersionResolved: true}
	auth := &voiceTicketAuthStub{claims: &JWTClaims{
		UserID: user.ID, TokenVersion: 7, AuthEpoch: "epoch", SessionID: "session",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))},
	}}
	users := &voiceTicketUsersStub{user: user}
	return &CreationVoiceTicketService{auth: auth, users: users,
		store: &voiceTicketStoreStub{records: make(map[string]*CreationVoiceTicketRecord)}, now: func() time.Time { return now }}, auth, users
}

func TestCreationVoiceTicketSingleUseAndBoundIdentity(t *testing.T) {
	svc, _, _ := newVoiceTicketTestService()
	ticket, expires, err := svc.Issue(context.Background(), "signed-user-jwt", 20, "ip-ua")
	require.NoError(t, err)
	require.Len(t, ticket, 43)
	require.Equal(t, svc.now().Add(CreationVoiceTicketTTL), expires)
	store := svc.store.(*voiceTicketStoreStub)
	require.NotContains(t, store.records, ticket)
	require.Contains(t, store.records, hashCreationVoiceTicket(ticket))
	user, record, err := svc.Consume(context.Background(), ticket, "ip-ua")
	require.NoError(t, err)
	require.Equal(t, int64(10), user.ID)
	require.Equal(t, int64(20), record.GroupID)
	require.Equal(t, "session", record.SessionID)
	_, _, err = svc.Consume(context.Background(), ticket, "ip-ua")
	require.ErrorIs(t, err, ErrCreationVoiceTicketInvalid)
}

func TestCreationVoiceTicketRejectsReplayRace(t *testing.T) {
	svc, _, _ := newVoiceTicketTestService()
	ticket, _, err := svc.Issue(context.Background(), "jwt", 20, "ip-ua")
	require.NoError(t, err)
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, _, err := svc.Consume(context.Background(), ticket, "ip-ua"); results <- err }()
	}
	first, second := <-results, <-results
	require.True(t, (first == nil && errors.Is(second, ErrCreationVoiceTicketInvalid)) || (second == nil && errors.Is(first, ErrCreationVoiceTicketInvalid)))
}

func TestCreationVoiceTicketRechecksExpiryFingerprintAndRevocation(t *testing.T) {
	for _, scenario := range []string{"expired", "fingerprint", "password", "disabled", "revoked", "cache unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			svc, auth, users := newVoiceTicketTestService()
			ticket, _, err := svc.Issue(context.Background(), "jwt", 20, "ip-ua")
			require.NoError(t, err)
			fingerprint := "ip-ua"
			want := ErrCreationVoiceTicketInvalid
			switch scenario {
			case "expired":
				now := svc.now()
				svc.now = func() time.Time { return now.Add(61 * time.Second) }
			case "fingerprint":
				fingerprint = "different-ip-ua"
			case "password":
				users.user.TokenVersion++
			case "disabled":
				users.user.Status = "disabled"
			case "revoked":
				auth.err = ErrTokenRevoked
			case "cache unavailable":
				auth.err = ErrServiceUnavailable
				want = ErrCreationVoiceTicketUnavailable
			}
			_, _, err = svc.Consume(context.Background(), ticket, fingerprint)
			require.ErrorIs(t, err, want)
		})
	}
}

func TestCreationVoiceTicketCannotOutliveJWT(t *testing.T) {
	svc, auth, _ := newVoiceTicketTestService()
	auth.claims.ExpiresAt = jwt.NewNumericDate(svc.now().Add(10 * time.Second))
	_, expires, err := svc.Issue(context.Background(), "jwt", 20, "ip-ua")
	require.NoError(t, err)
	require.Equal(t, auth.claims.ExpiresAt.Time, expires)
}
