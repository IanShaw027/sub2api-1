<template>
  <BaseDialog
    :show="open"
    :title="t('admin.proxies.accountsTitle', { name: proxy?.name || '' })"
    width="normal"
    @close="$emit('close')"
  >
    <div v-if="loading" class="flex items-center justify-center py-8 text-sm text-muted">
      <Icon name="refresh" size="md" class="mr-2 animate-spin" />
      {{ t('common.loading') }}
    </div>
    <div v-else-if="accounts.length === 0" class="py-6 text-center text-sm text-muted">
      {{ t('admin.proxies.accountsEmpty') }}
    </div>
    <div v-else class="max-h-80 overflow-auto">
      <table class="min-w-full divide-y divide-line text-sm">
        <thead class="bg-surface-2 text-xs uppercase text-muted">
          <tr>
            <th class="px-4 py-2 text-left">{{ t('admin.proxies.accountName') }}</th>
            <th class="px-4 py-2 text-left">{{ t('admin.accounts.columns.platformType') }}</th>
            <th class="px-4 py-2 text-left">{{ t('admin.proxies.accountNotes') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-line bg-surface">
          <tr v-for="account in accounts" :key="account.id">
            <td class="px-4 py-2 font-medium text-foreground">{{ account.name }}</td>
            <td class="px-4 py-2">
              <PlatformTypeBadge :platform="account.platform" :type="account.type" />
            </td>
            <td class="px-4 py-2 text-muted">
              {{ account.notes || '-' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn-glass-secondary" @click="$emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Proxy, ProxyAccountSummary } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'

const props = defineProps<{
  open: boolean
  proxy: Proxy | null
}>()

defineEmits<{
  close: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const accounts = ref<ProxyAccountSummary[]>([])
const loading = ref(false)

watch(
  () => [props.open, props.proxy] as const,
  async ([open, proxy]) => {
    if (!open || !proxy) {
      accounts.value = []
      return
    }
    accounts.value = []
    loading.value = true
    try {
      accounts.value = await adminAPI.proxies.getProxyAccounts(proxy.id)
    } catch (error: any) {
      appStore.showError(error.response?.data?.detail || t('admin.proxies.accountsFailed'))
      console.error('Error loading proxy accounts:', error)
    } finally {
      loading.value = false
    }
  },
  { immediate: true }
)
</script>
