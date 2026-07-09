package http2

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	xhttp2 "golang.org/x/net/http2"
)

type PriorityFrame struct {
	StreamID   uint32
	Dependency uint32
	Exclusive  bool
	Weight     uint8
}

type ParsedFingerprint struct {
	Settings          []xhttp2.Setting
	ConnWindowUpdates []uint32
	Priorities        []PriorityFrame
	PseudoHeaderOrder []string
}

// FingerprintFromSettings builds a compatibility summary from an unordered settings map.
// It intentionally canonicalizes by setting ID because map iteration loses wire order.
// Use Fingerprint(...) with an ordered []Setting whenever capture fidelity matters.
func FingerprintFromSettings(settings map[uint16]uint32) string {
	if len(settings) == 0 {
		return ""
	}
	keys := make([]int, 0, len(settings))
	for key := range settings {
		keys = append(keys, int(key))
	}
	sort.Ints(keys)
	ordered := make([]xhttp2.Setting, 0, len(keys))
	for _, key := range keys {
		ordered = append(ordered, xhttp2.Setting{ID: xhttp2.SettingID(key), Val: settings[uint16(key)]})
	}
	return Fingerprint(ordered, nil, nil, nil)
}

func Fingerprint(settings []xhttp2.Setting, connWindowUpdates []uint32, priorities []PriorityFrame, pseudoHeaderOrder []string) string {
	if len(settings) == 0 && len(connWindowUpdates) == 0 && len(priorities) == 0 && len(pseudoHeaderOrder) == 0 {
		return ""
	}

	segments := make([]string, 0, 3)
	if len(settings) > 0 {
		parts := make([]string, 0, len(settings))
		for _, setting := range settings {
			parts = append(parts, fmt.Sprintf("%d:%d", setting.ID, setting.Val))
		}
		segments = append(segments, strings.Join(parts, ","))
	}
	if len(connWindowUpdates) > 0 {
		parts := make([]string, 0, len(connWindowUpdates))
		for _, update := range connWindowUpdates {
			parts = append(parts, fmt.Sprintf("%d", update))
		}
		segments = append(segments, "wu:"+strings.Join(parts, ","))
	}
	if len(priorities) > 0 {
		parts := make([]string, 0, len(priorities))
		for _, priority := range priorities {
			exclusive := 0
			if priority.Exclusive {
				exclusive = 1
			}
			parts = append(parts, fmt.Sprintf("%d:%d:%d:%d", priority.StreamID, priority.Dependency, exclusive, priority.Weight))
		}
		segments = append(segments, "p:"+strings.Join(parts, ","))
	}
	if len(pseudoHeaderOrder) > 0 {
		segments = append(segments, "ph:"+strings.Join(pseudoHeaderOrder, ","))
	}
	return strings.Join(segments, "|")
}

func ValidateFingerprint(raw string) error {
	_, err := ParseFingerprint(raw)
	return err
}

func ParseFingerprint(raw string) (*ParsedFingerprint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return &ParsedFingerprint{}, nil
	}

	parsed := &ParsedFingerprint{}
	var seenSettings, seenWindowUpdates, seenPriorities, seenPseudoHeaders bool
	for _, segment := range strings.Split(raw, "|") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return nil, fmt.Errorf("empty segment")
		}
		switch {
		case strings.HasPrefix(segment, "wu:"):
			if seenWindowUpdates {
				return nil, fmt.Errorf("duplicate window update segment")
			}
			seenWindowUpdates = true
			values, err := parseUIntList(strings.TrimPrefix(segment, "wu:"))
			if err != nil {
				return nil, fmt.Errorf("invalid window update segment: %w", err)
			}
			parsed.ConnWindowUpdates = values
		case strings.HasPrefix(segment, "p:"):
			if seenPriorities {
				return nil, fmt.Errorf("duplicate priority segment")
			}
			seenPriorities = true
			values, err := parsePriorityList(strings.TrimPrefix(segment, "p:"))
			if err != nil {
				return nil, fmt.Errorf("invalid priority segment: %w", err)
			}
			parsed.Priorities = values
		case strings.HasPrefix(segment, "ph:"):
			if seenPseudoHeaders {
				return nil, fmt.Errorf("duplicate pseudo header segment")
			}
			seenPseudoHeaders = true
			values, err := parsePseudoHeaderList(strings.TrimPrefix(segment, "ph:"))
			if err != nil {
				return nil, fmt.Errorf("invalid pseudo header segment: %w", err)
			}
			parsed.PseudoHeaderOrder = values
		default:
			if seenSettings {
				return nil, fmt.Errorf("duplicate settings segment")
			}
			seenSettings = true
			values, err := parseSettingsList(segment)
			if err != nil {
				return nil, fmt.Errorf("invalid settings segment: %w", err)
			}
			parsed.Settings = values
		}
	}
	return parsed, nil
}

func validateSettingsList(raw string) error {
	_, err := parseSettingsList(raw)
	return err
}

func validatePriorityList(raw string) error {
	_, err := parsePriorityList(raw)
	return err
}

func validatePseudoHeaderList(raw string) error {
	_, err := parsePseudoHeaderList(raw)
	return err
}

func parseSettingsList(raw string) ([]xhttp2.Setting, error) {
	values := make([]xhttp2.Setting, 0)
	err := validateColonTuples(raw, 2, func(parts []string) error {
		id, err := parseUintMax(parts[0], math.MaxUint16, "setting id")
		if err != nil {
			return err
		}
		val, err := parseUintMax(parts[1], math.MaxUint32, "setting value")
		if err != nil {
			return err
		}
		values = append(values, xhttp2.Setting{ID: xhttp2.SettingID(id), Val: uint32(val)})
		return nil
	})
	return values, err
}

func parsePriorityList(raw string) ([]PriorityFrame, error) {
	values := make([]PriorityFrame, 0)
	err := validateColonTuples(raw, 4, func(parts []string) error {
		streamID, err := parseUintMax(parts[0], math.MaxUint32, "stream id")
		if err != nil {
			return err
		}
		dependency, err := parseUintMax(parts[1], math.MaxUint32, "dependency")
		if err != nil {
			return err
		}
		exclusive, err := parseUint(parts[2])
		if err != nil {
			return fmt.Errorf("exclusive flag: %w", err)
		}
		if exclusive > 1 {
			return fmt.Errorf("exclusive flag must be 0 or 1")
		}
		weight, err := parseUintMax(parts[3], math.MaxUint8, "weight")
		if err != nil {
			return err
		}
		values = append(values, PriorityFrame{
			StreamID:   uint32(streamID),
			Dependency: uint32(dependency),
			Exclusive:  exclusive == 1,
			Weight:     uint8(weight),
		})
		return nil
	})
	return values, err
}

func parsePseudoHeaderList(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("empty pseudo header list")
	}
	values := make([]string, 0)
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, fmt.Errorf("empty pseudo header")
		}
		if !strings.HasPrefix(item, ":") {
			return nil, fmt.Errorf("pseudo header %q must start with ':'", item)
		}
		values = append(values, item)
	}
	return values, nil
}

func validateUIntList(raw string) error {
	_, err := parseUIntList(raw)
	return err
}

func parseUIntList(raw string) ([]uint32, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("empty integer list")
	}
	values := make([]uint32, 0)
	for _, item := range strings.Split(raw, ",") {
		n, err := parseUintMax(item, math.MaxUint32, "window update")
		if err != nil {
			return nil, err
		}
		values = append(values, uint32(n))
	}
	return values, nil
}

func validateColonTuples(raw string, width int, validate func([]string) error) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("empty tuple list")
	}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return fmt.Errorf("empty tuple")
		}
		parts := strings.Split(item, ":")
		if len(parts) != width {
			return fmt.Errorf("tuple %q must have %d fields", item, width)
		}
		if err := validate(parts); err != nil {
			return err
		}
	}
	return nil
}

func parseUint(raw string) (uint64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty integer")
	}
	return strconv.ParseUint(raw, 10, 64)
}

func parseUintMax(raw string, max uint64, field string) (uint64, error) {
	value, err := parseUint(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", field, err)
	}
	if value > max {
		return 0, fmt.Errorf("%s must be <= %d", field, max)
	}
	return value, nil
}
