<template>
  <div class="acct-usage-cell-wrap" :class="{ 'is-compact': compact }">
    <UsageWindowCell
      v-if="compact"
      class="acct-usage-cell-summary"
      :windows="summaryWindows"
    />
    <span v-if="compact && summaryWindows.length === 0" class="acct-usage-cell-empty">-</span>
    <div class="acct-usage-cell-detail">
  <div ref="rootRef" v-if="showUsageWindows">
    <!-- Anthropic OAuth and Setup Token accounts: fetch real usage data -->
    <template
      v-if="
        account.platform === 'anthropic' &&
        (account.type === 'oauth' || account.type === 'setup-token')
      "
    >
      <!-- Loading state -->
      <div v-if="loading" class="space-y-1.5">
        <!-- OAuth: 3 rows, Setup Token: 1 row -->
        <div class="flex items-center gap-1">
          <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
          <div class="h-1.5 w-8 animate-pulse rounded-full bg-surface-3"></div>
          <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
        </div>
        <template v-if="account.type === 'oauth'">
          <div class="flex items-center gap-1">
            <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
            <div class="h-1.5 w-8 animate-pulse rounded-full bg-surface-3"></div>
            <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
          </div>
          <div class="flex items-center gap-1">
            <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
            <div class="h-1.5 w-8 animate-pulse rounded-full bg-surface-3"></div>
            <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
          </div>
        </template>
      </div>

      <!-- Error state -->
      <div v-else-if="error" class="text-xs text-danger-500">
        {{ error }}
      </div>

      <!-- Usage data -->
      <div v-else-if="usageInfo" class="space-y-1">
        <!-- API error (degraded response) -->
        <div v-if="usageInfo.error" class="text-xs text-warning-text truncate max-w-[200px]" :title="usageInfo.error">
          {{ usageInfo.error }}
        </div>
        <!-- 5h Window -->
        <UsageProgressBar
          v-if="usageInfo.five_hour"
          label="5h"
          :utilization="usageInfo.five_hour.utilization"
          :resets-at="usageInfo.five_hour.resets_at"
          :window-stats="usageInfo.five_hour.window_stats"
          color="indigo"
        />

        <!-- 7d Window (OAuth only) -->
        <UsageProgressBar
          v-if="usageInfo.seven_day"
          label="7d"
          :utilization="usageInfo.seven_day.utilization"
          :resets-at="usageInfo.seven_day.resets_at"
          :predicted-total-cost="usageInfo.seven_day.predicted_total_cost"
          color="emerald"
        />

        <!-- 7d Sonnet Window (OAuth only) -->
        <UsageProgressBar
          v-if="usageInfo.seven_day_sonnet"
          label="7d S"
          :utilization="usageInfo.seven_day_sonnet.utilization"
          :resets-at="usageInfo.seven_day_sonnet.resets_at"
          color="purple"
        />

        <!-- 7d Fable Window (7d_oi) -->
        <UsageProgressBar
          v-if="usageInfo.seven_day_fable"
          label="7d F"
          :utilization="usageInfo.seven_day_fable.utilization"
          :resets-at="usageInfo.seven_day_fable.resets_at"
          color="amber"
        />

        <!-- Passive sampling label + active query button -->
        <div class="flex items-center gap-1.5 mt-0.5">
          <span
            v-if="usageInfo.source === 'passive'"
            class="text-[9px] text-muted italic"
          >
            {{ t('admin.accounts.usageWindow.passiveSampled') }}
          </span>
          <button
            type="button"
            class="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[9px] font-medium text-accent hover:bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] transition-colors"
            :disabled="activeQueryLoading"
            @click="loadActiveUsage"
          >
            <svg
              class="h-2.5 w-2.5"
              :class="{ 'animate-spin': activeQueryLoading }"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
              />
            </svg>
            {{ t('admin.accounts.usageWindow.activeQuery') }}
          </button>
        </div>
      </div>

      <!-- No data yet -->
      <div v-else class="space-y-1">
        <div class="text-xs text-muted">-</div>
      </div>
    </template>

    <!-- OpenAI OAuth accounts: single source from /usage API -->
    <template v-else-if="account.platform === 'openai' && account.type === 'oauth'">
      <div v-if="hasOpenAIUsageFallback" class="space-y-1">
        <UsageProgressBar
          v-if="usageInfo?.five_hour"
          label="5h"
          :utilization="usageInfo.five_hour.utilization"
          :resets-at="usageInfo.five_hour.resets_at"
          :window-stats="usageInfo.five_hour.window_stats"
          :show-now-when-idle="true"
          color="indigo"
        />
        <UsageProgressBar
          v-if="usageInfo?.seven_day"
          label="7d"
          :utilization="usageInfo.seven_day.utilization"
          :resets-at="usageInfo.seven_day.resets_at"
          :window-stats="usageInfo.seven_day.window_stats"
          :predicted-total-cost="usageInfo.seven_day.predicted_total_cost"
          :show-now-when-idle="true"
          color="emerald"
        />
        <!--
          Upstream codex /wham/usage quota query + reset. The local active-sampling
          refresh button is rendered via the pre-actions slot so the user sees a
          single row of related buttons instead of two stacked rows.
        -->
        <OpenAIQuotaResetCell :account="account" @account-updated="handleQuotaResetAccountUpdated">
          <template #pre-actions>
            <button
              type="button"
              class="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-medium text-accent hover:bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] transition-colors disabled:cursor-not-allowed disabled:opacity-50"
              :disabled="activeQueryLoading"
              @click="loadActiveUsage"
            >
              <svg
                class="h-2.5 w-2.5"
                :class="{ 'animate-spin': activeQueryLoading }"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
              {{ t('admin.accounts.usageWindow.activeQuery') }}
            </button>
          </template>
        </OpenAIQuotaResetCell>
      </div>
      <div v-else-if="loading" class="space-y-1.5">
        <div class="flex items-center gap-1">
          <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
          <div class="h-1.5 w-8 animate-pulse rounded-full bg-surface-3"></div>
          <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
        </div>
        <div class="flex items-center gap-1">
          <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
          <div class="h-1.5 w-8 animate-pulse rounded-full bg-surface-3"></div>
          <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
        </div>
      </div>
      <div v-else>
        <div class="text-xs text-muted">-</div>
        <!-- Always allow on-demand upstream quota query, even before local data exists. -->
        <OpenAIQuotaResetCell
          :account="account"
          class="mt-1"
          @account-updated="handleQuotaResetAccountUpdated"
        />
      </div>
    </template>

    <!-- Antigravity OAuth accounts: fetch usage from API -->
    <template v-else-if="account.platform === 'antigravity' && account.type === 'oauth'">
      <AntigravityUsageBlock :account="account" :usage-info="usageInfo" :loading="loading" :error="error" />
    </template>

    <!-- Kiro OAuth accounts: 30-day subscription quota -->
    <template v-else-if="account.platform === 'kiro' && account.type === 'oauth'">
      <KiroUsageBlock :usage-info="usageInfo" :loading="loading" :error="error" />
    </template>

    <!-- Grok OAuth accounts: passive xAI quota headers + local Sub2API usage -->
    <template v-else-if="account.platform === 'grok' && account.type === 'oauth'">
      <GrokUsageBlock :account="account" :usage-info="usageInfo" :loading="loading" :error="error" @probed="handleGrokProbed" />
    </template>

    <!-- CN providers (Kimi / Zhipu / DeepSeek): coding-plan quota or payg balance -->
    <template v-else-if="account.platform === 'kimi' || account.platform === 'zhipu' || account.platform === 'deepseek'">
      <!-- 挂在 CN 平台下的 Ollama Cloud 账号（资格由后端下发 eligible）：用量由
           Ollama 用量窗口负责。这类账号不是国产厂商订阅，CN 的额度/余额探测端点由
           base_url 衍生，对 ollama.com 会被后端出站 URL 白名单拒绝，渲染出来只会
           给用户一行探测报错，因此不再渲染 CN 子单元格与占位符。 -->
      <OllamaCloudUsageCell
        v-if="account.ollama_cloud_usage?.eligible"
        :account="account"
        @updated="handleOllamaCloudUsageUpdated"
      />
      <div v-else class="space-y-1">
        <!-- 子单元格各自按 模式×平台 判定可见；两者都不可见时（智谱 payg 无公开
             余额端点、coding 探测也不适用）才回落到占位符。 -->
        <div
          v-if="!cnQuotaCellVisible && !cnBalanceCellVisible"
          class="text-xs text-muted"
          :title="t('admin.accounts.cnProviders.noBalanceEndpoint')"
        >-</div>
        <CNProviderQuotaCell :account="account" />
        <CNProviderBalanceCell :account="account" />
      </div>
    </template>

    <!-- Gemini platform: show quota + local usage window -->
    <template v-else-if="account.platform === 'gemini'">
      <GeminiUsageBlock
        :account="account"
        :usage-info="usageInfo"
        :loading="loading"
        :error="error"
        :today-stats="todayStats"
        :today-stats-loading="todayStatsLoading"
      />
    </template>

    <!-- Other accounts: no usage window -->
    <template v-else>
      <div class="text-xs text-muted">-</div>
    </template>
  </div>

  <!-- Non-OAuth/Setup-Token accounts -->
  <div ref="rootRef" v-else>
    <!-- Gemini API Key accounts: show quota info -->
    <AccountQuotaInfo v-if="account.platform === 'gemini'" :account="account" />
    <!-- Key/Bedrock accounts: show today stats + optional quota bars -->
    <div v-else class="space-y-1">
      <OllamaCloudUsageCell
        v-if="account.ollama_cloud_usage?.eligible"
        :account="account"
        @updated="handleOllamaCloudUsageUpdated"
      />
      <!-- Today stats row (requests, tokens, cost, user_cost) -->
      <div
        v-if="todayStats"
        class="mb-0.5 flex items-center"
      >
        <div class="flex items-center gap-1.5 text-[9px] text-muted">
          <span class="rounded bg-surface-2 px-1.5 py-0.5">
            {{ formatKeyRequests }} req
          </span>
          <span class="rounded bg-surface-2 px-1.5 py-0.5">
            {{ formatKeyTokens }}
          </span>
          <span class="rounded bg-surface-2 px-1.5 py-0.5" :title="t('usage.accountBilled')">
            A ${{ formatKeyCost }}
          </span>
          <span
            v-if="todayStats.user_cost != null"
            class="rounded bg-surface-2 px-1.5 py-0.5"
            :title="t('usage.userBilled')"
          >
            U ${{ formatKeyUserCost }}
          </span>
        </div>
      </div>
      <!-- Loading skeleton for today stats -->
      <div
        v-else-if="todayStatsLoading"
        class="mb-0.5 flex items-center gap-1"
      >
        <div class="h-3 w-10 animate-pulse rounded bg-surface-3"></div>
        <div class="h-3 w-8 animate-pulse rounded bg-surface-3"></div>
        <div class="h-3 w-12 animate-pulse rounded bg-surface-3"></div>
      </div>

      <!-- API Key accounts with quota limits: show progress bars -->
      <UsageProgressBar
        v-if="quotaDailyBar"
        label="1d"
        :utilization="quotaDailyBar.utilization"
        :resets-at="quotaDailyBar.resetsAt"
        color="indigo"
      />
      <UsageProgressBar
        v-if="quotaWeeklyBar"
        label="7d"
        :utilization="quotaWeeklyBar.utilization"
        :resets-at="quotaWeeklyBar.resetsAt"
        color="emerald"
      />
      <UsageProgressBar
        v-if="quotaTotalBar"
        label="total"
        :utilization="quotaTotalBar.utilization"
        color="purple"
      />

      <!-- No data at all -->
      <div
        v-if="!todayStats && !todayStatsLoading && !hasApiKeyQuota && !account.ollama_cloud_usage?.eligible"
        class="text-xs text-muted"
      >-</div>
    </div>
  </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Account, AccountUsageInfo, WindowStats } from '@/types'
import { buildOpenAIUsageRefreshKey } from '@/utils/accountUsageRefresh'
import { enqueueUsageRequest } from '@/utils/usageLoadQueue'
import UsageProgressBar from './UsageProgressBar.vue'
import AccountQuotaInfo from './AccountQuotaInfo.vue'
import OpenAIQuotaResetCell from './OpenAIQuotaResetCell.vue'
import CNProviderQuotaCell from './CNProviderQuotaCell.vue'
import CNProviderBalanceCell from './CNProviderBalanceCell.vue'
import OllamaCloudUsageCell from './OllamaCloudUsageCell.vue'
import { cnQuotaCellVisible as cnQuotaCellVisibleFn, cnBalanceCellVisible as cnBalanceCellVisibleFn } from './credentialsBuilder'
import UsageWindowCell, { type UsageWindow } from '@/components/common/cells/UsageWindowCell.vue'
import AntigravityUsageBlock from './usage/AntigravityUsageBlock.vue'
import KiroUsageBlock from './usage/KiroUsageBlock.vue'
import GrokUsageBlock from './usage/GrokUsageBlock.vue'
import GeminiUsageBlock from './usage/GeminiUsageBlock.vue'
import { useApiKeyQuota } from './usage/useApiKeyQuota'
import { useAntigravityUsage } from './usage/useAntigravityUsage'
import { useTodayStatsFormat } from './usage/useTodayStatsFormat'

// Module-level cache shared across all AccountUsageCell instances
const _usageCache = new Map<number, { data: AccountUsageInfo; ts: number }>()
const USAGE_CACHE_TTL = 5 * 60 * 1000 // 5 minutes

const props = withDefaults(
  defineProps<{
    account: Account
    todayStats?: WindowStats | null
    todayStatsLoading?: boolean
    manualRefreshToken?: number
    batchedUsage?: AccountUsageInfo | null
    batchedUsageError?: string | null
    batchedUsageLoading?: boolean
    requestBatchedUsage?: ((account: Account, options?: { force?: boolean }) => void) | null
    /** Render a compact 2-line summary (5h/7d or 1d/7d) with the full detail moved into a hover popover.
     * Used by the accounts table (glass-04) to keep row height fixed; other consumers (e.g. the
     * account monitor view) keep the full always-expanded layout by leaving this false. */
    compact?: boolean
  }>(),
  {
    todayStats: null,
    todayStatsLoading: false,
    manualRefreshToken: 0,
    batchedUsage: null,
    batchedUsageError: null,
    batchedUsageLoading: false,
    requestBatchedUsage: null,
    compact: false
  }
)

const emit = defineEmits<{
  'account-updated': [account: Account]
  'usage-loaded': [usage: AccountUsageInfo]
}>()

const { t } = useI18n()
const desktopViewportQuery = '(min-width: 768px)'

const unmounted = ref(false)
onBeforeUnmount(() => { unmounted.value = true })

const loading = ref(false)
const activeQueryLoading = ref(false)
const error = ref<string | null>(null)
const usageInfo = ref<AccountUsageInfo | null>(null)
watch(usageInfo, (usage) => {
  if (usage) emit('usage-loaded', usage)
})
const rootRef = ref<HTMLElement | null>(null)
const isDesktopViewport = ref(
  typeof window === 'undefined' ? true : window.matchMedia(desktopViewportQuery).matches
)
const hasEnteredViewport = ref(false)
const pendingAutoLoad = ref(false)
const pendingAutoLoadSource = ref<'passive' | 'active' | undefined>(undefined)

let desktopViewportMediaQuery: MediaQueryList | null = null
let desktopViewportListener: ((event: MediaQueryListEvent) => void) | null = null
let visibilityObserver: IntersectionObserver | null = null

// Show usage windows for OAuth and Setup Token accounts
const showUsageWindows = computed(() => {
  // Gemini: we can always compute local usage windows from DB logs (simulated quotas).
  if (props.account.platform === 'gemini') return true
  // CN providers: apikey 账号也有滚动用量窗口（coding plan）或余额（payg），
  // 由 CNProviderQuotaCell / CNProviderBalanceCell 自行探测与展示。
  if (
    props.account.platform === 'kimi' ||
    props.account.platform === 'zhipu' ||
    props.account.platform === 'deepseek'
  ) {
    return true
  }
  return props.account.type === 'oauth' || props.account.type === 'setup-token'
})

const shouldFetchUsage = computed(() => {
  if (props.account.platform === 'anthropic') {
    return props.account.type === 'oauth' || props.account.type === 'setup-token'
  }
  if (props.account.platform === 'gemini') {
    return true
  }
  if (props.account.platform === 'kiro') {
    return props.account.type === 'oauth'
  }
  if (props.account.platform === 'antigravity') {
    return props.account.type === 'oauth'
  }
  if (props.account.platform === 'grok') {
    return props.account.type === 'oauth'
  }
  if (props.account.platform === 'openai') {
    return props.account.type === 'oauth'
  }
  return false
})

// CN 供应商子单元格可见性（与 CNProviderQuotaCell / CNProviderBalanceCell 共用
// credentialsBuilder 的单一实现）：都不可见时显示 `-` 占位符。
const cnAccountMode = computed(() => {
  const mode = props.account.credentials?.account_mode
  return typeof mode === 'string' ? mode : ''
})
const cnQuotaCellVisible = computed(() => cnQuotaCellVisibleFn(props.account.platform, cnAccountMode.value))
const cnBalanceCellVisible = computed(() => cnBalanceCellVisibleFn(props.account.platform, cnAccountMode.value))

const isBatchManaged = computed(() => typeof props.requestBatchedUsage === 'function')

const hasOpenAIUsageFallback = computed(() => {
  if (props.account.platform !== 'openai' || props.account.type !== 'oauth') return false
  return !!usageInfo.value?.five_hour || !!usageInfo.value?.seven_day
})

const openAIUsageRefreshKey = computed(() => buildOpenAIUsageRefreshKey(props.account))

const shouldAutoLoadUsageOnMount = computed(() => {
  return shouldFetchUsage.value
})

const shouldLazyLoadOnMobile = computed(() => {
  return shouldFetchUsage.value && !isDesktopViewport.value
})

const { hasApiKeyQuota, quotaDailyBar, quotaWeeklyBar, quotaTotalBar } = useApiKeyQuota(
  computed(() => props.account)
)

const {
  hasAntigravityQuotaFromAPI,
  antigravity3ProUsageFromAPI,
  antigravity3FlashUsageFromAPI,
  antigravity3ImageUsageFromAPI,
  antigravityClaudeUsageFromAPI
} = useAntigravityUsage(
  computed(() => props.account),
  usageInfo
)

const { formatKeyRequests, formatKeyTokens, formatKeyCost, formatKeyUserCost } = useTodayStatsFormat(
  computed(() => props.todayStats)
)

const isAnthropicOAuthOrSetupToken = computed(() => {
  return props.account.platform === 'anthropic' && (props.account.type === 'oauth' || props.account.type === 'setup-token')
})

const requestParentBatchUsage = (options?: { force?: boolean }) => {
  if (!isBatchManaged.value || !shouldFetchUsage.value) return
  props.requestBatchedUsage?.(props.account, options)
}

const syncManagedUsageState = () => {
  if (!isBatchManaged.value) return
  usageInfo.value = props.batchedUsage ?? null
  error.value = props.batchedUsageError ?? null
  loading.value = props.batchedUsageLoading === true
}

const loadUsage = async (options?: { source?: 'passive' | 'active'; bypassCache?: boolean }) => {
  if (!shouldFetchUsage.value) return
  if (isBatchManaged.value) {
    requestParentBatchUsage({ force: options?.bypassCache === true })
    return
  }

  // Check cache
  if (!options?.bypassCache) {
    const cached = _usageCache.get(props.account.id)
    if (cached && Date.now() - cached.ts < USAGE_CACHE_TTL) {
      usageInfo.value = cached.data
      loading.value = false
      return
    }
  }

  loading.value = true
  error.value = null

  try {
		const fetchFn = () => options?.source
			? adminAPI.accounts.getUsage(props.account.id, options.source, options.bypassCache === true)
			: adminAPI.accounts.getUsage(props.account.id)
    const result = await enqueueUsageRequest(props.account, fetchFn)
    if (!unmounted.value) {
      usageInfo.value = result
      _usageCache.set(props.account.id, { data: result, ts: Date.now() })
    }
  } catch (e: any) {
    if (!unmounted.value) {
      error.value = t('common.error')
      console.error('Failed to load usage:', e)
    }
  } finally {
    if (!unmounted.value) loading.value = false
  }
}

const flushPendingAutoLoad = () => {
  if (!pendingAutoLoad.value) return
  const source = pendingAutoLoadSource.value
  pendingAutoLoad.value = false
  pendingAutoLoadSource.value = undefined
  loadUsage({ source }).catch((e) => {
    console.error('Failed to load deferred usage:', e)
  })
}

const requestAutoLoad = (source?: 'passive' | 'active') => {
  if (!shouldFetchUsage.value) return
  if (shouldLazyLoadOnMobile.value && !hasEnteredViewport.value) {
    pendingAutoLoad.value = true
    pendingAutoLoadSource.value = source
    return
  }
  loadUsage({ source }).catch((e) => {
    console.error('Failed to auto load usage:', e)
  })
}

const detachVisibilityObserver = () => {
  visibilityObserver?.disconnect()
  visibilityObserver = null
}

const attachVisibilityObserver = () => {
  detachVisibilityObserver()
  if (!shouldLazyLoadOnMobile.value || hasEnteredViewport.value) return
  if (typeof window === 'undefined' || typeof IntersectionObserver === 'undefined') {
    hasEnteredViewport.value = true
    flushPendingAutoLoad()
    return
  }
  if (!rootRef.value) return

  visibilityObserver = new IntersectionObserver((entries) => {
    if (!entries.some((entry) => entry.isIntersecting)) return
    hasEnteredViewport.value = true
    detachVisibilityObserver()
    flushPendingAutoLoad()
  }, {
    root: null,
    rootMargin: '200px 0px',
    threshold: 0.01
  })
  visibilityObserver.observe(rootRef.value)
}

const loadActiveUsage = async () => {
  activeQueryLoading.value = true
  try {
    usageInfo.value = await adminAPI.accounts.getUsage(props.account.id, 'active', true)
  } catch (e: any) {
    console.error('Failed to load active usage:', e)
  } finally {
    activeQueryLoading.value = false
  }
}

// The probe persists upstream quota state; refresh this cell so its compact
// bars and entitlement status reflect the newly observed snapshot.
const handleGrokProbed = async () => {
  await loadUsage({ source: 'active', bypassCache: true })
}

/** Compact-mode summary rows (glass-04 table). Prefers the Anthropic-style 5h/7d windows,
 * falls back to the API-key style 1d/7d/total quota bars, otherwise renders empty (the
 * compact trigger then shows a plain "-" and the full detail still covers every other case). */
const summaryWindows = computed((): UsageWindow[] => {
  if (usageInfo.value?.five_hour || usageInfo.value?.seven_day) {
    const windows: UsageWindow[] = []
    if (usageInfo.value.five_hour) windows.push({ label: '5h', percent: usageInfo.value.five_hour.utilization })
    if (usageInfo.value.seven_day) windows.push({ label: '7d', percent: usageInfo.value.seven_day.utilization })
    return windows
  }
  if (quotaDailyBar.value || quotaWeeklyBar.value || quotaTotalBar.value) {
    const windows: UsageWindow[] = []
    if (quotaDailyBar.value) windows.push({ label: '1d', percent: quotaDailyBar.value.utilization })
    if (quotaWeeklyBar.value) windows.push({ label: '7d', percent: quotaWeeklyBar.value.utilization })
    if (!quotaDailyBar.value && !quotaWeeklyBar.value && quotaTotalBar.value) {
      windows.push({ label: 'total', percent: quotaTotalBar.value.utilization })
    }
    return windows
  }
  if (hasAntigravityQuotaFromAPI.value) {
    const windows: UsageWindow[] = []
    if (antigravityClaudeUsageFromAPI.value) {
      windows.push({ label: 'Claude', percent: antigravityClaudeUsageFromAPI.value.utilization })
    }
    if (antigravity3ProUsageFromAPI.value) {
      windows.push({ label: 'G3Pro', percent: antigravity3ProUsageFromAPI.value.utilization })
    }
    if (windows.length < 2 && antigravity3FlashUsageFromAPI.value) {
      windows.push({ label: 'Flash', percent: antigravity3FlashUsageFromAPI.value.utilization })
    }
    if (windows.length < 2 && antigravity3ImageUsageFromAPI.value) {
      windows.push({ label: 'Img', percent: antigravity3ImageUsageFromAPI.value.utilization })
    }
    if (windows.length > 0) return windows.slice(0, 2)
  }
  return []
})

const handleQuotaResetAccountUpdated = (account: Account) => {
  emit('account-updated', account)
}

const handleOllamaCloudUsageUpdated = (state: NonNullable<Account['ollama_cloud_usage']>) => {
  emit('account-updated', { ...props.account, ollama_cloud_usage: state })
}

onMounted(() => {
  if (typeof window !== 'undefined') {
    desktopViewportMediaQuery = window.matchMedia(desktopViewportQuery)
    isDesktopViewport.value = desktopViewportMediaQuery.matches
    desktopViewportListener = (event: MediaQueryListEvent) => {
      isDesktopViewport.value = event.matches
    }
    if (typeof desktopViewportMediaQuery.addEventListener === 'function') {
      desktopViewportMediaQuery.addEventListener('change', desktopViewportListener)
    } else {
      desktopViewportMediaQuery.addListener(desktopViewportListener)
    }
  }

  if (isBatchManaged.value) {
    syncManagedUsageState()
    requestParentBatchUsage()
    return
  }

  if (!shouldAutoLoadUsageOnMount.value) return
  const source = isAnthropicOAuthOrSetupToken.value ? 'passive' : undefined
  requestAutoLoad(source)
})

watch(
  () => [props.batchedUsage, props.batchedUsageError, props.batchedUsageLoading, isBatchManaged.value] as const,
  () => {
    syncManagedUsageState()
  },
  { immediate: true, deep: true }
)

watch(isBatchManaged, (managed, wasManaged) => {
  if (managed && !wasManaged) {
    syncManagedUsageState()
    requestParentBatchUsage()
  }
})

watch(
  () => [props.account.id, props.account.platform, props.account.type, isBatchManaged.value] as const,
  ([accountID, platform, accountType, managed], [previousAccountID, previousPlatform, previousAccountType]) => {
    if (
      accountID === previousAccountID &&
      platform === previousPlatform &&
      accountType === previousAccountType
    ) {
      return
    }
    if (!managed || !shouldFetchUsage.value) return
    syncManagedUsageState()
    requestParentBatchUsage()
  },
  { flush: 'post' }
)

watch(openAIUsageRefreshKey, (nextKey, prevKey) => {
  if (!prevKey || nextKey === prevKey) return
  if (props.account.platform !== 'openai' || props.account.type !== 'oauth') return

  if (isBatchManaged.value) {
    requestParentBatchUsage({ force: true })
    return
  }

  _usageCache.delete(props.account.id)
  requestAutoLoad()
})

watch(
  () => props.manualRefreshToken,
  (nextToken, prevToken) => {
    if (nextToken === prevToken) return
    if (!shouldFetchUsage.value) return

    if (isBatchManaged.value) {
      requestParentBatchUsage({ force: true })
      return
    }

    const source = isAnthropicOAuthOrSetupToken.value ? 'passive' : undefined
    _usageCache.delete(props.account.id)
    loadUsage({ source, bypassCache: true }).catch((e) => {
      console.error('Failed to refresh usage after manual refresh:', e)
    })
  }
)

watch(
  [rootRef, shouldLazyLoadOnMobile],
  () => {
    if (shouldLazyLoadOnMobile.value) {
      attachVisibilityObserver()
      return
    }
    detachVisibilityObserver()
  },
  { immediate: true, flush: 'post' }
)

watch(isDesktopViewport, (isDesktop) => {
  if (isDesktop) {
    detachVisibilityObserver()
    hasEnteredViewport.value = true
    flushPendingAutoLoad()
    return
  }
  hasEnteredViewport.value = false
  attachVisibilityObserver()
})

onUnmounted(() => {
  detachVisibilityObserver()
  if (desktopViewportMediaQuery && desktopViewportListener) {
    if (typeof desktopViewportMediaQuery.removeEventListener === 'function') {
      desktopViewportMediaQuery.removeEventListener('change', desktopViewportListener)
    } else {
      desktopViewportMediaQuery.removeListener(desktopViewportListener)
    }
  }
  desktopViewportListener = null
  desktopViewportMediaQuery = null
})
</script>

<style scoped>
/* Non-compact consumers (e.g. the account monitor view) render exactly as before: a plain
 * always-visible block with no positioning changes. */
.acct-usage-cell-detail {
  min-width: 0;
}

.acct-usage-cell-empty {
  font-size: 13px;
  color: var(--muted);
}

/* Compact mode (glass-04 accounts table): show only the 2-line summary inline, and reveal the
 * full detail (every branch above, unchanged) as a floating panel on hover/focus so it never
 * grows the table row. */
.acct-usage-cell-wrap.is-compact {
  position: relative;
  min-width: 0;
}

.acct-usage-cell-wrap.is-compact .acct-usage-cell-detail {
  position: absolute;
  top: 100%;
  left: 0;
  z-index: 30;
  margin-top: 4px;
  width: max-content;
  min-width: 220px;
  max-width: 280px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--surface);
  box-shadow: var(--shadow);
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.12s ease;
}

.acct-usage-cell-wrap.is-compact:hover .acct-usage-cell-detail,
.acct-usage-cell-wrap.is-compact:focus-within .acct-usage-cell-detail {
  opacity: 1;
  pointer-events: auto;
}
</style>
