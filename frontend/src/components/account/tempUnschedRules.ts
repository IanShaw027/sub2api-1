export interface TempUnschedRuleForm {
  error_code: number | null
  keywords: string
  duration_minutes: number | null
  description: string
}

export interface TempUnschedRulePayload {
  error_code: number
  keywords: string[]
  duration_minutes: number
  description: string
}

export interface BuildTempUnschedRulesResult {
  rules: TempUnschedRulePayload[]
  invalid: boolean
}

const toPositiveNumber = (value: unknown): number | null => {
  const num = typeof value === 'string' ? Number(value.trim()) : Number(value)
  if (!Number.isFinite(num) || num <= 0) {
    return null
  }
  return num
}

export const splitTempUnschedKeywords = (value: string): string[] => {
  return String(value || '')
    .split(/[,;]/)
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
}

export const formatTempUnschedKeywords = (value: unknown): string => {
  if (Array.isArray(value)) {
    return value
      .filter((item): item is string => typeof item === 'string')
      .map((item) => item.trim())
      .filter((item) => item.length > 0)
      .join(', ')
  }
  if (typeof value === 'string') {
    return value
  }
  return ''
}

const toTempUnschedRulePayload = (rule: TempUnschedRuleForm): TempUnschedRulePayload => ({
  error_code: Number(rule.error_code),
  keywords: splitTempUnschedKeywords(rule.keywords),
  duration_minutes: Number(rule.duration_minutes),
  description: String(rule.description || '').trim()
})

const isValidTempUnschedRulePayload = (rule: TempUnschedRulePayload): boolean =>
  Number.isInteger(rule.error_code) &&
  rule.error_code >= 100 &&
  rule.error_code <= 599 &&
  Number.isInteger(rule.duration_minutes) &&
  rule.duration_minutes > 0

// buildTempUnschedRulesResult 把表单行转成持久化 payload，并显式返回是否存在坏行。
// keywords 允许为空：表示"仅凭错误码匹配"（用于 524/522/521/520 等 body 为空的上游码）。
export const buildTempUnschedRulesResult = (rules: TempUnschedRuleForm[]): BuildTempUnschedRulesResult => {
  const out: TempUnschedRulePayload[] = []
  let invalid = false

  for (const rule of rules) {
    const payload = toTempUnschedRulePayload(rule)
    if (!isValidTempUnschedRulePayload(payload)) {
      invalid = true
      continue
    }
    out.push(payload)
  }

  return { rules: out, invalid }
}

// buildTempUnschedRules 保持旧调用点兼容；保存入口应使用 buildTempUnschedRulesResult 避免部分保存。
export const buildTempUnschedRules = (rules: TempUnschedRuleForm[]): TempUnschedRulePayload[] =>
  buildTempUnschedRulesResult(rules).rules

export const buildTempUnschedRulesValidationPayload = (rules: TempUnschedRuleForm[]): TempUnschedRulePayload[] =>
  rules.map(toTempUnschedRulePayload)

export const loadTempUnschedRules = (credentials?: Record<string, unknown>): TempUnschedRuleForm[] => {
  const rawRules = credentials?.temp_unschedulable_rules
  if (!Array.isArray(rawRules)) {
    return []
  }

  return rawRules.map((rule) => {
    const entry = (rule && typeof rule === 'object' ? rule : {}) as Record<string, unknown>
    return {
      error_code: toPositiveNumber(entry.error_code),
      keywords: formatTempUnschedKeywords(entry.keywords),
      duration_minutes: toPositiveNumber(entry.duration_minutes),
      description: typeof entry.description === 'string' ? entry.description : ''
    }
  })
}
