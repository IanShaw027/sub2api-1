<template>
  <div ref="rootRef" v-if="showUsageWindows">
    <!-- Anthropic OAuth and Setup Token accounts: fetch real usage data -->
    <template
      v-if="
        account.platform === 'anthropic' &&
        (account.type === 'oauth' || account.type === 'setup-token' || account.type === 'service_account')
      "
    >
      <!-- Loading state -->
      <div v-if="loading" class="space-y-1.5">
        <!-- OAuth: 3 rows, Setup Token: 1 row -->
        <div class="flex items-center gap-1">
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-1.5 w-8 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
        </div>
        <template v-if="account.type === 'oauth'">
          <div class="flex items-center gap-1">
            <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
            <div class="h-1.5 w-8 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
            <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
          </div>
          <div class="flex items-center gap-1">
            <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
            <div class="h-1.5 w-8 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
            <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
          </div>
        </template>
      </div>

      <!-- Error state -->
      <div v-else-if="error" class="text-xs text-red-500">
        {{ error }}
      </div>

      <!-- Usage data -->
      <div v-else-if="usageInfo" class="space-y-1">
        <!-- API error (degraded response) -->
        <div v-if="usageInfo.error" class="text-xs text-amber-600 dark:text-amber-400 truncate max-w-[200px]" :title="usageInfo.error">
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

        <!-- Passive sampling label + active query button -->
        <div class="flex items-center gap-1.5 mt-0.5">
          <span
            v-if="usageInfo.source === 'passive'"
            class="text-[9px] text-gray-400 dark:text-gray-500 italic"
          >
            {{ t('admin.accounts.usageWindow.passiveSampled') }}
          </span>
          <button
            type="button"
            class="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[9px] font-medium text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900/30 transition-colors"
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
        <div class="text-xs text-gray-400">-</div>
      </div>
    </template>

    <!-- OpenAI OAuth accounts: single source from /usage API -->
    <template v-else-if="account.platform === 'openai' && account.type === 'oauth'">
      <div v-if="hasOpenAIUsageContent" class="space-y-1">
        <div v-if="showOpenAIResponseUsageBars && openAIResponseUsageBars.length" class="space-y-1">
          <UsageProgressBar
            v-for="item in openAIResponseUsageBars"
            :key="item.key"
            :label="item.label"
            :utilization="item.progress.utilization"
            :resets-at="item.progress.resets_at"
            :window-stats="item.progress.window_stats"
            :show-now-when-idle="true"
            :color="item.color"
          />
        </div>
        <div v-if="openAIImageUsageSummary.length" class="flex items-center gap-1 text-[10px]">
          <span class="shrink-0 font-medium text-amber-600 dark:text-amber-400">img:</span>
          <template v-for="(item, idx) in openAIImageUsageSummary" :key="item.label">
            <span v-if="idx > 0" class="text-gray-300 dark:text-gray-600">|</span>
            <span class="text-gray-600 dark:text-gray-400">
              {{ item.label }} {{ formatCompactNumber(item.requests, { allowBillions: false }) }}req ${{ item.userCost.toFixed(2) }}
            </span>
          </template>
        </div>
      </div>

      <div v-else-if="loading" class="space-y-1.5">
        <div class="flex items-center gap-1">
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-1.5 w-8 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
        </div>
        <div class="flex items-center gap-1">
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-1.5 w-8 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
        </div>
      </div>
      <div v-else class="text-xs text-gray-400">-</div>

      <!-- Codex invite reset: inline query / reset + invite badge (always available for OpenAI OAuth, even without usage data) -->
      <div class="flex flex-wrap items-center gap-1.5 mt-0.5">
        <button
          type="button"
          class="inline-flex items-center gap-0.5 rounded bg-emerald-50 px-1.5 py-0.5 text-[9px] font-medium text-emerald-700 hover:bg-emerald-100 dark:bg-emerald-900/30 dark:text-emerald-300 dark:hover:bg-emerald-900/50 transition-colors"
          :title="t('admin.accounts.inviteResetOpenDialog')"
          @click="openInviteResetModal"
        >
          <Icon name="gift" size="xs" />
          {{ t('admin.accounts.inviteResetCountShort') }}
          <span v-if="effectiveInviteResetStatus" class="tabular-nums">{{ inviteResetAvailableCount }}</span>
        </button>
        <button
          type="button"
          class="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[9px] font-medium text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900/30 transition-colors"
          :disabled="activeQueryLoading || inviteResetQuerying"
          @click="queryInviteResetAndUsage"
        >
          <Icon name="refresh" size="xs" :class="(activeQueryLoading || inviteResetQuerying) && 'animate-spin'" />
          {{ t('admin.accounts.inviteResetQuery') }}
        </button>
        <button
          type="button"
          class="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[9px] font-medium text-orange-600 hover:bg-orange-50 disabled:opacity-40 disabled:hover:bg-transparent dark:text-orange-400 dark:hover:bg-orange-900/30 transition-colors"
          :disabled="inviteResetConsuming || !inviteResetHasCredit"
          @click="consumeInviteReset"
        >
          <Icon name="sync" size="xs" :class="inviteResetConsuming && 'animate-spin'" />
          {{ t('admin.accounts.inviteResetReset') }}
        </button>
      </div>
    </template>


    <!-- Kiro OAuth accounts: total quota display -->
    <template v-else-if="account.platform === 'kiro' && account.type === 'oauth'">
      <div v-if="loading" class="space-y-1.5">
        <div class="flex items-center gap-1">
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-1.5 w-8 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
        </div>
      </div>
      <div v-else-if="error" class="text-xs text-red-500">
        {{ error }}
      </div>
      <div v-else-if="usageInfo?.error" class="text-xs text-amber-600 dark:text-amber-400 truncate max-w-[220px]" :title="usageInfo.error">
        {{ usageInfo.error }}
      </div>
      <div v-else-if="needsReauth" class="space-y-1">
        <span class="inline-block rounded px-1.5 py-0.5 text-[10px] font-medium bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300">
          {{ t('admin.accounts.needsReauth') }}
        </span>
      </div>
      <div v-else-if="usageInfo?.kiro_quota" class="space-y-1">
        <div v-if="usageInfo.kiro_subscription_title" class="mb-1 flex items-center gap-1">
          <span class="inline-block rounded px-1.5 py-0.5 text-[10px] font-medium bg-cyan-100 text-cyan-700 dark:bg-cyan-900/40 dark:text-cyan-300">
            {{ usageInfo.kiro_subscription_title }}
          </span>
        </div>
        <UsageProgressBar
          :label="kiroUsageLabel"
          :utilization="usageInfo.kiro_quota.utilization"
          :resets-at="usageInfo.kiro_quota.resets_at"
          :window-stats="kiroQuotaStats"
          color="cyan"
        />
        <div class="text-[10px] text-gray-500 dark:text-gray-400">
          {{ kiroUsageSummary }}
        </div>
        <div v-if="kiroQuotaBreakdownSummary.length" class="space-y-0.5 text-[10px] text-gray-500 dark:text-gray-400">
          <div
            v-for="item in kiroQuotaBreakdownSummary"
            :key="item.key"
            class="flex items-center justify-between gap-2"
          >
            <span>{{ item.label }}</span>
            <span class="tabular-nums">{{ item.summary }}</span>
          </div>
        </div>
      </div>
      <div v-else class="text-xs text-gray-400">-</div>
    </template>

    <!-- Antigravity OAuth accounts: fetch usage from API -->
    <template v-else-if="account.platform === 'antigravity' && account.type === 'oauth'">
      <!-- 账户类型徽章 -->
      <div v-if="antigravityTierLabel" class="mb-1 flex items-center gap-1">
        <span
          :class="[
            'inline-block rounded px-1.5 py-0.5 text-[10px] font-medium',
            antigravityTierClass
          ]"
        >
          {{ antigravityTierLabel }}
        </span>
        <!-- 不合格账户警告图标 -->
        <span
          v-if="hasIneligibleTiers"
          class="group relative cursor-help"
        >
          <svg
            class="h-3.5 w-3.5 text-red-500"
            fill="currentColor"
            viewBox="0 0 20 20"
          >
            <path
              fill-rule="evenodd"
              d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z"
              clip-rule="evenodd"
            />
          </svg>
          <span
            class="pointer-events-none absolute left-0 top-full z-50 mt-1 w-80 whitespace-normal break-words rounded bg-gray-900 px-3 py-2 text-xs leading-relaxed text-white opacity-0 shadow-lg transition-opacity group-hover:opacity-100 dark:bg-gray-700"
          >
            {{ t('admin.accounts.ineligibleWarning') }}
          </span>
        </span>
      </div>

      <!-- Forbidden state (403) -->
      <div v-if="isForbidden" class="space-y-1">
        <span
          :class="[
            'inline-block rounded px-1.5 py-0.5 text-[10px] font-medium',
            forbiddenBadgeClass
          ]"
        >
          {{ forbiddenLabel }}
        </span>
        <div v-if="validationURL" class="flex items-center gap-1">
          <a
            :href="validationURL"
            target="_blank"
            rel="noopener noreferrer"
            class="text-[10px] text-blue-600 hover:text-blue-800 hover:underline dark:text-blue-400 dark:hover:text-blue-300"
            :title="t('admin.accounts.openVerification')"
          >
            {{ t('admin.accounts.openVerification') }}
          </a>
          <button
            type="button"
            class="text-[10px] text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
            :title="t('admin.accounts.copyLink')"
            @click="copyValidationURL"
          >
            {{ linkCopied ? t('admin.accounts.linkCopied') : t('admin.accounts.copyLink') }}
          </button>
        </div>
      </div>

      <!-- Needs reauth (401) -->
      <div v-else-if="needsReauth" class="space-y-1">
        <span class="inline-block rounded px-1.5 py-0.5 text-[10px] font-medium bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300">
          {{ t('admin.accounts.needsReauth') }}
        </span>
      </div>

      <!-- Degraded error (non-403, non-401) -->
      <div v-else-if="usageInfo?.error" class="space-y-1">
        <span class="inline-block rounded px-1.5 py-0.5 text-[10px] font-medium bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
          {{ usageErrorLabel }}
        </span>
      </div>

      <!-- Loading state -->
      <div v-else-if="loading" class="space-y-1.5">
        <div class="flex items-center gap-1">
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-1.5 w-8 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
        </div>
      </div>

      <!-- Error state -->
      <div v-else-if="error" class="text-xs text-red-500">
        {{ error }}
      </div>

      <!-- Usage data from API -->
      <div v-else-if="hasAntigravityQuotaFromAPI" class="space-y-1">
        <!-- Gemini 3 Pro -->
        <UsageProgressBar
          v-if="antigravity3ProUsageFromAPI !== null"
          :label="t('admin.accounts.usageWindow.gemini3Pro')"
          :utilization="antigravity3ProUsageFromAPI.utilization"
          :resets-at="antigravity3ProUsageFromAPI.resetTime"
          color="indigo"
        />

        <!-- Gemini 3 Flash -->
        <UsageProgressBar
          v-if="antigravity3FlashUsageFromAPI !== null"
          :label="t('admin.accounts.usageWindow.gemini3Flash')"
          :utilization="antigravity3FlashUsageFromAPI.utilization"
          :resets-at="antigravity3FlashUsageFromAPI.resetTime"
          color="emerald"
        />

        <!-- Gemini 3 Image -->
        <UsageProgressBar
          v-if="antigravity3ImageUsageFromAPI !== null"
          :label="t('admin.accounts.usageWindow.gemini3Image')"
          :utilization="antigravity3ImageUsageFromAPI.utilization"
          :resets-at="antigravity3ImageUsageFromAPI.resetTime"
          color="purple"
        />

        <!-- Claude -->
        <UsageProgressBar
          v-if="antigravityClaudeUsageFromAPI !== null"
          :label="t('admin.accounts.usageWindow.claude')"
          :utilization="antigravityClaudeUsageFromAPI.utilization"
          :resets-at="antigravityClaudeUsageFromAPI.resetTime"
          color="amber"
        />

        <div v-if="aiCreditsDisplay" class="mt-1 text-[10px] text-gray-500 dark:text-gray-400">
          💳 {{ t('admin.accounts.aiCreditsBalance') }}: {{ aiCreditsDisplay }}
        </div>
      </div>
      <div v-else-if="aiCreditsDisplay" class="text-[10px] text-gray-500 dark:text-gray-400">
        💳 {{ t('admin.accounts.aiCreditsBalance') }}: {{ aiCreditsDisplay }}
      </div>
      <div v-else class="text-xs text-gray-400">-</div>
    </template>

    <!-- Gemini platform: align with Antigravity-style family windows -->
    <!-- Grok OAuth accounts: upstream ratio + local 7d Sub2API usage -->
    <template v-else-if="account.platform === 'grok' && account.type === 'oauth'">
      <div v-if="loading" class="space-y-1.5">
        <div class="flex items-center gap-1">
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-1.5 w-8 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
          <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
        </div>
      </div>
      <div v-else-if="error" class="text-xs text-red-500">
        {{ error }}
      </div>
      <div v-else-if="usageInfo?.seven_day" class="space-y-1">
        <UsageProgressBar
          label="7d"
          :utilization="usageInfo.seven_day.utilization"
          :resets-at="usageInfo.seven_day.resets_at"
          :window-stats="usageInfo.seven_day.window_stats"
          color="emerald"
        />
      </div>
      <div v-else class="text-xs text-gray-400">-</div>
    </template>

    <!-- Gemini platform: show quota + local usage window -->
    <template v-else-if="account.platform === 'gemini'">
      <div class="space-y-1">
        <div v-if="isForbidden" class="space-y-1">
          <span
            :class="[
              'inline-block rounded px-1.5 py-0.5 text-[10px] font-medium',
              forbiddenBadgeClass
            ]"
          >
            {{ forbiddenLabel }}
          </span>
          <div v-if="validationURL" class="flex items-center gap-1">
            <a
              :href="validationURL"
              target="_blank"
              rel="noopener noreferrer"
              class="text-[10px] text-blue-600 hover:text-blue-800 hover:underline dark:text-blue-400 dark:hover:text-blue-300"
              :title="t('admin.accounts.openVerification')"
            >
              {{ t('admin.accounts.openVerification') }}
            </a>
            <button
              type="button"
              class="text-[10px] text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
              :title="t('admin.accounts.copyLink')"
              @click="copyValidationURL"
            >
              {{ linkCopied ? t('admin.accounts.linkCopied') : t('admin.accounts.copyLink') }}
            </button>
          </div>
        </div>
        <div v-else-if="needsReauth" class="space-y-1">
          <span class="inline-block rounded px-1.5 py-0.5 text-[10px] font-medium bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-300">
            {{ t('admin.accounts.needsReauth') }}
          </span>
        </div>
        <div v-else-if="usageInfo?.error" class="space-y-1">
          <span class="inline-block rounded px-1.5 py-0.5 text-[10px] font-medium bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300">
            {{ usageErrorLabel }}
          </span>
        </div>
        <div v-else-if="loading" class="space-y-1">
          <div class="flex items-center gap-1">
            <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
            <div class="h-1.5 w-8 animate-pulse rounded-full bg-gray-200 dark:bg-gray-700"></div>
            <div class="h-3 w-[32px] animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
          </div>
        </div>
        <div v-else-if="error" class="text-xs text-red-500">
          {{ error }}
        </div>
        <div v-else-if="geminiUsageBars.length" class="space-y-1">
          <UsageProgressBar
            v-for="bar in geminiUsageBars"
            :key="bar.key"
            :label="bar.label"
            :utilization="bar.utilization"
            :resets-at="bar.resetsAt"
            :window-stats="bar.windowStats"
            :color="bar.color"
          />
        </div>
        <div v-else class="text-xs text-gray-400">
          -
        </div>
      </div>
    </template>

    <!-- Other accounts: no usage window -->
    <template v-else>
      <div class="text-xs text-gray-400">-</div>
    </template>
  </div>

  <!-- Non-OAuth/Setup-Token accounts -->
  <div ref="rootRef" v-else>
    <!-- Key/Bedrock accounts: show today stats + optional quota bars -->
    <div class="space-y-1">
      <!-- Today stats row (requests, tokens, cost, user_cost) -->
      <div
        v-if="account.platform !== 'gemini' && todayStats"
        class="mb-0.5 flex items-center"
      >
        <div class="flex items-center gap-1.5 text-[9px] text-gray-500 dark:text-gray-400">
          <span class="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-gray-800">
            {{ formatKeyRequests }} req
          </span>
          <span class="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-gray-800">
            {{ formatKeyTokens }}
          </span>
          <span class="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-gray-800" :title="t('usage.accountBilled')">
            A ${{ formatKeyCost }}
          </span>
          <span
            v-if="todayStats.user_cost != null"
            class="rounded bg-gray-100 px-1.5 py-0.5 dark:bg-gray-800"
            :title="t('usage.userBilled')"
          >
            U ${{ formatKeyUserCost }}
          </span>
        </div>
      </div>
      <!-- Loading skeleton for today stats -->
      <div
        v-else-if="account.platform !== 'gemini' && todayStatsLoading"
        class="mb-0.5 flex items-center gap-1"
      >
        <div class="h-3 w-10 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
        <div class="h-3 w-8 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
        <div class="h-3 w-12 animate-pulse rounded bg-gray-200 dark:bg-gray-700"></div>
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
      <div v-if="!todayStats && !todayStatsLoading && !hasApiKeyQuota" class="text-xs text-gray-400">-</div>
    </div>
  </div>

  <CodexInviteResetModal
    v-if="isOpenAIOAuthAccount"
    :show="showInviteResetModal"
    :account="account"
    :initial-status="effectiveInviteResetStatus"
    @close="showInviteResetModal = false"
    @updated="onInviteResetUpdated"
  />
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Account, AccountUsageInfo, Group, KiroQuotaBreakdown, UsageProgress, WindowStats } from '@/types'
import type { CodexInviteResetStatus } from '@/api/admin/accounts'
import { buildGeminiUsageRefreshKey, buildOpenAIUsageRefreshKey } from '@/utils/accountUsageRefresh'
import { enqueueUsageRequest } from '@/utils/usageLoadQueue'
import { Icon } from '@/components/icons'
import { formatCompactNumber } from '@/utils/format'
import UsageProgressBar from './UsageProgressBar.vue'
import CodexInviteResetModal from './CodexInviteResetModal.vue'

// Module-level cache shared across all AccountUsageCell instances
const _usageCache = new Map<number, { data: AccountUsageInfo; ts: number }>()
const USAGE_CACHE_TTL = 5 * 60 * 1000 // 5 minutes

const props = withDefaults(
  defineProps<{
    account: Account
    todayStats?: WindowStats | null
    todayStatsLoading?: boolean
    manualRefreshToken?: number
    activeGroupId?: number | null
    batchedUsage?: AccountUsageInfo | null
    batchedUsageError?: string | null
    batchedUsageLoading?: boolean
    requestBatchedUsage?: ((account: Account, options?: { force?: boolean }) => void) | null
  }>(),
  {
    todayStats: null,
    todayStatsLoading: false,
    manualRefreshToken: 0,
    activeGroupId: null,
    batchedUsage: null,
    batchedUsageError: null,
    batchedUsageLoading: false,
    requestBatchedUsage: null
  }
)

const { t } = useI18n()
const desktopViewportQuery = '(min-width: 768px)'

const unmounted = ref(false)
onBeforeUnmount(() => { unmounted.value = true })

const loading = ref(false)
const activeQueryLoading = ref(false)
const error = ref<string | null>(null)
const usageInfo = ref<AccountUsageInfo | null>(null)
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
  if (props.account.platform === 'anthropic') {
    return props.account.type === 'oauth' || props.account.type === 'setup-token' || props.account.type === 'service_account'
  }
  return props.account.type === 'oauth' || props.account.type === 'setup-token'
})

const shouldFetchUsage = computed(() => {
  if (props.account.platform === 'anthropic') {
    return props.account.type === 'oauth' || props.account.type === 'setup-token' || props.account.type === 'service_account'
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

const isBatchManaged = computed(() => typeof props.requestBatchedUsage === 'function')

const openAIResponseUsageBars = computed(() => {
  if (props.account.platform !== 'openai' || props.account.type !== 'oauth') return []
  const info = usageInfo.value
  if (!info) return []
  const items: Array<{ key: string; label: string; progress: UsageProgress; color: 'indigo' | 'emerald' }> = []
  if (hasUsageProgressData(info.five_hour)) {
    items.push({
      key: 'responses-5h',
      label: '5h',
      progress: info.five_hour,
      color: 'indigo'
    })
  }
  if (hasUsageProgressData(info.seven_day)) {
    items.push({
      key: 'responses-7d',
      label: '7d',
      progress: info.seven_day,
      color: 'emerald'
    })
  }
  return items
})

const openAICurrentImageGroups = computed(() => {
  if (props.account.platform !== 'openai' || props.account.type !== 'oauth') {
    return []
  }

  const groups = Array.isArray(props.account.groups)
    ? props.account.groups.filter((group): group is Group => group.platform === 'openai')
    : []

  if (!groups.length) return []

  if (typeof props.activeGroupId === 'number' && Number.isFinite(props.activeGroupId) && props.activeGroupId > 0) {
    return groups.filter((group) => group.id === props.activeGroupId)
  }

  return groups
})

const hasUsageProgressData = (progress?: UsageProgress | null): progress is UsageProgress => {
  if (!progress) return false
  const stats = progress.window_stats
  return progress.utilization > 0 ||
    (progress.used_requests ?? 0) > 0 ||
    (stats?.requests ?? 0) > 0 ||
    (stats?.tokens ?? 0) > 0 ||
    (stats?.cost ?? 0) > 0 ||
    (stats?.user_cost ?? 0) > 0 ||
    (stats?.standard_cost ?? 0) > 0
}

const isRouteVisible = (supported: boolean | undefined, hasData: boolean) => {
  if (supported === false) return false
  return supported === true || hasData
}

const openAIEnabledImageRoutes = computed(() => {
  const groups = openAICurrentImageGroups.value
  const info = usageInfo.value
  const codexSupported = info?.openai_image_codex_supported
  const hasCodexData = hasUsageProgressData(info?.openai_image_codex_five_hour) || hasUsageProgressData(info?.openai_image_codex_seven_day)

  if (!groups.length) {
    return {
      codex: isRouteVisible(codexSupported, hasCodexData),
      constrained: false,
      masterEnabled: false,
      hasGroups: false
    }
  }

  let codex = false
  let masterEnabled = false

  for (const group of groups) {
    if (!group.allow_image_generation) continue
    masterEnabled = true
    if (isRouteVisible(codexSupported, hasCodexData)) {
      codex = true
    }
  }

  return { codex, constrained: masterEnabled, masterEnabled, hasGroups: true }
})

const showOpenAIResponseUsageBars = computed(() => {
  if (props.account.platform !== 'openai' || props.account.type !== 'oauth') return false
  return openAIResponseUsageBars.value.length > 0
})

const openAIImageUsageSummary = computed(() => {
  if (props.account.platform !== 'openai' || props.account.type !== 'oauth') return []
  const info = usageInfo.value
  if (!info) return []
  const { codex: showCodex } = openAIEnabledImageRoutes.value
  const items: Array<{ label: string; requests: number; userCost: number }> = []
  const appendItem = (visible: boolean, label: string, progress?: UsageProgress | null) => {
    if (!visible || !hasUsageProgressData(progress)) return
    const stats = progress?.window_stats
    items.push({
      label,
      requests: stats?.requests ?? progress?.used_requests ?? 0,
      userCost: stats?.user_cost ?? stats?.cost ?? 0
    })
  }
  appendItem(showCodex, '5h', info.openai_image_codex_five_hour)
  appendItem(showCodex, '7d', info.openai_image_codex_seven_day)
  return items
})

const hasOpenAIImageUsageProgressData = computed(() => {
  if (props.account.platform !== 'openai' || props.account.type !== 'oauth') return false
  const info = usageInfo.value
  if (!info) return false
  const { codex: showCodex } = openAIEnabledImageRoutes.value
  return showCodex && (hasUsageProgressData(info.openai_image_codex_five_hour) || hasUsageProgressData(info.openai_image_codex_seven_day))
})

const hasOpenAIUsageContent = computed(() => {
  if (props.account.platform !== 'openai' || props.account.type !== 'oauth') return false
  return (showOpenAIResponseUsageBars.value && openAIResponseUsageBars.value.length > 0) ||
    hasOpenAIImageUsageProgressData.value ||
    !!usageInfo.value?.error
})

const kiroQuotaStats = computed<WindowStats | null>(() => {
  if (props.account.platform !== 'kiro') return null
  if (usageInfo.value?.kiro_quota?.window_stats) return usageInfo.value.kiro_quota.window_stats
  return null
})

const kiroUsageLabel = computed(() => {
  const limit = usageInfo.value?.kiro_usage_limit
  if (typeof limit !== 'number' || Number.isNaN(limit) || limit <= 0) return '$0'
  return `$${formatKiroMoney(limit)}`
})

const kiroUsageSummary = computed(() => t('admin.accounts.kiro.usageSummary', {
  used: formatKiroMoney(usageInfo.value?.kiro_current_usage),
  limit: formatKiroMoney(usageInfo.value?.kiro_usage_limit),
  remaining: formatKiroMoney(usageInfo.value?.kiro_remaining)
}))

const formatKiroQuotaBreakdownSummary = (quota?: KiroQuotaBreakdown | null) => {
  if (!quota) return ''
  return t('admin.accounts.kiro.quotaPartSummary', {
    used: formatKiroMoney(quota.current_usage),
    limit: formatKiroMoney(quota.usage_limit),
    remaining: formatKiroMoney(quota.remaining)
  })
}

const kiroQuotaBreakdownSummary = computed(() => {
  const info = usageInfo.value
  if (!info) return []

  return [
    {
      key: 'monthly',
      label: t('admin.accounts.kiro.monthlyQuota'),
      summary: formatKiroQuotaBreakdownSummary(info.kiro_monthly_quota)
    },
    {
      key: 'bonus',
      label: t('admin.accounts.kiro.bonusQuota'),
      summary: formatKiroQuotaBreakdownSummary(info.kiro_bonus_quota)
    },
    {
      key: 'free_trial',
      label: t('admin.accounts.kiro.freeTrialQuota'),
      summary: formatKiroQuotaBreakdownSummary(info.kiro_free_trial_quota)
    }
  ].filter((item) => item.summary)
})

const openAIUsageRefreshKey = computed(() => buildOpenAIUsageRefreshKey(props.account))
const geminiUsageRefreshKey = computed(() => buildGeminiUsageRefreshKey(props.account))

const shouldAutoLoadUsageOnMount = computed(() => {
  return shouldFetchUsage.value
})

const shouldLazyLoadOnMobile = computed(() => {
  return shouldFetchUsage.value && !isDesktopViewport.value
})

// Antigravity quota types (用于 API 返回的数据)
interface AntigravityUsageResult {
  utilization: number
  resetTime: string | null
}

// ===== Antigravity quota from API (usageInfo.antigravity_quota) =====

// 检查是否有从 API 获取的配额数据
const hasAntigravityQuotaFromAPI = computed(() => {
  return usageInfo.value?.antigravity_quota && Object.keys(usageInfo.value.antigravity_quota).length > 0
})

// 从 API 配额数据中获取使用率（多模型取最高使用率）
const getAntigravityUsageFromAPI = (
  modelNames: string[]
): AntigravityUsageResult | null => {
  const quota = usageInfo.value?.antigravity_quota
  if (!quota) return null

  let maxUtilization = 0
  let earliestReset: string | null = null

  for (const model of modelNames) {
    const modelQuota = quota[model]
    if (!modelQuota) continue

    if (modelQuota.utilization > maxUtilization) {
      maxUtilization = modelQuota.utilization
    }
    if (modelQuota.reset_time) {
      if (!earliestReset || modelQuota.reset_time < earliestReset) {
        earliestReset = modelQuota.reset_time
      }
    }
  }

  // 如果没有找到任何匹配的模型
  if (maxUtilization === 0 && earliestReset === null) {
    const hasAnyData = modelNames.some((m) => quota[m])
    if (!hasAnyData) return null
  }

  return {
    utilization: maxUtilization,
    resetTime: earliestReset
  }
}

// Gemini 3 Pro from API
const antigravity3ProUsageFromAPI = computed(() =>
  getAntigravityUsageFromAPI(['gemini-3-pro-low', 'gemini-3-pro-high', 'gemini-3-pro-preview'])
)

// Gemini 3 Flash from API
const antigravity3FlashUsageFromAPI = computed(() => getAntigravityUsageFromAPI(['gemini-3-flash']))

// Gemini Image from API
const antigravity3ImageUsageFromAPI = computed(() =>
  getAntigravityUsageFromAPI(['gemini-2.5-flash-image', 'gemini-3.1-flash-image', 'gemini-3-pro-image'])
)

// Claude from API (all Claude model variants)
const antigravityClaudeUsageFromAPI = computed(() =>
  getAntigravityUsageFromAPI([
    'claude-fable-5',
    'claude-sonnet-4-5', 'claude-opus-4-5-thinking',
    'claude-sonnet-4-6', 'claude-opus-4-6', 'claude-opus-4-6-thinking',
    'claude-opus-4-7', 'claude-opus-4-8',
  ])
)

const aiCreditsDisplay = computed(() => {
  const credits = usageInfo.value?.ai_credits
  if (!credits || credits.length === 0) return null
  const total = credits.reduce((sum, credit) => sum + (credit.amount ?? 0), 0)
  if (total <= 0) return null
  return total.toFixed(0)
})

// Antigravity 账户类型（从 load_code_assist 响应中提取）
const antigravityTier = computed(() => {
  const extra = props.account.extra as Record<string, unknown> | undefined
  if (!extra) return null

  const loadCodeAssist = extra.load_code_assist as Record<string, unknown> | undefined
  if (!loadCodeAssist) return null

  // 优先取 paidTier，否则取 currentTier
  const paidTier = loadCodeAssist.paidTier as Record<string, unknown> | undefined
  if (paidTier && typeof paidTier.id === 'string') {
    return paidTier.id
  }

  const currentTier = loadCodeAssist.currentTier as Record<string, unknown> | undefined
  if (currentTier && typeof currentTier.id === 'string') {
    return currentTier.id
  }

  return null
})

type GeminiWindowBar = {
  key: string
  label: string
  utilization: number
  resetsAt: string | null
  windowStats?: WindowStats | null
  color: 'indigo' | 'emerald'
}

const pickGeminiWindowBar = (
  family: 'pro' | 'flash',
  label: string,
  color: 'indigo' | 'emerald'
): GeminiWindowBar | null => {
  const daily = family === 'pro'
    ? usageInfo.value?.gemini_pro_daily
    : usageInfo.value?.gemini_flash_daily
  const minute = family === 'pro'
    ? usageInfo.value?.gemini_pro_minute
    : usageInfo.value?.gemini_flash_minute
  const window = daily || minute
  const sharedWindow = daily
    ? usageInfo.value?.gemini_shared_daily
    : minute
      ? usageInfo.value?.gemini_shared_minute
      : null

  if (!window) return null

  return {
    key: `${family}_${daily ? 'daily' : 'minute'}`,
    label,
    utilization: Math.max(window.utilization, sharedWindow?.utilization ?? 0),
    resetsAt: sharedWindow?.resets_at ?? window.resets_at,
    windowStats: window.window_stats,
    color
  }
}

const geminiUsageBars = computed(() => {
  if (props.account.platform !== 'gemini') return []
  const sharedDaily = usageInfo.value?.gemini_shared_daily
  const sharedMinute = usageInfo.value?.gemini_shared_minute
  const hasFamilyWindows =
    !!usageInfo.value?.gemini_pro_daily ||
    !!usageInfo.value?.gemini_flash_daily ||
    !!usageInfo.value?.gemini_pro_minute ||
    !!usageInfo.value?.gemini_flash_minute

  if (!hasFamilyWindows && (sharedDaily || sharedMinute)) {
    const sharedWindow = sharedDaily || sharedMinute
    return [
      {
        key: sharedDaily ? 'shared_daily' : 'shared_minute',
        label: sharedDaily ? '1d' : '1m',
        utilization: sharedWindow?.utilization ?? 0,
        resetsAt: sharedWindow?.resets_at ?? null,
        windowStats: sharedWindow?.window_stats ?? null,
        color: 'indigo' as const
      }
    ]
  }

  const bars = [
    pickGeminiWindowBar('pro', t('admin.accounts.usageWindow.geminiProDaily'), 'indigo'),
    pickGeminiWindowBar('flash', t('admin.accounts.usageWindow.geminiFlashDaily'), 'emerald')
  ]

  return bars.filter((bar): bar is GeminiWindowBar => bar !== null)
})

// 账户类型显示标签
const antigravityTierLabel = computed(() => {
  switch (antigravityTier.value) {
    case 'free-tier':
      return t('admin.accounts.tier.free')
    case 'g1-pro-tier':
      return t('admin.accounts.tier.pro')
    case 'g1-ultra-tier':
      return t('admin.accounts.tier.ultra')
    default:
      return null
  }
})

// 账户类型徽章样式
const antigravityTierClass = computed(() => {
  switch (antigravityTier.value) {
    case 'free-tier':
      return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
    case 'g1-pro-tier':
      return 'bg-blue-100 text-blue-600 dark:bg-blue-900/40 dark:text-blue-300'
    case 'g1-ultra-tier':
      return 'bg-purple-100 text-purple-600 dark:bg-purple-900/40 dark:text-purple-300'
    default:
      return ''
  }
})

// 检测账户是否有不合格状态（ineligibleTiers）
const hasIneligibleTiers = computed(() => {
  const extra = props.account.extra as Record<string, unknown> | undefined
  if (!extra) return false

  const loadCodeAssist = extra.load_code_assist as Record<string, unknown> | undefined
  if (!loadCodeAssist) return false

  const ineligibleTiers = loadCodeAssist.ineligibleTiers as unknown[] | undefined
  return Array.isArray(ineligibleTiers) && ineligibleTiers.length > 0
})

// Antigravity 403 forbidden 状态
const isForbidden = computed(() => !!usageInfo.value?.is_forbidden)
const forbiddenType = computed(() => usageInfo.value?.forbidden_type || 'forbidden')
const validationURL = computed(() => usageInfo.value?.validation_url || '')

// 需要重新授权（401）
const needsReauth = computed(() => !!usageInfo.value?.needs_reauth)

// 降级错误标签（rate_limited / network_error）
const usageErrorLabel = computed(() => {
  const code = usageInfo.value?.error_code
  if (code === 'rate_limited') return t('admin.accounts.rateLimited')
  return t('admin.accounts.usageError')
})

const forbiddenLabel = computed(() => {
  switch (forbiddenType.value) {
    case 'validation':
      return t('admin.accounts.forbiddenValidation')
    case 'violation':
      return t('admin.accounts.forbiddenViolation')
    default:
      return t('admin.accounts.forbidden')
  }
})

const forbiddenBadgeClass = computed(() => {
  if (forbiddenType.value === 'validation') {
    return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-300'
  }
  return 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
})

const linkCopied = ref(false)
const copyValidationURL = async () => {
  if (!validationURL.value) return
  try {
    await navigator.clipboard.writeText(validationURL.value)
    linkCopied.value = true
    setTimeout(() => { linkCopied.value = false }, 2000)
  } catch {
    // fallback: ignore
  }
}

const formatKiroMoney = (value?: number | null) => {
  if (value == null || Number.isNaN(value)) return '0'
  if (value >= 100) return value.toFixed(0)
  if (value >= 10) return value.toFixed(1)
  return value.toFixed(2)
}

const isAnthropicUsageAccount = computed(() => {
  return props.account.platform === 'anthropic' && (props.account.type === 'oauth' || props.account.type === 'setup-token' || props.account.type === 'service_account')
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
      ? adminAPI.accounts.getUsage(props.account.id, options.source)
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
    usageInfo.value = await adminAPI.accounts.getUsage(props.account.id, 'active')
  } catch (e: any) {
    console.error('Failed to load active usage:', e)
  } finally {
    activeQueryLoading.value = false
  }
}

// ===== Codex invite reset (OpenAI OAuth only) =====
const isOpenAIOAuthAccount = computed(
  () => props.account.platform === 'openai' && props.account.type === 'oauth'
)
const inviteResetStatus = ref<CodexInviteResetStatus | null>(null)
const inviteResetQuerying = ref(false)
const inviteResetConsuming = ref(false)
const showInviteResetModal = ref(false)

const persistedInviteResetStatus = computed<CodexInviteResetStatus | null>(() => {
  if (!isOpenAIOAuthAccount.value) return null
  const extra = props.account.extra ?? {}
  const count = Number(extra.codex_invite_reset_available_count)
  if (!Number.isFinite(count)) return null
  const credits = Array.isArray(extra.codex_invite_reset_credits)
    ? extra.codex_invite_reset_credits
      .filter((credit) => credit && typeof credit === 'object')
      .map((credit: any) => ({
        id: String(credit.id ?? ''),
        status: typeof credit.status === 'string' ? credit.status : undefined,
        title: typeof credit.title === 'string' ? credit.title : undefined,
        description: typeof credit.description === 'string' ? credit.description : undefined,
        profile_user_id: typeof credit.profile_user_id === 'string' ? credit.profile_user_id : undefined,
        profile_image_url: typeof credit.profile_image_url === 'string' ? credit.profile_image_url : undefined
      }))
      .filter((credit) => credit.id)
    : []
  return {
    referral_key: '',
    requires_consent: true,
    available_count: count,
    credits
  }
})
const effectiveInviteResetStatus = computed(() => inviteResetStatus.value ?? persistedInviteResetStatus.value)
const inviteResetAvailableCredits = computed(() =>
  (effectiveInviteResetStatus.value?.credits ?? []).filter((credit) => {
    const state = credit.status?.toLowerCase()
    return !state || state === 'available'
  })
)
const inviteResetAvailableCount = computed(
  () => effectiveInviteResetStatus.value?.available_count ?? inviteResetAvailableCredits.value.length
)
const inviteResetHasCredit = computed(() => inviteResetAvailableCount.value > 0)

const loadInviteResetStatus = async () => {
  if (!isOpenAIOAuthAccount.value) return
  inviteResetStatus.value = await adminAPI.accounts.getCodexInviteResetStatus(props.account.id)
}

// 查询同时刷新用量窗口和重置次数，两者互不阻塞。
const queryInviteResetAndUsage = async () => {
  if (inviteResetQuerying.value) return
  inviteResetQuerying.value = true
  try {
    const [, statusResult] = await Promise.allSettled([loadActiveUsage(), loadInviteResetStatus()])
    if (statusResult.status === 'rejected') {
      const reason: any = statusResult.reason
      useAppStore().showError(reason?.message || t('admin.accounts.inviteResetLoadFailed'))
    }
  } finally {
    inviteResetQuerying.value = false
  }
}

const consumeInviteReset = async () => {
  if (inviteResetConsuming.value) return
  const credit = inviteResetAvailableCredits.value[0]
  if (!inviteResetHasCredit.value) {
    useAppStore().showError(t('admin.accounts.inviteResetNoCredit'))
    return
  }
  inviteResetConsuming.value = true
  const appStore = useAppStore()
  try {
    const result = await adminAPI.accounts.consumeCodexInviteReset(props.account.id, credit?.id ?? '')
    if (!result.code || result.code === 'reset') {
      appStore.showSuccess(t('admin.accounts.inviteResetConsumeSuccess'))
    } else {
      appStore.showError(inviteResetConsumeMessage(result.code))
    }
    await Promise.allSettled([loadActiveUsage(), loadInviteResetStatus()])
  } catch (e: any) {
    appStore.showError(e?.message || t('admin.accounts.inviteResetConsumeFailed'))
  } finally {
    inviteResetConsuming.value = false
  }
}

const inviteResetConsumeMessage = (code: string) => {
  if (code === 'nothing_to_reset') return t('admin.accounts.inviteResetNothingToReset')
  if (code === 'already_redeemed') return t('admin.accounts.inviteResetAlreadyRedeemed')
  if (code === 'no_credit') return t('admin.accounts.inviteResetNoCredit')
  return t('admin.accounts.inviteResetConsumeFailed')
}

const openInviteResetModal = async () => {
  showInviteResetModal.value = true
  // 弹窗打开时若还没查过次数，顺带拉一次，让弹窗直接复用。
  if (!effectiveInviteResetStatus.value) {
    try {
      await loadInviteResetStatus()
    } catch {
      // 弹窗内部会自行重试并提示，这里静默。
    }
  }
}

const onInviteResetUpdated = () => {
  loadInviteResetStatus().catch(() => {})
}

// ===== API Key quota progress bars =====

interface QuotaBarInfo {
  utilization: number
  resetsAt: string | null
}

const makeQuotaBar = (
  used: number,
  limit: number,
  startKey?: string
): QuotaBarInfo => {
  const utilization = limit > 0 ? (used / limit) * 100 : 0
  let resetsAt: string | null = null
  if (startKey) {
    const extra = props.account.extra as Record<string, unknown> | undefined
    const isDaily = startKey.includes('daily')
    const mode = isDaily
      ? (extra?.quota_daily_reset_mode as string) || 'rolling'
      : (extra?.quota_weekly_reset_mode as string) || 'rolling'

    if (mode === 'fixed') {
      // Use pre-computed next reset time for fixed mode
      const resetAtKey = isDaily ? 'quota_daily_reset_at' : 'quota_weekly_reset_at'
      resetsAt = (extra?.[resetAtKey] as string) || null
    } else {
      // Rolling mode: compute from start + period
      const startStr = extra?.[startKey] as string | undefined
      if (startStr) {
        const startDate = new Date(startStr)
        const periodMs = isDaily ? 24 * 60 * 60 * 1000 : 7 * 24 * 60 * 60 * 1000
        resetsAt = new Date(startDate.getTime() + periodMs).toISOString()
      }
    }
  }
  return { utilization, resetsAt }
}

const hasApiKeyQuota = computed(() => {
  if (props.account.type !== 'apikey' && props.account.type !== 'bedrock') return false
  return (
    (props.account.quota_daily_limit ?? 0) > 0 ||
    (props.account.quota_weekly_limit ?? 0) > 0 ||
    (props.account.quota_limit ?? 0) > 0
  )
})

const quotaDailyBar = computed((): QuotaBarInfo | null => {
  const limit = props.account.quota_daily_limit ?? 0
  if (limit <= 0) return null
  return makeQuotaBar(props.account.quota_daily_used ?? 0, limit, 'quota_daily_start')
})

const quotaWeeklyBar = computed((): QuotaBarInfo | null => {
  const limit = props.account.quota_weekly_limit ?? 0
  if (limit <= 0) return null
  return makeQuotaBar(props.account.quota_weekly_used ?? 0, limit, 'quota_weekly_start')
})

const quotaTotalBar = computed((): QuotaBarInfo | null => {
  const limit = props.account.quota_limit ?? 0
  if (limit <= 0) return null
  return makeQuotaBar(props.account.quota_used ?? 0, limit)
})

// ===== Key account today stats formatters =====

const formatKeyRequests = computed(() => {
  if (!props.todayStats) return ''
  return formatCompactNumber(props.todayStats.requests, { allowBillions: false })
})

const formatKeyTokens = computed(() => {
  if (!props.todayStats) return ''
  return formatCompactNumber(props.todayStats.tokens)
})

const formatKeyCost = computed(() => {
  if (!props.todayStats) return '0.00'
  return props.todayStats.cost.toFixed(2)
})

const formatKeyUserCost = computed(() => {
  if (!props.todayStats || props.todayStats.user_cost == null) return '0.00'
  return props.todayStats.user_cost.toFixed(2)
})

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
  const source = isAnthropicUsageAccount.value ? 'passive' : undefined
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

watch(geminiUsageRefreshKey, (nextKey, prevKey) => {
  if (!prevKey || nextKey === prevKey) return
  if (props.account.platform !== 'gemini') return

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

    const source = isAnthropicUsageAccount.value ? 'passive' : undefined
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
