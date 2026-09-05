<template>
  <div class="border-t border-line pt-4 space-y-4">
    <div class="mb-3">
      <h3 class="input-label mb-0 text-base font-semibold">{{ t('admin.accounts.quotaControl.title') }}</h3>
      <p class="mt-1 text-xs text-muted">
        {{ t('admin.accounts.quotaControl.hint') }}
      </p>
    </div>

    <!-- Window Cost Limit -->
    <div class="rounded-lg border border-line p-4">
      <div class="mb-3 flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.windowCost.label') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.quotaControl.windowCost.hint') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="windowCostEnabled" />
      </div>

      <div v-if="windowCostEnabled" class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.windowCost.limit') }}</label>
          <div class="relative">
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted">$</span>
            <input
              v-model.number="windowCostLimit"
              type="number"
              min="0"
              step="1"
              class="input pl-7"
              :placeholder="t('admin.accounts.quotaControl.windowCost.limitPlaceholder')"
            />
          </div>
          <p class="input-hint">{{ t('admin.accounts.quotaControl.windowCost.limitHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.windowCost.stickyReserve') }}</label>
          <div class="relative">
            <span class="absolute left-3 top-1/2 -translate-y-1/2 text-muted">$</span>
            <input
              v-model.number="windowCostStickyReserve"
              type="number"
              min="0"
              step="1"
              class="input pl-7"
              :placeholder="t('admin.accounts.quotaControl.windowCost.stickyReservePlaceholder')"
            />
          </div>
          <p class="input-hint">{{ t('admin.accounts.quotaControl.windowCost.stickyReserveHint') }}</p>
        </div>
      </div>
    </div>

    <!-- Session Limit -->
    <div class="rounded-lg border border-line p-4">
      <div class="mb-3 flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.sessionLimit.label') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.quotaControl.sessionLimit.hint') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="sessionLimitEnabled" />
      </div>

      <div v-if="sessionLimitEnabled" class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.sessionLimit.maxSessions') }}</label>
          <input
            v-model.number="maxSessions"
            type="number"
            min="1"
            step="1"
            class="input"
            :placeholder="t('admin.accounts.quotaControl.sessionLimit.maxSessionsPlaceholder')"
          />
          <p class="input-hint">{{ t('admin.accounts.quotaControl.sessionLimit.maxSessionsHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.sessionLimit.idleTimeout') }}</label>
          <div class="relative">
            <input
              v-model.number="sessionIdleTimeout"
              type="number"
              min="1"
              step="1"
              class="input pr-12"
              :placeholder="t('admin.accounts.quotaControl.sessionLimit.idleTimeoutPlaceholder')"
            />
            <span class="absolute right-3 top-1/2 -translate-y-1/2 text-muted">{{ t('common.minutes') }}</span>
          </div>
          <p class="input-hint">{{ t('admin.accounts.quotaControl.sessionLimit.idleTimeoutHint') }}</p>
        </div>
      </div>
    </div>

    <!-- RPM Limit -->
    <div class="rounded-lg border border-line p-4">
      <div class="mb-3 flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.rpmLimit.label') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.quotaControl.rpmLimit.hint') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="rpmLimitEnabled" />
      </div>

      <div v-if="rpmLimitEnabled" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpm') }}</label>
          <input
            v-model.number="baseRpm"
            type="number"
            min="1"
            max="1000"
            step="1"
            class="input"
            :placeholder="t('admin.accounts.quotaControl.rpmLimit.baseRpmPlaceholder')"
          />
          <p class="input-hint">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpmHint') }}</p>
        </div>

        <div>
          <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.strategy') }}</label>
          <div class="flex gap-2">
            <button
              type="button"
              @click="rpmStrategy = 'tiered'"
              :class="[
 'flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-all',
 rpmStrategy === 'tiered'
 ? 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent'
 : 'bg-surface-2 text-muted hover:bg-surface-3'
 ]"
            >
              <div class="text-center">
                <div>{{ t('admin.accounts.quotaControl.rpmLimit.strategyTiered') }}</div>
                <div class="mt-0.5 text-[10px] opacity-70">{{ t('admin.accounts.quotaControl.rpmLimit.strategyTieredHint') }}</div>
              </div>
            </button>
            <button
              type="button"
              @click="rpmStrategy = 'sticky_exempt'"
              :class="[
 'flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-all',
 rpmStrategy === 'sticky_exempt'
 ? 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent'
 : 'bg-surface-2 text-muted hover:bg-surface-3'
 ]"
            >
              <div class="text-center">
                <div>{{ t('admin.accounts.quotaControl.rpmLimit.strategyStickyExempt') }}</div>
                <div class="mt-0.5 text-[10px] opacity-70">{{ t('admin.accounts.quotaControl.rpmLimit.strategyStickyExemptHint') }}</div>
              </div>
            </button>
          </div>
        </div>

        <div v-if="rpmStrategy === 'tiered'">
          <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.stickyBuffer') }}</label>
          <input
            v-model.number="rpmStickyBuffer"
            type="number"
            min="1"
            step="1"
            class="input"
            :placeholder="t('admin.accounts.quotaControl.rpmLimit.stickyBufferPlaceholder')"
          />
          <p class="input-hint">{{ t('admin.accounts.quotaControl.rpmLimit.stickyBufferHint') }}</p>
        </div>

      </div>

      <!-- 用户消息限速模式（独立于 RPM 开关，始终可见） -->
      <div class="mt-4">
        <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.userMsgQueue') }}</label>
        <p class="mt-1 text-xs text-muted mb-2">
          {{ t('admin.accounts.quotaControl.rpmLimit.userMsgQueueHint') }}
        </p>
        <div class="flex space-x-2">
          <button type="button" v-for="opt in umqModeOptions" :key="opt.value"
            @click="userMsgQueueMode = opt.value"
            :class="[
 'px-3 py-1.5 text-sm rounded-md border transition-colors',
 userMsgQueueMode === opt.value
 ? 'bg-accent text-white border-accent'
 : 'bg-surface text-foreground border-line hover:bg-surface-2'
 ]">
            {{ opt.label }}
          </button>
        </div>
      </div>
    </div>

    <!-- Session ID Masking -->
    <div class="rounded-lg border border-line p-4">
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.sessionIdMasking.label') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.quotaControl.sessionIdMasking.hint') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="sessionIdMaskingEnabled" />
      </div>
    </div>

    <!-- Cache TTL Override -->
    <div class="rounded-lg border border-line p-4">
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.cacheTTLOverride.label') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.quotaControl.cacheTTLOverride.hint') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="cacheTTLOverrideEnabled" />
      </div>
      <div v-if="cacheTTLOverrideEnabled" class="mt-3">
        <label class="input-label text-xs">{{ t('admin.accounts.quotaControl.cacheTTLOverride.target') }}</label>
        <select
          v-model="cacheTTLOverrideTarget"
          class="mt-1 block w-full rounded-md border border-line bg-surface px-3 py-2 text-sm shadow-sm focus:border-accent focus:outline-none focus:ring-1 focus:ring-accent"
        >
          <option value="5m">5m</option>
          <option value="1h">1h</option>
        </select>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.quotaControl.cacheTTLOverride.targetHint') }}
        </p>
      </div>
    </div>

    <!-- Custom Base URL Relay -->
    <div class="rounded-lg border border-line p-4">
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.customBaseUrl.label') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.quotaControl.customBaseUrl.hint') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="customBaseUrlEnabled" />
      </div>
      <div v-if="customBaseUrlEnabled" class="mt-3">
        <input
          v-model="customBaseUrl"
          type="text"
          class="input"
          :placeholder="t('admin.accounts.quotaControl.customBaseUrl.urlHint')"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// Shared "Anthropic OAuth/SetupToken quota control" section, used by both
// CreateAccountModal.vue and EditAccountModal.vue. The outer v-if condition
// (which differs slightly: form.platform/accountCategory vs account?.platform/
// account?.type) stays in each host template, wrapping this component.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import InlineToggleSwitch from '@/components/account/shared/InlineToggleSwitch.vue'

const { t } = useI18n()

const windowCostEnabled = defineModel<boolean>('windowCostEnabled', { required: true })
const windowCostLimit = defineModel<number | null>('windowCostLimit', { required: true })
const windowCostStickyReserve = defineModel<number | null>('windowCostStickyReserve', { required: true })
const sessionLimitEnabled = defineModel<boolean>('sessionLimitEnabled', { required: true })
const maxSessions = defineModel<number | null>('maxSessions', { required: true })
const sessionIdleTimeout = defineModel<number | null>('sessionIdleTimeout', { required: true })
const rpmLimitEnabled = defineModel<boolean>('rpmLimitEnabled', { required: true })
const baseRpm = defineModel<number | null>('baseRpm', { required: true })
const rpmStrategy = defineModel<string>('rpmStrategy', { required: true })
const rpmStickyBuffer = defineModel<number | null>('rpmStickyBuffer', { required: true })
const userMsgQueueMode = defineModel<string>('userMsgQueueMode', { required: true })
const sessionIdMaskingEnabled = defineModel<boolean>('sessionIdMaskingEnabled', { required: true })
const cacheTTLOverrideEnabled = defineModel<boolean>('cacheTTLOverrideEnabled', { required: true })
const cacheTTLOverrideTarget = defineModel<string>('cacheTTLOverrideTarget', { required: true })
const customBaseUrlEnabled = defineModel<boolean>('customBaseUrlEnabled', { required: true })
const customBaseUrl = defineModel<string>('customBaseUrl', { required: true })

const umqModeOptions = computed(() => [
  { value: '', label: t('admin.accounts.quotaControl.rpmLimit.umqModeOff') },
  { value: 'throttle', label: t('admin.accounts.quotaControl.rpmLimit.umqModeThrottle') },
  { value: 'serialize', label: t('admin.accounts.quotaControl.rpmLimit.umqModeSerialize') }
])
</script>
