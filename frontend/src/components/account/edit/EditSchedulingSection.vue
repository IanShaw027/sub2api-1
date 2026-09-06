<template>
      <div
        v-if="supportsAccountSchedulingThresholdOverride"
        class="border-t border-line pt-4"
        data-testid="account-scheduling-threshold-section"
      >
        <div class="mb-3 flex items-center justify-between">
          <div>
            <label class="input-label mb-0">{{ t('admin.accounts.accountSchedulingThresholdOverride') }}</label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.accountSchedulingThresholdOverrideHint') }}
            </p>
          </div>
          <input
            v-model="accountSchedulingThresholdOverrideEnabled"
            data-testid="account-scheduling-threshold-override-enabled"
            type="checkbox"
            class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
          />
        </div>
        <div v-if="accountSchedulingThresholdOverrideEnabled">
          <label class="input-label">{{ t('admin.accounts.accountSchedulingThresholdOverrideValue') }}</label>
          <input
            v-model.number="accountSchedulingThresholdOverrideValue"
            data-testid="account-scheduling-threshold-override-value"
            type="number"
            min="1"
            max="100"
            class="input"
          />
          <p class="input-hint">{{ t('admin.accounts.accountSchedulingThresholdOverrideDisabledHint') }}</p>
        </div>
      </div>

      <InterceptWarmupRequestsSection
        :show="account?.platform === 'anthropic' || account?.platform === 'antigravity'"
        v-model:interceptWarmupRequests="interceptWarmupRequests"
      />

      <!-- eslint-disable vue/no-mutating-props -- `form` is a reactive object shared with the
           host by design; the bindings below set the object's own fields, not the prop binding
           itself (mirrors the pre-split direct `form.concurrency = ...` mutation). -->
      <div v-if="!isSparkShadow">
        <div class="mb-1 flex items-center gap-2">
          <label class="input-label mb-0">{{ t('admin.accounts.proxy') }}</label>
          <ProxyAdBanner />
        </div>
        <ProxySelector v-model="form.proxy_id" :proxies="proxies" />
      </div>

      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <div>
          <label class="input-label">{{ t('admin.accounts.concurrency') }}</label>
          <input v-model.number="form.concurrency" type="number" min="1" class="input"
            @input="form.concurrency = Math.max(1, form.concurrency || 1)" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.loadFactor') }}</label>
          <input v-model.number="form.load_factor" type="number" min="1"
            class="input" :placeholder="String(form.concurrency || 1)"
            @input="form.load_factor = (form.load_factor && form.load_factor >= 1) ? form.load_factor : null" />
          <p class="input-hint">{{ t('admin.accounts.loadFactorHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.priority') }}</label>
          <input
            v-model.number="form.priority"
            type="number"
            min="1"
            class="input"
            data-tour="account-form-priority"
          />
          <p class="input-hint">{{ t('admin.accounts.priorityHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.billingRateMultiplier') }}</label>
          <input
            v-model.number="form.rate_multiplier"
            type="number"
            min="0"
            step="0.001"
            class="input disabled:cursor-not-allowed disabled:opacity-60"
            data-testid="account-rate-multiplier"
            :disabled="upstreamBillingRateSyncEnabled"
          />
          <p class="input-hint">
            {{
              t(
                upstreamBillingRateSyncEnabled
                  ? 'admin.accounts.upstreamBilling.syncRateManagedHint'
                  : 'admin.accounts.billingRateMultiplierHint'
              )
            }}
          </p>
          <div
            v-if="account?.type === 'apikey'"
            class="mt-3 flex items-center justify-between gap-3"
          >
            <div class="min-w-0">
              <p class="text-xs font-medium text-foreground">
                {{ t('admin.accounts.upstreamBilling.syncRate') }}
              </p>
              <p class="mt-1 text-xs text-muted">
                {{ t('admin.accounts.upstreamBilling.syncRateHint') }}
              </p>
            </div>
            <Toggle
              :model-value="upstreamBillingRateSyncEnabled"
              data-testid="upstream-billing-rate-sync"
              :aria-label="t('admin.accounts.upstreamBilling.syncRate')"
              @update:model-value="handleUpstreamBillingRateSyncChange"
            />
          </div>
        </div>
      </div>
      <div class="border-t border-line pt-4">
        <label class="input-label">{{ t('admin.accounts.expiresAt') }}</label>
        <input v-model="expiresAtInput" type="datetime-local" class="input" />
        <p class="input-hint">
          {{ t('admin.accounts.expiresAtHint') }}
          {{ t('admin.accounts.expiresAtTimezoneHint', { timezone: browserTimeZone }) }}
        </p>
      </div>
      <!-- eslint-enable vue/no-mutating-props -->
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import ProxyAdBanner from '@/components/common/ProxyAdBanner.vue'
import InterceptWarmupRequestsSection from '@/components/account/shared/InterceptWarmupRequestsSection.vue'
import { formatDateTimeLocalInput, parseDateTimeLocalInput } from '@/utils/format'
import type { Account, Proxy } from '@/types'

interface Props {
  account: Account | null
  proxies: Proxy[]
  form: any
  isSparkShadow: boolean
  supportsAccountSchedulingThresholdOverride: boolean
  upstreamBillingRateSyncEnabled: boolean
  handleUpstreamBillingRateSyncChange: (enabled: boolean) => void
  browserTimeZone: string
}

const props = defineProps<Props>()

const accountSchedulingThresholdOverrideEnabled = defineModel<boolean>('accountSchedulingThresholdOverrideEnabled', { required: true })
const accountSchedulingThresholdOverrideValue = defineModel<number>('accountSchedulingThresholdOverrideValue', { required: true })
const interceptWarmupRequests = defineModel<boolean>('interceptWarmupRequests', { required: true })

const { t } = useI18n()

const expiresAtInput = computed({
  get: () => formatDateTimeLocalInput(props.form.expires_at),
  set: (value: string) => {
    /* eslint-disable-next-line vue/no-mutating-props -- `form` is a reactive object shared with
       the host by design; this sets the object's own field, not the prop binding itself
       (mirrors the pre-split direct `form.expires_at = ...` mutation). */
    props.form.expires_at = parseDateTimeLocalInput(value)
  }
})
</script>
