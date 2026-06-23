package replay

import (
	"fmt"
	"slices"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/parser"
	utls "github.com/refraction-networking/utls"
)

func ReplayProfileFromObserved(observed *parser.ObservedClientHello) (*ReplayProfile, error) {
	if observed == nil {
		return nil, fmt.Errorf("observed client hello is required")
	}

	return &ReplayProfile{
		EnableGREASE:                   observed.EnableGREASE,
		CipherSuites:                   normalizeReplayUint16s(observed.CipherSuites),
		Curves:                         normalizeReplayUint16s(observed.Curves),
		PointFormats:                   slices.Clone(observed.PointFormats),
		SignatureAlgorithms:            normalizeReplayUint16s(observed.SignatureAlgorithms),
		SignatureAlgorithmsCert:        normalizeReplayUint16s(observed.SignatureAlgorithmsCert),
		ALPNProtocols:                  slices.Clone(observed.ALPNProtocols),
		SupportedVersions:              normalizeReplayUint16s(observed.SupportedVersions),
		KeyShareGroups:                 normalizeReplayUint16s(observed.KeyShareGroups),
		PSKModes:                       slices.Clone(observed.PSKModes),
		Extensions:                     normalizeReplayExtensions(observed.ExtensionsOrder),
		ExtensionPayloads:              replayExtensionPayloads(observed),
		CompressCertAlgos:              slices.Clone(observed.CompressCertAlgos),
		DelegatedCredentialsAlgorithms: normalizeReplayUint16s(observed.DelegatedCredentialsAlgorithms),
		ApplicationSettingsProtocols:   slices.Clone(observed.ApplicationSettingsProtocols),
	}, nil
}

func normalizeReplayUint16s(values []uint16) []uint16 {
	if len(values) == 0 {
		return nil
	}
	out := make([]uint16, 0, len(values))
	for _, value := range values {
		if isGREASEValue(value) {
			continue
		}
		out = append(out, value)
	}
	return out
}

func normalizeReplayExtensions(values []uint16) []uint16 {
	if len(values) == 0 {
		return nil
	}
	out := make([]uint16, 0, len(values))
	for _, value := range values {
		if isGREASEValue(value) {
			out = append(out, utls.GREASE_PLACEHOLDER)
			continue
		}
		out = append(out, value)
	}
	return out
}

func replayExtensionPayloads(observed *parser.ObservedClientHello) map[uint16][]byte {
	if observed == nil || len(observed.ExtensionMetadata) == 0 {
		return nil
	}

	payloads := map[uint16][]byte{}
	for _, id := range observed.ExtensionsOrder {
		if isGREASEValue(id) || isModeledReplayExtension(id) {
			continue
		}
		payload, ok := observed.ExtensionMetadata[id]
		if !ok {
			continue
		}
		payloads[id] = slices.Clone(payload)
	}
	if len(payloads) == 0 {
		return nil
	}
	return payloads
}

func isModeledReplayExtension(id uint16) bool {
	switch id {
	case 0, 5, 10, 11, 13, 16, 18, 23, 27, 34, 35, 43, 45, 50, 51, 17513, 17613, 0xfe0d, 0xff01:
		return true
	default:
		return false
	}
}

func isGREASEValue(v uint16) bool {
	return v&0x0f0f == 0x0a0a && v>>8 == v&0xff
}
