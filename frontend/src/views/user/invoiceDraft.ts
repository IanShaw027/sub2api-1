export interface InvoiceDraft {
  title: string
  tax_number: string
  email: string
  contact_name: string
  contact_phone: string
  request_note: string
}

const STORAGE_PREFIX = 'sub2api:invoice-draft:v1:'

function key(userID: number | string): string {
  return `${STORAGE_PREFIX}${String(userID)}`
}

export function loadInvoiceDraft(userID: number | string, fallbackEmail = ''): InvoiceDraft {
  const empty: InvoiceDraft = { title: '', tax_number: '', email: fallbackEmail, contact_name: '', contact_phone: '', request_note: '' }
  try {
    const raw = localStorage.getItem(key(userID))
    if (!raw) return empty
    const parsed = JSON.parse(raw) as Partial<InvoiceDraft>
    return {
      title: typeof parsed.title === 'string' ? parsed.title : '',
      tax_number: typeof parsed.tax_number === 'string' ? parsed.tax_number : '',
      email: typeof parsed.email === 'string' && parsed.email ? parsed.email : fallbackEmail,
      contact_name: typeof parsed.contact_name === 'string' ? parsed.contact_name : '',
      contact_phone: typeof parsed.contact_phone === 'string' ? parsed.contact_phone : '',
      request_note: typeof parsed.request_note === 'string' ? parsed.request_note : '',
    }
  } catch {
    return empty
  }
}

export function saveInvoiceDraft(userID: number | string, draft: InvoiceDraft): void {
  try {
    localStorage.setItem(key(userID), JSON.stringify(draft))
  } catch {
    // Private browsing and storage quota failures should not block invoice applications.
  }
}
