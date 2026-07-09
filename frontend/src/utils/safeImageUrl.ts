export function safeImageUrl(raw: unknown, currentOrigin?: string): string {
  if (typeof raw !== 'string') return ''
  const value = raw.trim()
  if (!value) return ''

  const fallbackOrigin = currentOrigin || (typeof window !== 'undefined' ? window.location.origin : 'http://localhost')

  try {
    const parsed = new URL(value, fallbackOrigin)
    const protocol = parsed.protocol.toLowerCase()
    if (protocol === 'https:') {
      return parsed.toString()
    }
    if ((protocol === 'http:' || protocol === 'https:') && parsed.origin === fallbackOrigin) {
      return parsed.toString()
    }
  } catch {
    return ''
  }

  return ''
}
