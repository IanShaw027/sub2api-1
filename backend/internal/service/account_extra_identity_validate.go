package service

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
)

// ValidateAccountExtraIdentity validates outbound identity-related extra fields.
// Create/Update wiring is intentionally left to the merge step; Task 5 exports
// the shared validator without touching account create/update ownership files.
func ValidateAccountExtraIdentity(extra map[string]any) error {
	if len(extra) == 0 {
		return nil
	}
	if v, ok := extra["openai_device_id"]; ok {
		if err := validateOpenAIDeviceIDExtra(v); err != nil {
			return err
		}
	}
	if v, ok := extra["tls_fingerprint_profile_id"]; ok {
		id, ok := identityExtraInt64(v)
		if ok && id == -1 {
			return fmt.Errorf("tls_fingerprint_profile_id must not be -1")
		}
	}
	return nil
}

func validateOpenAIDeviceIDExtra(value any) error {
	deviceID, ok := value.(string)
	if !ok {
		return fmt.Errorf("openai_device_id must be a UUID string")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil
	}
	parsed, err := uuid.Parse(deviceID)
	if err != nil || parsed.Version() == 0 || parsed.Variant() != uuid.RFC4122 {
		return fmt.Errorf("openai_device_id must be a valid RFC4122 UUID")
	}
	return nil
}

func identityExtraInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		if math.Trunc(v) == v {
			return int64(v), true
		}
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i, true
		}
	}
	return 0, false
}
