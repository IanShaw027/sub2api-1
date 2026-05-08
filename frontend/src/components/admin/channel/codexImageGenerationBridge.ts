export type CodexImageGenerationBridgeMode = 'inherit' | 'enabled' | 'disabled'

export function codexImageGenerationBridgeModeFromOverride(
  override: boolean | null | undefined
): CodexImageGenerationBridgeMode {
  if (override === true) {
    return 'enabled'
  }
  if (override === false) {
    return 'disabled'
  }
  return 'inherit'
}

export function codexImageGenerationBridgeOverrideFromMode(
  mode: CodexImageGenerationBridgeMode
): boolean | null {
  switch (mode) {
    case 'enabled':
      return true
    case 'disabled':
      return false
    default:
      return null
  }
}
