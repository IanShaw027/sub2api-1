<template>
  <Teleport to="body">
    <div v-if="show && position">
      <!-- Backdrop: click anywhere outside to close -->
      <div class="fixed inset-0 z-40" @click="emit('close')"></div>
      <div
        class="action-menu-content fixed z-[45] w-52 overflow-hidden rounded-xl bg-white shadow-lg ring-1 ring-black/5 dark:bg-dark-800"
        :style="{ top: position.top + 'px', left: position.left + 'px' }"
        @click.stop
      >
        <div class="py-1">
          <template v-if="account">
            <button @click="$emit('test', account); $emit('close')" class="flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="play" size="sm" class="text-green-500" :stroke-width="2" />
              {{ t('admin.accounts.testConnection') }}
            </button>
            <button @click="$emit('stats', account); $emit('close')" class="flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="chart" size="sm" class="text-indigo-500" />
              {{ t('admin.accounts.viewStats') }}
            </button>
            <button @click="$emit('schedule', account); $emit('close')" class="flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="clock" size="sm" class="text-orange-500" />
              {{ t('admin.scheduledTests.schedule') }}
            </button>
            <template v-if="account.type === 'oauth' || account.type === 'setup-token'">
              <button
                v-if="supportsReauth || isKiroOAuth"
                @click="$emit('reauth', account); $emit('close')"
                :class="[
                  'flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700',
                  isKiroOAuth ? 'text-cyan-600' : 'text-blue-600'
                ]"
              >
                <Icon name="link" size="sm" />
                {{ t('admin.accounts.reAuthorize') }}
              </button>
              <button @click="$emit('refresh-token', account); $emit('close')" class="flex w-full items-center gap-2 px-4 py-2 text-sm text-purple-600 hover:bg-gray-100 dark:hover:bg-dark-700">
                <Icon name="refresh" size="sm" />
                {{ t('admin.accounts.refreshToken') }}
              </button>
            </template>
            <button v-if="supportsPrivacy" @click="$emit('set-privacy', account); $emit('close')" class="flex w-full items-center gap-2 px-4 py-2 text-sm text-emerald-600 hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="shield" size="sm" />
              {{ t('admin.accounts.setPrivacy') }}
            </button>
            <div v-if="hasRecoverableState" class="my-1 border-t border-gray-100 dark:border-dark-700"></div>
            <button v-if="hasRecoverableState" @click="$emit('recover-state', account); $emit('close')" class="flex w-full items-center gap-2 px-4 py-2 text-sm text-emerald-600 hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="sync" size="sm" />
              {{ t('admin.accounts.recoverState') }}
            </button>
            <button v-if="hasQuotaLimit" @click="$emit('reset-quota', account); $emit('close')" class="flex w-full items-center gap-2 px-4 py-2 text-sm text-teal-600 hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="refresh" size="sm" />
              {{ t('admin.accounts.resetQuota') }}
            </button>
          </template>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, watch, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import type { Account } from '@/types'

const props = defineProps<{ show: boolean; account: Account | null; position: { top: number; left: number } | null }>()
const emit = defineEmits(['close', 'test', 'stats', 'schedule', 'reauth', 'refresh-token', 'recover-state', 'reset-quota', 'set-privacy'])
const { t } = useI18n()
const now = ref(Date.now())
let clockTimer: ReturnType<typeof setInterval> | null = null

const startClock = () => {
  if (clockTimer !== null) return
  clockTimer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
}

const stopClock = () => {
  if (clockTimer === null) return
  clearInterval(clockTimer)
  clockTimer = null
}

const isRateLimited = computed(() => {
  if (props.account?.rate_limit_reset_at && new Date(props.account.rate_limit_reset_at).getTime() > now.value) {
    return true
  }
  const modelLimits = (props.account?.extra as Record<string, unknown> | undefined)?.model_rate_limits as
    | Record<string, { rate_limit_reset_at: string }>
    | undefined
  if (modelLimits) {
    return Object.values(modelLimits).some(info => new Date(info.rate_limit_reset_at).getTime() > now.value)
  }
  return false
})
const isOverloaded = computed(() => props.account?.overload_until && new Date(props.account.overload_until).getTime() > now.value)
const isTempUnschedulable = computed(() => props.account?.temp_unschedulable_until && new Date(props.account.temp_unschedulable_until).getTime() > now.value)
const hasRecoverableState = computed(() => {
  return props.account?.status === 'error' || Boolean(isRateLimited.value) || Boolean(isOverloaded.value) || Boolean(isTempUnschedulable.value)
})
const isAntigravityOAuth = computed(() => props.account?.platform === 'antigravity' && props.account?.type === 'oauth')
const isOpenAIOAuth = computed(() => props.account?.platform === 'openai' && props.account?.type === 'oauth')
const isKiroOAuth = computed(() => props.account?.platform === 'kiro' && props.account?.type === 'oauth')
const supportsReauth = computed(() => props.account?.type === 'oauth' || props.account?.type === 'setup-token')
const supportsPrivacy = computed(() => isAntigravityOAuth.value || isOpenAIOAuth.value)
const hasQuotaLimit = computed(() => {
  return (props.account?.type === 'apikey' || props.account?.type === 'bedrock') && (
    (props.account?.quota_limit ?? 0) > 0 ||
    (props.account?.quota_daily_limit ?? 0) > 0 ||
    (props.account?.quota_weekly_limit ?? 0) > 0
  )
})

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') emit('close')
}

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      startClock()
      window.addEventListener('keydown', handleKeydown)
    } else {
      stopClock()
      window.removeEventListener('keydown', handleKeydown)
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  stopClock()
  window.removeEventListener('keydown', handleKeydown)
})
</script>
