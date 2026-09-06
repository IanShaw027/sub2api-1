import type { ApiKey, PublicSettings } from '@/types'
import { buildCcSwitchImportDeeplink, type CcSwitchClientType } from '@/utils/ccswitchImport'

// CC Switch deep-link construction, split out of KeysView.vue to keep the
// view under the frontend-health-cleanup 6.4 line cap.

export function openCcSwitchDeeplink(
  row: ApiKey,
  clientType: CcSwitchClientType,
  publicSettings: PublicSettings | null,
  onFailure: () => void
) {
  const baseUrl = publicSettings?.api_base_url || window.location.origin
  const platform = row.group?.platform || 'anthropic'

  const usageScript = `({
    request: {
      url: "{{baseUrl}}/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`
  const providerName = (publicSettings?.site_name || 'sub2api').trim() || 'sub2api'
  const deeplink = buildCcSwitchImportDeeplink({
    baseUrl,
    platform,
    clientType,
    providerName,
    apiKey: row.key,
    usageScript
  })

  try {
    window.open(deeplink, '_self')
    // Check if the protocol handler worked by detecting if we're still focused
    setTimeout(() => {
      if (document.hasFocus()) {
        // Still focused means the protocol handler likely failed
        onFailure()
      }
    }, 100)
  } catch (error) {
    onFailure()
  }
}

export const formatResetCountdown = (resetAt: string | null, now: Date, resetNowLabel: string): string => {
  if (!resetAt) return ''
  const diff = new Date(resetAt).getTime() - now.getTime()
  if (diff <= 0) return resetNowLabel
  const days = Math.floor(diff / 86400000)
  const hours = Math.floor((diff % 86400000) / 3600000)
  const mins = Math.floor((diff % 3600000) / 60000)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}
