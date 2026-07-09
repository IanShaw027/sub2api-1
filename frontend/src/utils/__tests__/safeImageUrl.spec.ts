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

  it('rejects unsafe or cross-origin non-https URLs', () => {
    expect(safeImageUrl('javascript:alert(1)', 'https://admin.example.com')).toBe('')
    expect(safeImageUrl('data:image/png;base64,abc', 'https://admin.example.com')).toBe('')
    expect(safeImageUrl('http://cdn.example.com/avatar.png', 'https://admin.example.com')).toBe('')
  })
})
