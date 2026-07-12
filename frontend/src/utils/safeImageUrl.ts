const inlineRasterImagePattern = /^data:image\/(?:png|jpe?g|gif|webp);base64,(?:[a-z0-9+/]{4})*(?:[a-z0-9+/]{4}|[a-z0-9+/]{2}==|[a-z0-9+/]{3}=)$/i

export function safeImageUrl(raw: unknown, currentOrigin?: string): string {
  if (typeof raw !== 'string') return ''
  const value = raw.trim()
  if (!value) return ''

  // Inline raster images are a supported persisted-avatar format when the
  // backend media store is disabled. Keep script-capable SVG/data payloads out.
  if (inlineRasterImagePattern.test(value)) {
    return value
  }

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
