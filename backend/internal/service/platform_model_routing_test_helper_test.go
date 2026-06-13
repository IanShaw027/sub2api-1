package service

func resetPlatformModelRoutingConfigCacheForTest() {
	platformModelRoutingConfigCache.Store((*cachedPlatformModelRoutingConfig)(nil))
	platformModelRoutingConfigSF.Forget(SettingKeyPlatformModelRoutingConfig)
}
