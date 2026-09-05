<template>
  <div class="border-t border-line pt-4 space-y-4">
    <div class="mb-3">
      <h3 class="input-label mb-0 text-base font-semibold">{{ t('admin.accounts.quotaControl.title') }}</h3>
      <p class="mt-1 text-xs text-muted">
        {{ hint }}
      </p>
    </div>
    <QuotaLimitCard
      :totalLimit="editQuotaLimit"
      :dailyLimit="editQuotaDailyLimit"
      :weeklyLimit="editQuotaWeeklyLimit"
      :quotaNotifyGlobalEnabled="quotaNotifyGlobalEnabled"
      :quotaNotifyDailyEnabled="quotaNotifyDailyEnabled"
      :quotaNotifyDailyThreshold="quotaNotifyDailyThreshold"
      :quotaNotifyDailyThresholdType="quotaNotifyDailyThresholdType"
      :quotaNotifyWeeklyEnabled="quotaNotifyWeeklyEnabled"
      :quotaNotifyWeeklyThreshold="quotaNotifyWeeklyThreshold"
      :quotaNotifyWeeklyThresholdType="quotaNotifyWeeklyThresholdType"
      :quotaNotifyTotalEnabled="quotaNotifyTotalEnabled"
      :quotaNotifyTotalThreshold="quotaNotifyTotalThreshold"
      :quotaNotifyTotalThresholdType="quotaNotifyTotalThresholdType"
      :dailyResetMode="editDailyResetMode"
      :dailyResetHour="editDailyResetHour"
      :weeklyResetMode="editWeeklyResetMode"
      :weeklyResetDay="editWeeklyResetDay"
      :weeklyResetHour="editWeeklyResetHour"
      :resetTimezone="editResetTimezone"
      @update:totalLimit="editQuotaLimit = $event"
      @update:dailyLimit="editQuotaDailyLimit = $event"
      @update:weeklyLimit="editQuotaWeeklyLimit = $event"
      @update:quotaNotifyDailyEnabled="quotaNotifyDailyEnabled = $event"
      @update:quotaNotifyDailyThreshold="quotaNotifyDailyThreshold = $event"
      @update:quotaNotifyDailyThresholdType="quotaNotifyDailyThresholdType = $event"
      @update:quotaNotifyWeeklyEnabled="quotaNotifyWeeklyEnabled = $event"
      @update:quotaNotifyWeeklyThreshold="quotaNotifyWeeklyThreshold = $event"
      @update:quotaNotifyWeeklyThresholdType="quotaNotifyWeeklyThresholdType = $event"
      @update:quotaNotifyTotalEnabled="quotaNotifyTotalEnabled = $event"
      @update:quotaNotifyTotalThreshold="quotaNotifyTotalThreshold = $event"
      @update:quotaNotifyTotalThresholdType="quotaNotifyTotalThresholdType = $event"
      @update:dailyResetMode="editDailyResetMode = $event"
      @update:dailyResetHour="editDailyResetHour = $event"
      @update:weeklyResetMode="editWeeklyResetMode = $event"
      @update:weeklyResetDay="editWeeklyResetDay = $event"
      @update:weeklyResetHour="editWeeklyResetHour = $event"
      @update:resetTimezone="editResetTimezone = $event"
    />
  </div>
</template>

<script setup lang="ts">
// Shared wrapper around the existing <QuotaLimitCard>, used 4x (2 branches x
// 2 hosts) with byte-identical bindings between CreateAccountModal.vue and
// EditAccountModal.vue. The `hint` text differs per branch/host, so it stays
// a plain prop; the outer v-if/v-else-if condition stays in each host
// template. quotaNotifyState's leaf fields are exposed as individual
// defineModel props (rather than passing the whole reactive object, which
// would need in-place mutation of a prop and trip vue/no-mutating-props) —
// each host binds them as `v-model:quota-notify-daily-enabled=
// "quotaNotifyState.daily.enabled"`, etc., which is functionally identical
// to the original inline `@update:...="quotaNotifyState.daily.enabled =
// $event"` handlers.
import { useI18n } from 'vue-i18n'
import QuotaLimitCard from '@/components/account/QuotaLimitCard.vue'
import type { QuotaThresholdType, QuotaResetMode } from '@/constants/account'

defineProps<{
  hint: string
  quotaNotifyGlobalEnabled: boolean
}>()

const { t } = useI18n()

const editQuotaLimit = defineModel<number | null>('editQuotaLimit', { required: true })
const editQuotaDailyLimit = defineModel<number | null>('editQuotaDailyLimit', { required: true })
const editQuotaWeeklyLimit = defineModel<number | null>('editQuotaWeeklyLimit', { required: true })
const editDailyResetMode = defineModel<QuotaResetMode | null>('editDailyResetMode', { required: true })
const editDailyResetHour = defineModel<number | null>('editDailyResetHour', { required: true })
const editWeeklyResetMode = defineModel<QuotaResetMode | null>('editWeeklyResetMode', { required: true })
const editWeeklyResetDay = defineModel<number | null>('editWeeklyResetDay', { required: true })
const editWeeklyResetHour = defineModel<number | null>('editWeeklyResetHour', { required: true })
const editResetTimezone = defineModel<string | null>('editResetTimezone', { required: true })

const quotaNotifyDailyEnabled = defineModel<boolean | null>('quotaNotifyDailyEnabled', { required: true })
const quotaNotifyDailyThreshold = defineModel<number | null>('quotaNotifyDailyThreshold', { required: true })
const quotaNotifyDailyThresholdType = defineModel<QuotaThresholdType | null>('quotaNotifyDailyThresholdType', { required: true })
const quotaNotifyWeeklyEnabled = defineModel<boolean | null>('quotaNotifyWeeklyEnabled', { required: true })
const quotaNotifyWeeklyThreshold = defineModel<number | null>('quotaNotifyWeeklyThreshold', { required: true })
const quotaNotifyWeeklyThresholdType = defineModel<QuotaThresholdType | null>('quotaNotifyWeeklyThresholdType', { required: true })
const quotaNotifyTotalEnabled = defineModel<boolean | null>('quotaNotifyTotalEnabled', { required: true })
const quotaNotifyTotalThreshold = defineModel<number | null>('quotaNotifyTotalThreshold', { required: true })
const quotaNotifyTotalThresholdType = defineModel<QuotaThresholdType | null>('quotaNotifyTotalThresholdType', { required: true })
</script>
