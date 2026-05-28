import { describe, expect, it } from 'vitest'

import { buildDingTalkPendingEmailCompletionPayload } from '../dingtalkEmailCompletionPayload'

describe('buildDingTalkPendingEmailCompletionPayload', () => {
  it('keeps affiliate code and adoption defaults for DingTalk pending sessions', () => {
    expect(buildDingTalkPendingEmailCompletionPayload(
      {
        email: 'fresh@example.com',
        password: 'secret-123',
        verifyCode: '246810',
        invitationCode: 'invite-1',
      },
      {
        provider: 'dingtalk',
        adopt_display_name: true,
        adopt_avatar: false,
      },
      'aff-123',
    )).toEqual({
      email: 'fresh@example.com',
      password: 'secret-123',
      verify_code: '246810',
      invitation_code: 'invite-1',
      aff_code: 'aff-123',
      adopt_display_name: true,
      adopt_avatar: false,
    })
  })
})
