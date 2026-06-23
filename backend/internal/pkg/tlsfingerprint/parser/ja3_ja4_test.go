package parser

import (
	"testing"

	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

func TestJA3AndJA4DeriveExactFingerprintsFromNativeClientHello(t *testing.T) {
	observed, err := ParseObservedClientHello(nativeClientHelloBytes(t))
	require.NoError(t, err)

	derived, err := DeriveFingerprints(observed)
	require.NoError(t, err)

	require.Equal(t, "771,4865-4866-4867-49195,0-10-11-13-16-43-45-51,29-23,0", derived.JA3Raw)
	require.Equal(t, "d91a5ebf506df5fff8496b890f19f1b8", derived.JA3Hash)
	require.Equal(t, "t13d0408h2_39e807bd56df_8b8b673a840a", derived.JA4)
	require.Equal(t, "h2,http/1.1", derived.ALPNFingerprint)
	require.Equal(t, "45bbd870db7bf6288bbb04148fa59180", derived.ReplayHash)
	require.Equal(t, "task1-v1", derived.ReplayHashVersion)
	require.Equal(t, parseVersion, derived.ParseVersion)
	require.Empty(t, derived.Http2Fingerprint)
}

func TestDeriveFingerprintsWithHTTP2SettingsPopulatesFingerprint(t *testing.T) {
	observed, err := ParseObservedClientHello(nativeClientHelloBytes(t))
	require.NoError(t, err)

	derived, err := DeriveFingerprintsWithHTTP2Settings(observed, map[uint16]uint32{
		1: 4096,
		3: 100,
		4: 65535,
	})
	require.NoError(t, err)

	require.Equal(t, "1:4096,3:100,4:65535", derived.Http2Fingerprint)
}

func TestReplayHashUsesCanonicalReplayIdentityForGrease(t *testing.T) {
	observedA := &ObservedClientHello{
		EnableGREASE:            true,
		CipherSuites:            []uint16{0x1a1a, utls.TLS_AES_128_GCM_SHA256, utls.TLS_AES_256_GCM_SHA384},
		Curves:                  []uint16{0x2a2a, uint16(utls.X25519), uint16(utls.CurveP256)},
		PointFormats:            []uint16{0},
		SignatureAlgorithms:     []uint16{0x3a3a, 0x0403, 0x0804},
		SignatureAlgorithmsCert: []uint16{0x4a4a, 0x0503},
		ALPNProtocols:           []string{"h2", "http/1.1"},
		SupportedVersions:       []uint16{0x5a5a, utls.VersionTLS13, utls.VersionTLS12},
		KeyShareGroups:          []uint16{0x6a6a, uint16(utls.X25519)},
		PSKModes:                []uint16{uint16(utls.PskModeDHE)},
		ExtensionsOrder:         []uint16{0x7a7a, 0, 10, 43, 0x8a8a, 13, 50},
	}
	observedB := &ObservedClientHello{
		EnableGREASE:            true,
		CipherSuites:            []uint16{0xbaba, utls.TLS_AES_128_GCM_SHA256, utls.TLS_AES_256_GCM_SHA384},
		Curves:                  []uint16{0xcaca, uint16(utls.X25519), uint16(utls.CurveP256)},
		PointFormats:            []uint16{0},
		SignatureAlgorithms:     []uint16{0xdada, 0x0403, 0x0804},
		SignatureAlgorithmsCert: []uint16{0xeaea, 0x0503},
		ALPNProtocols:           []string{"h2", "http/1.1"},
		SupportedVersions:       []uint16{0xfafa, utls.VersionTLS13, utls.VersionTLS12},
		KeyShareGroups:          []uint16{0x9a9a, uint16(utls.X25519)},
		PSKModes:                []uint16{uint16(utls.PskModeDHE)},
		ExtensionsOrder:         []uint16{0xaaaa, 0, 10, 43, 0x1a1a, 13, 50},
	}

	derivedA, err := DeriveFingerprints(observedA)
	require.NoError(t, err)
	derivedB, err := DeriveFingerprints(observedB)
	require.NoError(t, err)

	require.Equal(t, derivedA.ReplayHash, derivedB.ReplayHash)
}

func TestReplayHashChangesWhenReplayAffectingFieldsChange(t *testing.T) {
	base := &ObservedClientHello{
		CipherSuites:                   []uint16{utls.TLS_AES_128_GCM_SHA256, utls.TLS_AES_256_GCM_SHA384},
		Curves:                         []uint16{uint16(utls.X25519), uint16(utls.CurveP256)},
		PointFormats:                   []uint16{0},
		SignatureAlgorithms:            []uint16{0x0403, 0x0804},
		SignatureAlgorithmsCert:        []uint16{0x0503},
		ALPNProtocols:                  []string{"h2", "http/1.1"},
		SupportedVersions:              []uint16{utls.VersionTLS13, utls.VersionTLS12},
		KeyShareGroups:                 []uint16{uint16(utls.X25519)},
		PSKModes:                       []uint16{uint16(utls.PskModeDHE)},
		ExtensionsOrder:                []uint16{0, 16, 27, 34, 43, 45, 51, 17513},
		CompressCertAlgos:              []uint16{0x0002},
		DelegatedCredentialsAlgorithms: []uint16{0x0403},
		ApplicationSettingsProtocols:   []string{"h2"},
	}

	cases := []struct {
		name   string
		mutate func(*ObservedClientHello)
	}{
		{
			name: "compress cert algos",
			mutate: func(observed *ObservedClientHello) {
				observed.CompressCertAlgos = []uint16{0x0001}
			},
		},
		{
			name: "delegated credentials algorithms",
			mutate: func(observed *ObservedClientHello) {
				observed.DelegatedCredentialsAlgorithms = []uint16{0x0804}
			},
		},
		{
			name: "application settings protocols",
			mutate: func(observed *ObservedClientHello) {
				observed.ApplicationSettingsProtocols = []string{"http/1.1"}
			},
		},
	}

	baseDerived, err := DeriveFingerprints(base)
	require.NoError(t, err)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := cloneObservedClientHello(base)
			tc.mutate(mutated)

			derived, err := DeriveFingerprints(mutated)
			require.NoError(t, err)
			require.NotEqual(t, baseDerived.ReplayHash, derived.ReplayHash)
		})
	}
}

func TestReplayHashChangesWhenUnknownExtensionPayloadChanges(t *testing.T) {
	base := &ObservedClientHello{
		CipherSuites:      []uint16{utls.TLS_AES_128_GCM_SHA256},
		ExtensionsOrder:   []uint16{0xface},
		ExtensionMetadata: map[uint16][]byte{0xface: {0x01, 0x02}},
	}
	mutated := cloneObservedClientHello(base)
	mutated.ExtensionMetadata[0xface] = []byte{0x02, 0x03}

	baseDerived, err := DeriveFingerprints(base)
	require.NoError(t, err)
	mutatedDerived, err := DeriveFingerprints(mutated)
	require.NoError(t, err)

	require.NotEqual(t, baseDerived.ReplayHash, mutatedDerived.ReplayHash)
}

func cloneObservedClientHello(in *ObservedClientHello) *ObservedClientHello {
	if in == nil {
		return nil
	}
	out := *in
	out.RawClientHello = append([]byte(nil), in.RawClientHello...)
	out.GreaseValues = append([]uint16(nil), in.GreaseValues...)
	out.CipherSuites = append([]uint16(nil), in.CipherSuites...)
	out.Curves = append([]uint16(nil), in.Curves...)
	out.PointFormats = append([]uint16(nil), in.PointFormats...)
	out.SignatureAlgorithms = append([]uint16(nil), in.SignatureAlgorithms...)
	out.SignatureAlgorithmsCert = append([]uint16(nil), in.SignatureAlgorithmsCert...)
	out.ExtensionsOrder = append([]uint16(nil), in.ExtensionsOrder...)
	out.ALPNProtocols = append([]string(nil), in.ALPNProtocols...)
	out.SupportedVersions = append([]uint16(nil), in.SupportedVersions...)
	out.KeyShareGroups = append([]uint16(nil), in.KeyShareGroups...)
	out.PSKModes = append([]uint16(nil), in.PSKModes...)
	out.CompressCertAlgos = append([]uint16(nil), in.CompressCertAlgos...)
	out.DelegatedCredentialsAlgorithms = append([]uint16(nil), in.DelegatedCredentialsAlgorithms...)
	out.ApplicationSettingsProtocols = append([]string(nil), in.ApplicationSettingsProtocols...)
	if in.ExtensionMetadata != nil {
		out.ExtensionMetadata = make(map[uint16][]byte, len(in.ExtensionMetadata))
		for key, value := range in.ExtensionMetadata {
			out.ExtensionMetadata[key] = append([]byte(nil), value...)
		}
	}
	return &out
}
