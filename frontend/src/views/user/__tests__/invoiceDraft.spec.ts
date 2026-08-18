import { describe, expect, it, beforeEach } from 'vitest'
import { loadInvoiceDraft, saveInvoiceDraft } from '../invoiceDraft'

describe('invoice draft persistence', () => {
  beforeEach(() => localStorage.clear())

  it('scopes cached information to the authenticated user', () => {
    saveInvoiceDraft(7, { title: 'Acme', tax_number: 'T7', email: 'a@example.com', contact_name: '', contact_phone: '' })
    expect(loadInvoiceDraft(7)).toMatchObject({ title: 'Acme', tax_number: 'T7' })
    expect(loadInvoiceDraft(8, 'b@example.com')).toMatchObject({ title: '', email: 'b@example.com' })
  })

  it('ignores malformed or incomplete values', () => {
    localStorage.setItem('sub2api:invoice-draft:v1:7', '{bad')
    expect(loadInvoiceDraft(7, 'fallback@example.com').email).toBe('fallback@example.com')
  })
})
