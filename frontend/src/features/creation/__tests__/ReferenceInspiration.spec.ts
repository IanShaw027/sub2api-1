import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { readFileSync, statSync } from 'node:fs'
import { resolve } from 'node:path'
import { i18n } from '@/i18n'
import ReferenceInspiration from '../components/ReferenceInspiration.vue'
import { referenceCases } from '../components/referenceCases'

const copy = vi.hoisted(() => vi.fn())
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: copy }) }))

describe('curated reference inspiration', () => {
  let wrapper: VueWrapper | undefined

  beforeEach(() => {
    i18n.global.locale.value = 'en'
    copy.mockReset()
    copy.mockResolvedValue(true)
  })

  afterEach(() => {
    wrapper?.unmount()
    document.body.innerHTML = ''
  })

  function render() {
    wrapper = mount(ReferenceInspiration, {
      attachTo: document.body,
      global: { plugins: [i18n], stubs: { teleport: true, transition: true } },
    })
    return wrapper
  }

  it('ships diverse licensed cases with local JPEG previews and their original attribution records', () => {
    const manifest = JSON.parse(readFileSync(resolve(__dirname, '../../../../public/creation/inspiration/catalog-manifest.json'), 'utf8'))
    const included = manifest.cases.filter((item: { status: string }) => item.status === 'included')
    expect(referenceCases.length).toBe(manifest.includedCount)
    expect(referenceCases.map(item => item.id).sort()).toEqual(included.map((item: { id: string }) => item.id).sort())
    expect(manifest.cases).toHaveLength(manifest.totalCaseDirectories)
    expect(manifest.excludedCollections).toEqual(expect.arrayContaining([expect.objectContaining({ source: 'mageia', includedCount: 0 })]))
    expect(new Set(referenceCases.map(item => item.id)).size).toBe(referenceCases.length)
    expect(new Set(referenceCases.map(item => item.category)).size).toBe(5)
    for (const item of referenceCases) {
      expect(item.license).toBe('CC-BY-4.0')
      expect(item.repositoryUrl).toMatch(/^https:\/\/github.com\/jamez-bondos\/awesome-gpt4o-images\/tree\/[a-f0-9]{40}\/cases\/\d+$/)
      const file = resolve(__dirname, '../../../../public', item.image.slice(1))
      const bytes = readFileSync(file)
      expect([...bytes.subarray(0, 3)]).toEqual([255, 216, 255])
      expect(statSync(file).size).toBeGreaterThan(1_000)
      expect(statSync(file).size).toBeLessThan(200_000)
      expect(Math.max(item.width, item.height)).toBe(640)
      const attribution = readFileSync(resolve(__dirname, '../../../../public', item.attributionUrl.slice(1)), 'utf8')
      expect(attribution).toContain('license: CC-BY-4.0')
      expect(attribution).toContain(item.imageAuthor)
      expect(item.prompt.zh.trim().length).toBeGreaterThan(0)
      expect(item.prompt.en.trim().length).toBeGreaterThan(0)
    }
  })

  it('shows the images, prompt authors, image authors, license, and original case links', () => {
    const page = render()
    expect(page.findAll('.reference-case')).toHaveLength(Math.min(24, referenceCases.length))
    const first = referenceCases[0]
    const card = page.get(`[data-case-id="${first.id}"]`)
    expect(card.get('img').attributes('src')).toBe(first.image)
    expect(card.get('img').attributes('width')).toBe(String(first.width))
    expect(card.text()).toContain(first.promptAuthor)
    expect(card.text()).toContain(first.imageAuthor)
    expect(card.find(`a[href="${first.repositoryUrl}"]`).exists()).toBe(true)
    expect(card.find('a[href="https://creativecommons.org/licenses/by/4.0/"]').exists()).toBe(true)
  })

  it('preserves the existing curated IDs and order while extending the collection', () => {
    expect(referenceCases.slice(0, 14).map(item => item.id)).toEqual([90, 59, 79, 39, 72, 100, 7, 32, 9, 86, 63, 71, 3, 30].map(id => `gpt4o-${id}`))
  })

  it('combines category filtering with searches across translated titles and full prompts', async () => {
    const page = render()
    const category = page.findAll('.reference-categories button').find(button => button.text() === 'Architecture')!
    await category.trigger('click')
    expect(page.findAll('.reference-case')).toHaveLength(Math.min(24, referenceCases.filter(item => item.category === 'architecture').length))
    await page.get('input[type="search"]').setValue('Cyberpunk')
    expect(page.findAll('.reference-case')).toHaveLength(1)
    expect(page.get('.reference-case').attributes('data-case-id')).toBe('gpt4o-71')
    await page.get('input[type="search"]').setValue('no such reference')
    expect(page.get('[role="status"]').text()).toContain('No matching cases')
    await page.get('.reference-empty button').trigger('click')
    expect(page.findAll('.reference-case')).toHaveLength(Math.min(24, referenceCases.length))
  })

  it('pages through every imported case without duplicates and caps mounted images at 24', async () => {
    const page = render()
    const seen: string[] = []
    expect(page.get('[aria-label="Previous page"]').attributes('disabled')).toBeDefined()
    for (let index = 0; index < Math.ceil(referenceCases.length / 24); index += 1) {
      const cards = page.findAll('.reference-case')
      expect(cards.length).toBeLessThanOrEqual(24)
      seen.push(...cards.map(card => card.attributes('data-case-id')))
      if (index + 1 < Math.ceil(referenceCases.length / 24)) await page.get('[aria-label="Next page"]').trigger('click')
    }
    expect(seen).toEqual(referenceCases.map(item => item.id))
    expect(page.get('[aria-label="Next page"]').attributes('disabled')).toBeDefined()
  })

  it('searches the complete collection and resets pagination when a filter changes', async () => {
    const page = render()
    await page.get('[aria-label="Next page"]').trigger('click')
    const target = referenceCases.at(-1)!
    await page.get('input[type="search"]').setValue(target.title.en)
    const match = page.get(`[data-case-id="${target.id}"]`)
    await match.get('.reference-create').trigger('click')
    expect(page.emitted('create')).toEqual([[{ kind: 'image', prompt: target.prompt.en }]])
    await page.get('input[type="search"]').setValue('')
    expect(page.get('[aria-label="Previous page"]').attributes('disabled')).toBeDefined()
    expect(page.get('.reference-case').attributes('data-case-id')).toBe(referenceCases[0].id)
  })

  it('previews and copies the complete original prompt, not its card excerpt', async () => {
    const page = render()
    const example = referenceCases.find(item => item.id === 'gpt4o-9')!
    await page.get(`[data-case-id="${example.id}"] .reference-case-image`).trigger('click')
    await flushPromises()
    expect(page.get('.reference-prompt').text()).toBe(example.prompt.en)
    const copyButton = page.findAll('.ui-modal-footer button').find(button => button.text() === 'Copy prompt')!
    await copyButton.trigger('click')
    expect(copy).toHaveBeenCalledWith(example.prompt.en)
    expect(page.emitted('create')).toBeUndefined()
  })

  it('creates from a selected prompt without claiming an unavailable legacy model', async () => {
    const page = render()
    const first = referenceCases[0]
    await page.get(`[data-case-id="${first.id}"] .reference-create`).trigger('click')
    expect(page.emitted('create')).toEqual([[{ kind: 'image', prompt: first.prompt.en }]])
    await page.get(`[data-case-id="${first.id}"] .reference-case-image`).trigger('click')
    await flushPromises()
    const createButton = page.findAll('.ui-modal-footer button').find(button => button.text() === 'Create with prompt')!
    await createButton.trigger('click')
    expect(page.emitted('create')).toHaveLength(2)
    expect(page.find('[role="dialog"]').exists()).toBe(false)
  })

  it('uses the complete Chinese prompt in the Chinese interface', async () => {
    i18n.global.locale.value = 'zh'
    const page = render()
    const first = referenceCases[0]
    expect(page.get('.reference-case h3').text()).toBe(first.title.zh)
    await page.get('.reference-create').trigger('click')
    expect(page.emitted('create')).toEqual([[{ kind: 'image', prompt: first.prompt.zh }]])
  })

  it('keeps the case attribution and prompt available when a preview image fails', async () => {
    const page = render()
    await page.get('.reference-case img').trigger('error')
    const card = page.get('.reference-case')
    expect(card.text()).toContain('Preview unavailable')
    expect(card.text()).toContain(referenceCases[0].imageAuthor)
    await card.get('.reference-create').trigger('click')
    expect(page.emitted('create')).toHaveLength(1)
  })
})
