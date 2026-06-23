package replay

import (
	"net"
	"testing"
	_ "unsafe"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/parser"
	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

//go:linkname buildClientHelloSpecFromProfile github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint.buildClientHelloSpecFromProfile
func buildClientHelloSpecFromProfile(profile *tlsfingerprint.Profile) *utls.ClientHelloSpec

func TestReplayProfileFromObservedProducesReplayFields(t *testing.T) {
	observed, err := parser.ParseObservedClientHello(nativeClientHelloBytes(t))
	require.NoError(t, err)

	profile, err := ReplayProfileFromObserved(observed)
	require.NoError(t, err)

	require.Equal(t, observed.EnableGREASE, profile.EnableGREASE)
	require.Equal(t, observed.CipherSuites, profile.CipherSuites)
	require.Equal(t, observed.Curves, profile.Curves)
	require.Equal(t, observed.PointFormats, profile.PointFormats)
	require.Equal(t, observed.SignatureAlgorithms, profile.SignatureAlgorithms)
	require.Equal(t, observed.SignatureAlgorithmsCert, profile.SignatureAlgorithmsCert)
	require.Equal(t, observed.ALPNProtocols, profile.ALPNProtocols)
	require.Equal(t, observed.SupportedVersions, profile.SupportedVersions)
	require.Equal(t, observed.KeyShareGroups, profile.KeyShareGroups)
	require.Equal(t, observed.PSKModes, profile.PSKModes)
	require.Equal(t, observed.ExtensionsOrder, profile.Extensions)
}

func TestReplayProfileFromObservedPreservesReplayableGreaseSemantics(t *testing.T) {
	observed := &parser.ObservedClientHello{
		EnableGREASE:                   true,
		CipherSuites:                   []uint16{0x1a1a, utls.TLS_AES_128_GCM_SHA256, utls.TLS_AES_256_GCM_SHA384},
		Curves:                         []uint16{0x2a2a, uint16(utls.X25519), uint16(utls.CurveP256)},
		PointFormats:                   []uint16{0},
		SignatureAlgorithms:            []uint16{0x3a3a, 0x0403, 0x0804},
		SignatureAlgorithmsCert:        []uint16{0x4a4a, 0x0503},
		ALPNProtocols:                  []string{"h2", "http/1.1"},
		SupportedVersions:              []uint16{0x5a5a, utls.VersionTLS13, utls.VersionTLS12},
		KeyShareGroups:                 []uint16{0x6a6a, uint16(utls.X25519)},
		PSKModes:                       []uint16{uint16(utls.PskModeDHE)},
		ExtensionsOrder:                []uint16{0x7a7a, 0, 10, 43, 0x8a8a, 13, 50, 51},
		DelegatedCredentialsAlgorithms: []uint16{0x9a9a, 0x0403},
	}

	profile, err := ReplayProfileFromObserved(observed)
	require.NoError(t, err)

	require.Equal(t, []uint16{utls.TLS_AES_128_GCM_SHA256, utls.TLS_AES_256_GCM_SHA384}, profile.CipherSuites)
	require.Equal(t, []uint16{uint16(utls.X25519), uint16(utls.CurveP256)}, profile.Curves)
	require.Equal(t, []uint16{0x0403, 0x0804}, profile.SignatureAlgorithms)
	require.Equal(t, []uint16{0x0503}, profile.SignatureAlgorithmsCert)
	require.Equal(t, []uint16{utls.VersionTLS13, utls.VersionTLS12}, profile.SupportedVersions)
	require.Equal(t, []uint16{uint16(utls.X25519)}, profile.KeyShareGroups)
	require.Equal(t, []uint16{0x0403}, profile.DelegatedCredentialsAlgorithms)
	require.Equal(t, []uint16{utls.GREASE_PLACEHOLDER, 0, 10, 43, utls.GREASE_PLACEHOLDER, 13, 50, 51}, profile.Extensions)
}

func TestToTLSFingerprintProfileBuildsStableUTLSSpec(t *testing.T) {
	observed, err := parser.ParseObservedClientHello(nativeClientHelloBytes(t))
	require.NoError(t, err)

	replayProfile, err := ReplayProfileFromObserved(observed)
	require.NoError(t, err)

	tlsProfile := ToTLSFingerprintProfile(replayProfile)
	spec := buildClientHelloSpecFromProfile(tlsProfile)

	require.Equal(t, replayProfile.CipherSuites, spec.CipherSuites)
	require.Equal(t, []uint16{
		0,
		10,
		11,
		13,
		16,
		43,
		45,
		51,
	}, extensionIDsFromSpec(t, spec))
}

func TestToTLSFingerprintProfileHonorsSignatureAlgorithmsCert(t *testing.T) {
	replayProfile := &ReplayProfile{
		SignatureAlgorithms:     []uint16{0x0403},
		SignatureAlgorithmsCert: []uint16{0x0804},
		Extensions:              []uint16{13, 50},
	}

	spec := buildClientHelloSpecFromProfile(ToTLSFingerprintProfile(replayProfile))
	require.Len(t, spec.Extensions, 2)

	sigAlgs, ok := spec.Extensions[0].(*utls.SignatureAlgorithmsExtension)
	require.True(t, ok)
	require.Equal(t, []utls.SignatureScheme{0x0403}, sigAlgs.SupportedSignatureAlgorithms)

	sigAlgsCert, ok := spec.Extensions[1].(*utls.SignatureAlgorithmsCertExtension)
	require.True(t, ok)
	require.Equal(t, []utls.SignatureScheme{0x0804}, sigAlgsCert.SupportedSignatureAlgorithms)
}

func TestToTLSFingerprintProfileProjectsGreaseExtensionPositionsToUTLSSpec(t *testing.T) {
	replayProfile := &ReplayProfile{
		EnableGREASE:            true,
		CipherSuites:            []uint16{utls.TLS_AES_128_GCM_SHA256, utls.TLS_AES_256_GCM_SHA384},
		Curves:                  []uint16{uint16(utls.X25519), uint16(utls.CurveP256)},
		PointFormats:            []uint16{0},
		SignatureAlgorithms:     []uint16{0x0403, 0x0804},
		SignatureAlgorithmsCert: []uint16{0x0503},
		ALPNProtocols:           []string{"h2", "http/1.1"},
		SupportedVersions:       []uint16{utls.VersionTLS13, utls.VersionTLS12},
		KeyShareGroups:          []uint16{uint16(utls.X25519)},
		PSKModes:                []uint16{uint16(utls.PskModeDHE)},
		Extensions:              []uint16{utls.GREASE_PLACEHOLDER, 0, 10, 43, utls.GREASE_PLACEHOLDER, 13, 50, 51},
	}

	spec := buildClientHelloSpecFromProfile(ToTLSFingerprintProfile(replayProfile))

	require.Equal(t, replayProfile.Extensions, extensionIDsFromSpec(t, spec))
}

func TestReplayProfileFromObservedPreservesUnknownExtensionPayloads(t *testing.T) {
	observed := &parser.ObservedClientHello{
		ExtensionsOrder:   []uint16{0xface, 0xdead},
		ExtensionMetadata: map[uint16][]byte{0xface: []byte{0x01, 0x02}, 0xdead: []byte{0x03, 0x04, 0x05}},
	}

	profile, err := ReplayProfileFromObserved(observed)
	require.NoError(t, err)

	require.Equal(t, map[uint16][]byte{
		0xface: []byte{0x01, 0x02},
		0xdead: []byte{0x03, 0x04, 0x05},
	}, profile.ExtensionPayloads)
}

func TestToTLSFingerprintProfileReplaysUnknownExtensionPayloadBytes(t *testing.T) {
	replayProfile := &ReplayProfile{
		Extensions: []uint16{0xface, 0xdead},
		ExtensionPayloads: map[uint16][]byte{
			0xface: []byte{0x01, 0x02},
			0xdead: []byte{0x03, 0x04, 0x05},
		},
	}

	spec := buildClientHelloSpecFromProfile(ToTLSFingerprintProfile(replayProfile))

	require.Equal(t, []uint16{0xface, 0xdead}, extensionIDsFromSpec(t, spec))
	require.Equal(t, map[uint16][]byte{
		0xface: []byte{0x01, 0x02},
		0xdead: []byte{0x03, 0x04, 0x05},
	}, genericExtensionPayloads(spec))
}

func nativeClientHelloBytes(t *testing.T) []byte {
	t.Helper()

	spec := nativeClientHelloSpec()
	uconn := utls.UClient(&net.TCPConn{}, &utls.Config{ServerName: "cloud.example"}, utls.HelloCustom)
	require.NoError(t, uconn.ApplyPreset(spec))
	require.NoError(t, uconn.MarshalClientHello())
	require.NotEmpty(t, uconn.HandshakeState.Hello.Raw)

	return prependTLSRecordHeader(uconn.HandshakeState.Hello.Raw, utls.VersionTLS12)
}

func nativeClientHelloSpec() *utls.ClientHelloSpec {
	return &utls.ClientHelloSpec{
		CipherSuites: []uint16{
			utls.TLS_AES_128_GCM_SHA256,
			utls.TLS_AES_256_GCM_SHA384,
			utls.TLS_CHACHA20_POLY1305_SHA256,
			utls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
		CompressionMethods: []uint8{0},
		Extensions: []utls.TLSExtension{
			&utls.SNIExtension{},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{utls.X25519, utls.CurveP256}},
			&utls.SupportedPointsExtension{SupportedPoints: []uint8{0}},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{0x0403, 0x0804, 0x0401}},
			&utls.ALPNExtension{AlpnProtocols: []string{"h2", "http/1.1"}},
			&utls.SupportedVersionsExtension{Versions: []uint16{utls.VersionTLS13, utls.VersionTLS12}},
			&utls.PSKKeyExchangeModesExtension{Modes: []uint8{utls.PskModeDHE}},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{{Group: utls.X25519}}},
		},
		TLSVersMin: utls.VersionTLS12,
		TLSVersMax: utls.VersionTLS13,
	}
}

func prependTLSRecordHeader(handshake []byte, recordVersion uint16) []byte {
	record := make([]byte, 5+len(handshake))
	record[0] = 22
	record[1] = byte(recordVersion >> 8)
	record[2] = byte(recordVersion)
	record[3] = byte(len(handshake) >> 8)
	record[4] = byte(len(handshake))
	copy(record[5:], handshake)
	return record
}

func extensionIDsFromSpec(t *testing.T, spec *utls.ClientHelloSpec) []uint16 {
	t.Helper()

	ids := make([]uint16, 0, len(spec.Extensions))
	for _, ext := range spec.Extensions {
		switch typed := ext.(type) {
		case *utls.UtlsGREASEExtension:
			ids = append(ids, utls.GREASE_PLACEHOLDER)
		case *utls.SNIExtension:
			ids = append(ids, 0)
		case *utls.StatusRequestExtension:
			ids = append(ids, 5)
		case *utls.SupportedCurvesExtension:
			ids = append(ids, 10)
		case *utls.SupportedPointsExtension:
			ids = append(ids, 11)
		case *utls.SignatureAlgorithmsExtension:
			ids = append(ids, 13)
		case *utls.ALPNExtension:
			ids = append(ids, 16)
		case *utls.SCTExtension:
			ids = append(ids, 18)
		case *utls.ExtendedMasterSecretExtension:
			ids = append(ids, 23)
		case *utls.UtlsCompressCertExtension:
			ids = append(ids, 27)
		case *utls.FakeDelegatedCredentialsExtension:
			ids = append(ids, 34)
		case *utls.SessionTicketExtension:
			ids = append(ids, 35)
		case *utls.SupportedVersionsExtension:
			ids = append(ids, 43)
		case *utls.PSKKeyExchangeModesExtension:
			ids = append(ids, 45)
		case *utls.SignatureAlgorithmsCertExtension:
			ids = append(ids, 50)
		case *utls.KeyShareExtension:
			ids = append(ids, 51)
		case *utls.ApplicationSettingsExtension:
			ids = append(ids, 17513)
		case *utls.ApplicationSettingsExtensionNew:
			ids = append(ids, 17613)
		case *utls.GREASEEncryptedClientHelloExtension:
			ids = append(ids, 0xfe0d)
		case *utls.RenegotiationInfoExtension:
			ids = append(ids, 0xff01)
		case *utls.GenericExtension:
			ids = append(ids, typed.Id)
		default:
			t.Fatalf("unsupported extension type %T", ext)
		}
	}
	return ids
}

func genericExtensionPayloads(spec *utls.ClientHelloSpec) map[uint16][]byte {
	payloads := map[uint16][]byte{}
	for _, ext := range spec.Extensions {
		generic, ok := ext.(*utls.GenericExtension)
		if !ok {
			continue
		}
		payloads[generic.Id] = append([]byte(nil), generic.Data...)
	}
	return payloads
}
