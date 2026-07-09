package service

func resolveImageRateMultiplier(apiKey *APIKey, effectiveGroupMultiplier float64) float64 {
	if apiKey != nil && apiKey.Group != nil {
		p := apiKey.Group.Platform
		if p == PlatformOpenAI {
			// OpenAI 图片 + Grok 图片/非文本 显式定价，不应用文本倍率（参考 OpenAI 行为）
			return 1
		}
	}
	if apiKey != nil && apiKey.Group != nil && apiKey.Group.ImageRateIndependent {
		if apiKey.Group.ImageRateMultiplier < 0 {
			return 0
		}
		return apiKey.Group.ImageRateMultiplier
	}
	return effectiveGroupMultiplier
}

func resolveVideoRateMultiplier(apiKey *APIKey, effectiveGroupMultiplier float64) float64 {
	if apiKey != nil && apiKey.Group != nil && apiKey.Group.VideoRateIndependent {
		if apiKey.Group.VideoRateMultiplier < 0 {
			return 0
		}
		return apiKey.Group.VideoRateMultiplier
	}
	if apiKey != nil && apiKey.Group != nil && groupHasAnyConfiguredVideoPrice(apiKey.Group) {
		// Explicit video prices are final media prices, like explicit OpenAI image
		// prices. Do not apply the text/token group multiplier unless the group
		// opts into an independent video multiplier.
		return 1
	}
	return effectiveGroupMultiplier
}

func groupHasAnyConfiguredVideoPrice(group *Group) bool {
	return group != nil && (group.VideoPrice480pPerSec != nil ||
		group.VideoPrice720pPerSec != nil ||
		group.VideoPrice1080pPerSec != nil ||
		group.VideoPrice4kPerSec != nil ||
		group.VideoPrice480P != nil ||
		group.VideoPrice720P != nil ||
		group.VideoPrice1080P != nil)
}
