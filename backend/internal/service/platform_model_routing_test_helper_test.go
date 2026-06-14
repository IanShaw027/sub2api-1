package service

func resetPlatformModelRoutingConfigCacheForTest() {
	platformModelRoutingConfigCache.Store((*cachedPlatformModelRoutingConfig)(nil))
	platformModelRoutingConfigSF.Forget(SettingKeyPlatformModelRoutingConfig)
	platformDefaultAccountModelConfigCache.Store((*cachedPlatformModelRoutingConfig)(nil))
	platformDefaultAccountModelConfigSF.Forget(SettingKeyPlatformDefaultAccountModelConfig)
}
