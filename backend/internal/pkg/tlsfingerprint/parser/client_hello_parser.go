package parser

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/http2"
	utls "github.com/refraction-networking/utls"
)

const parseVersion = "task1-v1"

func ParseObservedClientHello(raw []byte) (*ObservedClientHello, error) {
	if len(raw) < 9 {
		return nil, fmt.Errorf("client hello too short")
	}

	pub := utls.UnmarshalClientHello(raw[5:])
	if pub == nil {
		return nil, fmt.Errorf("unmarshal client hello")
	}

	parsed, err := parseClientHelloBody(raw)
	if err != nil {
		return nil, err
	}

	observed := &ObservedClientHello{
		RawClientHello:                 slices.Clone(raw),
		LegacyVersion:                  parsed.clientHelloVersion,
		RecordVersion:                  parsed.recordVersion,
		ClientHelloVersion:             parsed.clientHelloVersion,
		SniPresent:                     pub.ServerName != "" || parsed.sniPresent,
		GreaseValues:                   parsed.greaseValues,
		CipherSuites:                   cloneUint16s(pub.CipherSuites),
		Curves:                         firstNonEmpty(parsed.curves, curveIDsToUint16(pub.SupportedCurves)),
		PointFormats:                   firstNonEmpty(parsed.pointFormats, uint8sToUint16(pub.SupportedPoints)),
		SignatureAlgorithms:            firstNonEmpty(parsed.signatureAlgorithms, signatureSchemesToUint16(pub.SupportedSignatureAlgorithms)),
		SignatureAlgorithmsCert:        signatureSchemesToUint16(pub.SupportedSignatureAlgorithmsCert),
		ExtensionsOrder:                parsed.extensionsOrder,
		ExtensionMetadata:              parsed.extensionMetadata,
		ALPNProtocols:                  firstNonEmptyStrings(parsed.alpnProtocols, slices.Clone(pub.AlpnProtocols)),
		SupportedVersions:              firstNonEmpty(parsed.supportedVersions, cloneUint16s(pub.SupportedVersions)),
		KeyShareGroups:                 firstNonEmpty(parsed.keyShareGroups, keySharesToGroups(pub.KeyShares)),
		PSKModes:                       firstNonEmpty(parsed.pskModes, uint8sToUint16(pub.PskModes)),
		CompressCertAlgos:              parsed.compressCertAlgos,
		DelegatedCredentialsAlgorithms: parsed.delegatedCredentialsAlgorithms,
		ApplicationSettingsProtocols:   parsed.applicationSettingsProtocols,
	}
	if len(observed.SignatureAlgorithmsCert) == 0 {
		observed.SignatureAlgorithmsCert = parsed.signatureAlgorithmsCert
	}
	if len(observed.GreaseValues) == 0 {
		observed.GreaseValues = collectGreaseValues(observed)
	}
	observed.EnableGREASE = len(observed.GreaseValues) > 0

	return observed, nil
}

func DeriveFingerprints(observed *ObservedClientHello) (*DerivedFingerprint, error) {
	return DeriveFingerprintsWithHTTP2Settings(observed, nil)
}

func DeriveFingerprintsWithHTTP2Settings(observed *ObservedClientHello, settings map[uint16]uint32) (*DerivedFingerprint, error) {
	if observed == nil {
		return nil, fmt.Errorf("observed client hello is required")
	}

	ja3Raw := buildJA3Raw(observed)
	ja3Hash := md5Hex(ja3Raw)
	alpnFingerprint := strings.Join(observed.ALPNProtocols, ",")
	replayMaterial := canonicalReplayIdentity(observed)

	return &DerivedFingerprint{
		ReplayHash:        md5Hex(replayMaterial),
		ReplayHashVersion: "task1-v1",
		JA3Raw:            ja3Raw,
		JA3Hash:           ja3Hash,
		JA4:               buildJA4(observed, ja3Hash),
		ALPNFingerprint:   alpnFingerprint,
		Http2Fingerprint:  http2.FingerprintFromSettings(settings),
		ParseVersion:      parseVersion,
	}, nil
}

func canonicalReplayIdentity(observed *ObservedClientHello) string {
	return strings.Join([]string{
		joinUint16s(normalizeReplayList(observed.CipherSuites)),
		joinUint16s(normalizeReplayList(observed.Curves)),
		joinUint16s(slices.Clone(observed.PointFormats)),
		joinUint16s(normalizeReplayList(observed.SignatureAlgorithms)),
		joinUint16s(normalizeReplayList(observed.SignatureAlgorithmsCert)),
		joinUint16s(normalizeReplayExtensions(observed.ExtensionsOrder)),
		strings.Join(observed.ALPNProtocols, ","),
		joinUint16s(normalizeReplayList(observed.SupportedVersions)),
		joinUint16s(normalizeReplayList(observed.KeyShareGroups)),
		joinUint16s(slices.Clone(observed.PSKModes)),
		joinUint16s(slices.Clone(observed.CompressCertAlgos)),
		joinUint16s(normalizeReplayList(observed.DelegatedCredentialsAlgorithms)),
		strings.Join(observed.ApplicationSettingsProtocols, ","),
		canonicalReplayExtensionPayloads(observed),
	}, "|")
}

func normalizeReplayList(values []uint16) []uint16 {
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

func canonicalReplayExtensionPayloads(observed *ObservedClientHello) string {
	if observed == nil || len(observed.ExtensionMetadata) == 0 {
		return ""
	}

	parts := make([]string, 0, len(observed.ExtensionMetadata))
	for _, id := range observed.ExtensionsOrder {
		if isGREASEValue(id) || isModeledReplayExtension(id) {
			continue
		}
		payload, ok := observed.ExtensionMetadata[id]
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%d:%x", id, payload))
	}
	return strings.Join(parts, ",")
}

func isModeledReplayExtension(id uint16) bool {
	switch id {
	case 0, 5, 10, 11, 13, 16, 18, 23, 27, 34, 35, 43, 45, 50, 51, 17513, 17613, 0xfe0d, 0xff01:
		return true
	default:
		return false
	}
}

func curveIDsToUint16(values []utls.CurveID) []uint16 {
	out := make([]uint16, 0, len(values))
	for _, value := range values {
		out = append(out, uint16(value))
	}
	return out
}

func uint8sToUint16(values []uint8) []uint16 {
	out := make([]uint16, 0, len(values))
	for _, value := range values {
		out = append(out, uint16(value))
	}
	return out
}

func signatureSchemesToUint16(values []utls.SignatureScheme) []uint16 {
	out := make([]uint16, 0, len(values))
	for _, value := range values {
		out = append(out, uint16(value))
	}
	return out
}

func keySharesToGroups(values []utls.KeyShare) []uint16 {
	out := make([]uint16, 0, len(values))
	for _, value := range values {
		out = append(out, uint16(value.Group))
	}
	return out
}

func collectGreaseValues(observed *ObservedClientHello) []uint16 {
	seen := map[uint16]struct{}{}
	var grease []uint16

	appendGrease := func(value uint16) {
		if !isGREASEValue(value) {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		grease = append(grease, value)
	}

	for _, value := range observed.CipherSuites {
		appendGrease(value)
	}
	for _, value := range observed.ExtensionsOrder {
		appendGrease(value)
	}
	for _, value := range observed.SupportedVersions {
		appendGrease(value)
	}
	for _, value := range observed.Curves {
		appendGrease(value)
	}
	for _, value := range observed.KeyShareGroups {
		appendGrease(value)
	}

	return grease
}

func isGREASEValue(v uint16) bool {
	return v&0x0f0f == 0x0a0a && v>>8 == v&0xff
}

func ungrease(v uint16) uint16 {
	if isGREASEValue(v) {
		return 0
	}
	return v
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

type parsedClientHello struct {
	recordVersion                  uint16
	clientHelloVersion             uint16
	sniPresent                     bool
	greaseValues                   []uint16
	extensionsOrder                []uint16
	extensionMetadata              map[uint16][]byte
	curves                         []uint16
	pointFormats                   []uint16
	signatureAlgorithms            []uint16
	signatureAlgorithmsCert        []uint16
	alpnProtocols                  []string
	supportedVersions              []uint16
	keyShareGroups                 []uint16
	pskModes                       []uint16
	compressCertAlgos              []uint16
	delegatedCredentialsAlgorithms []uint16
	applicationSettingsProtocols   []string
}

func parseClientHelloBody(raw []byte) (*parsedClientHello, error) {
	if len(raw) < 5 || raw[0] != 22 {
		return nil, fmt.Errorf("record is not a tls handshake")
	}
	recordVersion := readUint16(raw, 1)
	body := raw[5:]
	if len(body) < 42 || body[0] != 1 {
		return nil, fmt.Errorf("handshake is not client hello")
	}

	offset := 4
	clientHelloVersion := readUint16(body, offset)
	offset += 2 + 32
	if offset >= len(body) {
		return nil, fmt.Errorf("malformed client hello")
	}

	sessionIDLen := int(body[offset])
	offset++
	offset += sessionIDLen
	if offset+2 > len(body) {
		return nil, fmt.Errorf("malformed client hello session id")
	}

	cipherSuitesLen := int(readUint16(body, offset))
	offset += 2 + cipherSuitesLen
	if offset >= len(body) {
		return nil, fmt.Errorf("malformed client hello cipher suites")
	}

	compressionMethodsLen := int(body[offset])
	offset++
	offset += compressionMethodsLen
	if offset == len(body) {
		return &parsedClientHello{
			recordVersion:      recordVersion,
			clientHelloVersion: clientHelloVersion,
		}, nil
	}
	if offset+2 > len(body) {
		return nil, fmt.Errorf("malformed client hello extensions")
	}

	extLen := int(readUint16(body, offset))
	offset += 2
	if offset+extLen > len(body) {
		return nil, fmt.Errorf("malformed client hello extension block")
	}

	parsed := &parsedClientHello{
		recordVersion:      recordVersion,
		clientHelloVersion: clientHelloVersion,
		extensionMetadata:  make(map[uint16][]byte),
	}
	seenGrease := map[uint16]struct{}{}
	extBlockEnd := offset + extLen
	for offset+4 <= extBlockEnd {
		extType := readUint16(body, offset)
		offset += 2
		dataLen := int(readUint16(body, offset))
		offset += 2
		if offset+dataLen > extBlockEnd {
			return nil, fmt.Errorf("malformed extension payload")
		}
		data := body[offset : offset+dataLen]
		offset += dataLen

		parsed.extensionsOrder = append(parsed.extensionsOrder, extType)
		parsed.extensionMetadata[extType] = slices.Clone(data)
		if isGREASEValue(extType) {
			if _, ok := seenGrease[extType]; !ok {
				parsed.greaseValues = append(parsed.greaseValues, extType)
				seenGrease[extType] = struct{}{}
			}
		}

		switch extType {
		case 0:
			parsed.sniPresent = len(data) > 2
		case 10:
			parsed.curves = parseUint16Vector(data, 2)
		case 11:
			parsed.pointFormats = parseUint8Vector(data, 1)
		case 13:
			parsed.signatureAlgorithms = parseUint16Vector(data, 2)
		case 16:
			parsed.alpnProtocols = parseProtocolVector(data)
		case 43:
			parsed.supportedVersions = parseSupportedVersions(data)
		case 45:
			parsed.pskModes = parseUint8Vector(data, 1)
		case 50:
			parsed.signatureAlgorithmsCert = parseUint16Vector(data, 2)
		case 51:
			parsed.keyShareGroups = parseKeyShareGroups(data)
		case 27:
			parsed.compressCertAlgos = parseUint16Vector(data, 1)
		case 34:
			parsed.delegatedCredentialsAlgorithms = parseUint16Vector(data, 2)
		case 17513, 17613:
			parsed.applicationSettingsProtocols = parseProtocolVector(data)
		}
	}
	if offset != extBlockEnd {
		return nil, fmt.Errorf("malformed extension boundary")
	}

	return parsed, nil
}

func readUint16(data []byte, offset int) uint16 {
	return uint16(data[offset])<<8 | uint16(data[offset+1])
}

func parseUint16Vector(data []byte, prefixLen int) []uint16 {
	if len(data) < prefixLen {
		return nil
	}
	var length int
	switch prefixLen {
	case 1:
		length = int(data[0])
	case 2:
		length = int(readUint16(data, 0))
	default:
		return nil
	}
	start := prefixLen
	end := start + length
	if end > len(data) {
		return nil
	}
	out := make([]uint16, 0, length/2)
	for i := start; i+1 < end; i += 2 {
		out = append(out, readUint16(data, i))
	}
	return out
}

func parseUint8Vector(data []byte, prefixLen int) []uint16 {
	if len(data) < prefixLen {
		return nil
	}
	var length int
	switch prefixLen {
	case 1:
		length = int(data[0])
	case 2:
		length = int(readUint16(data, 0))
	default:
		return nil
	}
	start := prefixLen
	end := start + length
	if end > len(data) {
		return nil
	}
	out := make([]uint16, 0, length)
	for _, value := range data[start:end] {
		out = append(out, uint16(value))
	}
	return out
}

func parseProtocolVector(data []byte) []string {
	if len(data) < 2 {
		return nil
	}
	length := int(readUint16(data, 0))
	if 2+length > len(data) {
		return nil
	}
	payload := data[2 : 2+length]
	var protocols []string
	for i := 0; i < len(payload); {
		size := int(payload[i])
		i++
		if i+size > len(payload) {
			return nil
		}
		protocols = append(protocols, string(payload[i:i+size]))
		i += size
	}
	return protocols
}

func parseSupportedVersions(data []byte) []uint16 {
	return parseUint16Vector(data, 1)
}

func parseKeyShareGroups(data []byte) []uint16 {
	if len(data) < 2 {
		return nil
	}
	length := int(readUint16(data, 0))
	if 2+length > len(data) {
		return nil
	}
	payload := data[2 : 2+length]
	var groups []uint16
	for i := 0; i+3 < len(payload); {
		group := readUint16(payload, i)
		keyLen := int(readUint16(payload, i+2))
		groups = append(groups, group)
		i += 4 + keyLen
		if i > len(payload) {
			return nil
		}
	}
	return groups
}

func firstNonEmpty(primary, fallback []uint16) []uint16 {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

func firstNonEmptyStrings(primary, fallback []string) []string {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}

func cloneUint16s(values []uint16) []uint16 {
	if len(values) == 0 {
		return nil
	}
	return slices.Clone(values)
}
