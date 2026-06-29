package service

var antiBanPlatformKeys = []string{
	PlatformAnthropic,
	PlatformOpenAI,
	PlatformGemini,
	PlatformGrok,
	PlatformKiro,
	PlatformAntigravity,
}

var antiBanPlatformKeySet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(antiBanPlatformKeys))
	for _, key := range antiBanPlatformKeys {
		m[key] = struct{}{}
	}
	return m
}()

func cloneAntiBanPlatforms(src map[string]bool) map[string]bool {
	if src == nil {
		return nil
	}
	out := make(map[string]bool, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func normalizeAntiBanPlatforms(src map[string]bool) map[string]bool {
	out := make(map[string]bool, len(antiBanPlatformKeys))
	for _, key := range antiBanPlatformKeys {
		out[key] = false
	}
	for k, v := range src {
		if _, ok := antiBanPlatformKeySet[k]; ok {
			out[k] = v
		}
	}
	return out
}
