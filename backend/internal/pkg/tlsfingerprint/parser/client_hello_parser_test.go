package parser

import (
	"net"
	"testing"

	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

func TestObservedClientHelloParsesNativeClientHello(t *testing.T) {
	raw := nativeClientHelloBytes(t)

	observed, err := ParseObservedClientHello(raw)
	require.NoError(t, err)

	require.Equal(t, raw, observed.RawClientHello)
	require.Equal(t, uint16(utls.VersionTLS12), observed.RecordVersion)
	require.Equal(t, uint16(utls.VersionTLS12), observed.LegacyVersion)
	require.Equal(t, uint16(utls.VersionTLS12), observed.ClientHelloVersion)
	require.True(t, observed.SniPresent)
	require.Equal(t, []uint16{
		0,
		10,
		11,
		13,
		16,
		43,
		45,
		51,
	}, observed.ExtensionsOrder)
	require.Equal(t, []string{"h2", "http/1.1"}, observed.ALPNProtocols)
	require.Equal(t, []uint16{utls.VersionTLS13, utls.VersionTLS12}, observed.SupportedVersions)
	require.Equal(t, []uint16{uint16(utls.X25519), uint16(utls.CurveP256)}, observed.Curves)
	require.Equal(t, []uint16{0x0403, 0x0804, 0x0401}, observed.SignatureAlgorithms)
	require.Empty(t, observed.SignatureAlgorithmsCert)
	require.False(t, observed.EnableGREASE)
	require.Contains(t, observed.ExtensionMetadata, uint16(16))
	require.NotEmpty(t, observed.ExtensionMetadata[16])
}

func TestObservedClientHelloPreservesSignatureAlgorithmsCertSeparately(t *testing.T) {
	spec := nativeClientHelloSpec()
	spec.Extensions = append(spec.Extensions,
		&utls.SignatureAlgorithmsCertExtension{
			SupportedSignatureAlgorithms: []utls.SignatureScheme{0x0503, 0x0805},
		},
	)
	raw := clientHelloBytesFromSpec(t, spec)

	observed, err := ParseObservedClientHello(raw)
	require.NoError(t, err)

	require.Equal(t, []uint16{0x0403, 0x0804, 0x0401}, observed.SignatureAlgorithms)
	require.Equal(t, []uint16{0x0503, 0x0805}, observed.SignatureAlgorithmsCert)
	require.Contains(t, observed.ExtensionsOrder, uint16(50))
	require.Contains(t, observed.ExtensionMetadata, uint16(50))
}

func nativeClientHelloBytes(t *testing.T) []byte {
	t.Helper()

	spec := nativeClientHelloSpec()
	return clientHelloBytesFromSpec(t, spec)
}

func clientHelloBytesFromSpec(t *testing.T, spec *utls.ClientHelloSpec) []byte {
	t.Helper()

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
