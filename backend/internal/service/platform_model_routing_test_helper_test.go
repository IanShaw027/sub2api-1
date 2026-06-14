package service

func resetPlatformModelRoutingConfigCacheForTest() {
	platformDefaultAccountModelConfigCache.Store((*cachedPlatformModelRoutingConfig)(nil))
	platformDefaultAccountModelConfigSF.Forget(SettingKeyPlatformDefaultAccountModelConfig)
}
