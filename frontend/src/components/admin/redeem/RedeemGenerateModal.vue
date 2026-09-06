<template>
  <UiModal
    :open="open"
    :title="t('admin.redeem.generateCodesTitle')"
    width="md"
    :close-label="t('common.cancel')"
    @close="$emit('close')"
  >
    <form id="generate-redeem-form" class="space-y-5" @submit.prevent="handleGenerateCodes">
      <div>
        <label class="input-label">{{ t('admin.redeem.codeType') }}</label>
        <Select v-model="generateForm.type" :options="typeOptions" />
      </div>

      <div v-if="generateForm.type !== 'subscription' && generateForm.type !== 'invitation'">
        <label class="input-label">
          {{ generateForm.type === 'balance' ? t('admin.redeem.amount') : t('admin.redeem.columns.value') }}
        </label>
        <input
          v-model.number="generateForm.value"
          type="number"
          :step="generateForm.type === 'balance' ? '0.01' : '1'"
          :min="generateForm.type === 'balance' ? '0.01' : '1'"
          required
          class="field"
        />
      </div>

      <div v-if="generateForm.type === 'invitation'" class="notice notice-info">
        {{ t('admin.redeem.invitationHint') }}
      </div>

      <template v-if="generateForm.type === 'subscription'">
        <div>
          <label class="input-label">{{ t('admin.redeem.selectGroup') }}</label>
          <Select
            v-model="generateForm.group_id"
            :options="subscriptionGroupOptions"
            :placeholder="t('admin.redeem.selectGroupPlaceholder')"
          >
            <template #selected="{ option }">
              <GroupBadge
                v-if="option"
                :name="(option as unknown as GroupOption).label"
                :platform="(option as unknown as GroupOption).platform"
                :subscription-type="(option as unknown as GroupOption).subscriptionType"
                :rate-multiplier="(option as unknown as GroupOption).rate"
              />
              <span v-else class="text-muted">{{ t('admin.redeem.selectGroupPlaceholder') }}</span>
            </template>
            <template #option="{ option, selected }">
              <GroupOptionItem
                :name="(option as unknown as GroupOption).label"
                :platform="(option as unknown as GroupOption).platform"
                :subscription-type="(option as unknown as GroupOption).subscriptionType"
                :rate-multiplier="(option as unknown as GroupOption).rate"
                :description="(option as unknown as GroupOption).description"
                :selected="selected"
              />
            </template>
          </Select>
        </div>
        <div>
          <label class="input-label">{{ t('admin.redeem.validityDays') }}</label>
          <input v-model.number="generateForm.validity_days" type="number" min="1" max="365" required class="field" />
        </div>
      </template>

      <div>
        <label class="input-label">{{ t('admin.redeem.codeExpiry') }}</label>
        <SegmentedControl v-model="generateForm.expiry_option" :options="redeemCodeExpiryOptions" />
        <input
          v-if="generateForm.expiry_option === 'custom'"
          v-model.number="generateForm.custom_expiry_days"
          type="number"
          min="1"
          max="3650"
          required
          class="field mt-2"
          :placeholder="t('admin.redeem.customExpiryDays')"
        />
      </div>

      <div>
        <label class="input-label">{{ t('admin.redeem.count') }}</label>
        <input v-model.number="generateForm.count" type="number" min="1" max="100" required class="field" />
      </div>
    </form>

    <template #footer>
      <button type="button" class="btn btn-secondary" @click="$emit('close')">
        {{ t('common.cancel') }}
      </button>
      <button type="submit" form="generate-redeem-form" :disabled="generating" class="btn btn-primary">
        {{ generating ? t('admin.redeem.generating') : t('admin.redeem.generate') }}
      </button>
    </template>
  </UiModal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Group, GroupPlatform, RedeemCode, RedeemCodeType, SubscriptionType } from '@/types'
import UiModal from '@/components/ui/UiModal.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import Select from '@/components/common/Select.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'

interface GroupOption {
  value: number
  label: string
  description: string | null
  platform: GroupPlatform
  subscriptionType: SubscriptionType
  rate: number
}

const props = defineProps<{
  open: boolean
  subscriptionGroups: Group[]
}>()

const emit = defineEmits<{
  close: []
  generated: [codes: RedeemCode[]]
}>()

const { t } = useI18n()
const appStore = useAppStore()

const generating = ref(false)

const generateForm = reactive({
  type: 'balance' as RedeemCodeType,
  value: 10,
  count: 1,
  group_id: null as number | null,
  validity_days: 30,
  expiry_option: 'never' as RedeemCodeExpiryOption,
  custom_expiry_days: 7
})

type RedeemCodeExpiryOption = 'never' | '1' | '3' | '7' | 'custom'

const typeOptions = computed(() => [
  { value: 'balance', label: t('admin.redeem.balance') },
  { value: 'concurrency', label: t('admin.redeem.concurrency') },
  { value: 'subscription', label: t('admin.redeem.subscription') },
  { value: 'invitation', label: t('admin.redeem.invitation') }
])

const subscriptionGroupOptions = computed(() =>
  props.subscriptionGroups
    .filter((g) => g.subscription_type === 'subscription')
    .map((g) => ({
      value: g.id,
      label: g.name,
      description: g.description,
      platform: g.platform,
      subscriptionType: g.subscription_type,
      rate: g.rate_multiplier
    }))
)

const redeemCodeExpiryOptions = computed<{ value: RedeemCodeExpiryOption; label: string }[]>(() => [
  { value: 'never', label: t('admin.redeem.neverExpires') },
  { value: '1', label: t('admin.redeem.expiryPresetDays', { days: 1 }) },
  { value: '3', label: t('admin.redeem.expiryPresetDays', { days: 3 }) },
  { value: '7', label: t('admin.redeem.expiryPresetDays', { days: 7 }) },
  { value: 'custom', label: t('admin.redeem.customExpiry') }
])

// 监听类型变化，邀请码类型时自动设置 value 为 0
watch(
  () => generateForm.type,
  (newType) => {
    if (newType === 'invitation') {
      generateForm.value = 0
    } else if (generateForm.value === 0) {
      generateForm.value = 10
    }
  }
)

const getRedeemCodeExpiresInDays = () => {
  if (generateForm.expiry_option === 'never') {
    return undefined
  }
  if (generateForm.expiry_option === 'custom') {
    if (
      !Number.isFinite(generateForm.custom_expiry_days) ||
      generateForm.custom_expiry_days < 1
    ) {
      return null
    }
    return Math.floor(generateForm.custom_expiry_days)
  }
  return Number(generateForm.expiry_option)
}

const handleGenerateCodes = async () => {
  // 订阅类型必须选择分组
  if (generateForm.type === 'subscription' && !generateForm.group_id) {
    appStore.showError(t('admin.redeem.groupRequired'))
    return
  }

  const expiresInDays = getRedeemCodeExpiresInDays()
  if (expiresInDays === null) {
    appStore.showError(t('admin.redeem.expiryDaysRequired'))
    return
  }

  generating.value = true
  try {
    const result = await adminAPI.redeem.generate(
      generateForm.count,
      generateForm.type,
      generateForm.value,
      generateForm.type === 'subscription' ? generateForm.group_id : undefined,
      generateForm.type === 'subscription' ? generateForm.validity_days : undefined,
      expiresInDays
    )
    // 重置表单
    generateForm.group_id = null
    generateForm.validity_days = 30
    generateForm.expiry_option = 'never'
    generateForm.custom_expiry_days = 7
    emit('generated', result)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToGenerate'))
    console.error('Error generating codes:', error)
  } finally {
    generating.value = false
  }
}
</script>
