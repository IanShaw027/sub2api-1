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
