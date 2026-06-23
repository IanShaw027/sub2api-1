package tlsfingerprint

import (
	"testing"

	utls "github.com/refraction-networking/utls"
)

func TestBuildClientHelloSpecUsesParametricReplayExtensions(t *testing.T) {
	profile := &Profile{
		Name:                           "parametric extensions",
		Extensions:                     []uint16{27, 34, 17513, 17613},
		CompressCertAlgos:              []uint16{2, 1},
		DelegatedCredentialsAlgorithms: []uint16{1027, 2052},
		ApplicationSettingsProtocols:   []string{"h2"},
	}

	spec := buildClientHelloSpecFromProfile(profile)
	if len(spec.Extensions) != 4 {
		t.Fatalf("expected 4 extensions, got %d", len(spec.Extensions))
	}

	compress, ok := spec.Extensions[0].(*utls.UtlsCompressCertExtension)
	if !ok {
		t.Fatalf("extension 27 should use UtlsCompressCertExtension, got %T", spec.Extensions[0])
	}
	if got, want := uint16(compress.Algorithms[0]), uint16(2); got != want {
		t.Fatalf("first compress cert algo = %d, want %d", got, want)
	}

	delegated, ok := spec.Extensions[1].(*utls.FakeDelegatedCredentialsExtension)
	if !ok {
		t.Fatalf("extension 34 should use FakeDelegatedCredentialsExtension, got %T", spec.Extensions[1])
	}
	if got, want := uint16(delegated.SupportedSignatureAlgorithms[1]), uint16(2052); got != want {
		t.Fatalf("second delegated credentials algorithm = %d, want %d", got, want)
	}

	applicationSettings, ok := spec.Extensions[2].(*utls.ApplicationSettingsExtension)
	if !ok {
		t.Fatalf("extension 17513 should use ApplicationSettingsExtension, got %T", spec.Extensions[2])
	}
	if got, want := applicationSettings.SupportedProtocols[0], "h2"; got != want {
		t.Fatalf("application settings protocol = %q, want %q", got, want)
	}

	applicationSettingsNew, ok := spec.Extensions[3].(*utls.ApplicationSettingsExtensionNew)
	if !ok {
		t.Fatalf("extension 17613 should use ApplicationSettingsExtensionNew, got %T", spec.Extensions[3])
	}
	if got, want := applicationSettingsNew.SupportedProtocols[0], "h2"; got != want {
		t.Fatalf("new application settings protocol = %q, want %q", got, want)
	}
}

func TestBuildClientHelloSpecClampsTLSMaxWhenSupportedVersionsExtensionMissing(t *testing.T) {
	legacyTLS12Profile := &Profile{
		Name:                "legacy-http-profile",
		CipherSuites:        []uint16{255, 49196, 49195, 49188, 49187, 49162, 49161, 49160, 49200, 49199, 49192, 49191, 49172, 49171, 49170, 157, 156, 61, 60, 53, 47, 10},
		Curves:              []uint16{23, 24, 25},
		PointFormats:        []uint16{0},
		SignatureAlgorithms: []uint16{1025, 513, 1281, 1537, 1027, 515, 1283, 1539},
		SupportedVersions:   []uint16{utls.VersionTLS13, utls.VersionTLS12},
		KeyShareGroups:      []uint16{29},
		PSKModes:            []uint16{1},
		Extensions:          []uint16{0, 10, 11, 13, 5, 18, 23},
	}

	spec := buildClientHelloSpecFromProfile(legacyTLS12Profile)
	if got, want := spec.TLSVersMax, uint16(utls.VersionTLS12); got != want {
		t.Fatalf("TLSVersMax = 0x%04x, want 0x%04x when extension 43 is absent", got, want)
	}
}

func TestBuildClientHelloSpecKeepsTLS13WhenSupportedVersionsExtensionPresent(t *testing.T) {
	profile := &Profile{
		Name:              "tls13-http-profile",
		SupportedVersions: []uint16{utls.VersionTLS13, utls.VersionTLS12},
		KeyShareGroups:    []uint16{29},
		PSKModes:          []uint16{1},
		Extensions:        []uint16{0, 10, 11, 13, 23, 43, 45, 51},
	}

	spec := buildClientHelloSpecFromProfile(profile)
	if got, want := spec.TLSVersMax, uint16(utls.VersionTLS13); got != want {
		t.Fatalf("TLSVersMax = 0x%04x, want 0x%04x when extension 43 is present", got, want)
	}
}
