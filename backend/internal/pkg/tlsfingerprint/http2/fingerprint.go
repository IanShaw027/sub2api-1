package http2

import (
	"fmt"
	"sort"
	"strings"
)

func FingerprintFromSettings(settings map[uint16]uint32) string {
	if len(settings) == 0 {
		return ""
	}

	keys := make([]int, 0, len(settings))
	for key := range settings {
		keys = append(keys, int(key))
	}
	sort.Ints(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%d:%d", key, settings[uint16(key)]))
	}
	return strings.Join(parts, ",")
}
