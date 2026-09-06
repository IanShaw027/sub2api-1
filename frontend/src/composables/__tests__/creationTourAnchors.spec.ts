import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { parse } from 'vue/compiler-sfc'
import { afterEach, describe, expect, it } from 'vitest'
import { useIsMobile } from '../useIsMobile'

const originalMatchMedia = window.matchMedia
afterEach(() => { window.matchMedia = originalMatchMedia })

function createControls(file: string, anchor: string): string {
  const source = readFileSync(fileURLToPath(new URL(file, import.meta.url)), 'utf8')
  const { descriptor } = parse(source)
  const nodes = [...descriptor.template!.ast!.children]
  const controls: string[] = []
  while (nodes.length) {
    const node = nodes.shift()!
    if (node.type !== 1) continue
    nodes.push(...node.children)
    for (const prop of node.props) {
      const isAnchor = prop.type === 6
        ? prop.name === 'data-tour'
        : prop.name === 'bind' && prop.arg?.type === 4 && prop.arg.content === 'data-tour'
      if (isAnchor && prop.loc.source.includes(anchor)) {
        controls.push(`<button data-kind="${node.tag}" ${prop.loc.source}>Create</button>`)
      }
    }
  }
  expect(controls).toHaveLength(2)
  return `<div>${controls.join('')}</div>`
}

describe('responsive creation tour anchors', () => {
  it.each([
    ['../../views/admin/GroupsView.vue', 'groups-create-btn'],
    ['../../views/user/KeysView.vue', 'keys-create-btn']
  ])('keeps exactly one active anchor when resizing %s', async (file, anchor) => {
    const subscriptions: Array<{ matches: boolean; notify?: () => void }> = []
    window.matchMedia = (() => {
      const media = {
        matches: false,
        notify: undefined as (() => void) | undefined,
        addEventListener: (_event: string, listener: () => void) => { media.notify = listener },
        removeEventListener: () => undefined
      }
      subscriptions.push(media)
      return media
    }) as unknown as typeof window.matchMedia

    // Compile the actual page anchor bindings without mocking its business APIs.
    const wrapper = mount({ template: createControls(file, anchor), setup: useIsMobile })
    expect(wrapper.findAll(`[data-tour="${anchor}"]`)).toHaveLength(1)
    expect(wrapper.get(`[data-tour="${anchor}"]`).attributes('data-kind')).toBe('Fab')

    for (const media of subscriptions) {
      media.matches = true
      media.notify?.()
    }
    await nextTick()
    expect(wrapper.findAll(`[data-tour="${anchor}"]`)).toHaveLength(1)
    expect(wrapper.get(`[data-tour="${anchor}"]`).attributes('data-kind')).toBe('Button')
    wrapper.unmount()
  })
})
