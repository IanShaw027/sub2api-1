// Shared temp-unschedulable-rules state + logic used identically by
// CreateAccountModal.vue and EditAccountModal.vue. Extracted verbatim from
// both hosts (script bodies were byte-identical except for the
// createStableObjectKeyResolver key prefix, now passed in as `keyPrefix`).
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { createStableObjectKeyResolver } from '@/utils/stableObjectKey'

export interface TempUnschedRuleForm {
  error_code: number | null
  keywords: string
  duration_minutes: number | null
  description: string
}

export function useTempUnschedRules(keyPrefix: string) {
  const { t } = useI18n()
  const appStore = useAppStore()

  const tempUnschedEnabled = ref(false)
  const tempUnschedRules = ref<TempUnschedRuleForm[]>([])
  const getTempUnschedRuleKey = createStableObjectKeyResolver<TempUnschedRuleForm>(keyPrefix)

  const tempUnschedPresets = computed(() => [
    {
      label: t('admin.accounts.tempUnschedulable.presets.overloadLabel'),
      rule: {
        error_code: 529,
        keywords: 'overloaded, too many',
        duration_minutes: 60,
        description: t('admin.accounts.tempUnschedulable.presets.overloadDesc')
      }
    },
    {
      label: t('admin.accounts.tempUnschedulable.presets.rateLimitLabel'),
      rule: {
        error_code: 429,
        keywords: 'rate limit, too many requests',
        duration_minutes: 10,
        description: t('admin.accounts.tempUnschedulable.presets.rateLimitDesc')
      }
    },
    {
      label: t('admin.accounts.tempUnschedulable.presets.unavailableLabel'),
      rule: {
        error_code: 503,
        keywords: 'unavailable, maintenance',
        duration_minutes: 30,
        description: t('admin.accounts.tempUnschedulable.presets.unavailableDesc')
      }
    }
  ])

  const addTempUnschedRule = (preset?: TempUnschedRuleForm) => {
    if (preset) {
      tempUnschedRules.value.push({ ...preset })
      return
    }
    tempUnschedRules.value.push({
      error_code: null,
      keywords: '',
      duration_minutes: 30,
      description: ''
    })
  }

  const removeTempUnschedRule = (index: number) => {
    tempUnschedRules.value.splice(index, 1)
  }

  const moveTempUnschedRule = (index: number, direction: number) => {
    const target = index + direction
    if (target < 0 || target >= tempUnschedRules.value.length) return
    const rules = tempUnschedRules.value
    const current = rules[index]
    rules[index] = rules[target]
    rules[target] = current
  }

  const splitTempUnschedKeywords = (value: string) => {
    return value
      .split(/[,;]/)
      .map((item) => item.trim())
      .filter((item) => item.length > 0)
  }

  const buildTempUnschedRules = (rules: TempUnschedRuleForm[]) => {
    const out: Array<{
      error_code: number
      keywords: string[]
      duration_minutes: number
      description: string
    }> = []

    for (const rule of rules) {
      const errorCode = Number(rule.error_code)
      const duration = Number(rule.duration_minutes)
      const keywords = splitTempUnschedKeywords(rule.keywords)
      if (!Number.isFinite(errorCode) || errorCode < 100 || errorCode > 599) {
        continue
      }
      if (!Number.isFinite(duration) || duration <= 0) {
        continue
      }
      if (keywords.length === 0) {
        continue
      }
      out.push({
        error_code: Math.trunc(errorCode),
        keywords,
        duration_minutes: Math.trunc(duration),
        description: rule.description.trim()
      })
    }

    return out
  }

  const applyTempUnschedConfig = (credentials: Record<string, unknown>) => {
    if (!tempUnschedEnabled.value) {
      delete credentials.temp_unschedulable_enabled
      delete credentials.temp_unschedulable_rules
      return true
    }

    const rules = buildTempUnschedRules(tempUnschedRules.value)
    if (rules.length === 0) {
      appStore.showError(t('admin.accounts.tempUnschedulable.rulesInvalid'))
      return false
    }

    credentials.temp_unschedulable_enabled = true
    credentials.temp_unschedulable_rules = rules
    return true
  }

  return {
    tempUnschedEnabled,
    tempUnschedRules,
    getTempUnschedRuleKey,
    tempUnschedPresets,
    addTempUnschedRule,
    removeTempUnschedRule,
    moveTempUnschedRule,
    buildTempUnschedRules,
    applyTempUnschedConfig,
    splitTempUnschedKeywords
  }
}
