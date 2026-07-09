package replay

import (
	"slices"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

type ReplayProfile struct {
	Name                           string
	EnableGREASE                   bool
	CipherSuites                   []uint16
	Curves                         []uint16
	PointFormats                   []uint16
	SignatureAlgorithms            []uint16
	SignatureAlgorithmsCert        []uint16
	ALPNProtocols                  []string
	SupportedVersions              []uint16
	KeyShareGroups                 []uint16
	PSKModes                       []uint16
	Extensions                     []uint16
	ExtensionPayloads              map[uint16][]byte
	CompressCertAlgos              []uint16
	DelegatedCredentialsAlgorithms []uint16
	ApplicationSettingsProtocols   []string
}

func ToTLSFingerprintProfile(profile *ReplayProfile) *tlsfingerprint.Profile {
	if profile == nil {
		return nil
	}
	return &tlsfingerprint.Profile{
		Name:                           profile.Name,
		HTTP2Fingerprint:               "",
		EnableGREASE:                   profile.EnableGREASE,
		CipherSuites:                   slices.Clone(profile.CipherSuites),
		Curves:                         slices.Clone(profile.Curves),
		PointFormats:                   slices.Clone(profile.PointFormats),
		SignatureAlgorithms:            slices.Clone(profile.SignatureAlgorithms),
		SignatureAlgorithmsCert:        slices.Clone(profile.SignatureAlgorithmsCert),
		ALPNProtocols:                  slices.Clone(profile.ALPNProtocols),
		SupportedVersions:              slices.Clone(profile.SupportedVersions),
		KeyShareGroups:                 slices.Clone(profile.KeyShareGroups),
		PSKModes:                       slices.Clone(profile.PSKModes),
		Extensions:                     slices.Clone(profile.Extensions),
		ExtensionPayloads:              cloneExtensionPayloads(profile.ExtensionPayloads),
		CompressCertAlgos:              slices.Clone(profile.CompressCertAlgos),
		DelegatedCredentialsAlgorithms: slices.Clone(profile.DelegatedCredentialsAlgorithms),
		ApplicationSettingsProtocols:   slices.Clone(profile.ApplicationSettingsProtocols),
	}
}

func cloneExtensionPayloads(in map[uint16][]byte) map[uint16][]byte {
	if len(in) == 0 {
		return nil
	}
	out := make(map[uint16][]byte, len(in))
	for key, value := range in {
		out[key] = slices.Clone(value)
	}
	return out
}
