package parser

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
)

func buildJA3Raw(observed *ObservedClientHello) string {
	return fmt.Sprintf(
		"%d,%s,%s,%s,%s",
		observed.ClientHelloVersion,
		joinUint16s(normalizeForDerived(observed.CipherSuites)),
		joinUint16s(normalizeForDerived(observed.ExtensionsOrder)),
		joinUint16s(normalizeForDerived(observed.Curves)),
		joinUint16s(observed.PointFormats),
	)
}

func buildJA4(observed *ObservedClientHello, _ string) string {
	sniMarker := "i"
	if observed.SniPresent {
		sniMarker = "d"
	}
	ciphers := normalizeForDerived(observed.CipherSuites)
	extensions := normalizeForDerived(observed.ExtensionsOrder)
	sigAlgs := normalizeForDerived(observed.SignatureAlgorithms)

	cipherHex := uint16sToHex4(ciphers)
	slices.Sort(cipherHex)

	extensionHex := uint16sToHex4(filterJA4Extensions(extensions))
	slices.Sort(extensionHex)

	sigAlgHex := uint16sToHex4(sigAlgs)

	ja4A := fmt.Sprintf(
		"t%s%s%s%s%s",
		tlsVersionCode(normalizeForDerived(observed.SupportedVersions), observed.LegacyVersion),
		sniMarker,
		twoDigitCount(len(ciphers)),
		twoDigitCount(len(extensions)),
		ja4ALPN(observed.ALPNProtocols),
	)

	ja4B := sha256ListHash(cipherHex)
	ja4C := "000000000000"
	if len(extensionHex) > 0 {
		extensionInput := strings.Join(extensionHex, ",")
		if len(sigAlgHex) > 0 {
			extensionInput += "_" + strings.Join(sigAlgHex, ",")
		}
		ja4C = sha256Text12(extensionInput)
	}

	return fmt.Sprintf("%s_%s_%s", ja4A, ja4B, ja4C)
}

func normalizeForDerived(values []uint16) []uint16 {
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

func joinUint16s(values []uint16) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, fmt.Sprintf("%d", value))
	}
	return strings.Join(parts, "-")
}

func uint16sToHex4(values []uint16) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, fmt.Sprintf("%04x", value))
	}
	return out
}

func filterJA4Extensions(values []uint16) []uint16 {
	if len(values) == 0 {
		return nil
	}
	out := make([]uint16, 0, len(values))
	for _, value := range values {
		if value == 0 || value == 16 {
			continue
		}
		out = append(out, value)
	}
	return out
}

func tlsVersionCode(supportedVersions []uint16, legacyVersion uint16) string {
	version := legacyVersion
	if len(supportedVersions) > 0 {
		version = supportedVersions[0]
		for _, candidate := range supportedVersions[1:] {
			if candidate > version {
				version = candidate
			}
		}
	}

	switch version {
	case 772:
		return "13"
	case 771:
		return "12"
	case 770:
		return "11"
	case 769:
		return "10"
	case 768:
		return "s3"
	case 2:
		return "s2"
	case 65279:
		return "d1"
	case 65277:
		return "d2"
	case 65276:
		return "d3"
	default:
		return "00"
	}
}

func twoDigitCount(count int) string {
	if count > 99 {
		count = 99
	}
	return fmt.Sprintf("%02d", count)
}

func ja4ALPN(protocols []string) string {
	if len(protocols) == 0 || len(protocols[0]) == 0 {
		return "00"
	}
	alpn := []byte(protocols[0])
	first := alpn[0]
	last := alpn[len(alpn)-1]
	if isASCIIAlphaNumeric(first) && isASCIIAlphaNumeric(last) {
		return string([]byte{first, last})
	}
	hex := fmt.Sprintf("%x", alpn)
	return string([]byte{hex[0], hex[len(hex)-1]})
}

func isASCIIAlphaNumeric(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}

func sha256ListHash(hexValues []string) string {
	if len(hexValues) == 0 {
		return "000000000000"
	}
	return sha256Text12(strings.Join(hexValues, ","))
}

func sha256Text12(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])[:12]
}
