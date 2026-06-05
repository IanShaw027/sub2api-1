export type ResponseRewriteMatchMode = 'any' | 'all'

export interface ResponseRewriteRuleForm {
  status_code: number | null
  keywords: string
  match_mode: ResponseRewriteMatchMode
  response_message: string
  description: string
}

export interface ResponseRewriteRulePayload {
  status_code?: number
  keywords: string[]
  match_mode: ResponseRewriteMatchMode
  response_message: string
  description: string
}

const normalizeMatchMode = (value: unknown): ResponseRewriteMatchMode => {
  return value === 'all' ? 'all' : 'any'
}

const toOptionalStatusCode = (value: unknown): number | null => {
  const parsed = typeof value === 'string' ? Number(value.trim()) : Number(value)
  if (!Number.isFinite(parsed) || !Number.isInteger(parsed)) {
    return null
  }
  if (parsed < 100 || parsed > 599) {
    return null
  }
  return parsed
}

const normalizeKeywordsArray = (value: unknown): string[] => {
  if (!Array.isArray(value)) {
    return []
  }
  return value
    .map((item) => String(item ?? '').trim())
    .filter((item) => item.length > 0)
}

export const splitResponseRewriteKeywords = (value: string): string[] => {
  return String(value || '')
    .split(/[,;]/)
    .map((item) => item.trim())
    .filter((item) => item.length > 0)
}

export const hasResponseRewriteRuleInputs = (rules: ResponseRewriteRuleForm[]): boolean => {
  return rules.some((rule) => {
    return rule.status_code != null ||
      String(rule.keywords || '').trim().length > 0 ||
      String(rule.response_message || '').trim().length > 0 ||
      String(rule.description || '').trim().length > 0
  })
}

export const buildResponseRewriteRules = (rules: ResponseRewriteRuleForm[]): ResponseRewriteRulePayload[] => {
  const out: ResponseRewriteRulePayload[] = []

  for (const rule of rules) {
    const statusCode = toOptionalStatusCode(rule.status_code)
    const keywords = splitResponseRewriteKeywords(rule.keywords)
    const matchMode = normalizeMatchMode(rule.match_mode)
    const responseMessage = String(rule.response_message || '').trim()
    const description = String(rule.description || '').trim()

    if (!responseMessage) {
      continue
    }
    if (rule.match_mode !== 'any' && rule.match_mode !== 'all') {
      continue
    }
    if (statusCode == null && keywords.length === 0) {
      continue
    }

    const payload: ResponseRewriteRulePayload = {
      keywords,
      match_mode: matchMode,
      response_message: responseMessage,
      description
    }
    if (statusCode != null) {
      payload.status_code = statusCode
    }
    out.push(payload)
  }

  return out
}

export const loadResponseRewriteRules = (credentials?: Record<string, unknown>): ResponseRewriteRuleForm[] => {
  const rawRules = credentials?.response_rewrite_rules
  if (!Array.isArray(rawRules)) {
    return []
  }

  return rawRules.map((rule) => {
    const entry = (rule && typeof rule === 'object' ? rule : {}) as Record<string, unknown>
    return {
      status_code: toOptionalStatusCode(entry.status_code),
      keywords: normalizeKeywordsArray(entry.keywords).join(', '),
      match_mode: normalizeMatchMode(entry.match_mode),
      response_message: typeof entry.response_message === 'string' ? entry.response_message : '',
      description: typeof entry.description === 'string' ? entry.description : ''
    }
  })
}
