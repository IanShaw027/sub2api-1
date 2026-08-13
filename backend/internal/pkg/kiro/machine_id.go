package kiro

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func NormalizeMachineID(machineID string) string {
	trimmed := strings.TrimSpace(machineID)
	if len(trimmed) == 64 && isHex(trimmed) {
		return strings.ToLower(trimmed)
	}

	withoutDashes := strings.ReplaceAll(trimmed, "-", "")
	if len(withoutDashes) == 32 && isHex(withoutDashes) {
		lower := strings.ToLower(withoutDashes)
		return lower + lower
	}

	return ""
}

func GenerateMachineID(credentialMachineID, refreshToken string) string {
	if normalized := NormalizeMachineID(credentialMachineID); normalized != "" {
		return normalized
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("KotlinNativeAPI/" + refreshToken))
	return hex.EncodeToString(sum[:])
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}
