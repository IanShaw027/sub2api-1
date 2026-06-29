package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/stretchr/testify/require"
)

type tlsFingerprintProfileImportRepoStub struct {
	profiles []*model.TLSFingerprintProfile
	nextID   int64
}

func (r *tlsFingerprintProfileImportRepoStub) List(context.Context) ([]*model.TLSFingerprintProfile, error) {
	out := make([]*model.TLSFingerprintProfile, len(r.profiles))
	copy(out, r.profiles)
	return out, nil
}

func (r *tlsFingerprintProfileImportRepoStub) GetByID(_ context.Context, id int64) (*model.TLSFingerprintProfile, error) {
	for _, profile := range r.profiles {
		if profile.ID == id {
			return cloneTLSFingerprintProfile(profile), nil
		}
	}
	return nil, nil
}

func (r *tlsFingerprintProfileImportRepoStub) Create(_ context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	created := cloneTLSFingerprintProfile(profile)
	if r.nextID == 0 {
		r.nextID = 1
	}
	created.ID = r.nextID
	r.nextID++
	r.profiles = append(r.profiles, created)
	return cloneTLSFingerprintProfile(created), nil
}

func (r *tlsFingerprintProfileImportRepoStub) Update(_ context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	updated := cloneTLSFingerprintProfile(profile)
	for i, existing := range r.profiles {
		if existing.ID == profile.ID {
			r.profiles[i] = updated
			return cloneTLSFingerprintProfile(updated), nil
		}
	}
	r.profiles = append(r.profiles, updated)
	return cloneTLSFingerprintProfile(updated), nil
}

func (r *tlsFingerprintProfileImportRepoStub) Delete(_ context.Context, id int64) error {
	next := r.profiles[:0]
	for _, profile := range r.profiles {
		if profile.ID != id {
			next = append(next, profile)
		}
	}
	r.profiles = next
	return nil
}

func TestTLSFingerprintProfileServiceImportCapturesParsesCollectorYAML(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)

	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{`
name: "Codex Desktop 0.140.0-alpha.2 Windows x64"
description: "captured from live collector"
enable_grease: false
cipher_suites: [4865, 4866, 4867, 49195]
curves: [29, 23, 24]
point_formats: [0]
signature_algorithms: [1027, 2052, 1025]
alpn_protocols: ["h2", "http/1.1"]
supported_versions: [772, 771]
key_share_groups: [29]
psk_modes: [1]
extensions: [0, 65037, 23, 65281, 10, 11, 35, 16, 5, 13, 18, 51, 45, 43]
ja3_hash: ignored-for-storage
ja4: ignored-for-storage
`},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Imported)
	require.Equal(t, 0, result.Duplicates)
	require.Len(t, result.Profiles, 1)

	imported := result.Profiles[0].Profile
	require.Equal(t, int64(1), imported.ID)
	require.Equal(t, "Codex Desktop 0.140.0-alpha.2 Windows x64", imported.Name)
	require.NotNil(t, imported.Description)
	require.Equal(t, "captured from live collector", *imported.Description)
	require.False(t, imported.EnableGREASE)
	require.Equal(t, []uint16{4865, 4866, 4867, 49195}, imported.CipherSuites)
	require.Equal(t, []uint16{29, 23, 24}, imported.Curves)
	require.Equal(t, []uint16{0}, imported.PointFormats)
	require.Equal(t, []uint16{1027, 2052, 1025}, imported.SignatureAlgorithms)
	require.Equal(t, []string{"h2", "http/1.1"}, imported.ALPNProtocols)
	require.Equal(t, []uint16{772, 771}, imported.SupportedVersions)
	require.Equal(t, []uint16{29}, imported.KeyShareGroups)
	require.Equal(t, []uint16{1}, imported.PSKModes)
	require.Equal(t, []uint16{0, 65037, 23, 65281, 10, 11, 35, 16, 5, 13, 18, 51, 45, 43}, imported.Extensions)
	require.NotEmpty(t, result.Profiles[0].FingerprintHash)
}

func TestTLSFingerprintProfileServiceImportCapturesParsesTransport(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)

	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{`
name: "Codex Desktop over h2"
transport: "h2"
enable_grease: false
cipher_suites: [4865, 4866, 4867]
curves: [29, 23, 24]
point_formats: [0]
signature_algorithms: [1027, 2052, 1025]
alpn_protocols: ["h2", "http/1.1"]
supported_versions: [772, 771]
key_share_groups: [29]
psk_modes: [1]
extensions: [0, 10, 11, 13, 16, 43, 45, 51]
`},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Imported)
	require.Len(t, result.Profiles, 1)
	require.Equal(t, "h2", result.Profiles[0].Profile.Transport)
	require.Equal(t, "h2", repo.profiles[0].Transport)
}

func TestTLSFingerprintProfileServiceImportCapturesParsesDimensions(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)

	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{`
name: "Codex Desktop over h2"
transport: "h2"
os: "macos"
client_type: "codex-cli"
enable_grease: false
cipher_suites: [4865, 4866, 4867]
curves: [29, 23, 24]
point_formats: [0]
signature_algorithms: [1027, 2052, 1025]
alpn_protocols: ["h2", "http/1.1"]
supported_versions: [772, 771]
key_share_groups: [29]
psk_modes: [1]
extensions: [0, 10, 11, 13, 16, 43, 45, 51]
`},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Imported)
	require.Equal(t, "macos", result.Profiles[0].Profile.OS)
	require.Equal(t, "codex-cli", result.Profiles[0].Profile.ClientType)
	require.Equal(t, "macos", repo.profiles[0].OS)
	require.Equal(t, "codex-cli", repo.profiles[0].ClientType)
}

func TestTLSFingerprintProfileServiceImportCapturesParsesParametricReplayExtensions(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)

	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{`
name: "Codex Desktop with ALPS"
enable_grease: false
cipher_suites: [4865, 4866, 4867]
curves: [29, 23, 24]
point_formats: [0]
signature_algorithms: [1027, 2052, 1025]
alpn_protocols: ["h2", "http/1.1"]
supported_versions: [772, 771]
key_share_groups: [29]
psk_modes: [1]
extensions: [0, 27, 34, 17513, 17613, 10, 13, 16, 43, 45, 51]
compress_cert_algos: [2, 1]
delegated_credentials_algorithms: [1027, 2052]
application_settings_protocols: ["h2"]
`},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Imported)
	imported := result.Profiles[0].Profile
	require.Equal(t, []uint16{2, 1}, imported.CompressCertAlgos)
	require.Equal(t, []uint16{1027, 2052}, imported.DelegatedCredentialsAlgorithms)
	require.Equal(t, []string{"h2"}, imported.ApplicationSettingsProtocols)
	require.NotEmpty(t, result.Profiles[0].FingerprintHash)
}

func TestTLSFingerprintProfileServiceImportCapturesParsesSignatureAlgorithmsCertAndExtensionPayloads(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)
	payload := `{
  "name":"Replay profile with unknown extension payloads",
  "enable_grease":false,
  "cipher_suites":[4865,4866],
  "curves":[29,23],
  "point_formats":[0],
  "signature_algorithms":[1027,2052],
  "signature_algorithms_cert":[1284],
  "alpn_protocols":["http/1.1"],
  "supported_versions":[772,771],
  "key_share_groups":[29],
  "psk_modes":[1],
  "extensions":[0,11,10,13,43,45,50,51,65010],
  "extension_payloads":{"65010":"AQID"}
}`

	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{payload},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Imported)
	imported := result.Profiles[0].Profile
	require.Equal(t, []uint16{1284}, imported.SignatureAlgorithmsCert)
	require.Equal(t, map[uint16][]byte{65010: {1, 2, 3}}, imported.ExtensionPayloads)
}

func TestTLSFingerprintProfileServiceImportCapturesDedupesByFingerprintFields(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)

	payloadA := `{"name":"Codex Desktop Windows first","enable_grease":false,"cipher_suites":[4865,4866],"curves":[29,23],"point_formats":[0],"signature_algorithms":[1027],"alpn_protocols":["http/1.1"],"supported_versions":[772,771],"key_share_groups":[29],"psk_modes":[1],"extensions":[0,11,10,13,43,45,51]}`
	payloadB := `{"name":"Different name same fingerprint","enable_grease":false,"cipher_suites":[4865,4866],"curves":[29,23],"point_formats":[0],"signature_algorithms":[1027],"alpn_protocols":["http/1.1"],"supported_versions":[772,771],"key_share_groups":[29],"psk_modes":[1],"extensions":[0,11,10,13,43,45,51]}`

	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{payloadA, payloadB},
	})

	require.NoError(t, err)
	require.Equal(t, 1, result.Imported)
	require.Equal(t, 1, result.Duplicates)
	require.Len(t, result.Profiles, 2)
	require.False(t, result.Profiles[0].Duplicate)
	require.True(t, result.Profiles[1].Duplicate)
	require.Equal(t, result.Profiles[0].Profile.ID, result.Profiles[1].Profile.ID)
	require.Len(t, repo.profiles, 1)
}

func TestTLSFingerprintProfileServiceImportCapturesDoesNotDedupeDifferentTransport(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)

	payloadHTTP1 := `{"name":"Same TLS over HTTP1","transport":"http1","enable_grease":false,"cipher_suites":[4865,4866],"curves":[29,23],"point_formats":[0],"signature_algorithms":[1027],"alpn_protocols":["http/1.1"],"supported_versions":[772,771],"key_share_groups":[29],"psk_modes":[1],"extensions":[0,11,10,13,43,45,51]}`
	payloadH2 := `{"name":"Same TLS over H2","transport":"h2","enable_grease":false,"cipher_suites":[4865,4866],"curves":[29,23],"point_formats":[0],"signature_algorithms":[1027],"alpn_protocols":["http/1.1"],"supported_versions":[772,771],"key_share_groups":[29],"psk_modes":[1],"extensions":[0,11,10,13,43,45,51]}`

	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{payloadHTTP1, payloadH2},
	})

	require.NoError(t, err)
	require.Equal(t, 2, result.Imported)
	require.Equal(t, 0, result.Duplicates)
	require.Len(t, repo.profiles, 2)
	require.Equal(t, "http1", repo.profiles[0].Transport)
	require.Equal(t, "h2", repo.profiles[1].Transport)
}

func TestTLSFingerprintProfileReplayHashIncludesParametricReplayExtensions(t *testing.T) {
	base := &model.TLSFingerprintProfile{
		Name:                           "base",
		CipherSuites:                   []uint16{4865, 4866},
		Curves:                         []uint16{29, 23},
		PointFormats:                   []uint16{0},
		SignatureAlgorithms:            []uint16{1027, 2052},
		ALPNProtocols:                  []string{"h2", "http/1.1"},
		SupportedVersions:              []uint16{772, 771},
		KeyShareGroups:                 []uint16{29},
		PSKModes:                       []uint16{1},
		Extensions:                     []uint16{0, 27, 34, 17513, 10, 13, 16, 43, 45, 51},
		CompressCertAlgos:              []uint16{2, 1},
		DelegatedCredentialsAlgorithms: []uint16{1027, 2052},
		ApplicationSettingsProtocols:   []string{"h2"},
	}
	changed := cloneTLSFingerprintProfile(base)
	changed.CompressCertAlgos = []uint16{1}

	baseHash, err := TLSFingerprintProfileReplayHash(base)
	require.NoError(t, err)
	changedHash, err := TLSFingerprintProfileReplayHash(changed)
	require.NoError(t, err)
	require.NotEqual(t, baseHash, changedHash)
}

func TestTLSFingerprintProfileReplayHashChangesWhenTransportChanges(t *testing.T) {
	base := &model.TLSFingerprintProfile{
		Name:                "base",
		Transport:           "http1",
		CipherSuites:        []uint16{4865, 4866},
		Curves:              []uint16{29, 23},
		PointFormats:        []uint16{0},
		SignatureAlgorithms: []uint16{1027, 2052},
		ALPNProtocols:       []string{"http/1.1"},
		SupportedVersions:   []uint16{772, 771},
		KeyShareGroups:      []uint16{29},
		PSKModes:            []uint16{1},
		Extensions:          []uint16{0, 11, 10, 13, 43, 45, 51},
	}
	changed := cloneTLSFingerprintProfile(base)
	changed.Transport = "h2"

	baseHash, err := TLSFingerprintProfileReplayHash(base)
	require.NoError(t, err)
	changedHash, err := TLSFingerprintProfileReplayHash(changed)
	require.NoError(t, err)
	require.NotEqual(t, baseHash, changedHash)
}

func TestTLSFingerprintProfileReplayHashChangesWhenSignatureAlgorithmsCertChanges(t *testing.T) {
	base := &model.TLSFingerprintProfile{
		Name:                    "base",
		CipherSuites:            []uint16{4865, 4866},
		Curves:                  []uint16{29, 23},
		PointFormats:            []uint16{0},
		SignatureAlgorithms:     []uint16{1027, 2052},
		SignatureAlgorithmsCert: []uint16{1025},
		ALPNProtocols:           []string{"http/1.1"},
		SupportedVersions:       []uint16{772, 771},
		KeyShareGroups:          []uint16{29},
		PSKModes:                []uint16{1},
		Extensions:              []uint16{0, 11, 10, 13, 43, 45, 50, 51},
	}
	changed := cloneTLSFingerprintProfile(base)
	changed.SignatureAlgorithmsCert = []uint16{1284}

	baseHash, err := TLSFingerprintProfileReplayHash(base)
	require.NoError(t, err)
	changedHash, err := TLSFingerprintProfileReplayHash(changed)
	require.NoError(t, err)
	require.NotEqual(t, baseHash, changedHash)
}

func TestTLSFingerprintProfileReplayHashChangesWhenUnknownExtensionPayloadChanges(t *testing.T) {
	base := &model.TLSFingerprintProfile{
		Name:                "base",
		CipherSuites:        []uint16{4865, 4866},
		Curves:              []uint16{29, 23},
		PointFormats:        []uint16{0},
		SignatureAlgorithms: []uint16{1027, 2052},
		ALPNProtocols:       []string{"http/1.1"},
		SupportedVersions:   []uint16{772, 771},
		KeyShareGroups:      []uint16{29},
		PSKModes:            []uint16{1},
		Extensions:          []uint16{0, 11, 10, 13, 43, 45, 51, 65010},
		ExtensionPayloads: map[uint16][]byte{
			65010: {1, 2, 3},
		},
	}
	changed := cloneTLSFingerprintProfile(base)
	changed.ExtensionPayloads = map[uint16][]byte{
		65010: {1, 2, 4},
	}

	baseHash, err := TLSFingerprintProfileReplayHash(base)
	require.NoError(t, err)
	changedHash, err := TLSFingerprintProfileReplayHash(changed)
	require.NoError(t, err)
	require.NotEqual(t, baseHash, changedHash)
}

func TestTLSFingerprintProfileServiceImportCapturesRequiresCompleteCurrentSchemaFingerprint(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)

	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{`{"name":"incomplete","cipher_suites":[4865]}`},
	})

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "complete replayable TLS fingerprint")
	require.Len(t, repo.profiles, 0)
}

func TestTLSFingerprintProfileServiceImportCapturesRequiresParametricReplayExtensionFields(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)

	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{`{"name":"missing compress params","cipher_suites":[4865],"curves":[29],"point_formats":[0],"signature_algorithms":[1027],"alpn_protocols":["h2"],"supported_versions":[772],"key_share_groups":[29],"psk_modes":[1],"extensions":[0,27,10,13,16,43,45,51]}`},
	})

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "compress_cert_algos is required")
	require.Len(t, repo.profiles, 0)
}

func TestTLSFingerprintProfileServiceImportCapturesRequiresSignatureAlgorithmsCertWhenExtension50Present(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)

	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{`{"name":"missing sig cert params","cipher_suites":[4865],"curves":[29],"point_formats":[0],"signature_algorithms":[1027],"alpn_protocols":["http/1.1"],"supported_versions":[772],"key_share_groups":[29],"psk_modes":[1],"extensions":[0,10,11,13,43,45,50,51]}`},
	})

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "signature_algorithms_cert is required")
	require.Len(t, repo.profiles, 0)
}

func TestTLSFingerprintProfileServiceImportCapturesRequiresUnknownExtensionPayloads(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)

	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{`{"name":"missing unknown ext payload","cipher_suites":[4865],"curves":[29],"point_formats":[0],"signature_algorithms":[1027],"alpn_protocols":["http/1.1"],"supported_versions":[772],"key_share_groups":[29],"psk_modes":[1],"extensions":[0,10,11,13,43,45,51,65010]}`},
	})

	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "extension_payloads is required")
	require.Len(t, repo.profiles, 0)
}

func TestTLSFingerprintProfileServiceImportCapturesAllowsTLS12FingerprintWithoutTLS13Fields(t *testing.T) {
	repo := &tlsFingerprintProfileImportRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, nil)

	// A faithful TLS 1.2 ClientHello carries no supported_versions(43), key_share(51),
	// or psk_key_exchange_modes(45) extensions, so those fields are legitimately empty
	// and must not be required for a complete replayable fingerprint.
	result, err := svc.ImportTLSFingerprintCaptures(context.Background(), TLSFingerprintCaptureImportRequest{
		Profiles: []string{`{"name":"TLS 1.2 fingerprint","cipher_suites":[49195,49199,52393],"curves":[29,23,24],"point_formats":[0],"signature_algorithms":[1027,1025],"alpn_protocols":["http/1.1"],"extensions":[0,11,10,13,16,23,35,5]}`},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, repo.profiles, 1)
}

func cloneTLSFingerprintProfile(profile *model.TLSFingerprintProfile) *model.TLSFingerprintProfile {
	if profile == nil {
		return nil
	}
	clone := *profile
	clone.CipherSuites = append([]uint16(nil), profile.CipherSuites...)
	clone.Curves = append([]uint16(nil), profile.Curves...)
	clone.PointFormats = append([]uint16(nil), profile.PointFormats...)
	clone.SignatureAlgorithms = append([]uint16(nil), profile.SignatureAlgorithms...)
	clone.SignatureAlgorithmsCert = append([]uint16(nil), profile.SignatureAlgorithmsCert...)
	clone.ALPNProtocols = append([]string(nil), profile.ALPNProtocols...)
	clone.SupportedVersions = append([]uint16(nil), profile.SupportedVersions...)
	clone.KeyShareGroups = append([]uint16(nil), profile.KeyShareGroups...)
	clone.PSKModes = append([]uint16(nil), profile.PSKModes...)
	clone.Extensions = append([]uint16(nil), profile.Extensions...)
	if len(profile.ExtensionPayloads) > 0 {
		clone.ExtensionPayloads = make(map[uint16][]byte, len(profile.ExtensionPayloads))
		for key, value := range profile.ExtensionPayloads {
			clone.ExtensionPayloads[key] = append([]byte(nil), value...)
		}
	}
	clone.CompressCertAlgos = append([]uint16(nil), profile.CompressCertAlgos...)
	clone.DelegatedCredentialsAlgorithms = append([]uint16(nil), profile.DelegatedCredentialsAlgorithms...)
	clone.ApplicationSettingsProtocols = append([]string(nil), profile.ApplicationSettingsProtocols...)
	return &clone
}
