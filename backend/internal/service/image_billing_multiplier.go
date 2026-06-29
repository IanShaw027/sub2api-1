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
