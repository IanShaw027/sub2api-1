/**
 * 验证并规范化 URL
 * 默认只接受绝对 URL（以 http:// 或 https:// 开头），可按需允许相对路径
 * @param value 用户输入的 URL
 * @returns 规范化后的 URL，如果无效则返回空字符串
 */
type SanitizeOptions = {
  allowRelative?: boolean
  allowDataUrl?: boolean
}

const inlineRasterImagePattern = /^data:image\/(?:png|jpe?g|gif|webp);base64,(?:[a-z0-9+/]{4})*(?:[a-z0-9+/]{4}|[a-z0-9+/]{2}==|[a-z0-9+/]{3}=)$/i

function normalizeBasePath(baseUrl: string): string {
  const trimmed = (baseUrl || '/').trim()
  if (!trimmed || trimmed === '/') {
    return '/'
  }
  return `/${trimmed.replace(/^\/+|\/+$/g, '')}/`
}

function normalizeAppTarget(path: string): string {
  const trimmed = path.trim()
  if (!trimmed) {
    return '/'
  }
  return trimmed.startsWith('/') ? trimmed : `/${trimmed}`
}

export function buildAppPath(path: string, baseUrl: string = import.meta.env.BASE_URL): string {
  const normalizedBase = normalizeBasePath(baseUrl)
  const normalizedPath = normalizeAppTarget(path)
  if (normalizedBase === '/') {
    return normalizedPath
  }
  return `${normalizedBase.slice(0, -1)}${normalizedPath}`
}

export function buildAppAbsoluteUrl(
  path: string,
  origin: string,
  baseUrl: string = import.meta.env.BASE_URL
): string {
  return new URL(buildAppPath(path, baseUrl), origin).toString()
}

export function sanitizeUrl(value: string, options: SanitizeOptions = {}): string {
  const trimmed = value.trim()
  if (!trimmed) {
    return ''
  }

  if (options.allowRelative && trimmed.startsWith('/') && !trimmed.startsWith('//')) {
    return trimmed
  }

  // 仅允许安全的内联 raster 图片；拒绝 SVG/AVIF 和 padding 错误的 base64。
  if (options.allowDataUrl && inlineRasterImagePattern.test(trimmed)) {
    return trimmed
  }

  // 只接受绝对 URL，不使用 base URL 来避免相对路径被解析为当前域名
  // 检查是否以 http:// 或 https:// 开头
  if (!trimmed.match(/^https?:\/\//i)) {
    return ''
  }

  try {
    const parsed = new URL(trimmed)
    const protocol = parsed.protocol.toLowerCase()
    if (protocol !== 'http:' && protocol !== 'https:') {
      return ''
    }
    return parsed.toString()
  } catch {
    return ''
  }
}
