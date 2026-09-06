import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { compileStyle, parse } from 'vue/compiler-sfc'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import MessageContent from '../components/MessageContent.vue'

const copy = vi.hoisted(() => vi.fn(async () => true))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: copy }) }))
vi.mock('../api', () => ({ default: { extractMessageText: (value: unknown) => typeof value === 'string' ? value : '' } }))

describe('safe message code rendering', () => {
  let wrapper: VueWrapper | undefined
  const i18n = createI18n({ legacy: false, locale: 'en', messages: { en: {} } })
  function render(content: string, role: 'assistant' | 'user' = 'assistant') {
    wrapper = mount(MessageContent, { props: { role, content }, attachTo: document.body, global: { plugins: [i18n], stubs: { TokenStats: true } } })
    return wrapper
  }
  beforeEach(() => { copy.mockClear(); i18n.global.locale.value = 'en' })
  afterEach(() => { wrapper?.unmount(); document.body.innerHTML = '' })

  it('highlights JavaScript with real grammar tokens and both theme palettes', async () => {
    const page = render('```js\nconst answer = "hello"\n```')
    await vi.waitFor(() => expect(page.find('pre.shiki').exists()).toBe(true), { timeout: 10000 })
    expect(page.get('code').text()).toBe('const answer = "hello"')
    const tokens = page.findAll('code span[style]')
    expect(tokens.length).toBeGreaterThan(1)
    expect(tokens.some(token => token.attributes('style').includes('--shiki-light:'))).toBe(true)
    expect(tokens.some(token => token.attributes('style').includes('--shiki-dark:'))).toBe(true)
    await page.get('[aria-label="Copy code"]').trigger('click')
    await flushPromises()
    expect(copy).toHaveBeenCalledWith('const answer = "hello"\n')
    expect(page.find('[aria-label="Copied"]').exists()).toBe(true)
  })

  it('copies unknown-language code as text without interpreting embedded HTML', async () => {
    const page = render('```unregistered-language\n<img src=x onerror=alert(1)>\n```')
    await flushPromises()
    expect(page.find('img').exists()).toBe(false)
    expect(page.get('code').text()).toBe('<img src=x onerror=alert(1)>')
    await page.get('[aria-label="Copy code"]').trigger('click')
    expect(copy).toHaveBeenCalledWith('<img src=x onerror=alert(1)>\n')
  })

  it('sanitizes raw Markdown HTML and enforces external link isolation', async () => {
    const page = render('<script>alert(1)</script><img src=x onerror="alert(1)"><button onclick="alert(1)">bad</button><p style="position:fixed">Text</p>\n\n[unsafe](javascript:alert(1)) [safe](https://example.com)')
    await flushPromises()
    expect(page.find('script, button, [onerror], [onclick], [style]').exists()).toBe(false)
    expect(page.find('a[href^="javascript:"]').exists()).toBe(false)
    expect(page.get('a[href="https://example.com"]').attributes()).toMatchObject({ target: '_blank', rel: 'noopener noreferrer' })
  })

  it('preserves nested Markdown code blocks and gives each its own copy button', async () => {
    const page = render('> ```text\n> first\n> ```\n\n- Item\n\n  ```text\n  second\n  ```')
    await flushPromises()
    expect(page.find('blockquote .studio-code-block').exists()).toBe(true)
    expect(page.find('li .studio-code-block').exists()).toBe(true)
    const buttons = page.findAll('[aria-label="Copy code"]')
    expect(buttons).toHaveLength(2)
    await buttons[1]!.trigger('click')
    expect(copy).toHaveBeenCalledWith('second\n')
  })

  it('updates unfinished streaming fences without retaining stale controls or code', async () => {
    const page = render('```js\nconst old = 1')
    await page.setProps({ streaming: true, content: '```js\nconst newest = 2\n```' })
    await vi.waitFor(() => expect(page.find('pre.shiki').exists()).toBe(true), { timeout: 10000 })
    expect(page.get('code').text()).toBe('const newest = 2')
    expect(page.findAll('[aria-label="Copy code"]')).toHaveLength(1)
    await page.setProps({ content: 'No code now.' })
    await flushPromises()
    expect(page.find('pre').exists()).toBe(false)
    expect(page.find('[aria-label="Copy code"]').exists()).toBe(false)
    expect(page.text()).toContain('No code now.')
  })

  it('renders user input as literal text and removes assistant code controls on role changes', async () => {
    const page = render('```text\nhello\n```')
    await flushPromises()
    await page.setProps({ role: 'user', content: '<img src=x onerror=alert(1)>' })
    await flushPromises()
    expect(page.find('img, pre, button').exists()).toBe(false)
    expect(page.get('.studio-message-text').text()).toBe('<img src=x onerror=alert(1)>')
  })

  it('keeps large code blocks readable and copyable without blocking on highlighting', async () => {
    const source = 'x'.repeat(50001)
    const page = render(`\`\`\`js\n${source}\n\`\`\``)
    await flushPromises()
    expect(page.find('pre.shiki').exists()).toBe(false)
    await page.get('[aria-label="Copy code"]').trigger('click')
    expect(copy).toHaveBeenCalledWith(`${source}\n`)
  })

  it('localizes code copy actions without rebuilding the message', async () => {
    const page = render('```text\nhello\n```')
    await flushPromises()
    i18n.global.locale.value = 'zh'
    await flushPromises()
    expect(page.get('[aria-label="复制代码"]').exists()).toBe(true)
  })

  it('compiles dark theme selectors onto code tokens rather than the document root', () => {
    const filename = resolve(__dirname, '../components/MessageContent.vue')
    const { descriptor } = parse(readFileSync(filename, 'utf8'))
    const result = compileStyle({ source: descriptor.styles[0]!.content, filename, id: 'data-v-test', scoped: true })
    expect(result.errors).toEqual([])
    expect(result.code).toContain("html[data-theme='glass-dark'] .studio-markdown .shiki span")
    expect(result.code).not.toMatch(/html\[data-theme='glass-dark'\]\s*\{/)
  })
})
