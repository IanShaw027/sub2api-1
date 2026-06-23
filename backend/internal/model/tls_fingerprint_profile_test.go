package model

import "testing"

func TestTLSFingerprintProfileValidateAllowsCanonicalCaptureTransports(t *testing.T) {
	transports := []string{"", "http1", "h2", "websocket-http1", "websocket-h2"}
	for _, transport := range transports {
		t.Run(transport, func(t *testing.T) {
			profile := TLSFingerprintProfile{
				Name:      "transport-ok",
				Transport: transport,
			}
			if err := profile.Validate(); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestTLSFingerprintProfileValidateRejectsLegacyTransportAlias(t *testing.T) {
	profile := TLSFingerprintProfile{
		Name:      "legacy-transport",
		Transport: "websocket",
	}
	err := profile.Validate()
	if err == nil {
		t.Fatalf("Validate() error = nil, want transport validation error")
	}
	if !containsString(err.Error(), "transport must be empty, http1, h2, websocket-http1, or websocket-h2") {
		t.Fatalf("Validate() error = %q", err.Error())
	}
}

func TestTLSFingerprintProfileValidateRejectsInconsistentTLS13Fields(t *testing.T) {
	tests := []struct {
		name    string
		profile TLSFingerprintProfile
		wantErr string
	}{
		{
			name: "supported_versions without extension 43",
			profile: TLSFingerprintProfile{
				Name:              "bad-supported-versions",
				SupportedVersions: []uint16{772, 771},
				Extensions:        []uint16{0, 10, 11, 13, 23},
			},
			wantErr: "supported_versions requires extension 43",
		},
		{
			name: "key_share_groups without extension 51",
			profile: TLSFingerprintProfile{
				Name:           "bad-key-share",
				KeyShareGroups: []uint16{29},
				Extensions:     []uint16{0, 10, 11, 13, 23, 43},
			},
			wantErr: "key_share_groups requires extension 51",
		},
		{
			name: "psk_modes without extension 45",
			profile: TLSFingerprintProfile{
				Name:       "bad-psk-modes",
				PSKModes:   []uint16{1},
				Extensions: []uint16{0, 10, 11, 13, 23, 43, 51},
			},
			wantErr: "psk_modes requires extension 45",
		},
		{
			name: "signature_algorithms_cert without extension 50",
			profile: TLSFingerprintProfile{
				Name:                    "bad-sig-cert",
				SignatureAlgorithmsCert: []uint16{2052},
				Extensions:              []uint16{0, 10, 11, 13, 23},
			},
			wantErr: "signature_algorithms_cert requires extension 50",
		},
		{
			name: "compress_cert_algos without extension 27",
			profile: TLSFingerprintProfile{
				Name:              "bad-compress-cert",
				CompressCertAlgos: []uint16{2},
				Extensions:        []uint16{0, 10, 11, 13, 23},
			},
			wantErr: "compress_cert_algos requires extension 27",
		},
		{
			name: "delegated credentials without extension 34",
			profile: TLSFingerprintProfile{
				Name:                           "bad-delegated-creds",
				DelegatedCredentialsAlgorithms: []uint16{2052},
				Extensions:                     []uint16{0, 10, 11, 13, 23},
			},
			wantErr: "delegated_credentials_algorithms requires extension 34",
		},
		{
			name: "alps protocols without extension 17513 or 17613",
			profile: TLSFingerprintProfile{
				Name:                         "bad-alps",
				ApplicationSettingsProtocols: []string{"h2"},
				Extensions:                   []uint16{0, 10, 11, 13, 23},
			},
			wantErr: "application_settings_protocols requires extension 17513 or 17613",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want containing %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr && !containsString(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestTLSFingerprintProfileValidateAllowsConsistentTLS13Fields(t *testing.T) {
	profile := TLSFingerprintProfile{
		Name:                    "good-http-linux",
		SupportedVersions:       []uint16{772, 771},
		KeyShareGroups:          []uint16{4588, 29},
		PSKModes:                []uint16{1},
		Extensions:              []uint16{65281, 0, 11, 10, 35, 22, 23, 13, 43, 45, 51},
		SignatureAlgorithmsCert: nil,
	}

	if err := profile.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func containsString(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && indexString(haystack, needle) >= 0)
}

func indexString(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
