import { describe, expect, it } from 'vitest'

import { safeImageUrl } from '../safeImageUrl'

describe('safeImageUrl', () => {
  it('allows https images from remote origins', () => {
    expect(safeImageUrl('https://cdn.example.com/avatar.png', 'https://admin.example.com')).toBe('https://cdn.example.com/avatar.png')
  })

  it('allows same-origin http images', () => {
    expect(safeImageUrl('/uploads/avatar.png', 'http://localhost:3000')).toBe('http://localhost:3000/uploads/avatar.png')
    expect(safeImageUrl('http://localhost:3000/uploads/avatar.png', 'http://localhost:3000')).toBe('http://localhost:3000/uploads/avatar.png')
  })

  it('allows persisted inline raster avatars', () => {
    const avatar = 'data:image/webp;base64,Y29tcHJlc3NlZC1hdmF0YXI='
    expect(safeImageUrl(avatar, 'https://admin.example.com')).toBe(avatar)
  })

  it('rejects unsafe or cross-origin non-https URLs', () => {
    expect(safeImageUrl('javascript:alert(1)', 'https://admin.example.com')).toBe('')
    expect(safeImageUrl('data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=', 'https://admin.example.com')).toBe('')
    expect(safeImageUrl('data:image/avif;base64,YXZhdGFy', 'https://admin.example.com')).toBe('')
    expect(safeImageUrl('data:image/png;base64,', 'https://admin.example.com')).toBe('')
    expect(safeImageUrl('data:text/html;base64,PHNjcmlwdD48L3NjcmlwdD4=', 'https://admin.example.com')).toBe('')
    expect(safeImageUrl('data:image/png;base64,not valid base64', 'https://admin.example.com')).toBe('')
    expect(safeImageUrl('http://cdn.example.com/avatar.png', 'https://admin.example.com')).toBe('')
  })
})
