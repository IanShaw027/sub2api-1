package kiro

import "testing"

func TestNormalizeMachineID_CanonicalizesUppercase64Hex(t *testing.T) {
	const raw = "00112233445566778899AABBCCDDEEFF00112233445566778899AABBCCDDEEFF"
	const want = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

	if got := NormalizeMachineID(raw); got != want {
		t.Fatalf("NormalizeMachineID() = %q, want %q", got, want)
	}
}

func TestNormalizeMachineID_CanonicalizesUUIDAnd32Hex(t *testing.T) {
	const base = "00112233445566778899aabbccddeeff"
	const want = base + base

	tests := []string{
		"00112233-4455-6677-8899-AABBCCDDEEFF",
		"00112233445566778899AABBCCDDEEFF",
	}

	for _, raw := range tests {
		if got := NormalizeMachineID(raw); got != want {
			t.Fatalf("NormalizeMachineID(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestGenerateMachineID_PrefersCredentialMachineID(t *testing.T) {
	const credentialMachineID = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	const refreshToken = "refresh-token-1"

	if got := GenerateMachineID(credentialMachineID, refreshToken); got != credentialMachineID {
		t.Fatalf("GenerateMachineID() = %q, want credential machine id %q", got, credentialMachineID)
	}
}

func TestGenerateMachineID_NormalizesCredentialMachineID(t *testing.T) {
	const credentialMachineID = "00112233445566778899AABBCCDDEEFF00112233445566778899AABBCCDDEEFF"
	const want = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

	if got := GenerateMachineID(credentialMachineID, "refresh-token-1"); got != want {
		t.Fatalf("GenerateMachineID() = %q, want normalized credential machine id %q", got, want)
	}
}

func TestGenerateMachineID_DerivesStableMachineIDFromRefreshToken(t *testing.T) {
	const refreshToken = "refresh-token-2"

	first := GenerateMachineID("", refreshToken)
	second := GenerateMachineID("", refreshToken)
	if first == "" {
		t.Fatal("GenerateMachineID() returned empty machine id for refresh token")
	}
	if first != second {
		t.Fatalf("GenerateMachineID() should be stable, got %q and %q", first, second)
	}
}
