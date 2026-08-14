package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

const identityRejectReason = "IDENTITY_REJECT"

// ValidateAccountExtraWrites runs identity and capacity checks before any extra persist.
func ValidateAccountExtraWrites(extra map[string]any) error {
	if err := ValidateAccountExtraIdentity(extra); err != nil {
		return err
	}
	return ValidateAccountCapacityExtra(extra)
}

// ValidateAccountExtraIdentity validates outbound identity-related extra fields.
func ValidateAccountExtraIdentity(extra map[string]any) error {
	if len(extra) == 0 {
		return nil
	}
	if v, ok := extra["openai_device_id"]; ok {
		if err := validateOpenAIDeviceIDExtra(v); err != nil {
			return err
		}
	}
	if _, ok := extra["tls_fingerprint_profile_id"]; ok {
		id, present, err := optionalCapacityInt(extra, "tls_fingerprint_profile_id")
		if err != nil {
			return err
		}
		if present && id == -1 {
			return identityReject("tls_fingerprint_profile_id must not be -1")
		}
	}
	return nil
}

// ValidateAccountCapacityExtra validates capacity and TLS extras on write.
// concurrency is empty or 1–32; max_sessions 0–10000; idle 1–1440;
// rpm_sticky_buffer empty or 1–10000. Non-integer floats are rejected.
func ValidateAccountCapacityExtra(extra map[string]any) error {
	if len(extra) == 0 {
		return nil
	}

	if n, present, err := optionalCapacityInt(extra, "concurrency"); err != nil {
		return err
	} else if present && (n < 1 || n > 32) {
		return identityReject("concurrency must be empty or 1-32")
	}

	if n, present, err := optionalCapacityInt(extra, "max_sessions"); err != nil {
		return err
	} else if present && (n < 0 || n > 10000) {
		return identityReject("max_sessions must be 0-10000")
	}

	if n, present, err := optionalCapacityInt(extra, "session_idle_timeout_minutes"); err != nil {
		return err
	} else if present && (n < 1 || n > 1440) {
		return identityReject("session_idle_timeout_minutes must be 1-1440")
	}

	if n, present, err := optionalCapacityInt(extra, "rpm_sticky_buffer"); err != nil {
		return err
	} else if present && (n < 1 || n > 10000) {
		return identityReject("rpm_sticky_buffer must be empty or 1-10000")
	}

	if n, present, err := optionalCapacityInt(extra, "tls_fingerprint_profile_id"); err != nil {
		return err
	} else if present && n == -1 {
		return identityReject("tls_fingerprint_profile_id must not be -1")
	}

	if v, ok := extra["enable_tls_fingerprint"]; ok && v != nil {
		if _, ok := v.(bool); !ok {
			return identityReject("enable_tls_fingerprint must be a boolean")
		}
	}

	if v, ok := extra["codex_fingerprint_mode"]; ok && v != nil {
		mode, ok := v.(string)
		if !ok {
			return identityReject("codex_fingerprint_mode must be off, device, session, or full")
		}
		switch strings.TrimSpace(mode) {
		case "", "off", "device", "session", "full":
		default:
			return identityReject("codex_fingerprint_mode must be off, device, session, or full")
		}
	}

	return nil
}

func validateOpenAIDeviceIDExtra(value any) error {
	deviceID, ok := value.(string)
	if !ok {
		return identityReject("openai_device_id must be a UUID string")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil
	}
	parsed, err := uuid.Parse(deviceID)
	if err != nil || parsed.Version() == 0 || parsed.Variant() != uuid.RFC4122 {
		return identityReject("openai_device_id must be a valid RFC4122 UUID")
	}
	return nil
}

func optionalCapacityInt(extra map[string]any, key string) (int64, bool, error) {
	v, ok := extra[key]
	if !ok || v == nil {
		return 0, false, nil
	}
	if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
		return 0, false, nil
	}
	n, err := identityExtraInt64(v)
	if err != nil {
		return 0, true, identityReject(fmt.Sprintf("%s must be an integer", key))
	}
	return n, true, nil
}

func identityExtraInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		if v > uint64(math.MaxInt64) {
			return 0, fmt.Errorf("must be an integer")
		}
		return int64(v), nil
	case float32:
		return identityFloat64Int64(float64(v))
	case float64:
		return identityFloat64Int64(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i, nil
		}
		f, err := v.Float64()
		if err != nil {
			return 0, fmt.Errorf("must be an integer")
		}
		return identityFloat64Int64(f)
	case string:
		s := strings.TrimSpace(v)
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i, nil
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("must be an integer")
		}
		return identityFloat64Int64(f)
	default:
		return 0, fmt.Errorf("must be an integer")
	}
}

func identityFloat64Int64(v float64) (int64, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v {
		return 0, fmt.Errorf("must be an integer")
	}
	return int64(v), nil
}

func identityReject(message string) error {
	return infraerrors.BadRequest(identityRejectReason, message)
}
