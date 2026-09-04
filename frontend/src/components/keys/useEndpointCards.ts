import { computed, type Ref, type ComputedRef } from 'vue'
import type { PublicSettings } from '@/types'
import type { StatusBadgeTone } from '@/components/ui/types'

// Extracted out of KeysView.vue to keep the view under the
// frontend-health-cleanup 6.4 file-size warning threshold.

export interface EndpointCardEntry {
  url: string
  label: string
  description?: string
  badge?: string
  badgeTone?: StatusBadgeTone
}

export function useEndpointCards(
  publicSettings: Ref<PublicSettings | null>,
  t: (key: string) => string
): { endpointCards: ComputedRef<EndpointCardEntry[]> } {
  // Custom endpoints don't carry a protocol-description field from the API;
  // derive the prototype's "which clients use this" hint from the endpoint
  // name when it looks OpenAI-compatible, and fall back to the mock/API
  // description for any other custom endpoint shape (avoids feature loss).
  const describeCustomEndpoint = (endpoint: { name: string; description?: string }) => {
    if (/openai/i.test(endpoint.name)) {
      return t('keys.endpoints.openaiDesc')
    }
    return endpoint.description || undefined
  }

  const endpointCards = computed<EndpointCardEntry[]>(() => {
    const cards: EndpointCardEntry[] = []
    if (publicSettings.value?.api_base_url) {
      cards.push({
        url: publicSettings.value.api_base_url,
        label: t('keys.endpoints.titleAnthropic'),
        description: t('keys.endpoints.anthropicDesc'),
        badge: t('keys.endpoints.default'),
        badgeTone: 'accent'
      })
    }
    for (const endpoint of publicSettings.value?.custom_endpoints || []) {
      cards.push({
        url: endpoint.endpoint,
        label: `${t('keys.endpoints.titleCustomPrefix')}${endpoint.name}`,
        description: describeCustomEndpoint(endpoint),
        badge: t('keys.endpoints.custom'),
        badgeTone: 'muted'
      })
    }
    return cards
  })

  return { endpointCards }
}
