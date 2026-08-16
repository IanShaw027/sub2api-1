package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// DeriveSessionIDs returns stable outbound session/thread/window IDs from a
// profile session_namespace and a logical conversation anchor.
//
// HMAC key is the hex-decoded namespace when DecodeString succeeds (even-length
// hex, the common 32/64-char case). If decode fails (odd-length hex that still
// passes profile validate), the raw hex string bytes are used as the key.
//
//	session_id = UUID(HMAC-SHA256(key, "sess:"+anchor))
//	thread_id  = UUID(HMAC-SHA256(key, "thread:"+session_id))
//	window_id  = thread_id + ":0"
//
// Failover must call this with the target account's namespace so B never
// inherits A's thread. Invalid namespace is rejected; nothing is invented.
func DeriveSessionIDs(sessionNamespace, anchor string) (sessionID, threadID, windowID string, err error) {
	if err := validateSessionNamespace(sessionNamespace); err != nil {
		return "", "", "", err
	}
	key := sessionHMACKey(sessionNamespace)
	sessionID = hmacStableUUIDv4(key, "sess:"+anchor)
	threadID = hmacStableUUIDv4(key, "thread:"+sessionID)
	windowID = threadID + ":0"
	return sessionID, threadID, windowID, nil
}

func sessionHMACKey(sessionNamespace string) []byte {
	if decoded, err := hex.DecodeString(sessionNamespace); err == nil {
		return decoded
	}
	return []byte(sessionNamespace)
}

func hmacStableUUIDv4(key []byte, message string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(message))
	sum := mac.Sum(nil)
	b := make([]byte, 16)
	copy(b, sum[:16])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 1
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]),
		binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16])
}
