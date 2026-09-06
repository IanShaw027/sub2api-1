import { describe, expect, it } from 'vitest'
import { chartDataSchema, chatTools, executeChatTool } from '../chatTools'

const valid = () => ({ xKey: 'month', data: [{ month: 'Jan', sales: 3 }, { month: 'Feb', sales: 8 }], series: [{ key: 'sales', name: 'Sales' }] })

describe('local chart tool', () => {
  it('returns validated chart data and a matching protocol tool result', async () => {
    const result = await executeChatTool({ id: 'call-1', name: 'chart', arguments: valid() })
    expect(result).toMatchObject({ toolCallId: 'call-1', name: 'chart', data: { type: 'chart', chart: valid() } })
    expect(JSON.parse(result.content)).toEqual(valid())
    expect(result.isError).toBeUndefined()
    expect(chatTools.map(tool => tool.name)).toEqual(['chart'])
  })

  it('does not execute unknown tools or return made-up weather', async () => {
    const result = await executeChatTool({ id: 'call-2', name: 'weather', arguments: { location: 'Taipei' } })
    expect(result.isError).toBe(true)
    expect(result.data).toBeUndefined()
  })

  it.each([
    { ...valid(), data: [] },
    { ...valid(), data: [{ sales: 4 }] },
    { ...valid(), data: [{ month: 'Jan', sales: 'unknown' }] },
    { ...valid(), data: [{ month: 'Jan', sales: Infinity }] },
    { ...valid(), data: Array.from({ length: 501 }, () => ({ month: 'Jan', sales: 1 })) },
    { ...valid(), series: [{ key: 'sales', name: 'Sales', color: 'url(https://example.test)' }] },
    { ...valid(), series: [{ key: 'sales', name: 'A' }, { key: 'sales', name: 'B' }] },
    { ...valid(), xKey: '__proto__' },
  ])('rejects malformed or unbounded chart input', async input => {
    expect(chartDataSchema.safeParse(input).success).toBe(false)
    const result = await executeChatTool({ id: 'call-bad', name: 'chart', arguments: input })
    expect(result.isError).toBe(true)
    expect(result.data).toBeUndefined()
  })
})
