import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('Composite channel platform options', () => {
  it('includes the CN concrete providers for pricing and model mapping', () => {
    // compositePlatforms was extracted from ChannelsView.vue into
    // ChannelFormDialog.vue as part of the glass-ui-redesign line-count
    // reduction (task 11.5); the declaration now lives there.
    const source = readFileSync(resolve('src/components/admin/channel/ChannelFormDialog.vue'), 'utf8')
    const declaration = source.match(/const compositePlatforms:[^=]+=[^\n]+/)?.[0]

    expect(declaration).toContain("'kimi'")
    expect(declaration).toContain("'zhipu'")
    expect(declaration).toContain("'deepseek'")
  })
})
