import { z } from 'zod'
import type { ChatToolCall, ChatToolDefinition, ChatToolResult } from './localChat'

const key = z.string().min(1).max(80).refine(value => !['__proto__', 'prototype', 'constructor'].includes(value))
const label = z.string().max(200)

// Adapted from chat-vue's chart tool; no executable code or remote tools run here.
export const chartDataSchema = z.object({
  title: label.optional(),
  xKey: key,
  xLabel: label.optional(),
  yLabel: label.optional(),
  data: z.array(z.record(z.union([z.string().max(500), z.number().finite()]))).min(1).max(500),
  series: z.array(z.object({
    key,
    name: label.min(1),
    color: z.string().regex(/^#[\da-f]{6}$/i).optional(),
  })).min(1).max(8),
}).superRefine((chart, context) => {
  if (new Set(chart.series.map(series => series.key)).size !== chart.series.length) {
    context.addIssue({ code: z.ZodIssueCode.custom, message: 'Series keys must be unique', path: ['series'] })
  }
  chart.data.forEach((point, index) => {
    if (!Object.prototype.hasOwnProperty.call(point, chart.xKey)) {
      context.addIssue({ code: z.ZodIssueCode.custom, message: 'Every point needs an x-axis value', path: ['data', index] })
    }
    for (const series of chart.series) {
      if (typeof point[series.key] !== 'number') {
        context.addIssue({ code: z.ZodIssueCode.custom, message: 'Every series value must be a finite number', path: ['data', index, series.key] })
      }
    }
  })
})

export type ChartData = z.infer<typeof chartDataSchema>
export interface ChartToolOutput { type: 'chart'; chart: ChartData }

export const chatTools: ChatToolDefinition[] = [{
  name: 'chart',
  description: 'Draw a line chart from the data supplied in the conversation. Use numeric series values and do not invent measurements or claim unverified data is real.',
  inputSchema: {
    type: 'object',
    properties: {
      title: { type: 'string', maxLength: 200 },
      xKey: { type: 'string', minLength: 1, maxLength: 80 },
      xLabel: { type: 'string', maxLength: 200 },
      yLabel: { type: 'string', maxLength: 200 },
      data: { type: 'array', minItems: 1, maxItems: 500, items: { type: 'object', additionalProperties: { anyOf: [{ type: 'string' }, { type: 'number' }] } } },
      series: { type: 'array', minItems: 1, maxItems: 8, items: {
        type: 'object', properties: {
          key: { type: 'string', minLength: 1, maxLength: 80 },
          name: { type: 'string', minLength: 1, maxLength: 200 },
          color: { type: 'string', pattern: '^#[0-9a-fA-F]{6}$' },
        }, required: ['key', 'name'], additionalProperties: false,
      } },
    },
    required: ['xKey', 'data', 'series'],
    additionalProperties: false,
  },
}]

export async function executeChatTool(call: ChatToolCall): Promise<ChatToolResult> {
  const identity = { toolCallId: call.id, name: call.name }
  if (call.name !== 'chart') return { ...identity, content: 'This tool is not available.', isError: true }
  const result = chartDataSchema.safeParse(call.arguments)
  if (!result.success) {
    return { ...identity, content: JSON.stringify({ error: 'Invalid chart data', issues: result.error.issues.map(issue => ({ path: issue.path, message: issue.message })) }), isError: true }
  }
  const data: ChartToolOutput = { type: 'chart', chart: result.data }
  return { ...identity, content: JSON.stringify(result.data), data }
}
