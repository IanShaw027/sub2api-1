//go:build unit

package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testSessionNSA  = "0123456789abcdef0123456789abcdef"
	testSessionNSB  = "fedcba9876543210fedcba9876543210"
	testSessionNS33 = "0123456789abcdef0123456789abcdef0" // odd-length hex; DecodeString fails
)

func TestDeriveSessionIDsSameNamespaceAndAnchorIsStable(t *testing.T) {
	t.Parallel()

	s1, th1, w1, err := DeriveSessionIDs(testSessionNSA, "conv-1")
	require.NoError(t, err)
	s2, th2, w2, err := DeriveSessionIDs(testSessionNSA, "conv-1")
	require.NoError(t, err)

	require.Equal(t, s1, s2)
	require.Equal(t, th1, th2)
	require.Equal(t, w1, w2)
	require.NotEmpty(t, s1)
	require.NotEmpty(t, th1)
	require.NotEmpty(t, w1)
}

func TestDeriveSessionIDsDifferentAnchorsDiffer(t *testing.T) {
	t.Parallel()

	s1, th1, _, err := DeriveSessionIDs(testSessionNSA, "anchor-a")
	require.NoError(t, err)
	s2, th2, _, err := DeriveSessionIDs(testSessionNSA, "anchor-b")
	require.NoError(t, err)

	require.NotEqual(t, s1, s2)
	require.NotEqual(t, th1, th2)
}

func TestDeriveSessionIDsFailoverUsesTargetNamespace(t *testing.T) {
	t.Parallel()

	const anchor = "shared-client-session"
	_, threadA, windowA, err := DeriveSessionIDs(testSessionNSA, anchor)
	require.NoError(t, err)
	_, threadB, windowB, err := DeriveSessionIDs(testSessionNSB, anchor)
	require.NoError(t, err)

	require.NotEqual(t, threadA, threadB, "failover to B must not reuse A's thread")
	require.NotEqual(t, windowA, windowB, "failover to B must open a new window")
}

func TestDeriveSessionIDsWindowIDIsThreadColonZero(t *testing.T) {
	t.Parallel()

	_, threadID, windowID, err := DeriveSessionIDs(testSessionNSA, "win")
	require.NoError(t, err)
	require.Equal(t, threadID+":0", windowID)
}

func TestDeriveSessionIDsInvalidNamespaceErrors(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"abc",
		strings.Repeat("0", 31),
		strings.Repeat("0", 65),
		"0123456789ABCDEF0123456789ABCDEF", // uppercase rejected by profile validate
		"0123456789abcdef0123456789abcdeg",
	}
	for _, ns := range cases {
		t.Run(fmt.Sprintf("len=%d", len(ns)), func(t *testing.T) {
			t.Parallel()
			sessionID, threadID, windowID, err := DeriveSessionIDs(ns, "anchor")
			require.Error(t, err)
			require.Empty(t, sessionID)
			require.Empty(t, threadID)
			require.Empty(t, windowID)
		})
	}
}

func TestDeriveSessionIDsEmptyAnchorIsDeterministic(t *testing.T) {
	t.Parallel()

	s1, th1, w1, err := DeriveSessionIDs(testSessionNSA, "")
	require.NoError(t, err)
	s2, th2, w2, err := DeriveSessionIDs(testSessionNSA, "")
	require.NoError(t, err)
	require.Equal(t, s1, s2)
	require.Equal(t, th1, th2)
	require.Equal(t, w1, w2)

	s3, _, _, err := DeriveSessionIDs(testSessionNSA, "not-empty")
	require.NoError(t, err)
	require.NotEqual(t, s1, s3)
}

func TestDeriveSessionIDsHMACKeyIsHexDecodedWhenEven(t *testing.T) {
	t.Parallel()

	sessionID, threadID, windowID, err := DeriveSessionIDs(testSessionNSA, "conv-1")
	require.NoError(t, err)

	key, decodeErr := hex.DecodeString(testSessionNSA)
	require.NoError(t, decodeErr)
	wantSession := hmacStableUUIDv4ForTest(t, key, "sess:conv-1")
	wantThread := hmacStableUUIDv4ForTest(t, key, "thread:"+wantSession)
	require.Equal(t, wantSession, sessionID, "HMAC key must be hex-decoded namespace bytes")
	require.Equal(t, wantThread, threadID)
	require.Equal(t, wantThread+":0", windowID)

	rawKeySession := hmacStableUUIDv4ForTest(t, []byte(testSessionNSA), "sess:conv-1")
	require.NotEqual(t, rawKeySession, sessionID, "must not HMAC with raw hex string bytes when decode succeeds")
}

func TestDeriveSessionIDsHMACKeyFallsBackToRawBytesWhenOddHex(t *testing.T) {
	t.Parallel()

	_, decodeErr := hex.DecodeString(testSessionNS33)
	require.Error(t, decodeErr)

	sessionID, threadID, windowID, err := DeriveSessionIDs(testSessionNS33, "conv-1")
	require.NoError(t, err)

	wantSession := hmacStableUUIDv4ForTest(t, []byte(testSessionNS33), "sess:conv-1")
	wantThread := hmacStableUUIDv4ForTest(t, []byte(testSessionNS33), "thread:"+wantSession)
	require.Equal(t, wantSession, sessionID)
	require.Equal(t, wantThread, threadID)
	require.Equal(t, wantThread+":0", windowID)
}

func hmacStableUUIDv4ForTest(t *testing.T, key []byte, message string) string {
	t.Helper()
	mac := hmac.New(sha256.New, key)
	_, err := mac.Write([]byte(message))
	require.NoError(t, err)
	sum := mac.Sum(nil)
	b := make([]byte, 16)
	copy(b, sum[:16])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]),
		binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16])
}
