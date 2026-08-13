package service

import (
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func buildKiroTestFrame(t *testing.T, headers map[string]string, payload map[string]any) []byte {
	t.Helper()

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	headerNames := make([]string, 0, len(headers))
	for name := range headers {
		headerNames = append(headerNames, name)
	}
	sort.Strings(headerNames)

	headerBytes := make([]byte, 0, len(headers)*16)
	for _, name := range headerNames {
		value := headers[name]
		headerBytes = append(headerBytes, byte(len(name)))
		headerBytes = append(headerBytes, name...)
		headerBytes = append(headerBytes, 7)

		var valueLen [2]byte
		binary.BigEndian.PutUint16(valueLen[:], uint16(len(value)))
		headerBytes = append(headerBytes, valueLen[:]...)
		headerBytes = append(headerBytes, value...)
	}

	totalLength := kiroPreludeSize + len(headerBytes) + len(payloadBytes) + 4
	frame := make([]byte, totalLength)
	binary.BigEndian.PutUint32(frame[0:4], uint32(totalLength))
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(headerBytes)))
	binary.BigEndian.PutUint32(frame[8:12], crc32.ChecksumIEEE(frame[:8]))
	copy(frame[kiroPreludeSize:], headerBytes)
	copy(frame[kiroPreludeSize+len(headerBytes):], payloadBytes)
	binary.BigEndian.PutUint32(frame[totalLength-4:], crc32.ChecksumIEEE(frame[:totalLength-4]))
	return frame
}
