import { describe, expect, it } from 'vitest'

import { adminPaymentAPI } from '@/api/admin/payment'

describe('admin payment api', () => {
  it('does not expose removed admin payment channel endpoints', () => {
    expect('getChannels' in adminPaymentAPI).toBe(false)
    expect('createChannel' in adminPaymentAPI).toBe(false)
    expect('updateChannel' in adminPaymentAPI).toBe(false)
    expect('deleteChannel' in adminPaymentAPI).toBe(false)
  })
})
