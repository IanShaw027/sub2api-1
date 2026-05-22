import { oauthAffiliatePayload } from '@/utils/oauthAffiliate'

export interface DingTalkPendingEmailCompletionPayload {
  email: string
  password: string
  verifyCode?: string
  invitationCode?: string
}

export interface DingTalkPendingAuthSessionContext {
  provider?: string
  adopt_display_name?: boolean
  adopt_avatar?: boolean
}

export function buildDingTalkPendingEmailCompletionPayload(
  payload: DingTalkPendingEmailCompletionPayload,
  pendingSession: DingTalkPendingAuthSessionContext | null | undefined,
  affiliateCode?: string,
): Record<string, unknown> {
  const requestPayload: Record<string, unknown> = {
    email: payload.email,
    password: payload.password,
    verify_code: payload.verifyCode || undefined,
    invitation_code: payload.invitationCode || undefined,
    ...oauthAffiliatePayload(affiliateCode),
  }

  if (pendingSession?.provider === 'dingtalk') {
    if (typeof pendingSession.adopt_display_name === 'boolean') {
      requestPayload.adopt_display_name = pendingSession.adopt_display_name
    }
    if (typeof pendingSession.adopt_avatar === 'boolean') {
      requestPayload.adopt_avatar = pendingSession.adopt_avatar
    }
  }

  return requestPayload
}
