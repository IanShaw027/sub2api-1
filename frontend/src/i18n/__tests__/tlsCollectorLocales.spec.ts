import { describe, expect, it } from 'vitest'
import en from '../locales/en'
import zh from '../locales/zh'

describe('tlsCollector locales', () => {
  it('defines messages for every backend capture ignore reason', () => {
    expect(en.tlsCollector.ignored.platform_target_reached).toBeTruthy()
    expect(zh.tlsCollector.ignored.platform_target_reached).toBeTruthy()
  })
})
