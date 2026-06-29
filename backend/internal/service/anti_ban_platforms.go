package service

// antiBanPlatformKeys 从 AllGatewayPlatforms 派生，确保与全局注册表一致。
var antiBanPlatformKeys = func() []string {
	out := make([]string, len(AllGatewayPlatforms))
	copy(out, AllGatewayPlatforms)
	return out
}()

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
