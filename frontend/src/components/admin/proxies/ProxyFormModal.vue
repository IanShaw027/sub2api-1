<template>
  <BaseDialog
    :show="open"
    :title="isEdit ? t('admin.proxies.editProxy') : t('admin.proxies.createProxy')"
    width="normal"
    @close="$emit('close')"
  >
    <template v-if="!isEdit">
      <!-- Tab Switch -->
      <div class="mb-6 flex items-center justify-between gap-3 border-b border-line">
        <div class="flex min-w-0 shrink-0">
          <button
            type="button"
            :class="['-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors', createMode === 'standard' ? 'border-accent text-accent' : 'border-transparent text-muted hover:text-foreground']"
            @click="createMode = 'standard'"
          >
            <Icon name="plus" size="sm" class="mr-1.5 inline" />
            {{ t('admin.proxies.standardAdd') }}
          </button>
          <button
            type="button"
            :class="['-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors', createMode === 'batch' ? 'border-accent text-accent' : 'border-transparent text-muted hover:text-foreground']"
            @click="createMode = 'batch'"
          >
            <svg
              class="mr-1.5 inline h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z"
              />
            </svg>
            {{ t('admin.proxies.batchAdd') }}
          </button>
        </div>
        <ProxyAdBanner />
      </div>
    </template>

    <!-- Standard Add / Edit Form -->
    <form
      v-if="isEdit || createMode === 'standard'"
      id="proxy-form"
      class="space-y-5"
      @submit.prevent="isEdit ? handleUpdateProxy() : handleCreateProxy()"
    >
      <div>
        <label class="input-label">{{ t('admin.proxies.name') }}</label>
        <input
          v-model="form.name"
          type="text"
          required
          class="input"
          :placeholder="t('admin.proxies.enterProxyName')"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.protocol') }}</label>
        <Select v-model="form.protocol" :options="protocolSelectOptions" />
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="input-label">{{ t('admin.proxies.host') }}</label>
          <input
            v-model="form.host"
            type="text"
            required
            :placeholder="t('admin.proxies.form.hostPlaceholder')"
            class="input"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.port') }}</label>
          <input
            v-model.number="form.port"
            type="number"
            required
            min="1"
            max="65535"
            :placeholder="t('admin.proxies.form.portPlaceholder')"
            class="input"
          />
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.username') }}</label>
        <input
          v-model="form.username"
          type="text"
          class="input"
          :placeholder="t('admin.proxies.optionalAuth')"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.password') }}</label>
        <div class="relative">
          <input
            v-model="form.password"
            :type="passwordVisible ? 'text' : 'password'"
            class="input pr-10"
            :placeholder="isEdit ? t('admin.proxies.leaveEmptyToKeep') : t('admin.proxies.optionalAuth')"
            @input="passwordDirty = true"
          />
          <button
            type="button"
            class="absolute right-3 top-1/2 -translate-y-1/2 text-muted hover:text-foreground"
            @click="passwordVisible = !passwordVisible"
          >
            <Icon :name="passwordVisible ? 'eyeOff' : 'eye'" size="md" />
          </button>
        </div>
      </div>
      <div v-if="isEdit">
        <label class="input-label">{{ t('admin.proxies.status') }}</label>
        <Select v-model="form.status" :options="editStatusOptions" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.expiresAt') }}</label>
        <div class="mb-2 flex flex-wrap gap-2">
          <button
            v-for="d in EXPIRY_PRESETS"
            :key="d"
            type="button"
            class="btn btn-sm"
            :class="form.expires_at === addDaysToBase(baseDate, d) ? 'btn-primary' : 'btn-secondary'"
            @click="expiresDays = d"
          >
            {{ t('admin.proxies.nDays', { days: d }) }}
          </button>
        </div>
        <input
          v-model.number="expiresDays"
          type="number"
          min="0"
          class="input mb-2"
          :placeholder="t('admin.proxies.expiryDaysPlaceholder')"
        />
        <input v-model="form.expires_at" type="date" class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.proxies.fallbackMode') }}</label>
        <Select
          v-model="form.fallback_mode"
          :options="[
            { label: t('admin.proxies.fallbackNone'), value: 'none' },
            { label: t('admin.proxies.fallbackProxy'), value: 'proxy' },
            { label: t('admin.proxies.fallbackDirect'), value: 'direct' }
          ]"
        />
      </div>
      <div v-if="form.fallback_mode === 'proxy'">
        <label class="input-label">{{ t('admin.proxies.backupProxy') }}</label>
        <Select v-model="form.backup_proxy_id" :options="backupProxyOptions" />
      </div>
    </form>

    <!-- Batch Add Form -->
    <div v-else class="space-y-5">
      <div>
        <label class="input-label">{{ t('admin.proxies.batchInput') }}</label>
        <textarea
          v-model="batchInput"
          rows="10"
          class="input font-mono text-sm"
          :placeholder="t('admin.proxies.batchInputPlaceholder')"
          @input="parseBatchInput"
        ></textarea>
        <p class="input-hint mt-2">
          {{ t('admin.proxies.batchInputHint') }}
        </p>
      </div>

      <!-- Parse Result -->
      <div v-if="batchParseResult.total > 0" class="rounded-lg bg-surface-2 p-4">
        <div class="flex items-center gap-4 text-sm">
          <div class="flex items-center gap-1.5">
            <Icon name="checkCircle" size="sm" :stroke-width="2" class="text-accent" />
            <span class="text-foreground">
              {{ t('admin.proxies.parsedCount', { count: batchParseResult.valid }) }}
            </span>
          </div>
          <div v-if="batchParseResult.invalid > 0" class="flex items-center gap-1.5">
            <Icon name="exclamationCircle" size="sm" :stroke-width="2" class="text-warning-text" />
            <span class="text-warning-text">
              {{ t('admin.proxies.invalidCount', { count: batchParseResult.invalid }) }}
            </span>
          </div>
          <div v-if="batchParseResult.duplicate > 0" class="flex items-center gap-1.5">
            <svg
              class="h-4 w-4 text-muted"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 01-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 011.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 00-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 01-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 00-3.375-3.375h-1.5a1.125 1.125 0 01-1.125-1.125v-1.5a3.375 3.375 0 00-3.375-3.375H9.75"
              />
            </svg>
            <span class="text-muted">
              {{ t('admin.proxies.duplicateCount', { count: batchParseResult.duplicate }) }}
            </span>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn-glass-secondary" @click="$emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button
          v-if="isEdit"
          type="submit"
          form="proxy-form"
          :disabled="submitting"
          class="btn-glass-primary"
        >
          <svg
            v-if="submitting"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          {{ submitting ? t('admin.proxies.updating') : t('common.update') }}
        </button>
        <button
          v-else-if="createMode === 'standard'"
          type="submit"
          form="proxy-form"
          :disabled="submitting"
          class="btn-glass-primary"
        >
          <svg
            v-if="submitting"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          {{ submitting ? t('admin.proxies.creating') : t('common.create') }}
        </button>
        <button
          v-else
          type="button"
          :disabled="submitting || batchParseResult.valid === 0"
          class="btn-glass-primary"
          @click="handleBatchCreate"
        >
          <svg
            v-if="submitting"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          {{
            submitting
              ? t('admin.proxies.importing')
              : t('admin.proxies.importProxies', { count: batchParseResult.valid })
          }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Proxy, ProxyProtocol } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import ProxyAdBanner from '@/components/common/ProxyAdBanner.vue'

const props = defineProps<{
  open: boolean
  editingProxy: Proxy | null
  backupProxies: Proxy[]
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const isEdit = computed(() => !!props.editingProxy)
const submitting = ref(false)
const passwordVisible = ref(false)
const passwordDirty = ref(false)
const createMode = ref<'standard' | 'batch'>('standard')
const batchInput = ref('')
const batchParseResult = reactive({
  total: 0,
  valid: 0,
  invalid: 0,
  duplicate: 0,
  proxies: [] as Array<{
    protocol: ProxyProtocol
    host: string
    port: number
    username: string
    password: string
  }>
})

const protocolSelectOptions = computed(() => [
  { value: 'http', label: t('admin.proxies.protocols.http') },
  { value: 'https', label: t('admin.proxies.protocols.https') },
  { value: 'socks5', label: t('admin.proxies.protocols.socks5') },
  { value: 'socks5h', label: t('admin.proxies.protocols.socks5h') }
])

const editStatusOptions = computed(() => [
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') }
])

const backupProxyOptions = computed(() =>
  props.backupProxies
    .filter((p) => p.id !== props.editingProxy?.id)
    .map((p) => ({ label: `${p.name} (${p.host}:${p.port})`, value: p.id }))
)

const emptyForm = () => ({
  name: '',
  protocol: 'http' as ProxyProtocol,
  host: '',
  port: 8080,
  username: '',
  password: '',
  status: 'active' as 'active' | 'inactive' | 'expired',
  expires_at: '' as string,
  fallback_mode: 'none' as 'none' | 'proxy' | 'direct',
  backup_proxy_id: null as number | null,
  expiry_warn_days: 7 as number
})

const form = reactive(emptyForm())

// 有效期「选天数」⇄ 日历联动:天数自 base 起算(创建=今天;编辑=代理创建日)
const EXPIRY_PRESETS = [7, 30, 90, 180]
const toLocalDateStr = (dt: Date): string => {
  const y = dt.getFullYear()
  const m = String(dt.getMonth() + 1).padStart(2, '0')
  const d = String(dt.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}
const baseDateOrToday = (baseDateStr: string): Date => {
  const base = baseDateStr ? new Date(`${baseDateStr}T00:00:00`) : new Date()
  base.setHours(0, 0, 0, 0)
  return base
}
const addDaysToBase = (baseDateStr: string, n: number | null): string => {
  const days = Number(n)
  if (!days || days <= 0) return ''
  const dt = baseDateOrToday(baseDateStr)
  dt.setDate(dt.getDate() + days)
  return toLocalDateStr(dt)
}
const daysFromBase = (baseDateStr: string, targetDateStr: string): number | null => {
  if (!targetDateStr) return null
  const target = new Date(`${targetDateStr}T00:00:00`)
  return Math.round((target.getTime() - baseDateOrToday(baseDateStr).getTime()) / 86400000)
}

const baseDate = computed(() =>
  props.editingProxy?.created_at ? props.editingProxy.created_at.slice(0, 10) : ''
)
const expiresDays = computed<number | null>({
  get: () => daysFromBase(baseDate.value, form.expires_at),
  set: (v) => {
    form.expires_at = addDaysToBase(baseDate.value, v)
  }
})

const resetCreateForm = () => {
  Object.assign(form, emptyForm())
  createMode.value = 'standard'
  batchInput.value = ''
  batchParseResult.total = 0
  batchParseResult.valid = 0
  batchParseResult.invalid = 0
  batchParseResult.duplicate = 0
  batchParseResult.proxies = []
  passwordVisible.value = false
  passwordDirty.value = false
}

const populateEditForm = (proxy: Proxy) => {
  form.name = proxy.name
  form.protocol = proxy.protocol
  form.host = proxy.host
  form.port = proxy.port
  form.username = proxy.username || ''
  form.password = proxy.password || ''
  form.status = proxy.status === 'expired' ? 'inactive' : proxy.status
  form.expires_at = proxy.expires_at ? proxy.expires_at.slice(0, 10) : ''
  form.fallback_mode = proxy.fallback_mode || 'none'
  form.backup_proxy_id = proxy.backup_proxy_id ?? null
  form.expiry_warn_days = proxy.expiry_warn_days ?? 7
  passwordVisible.value = false
  passwordDirty.value = false
}

watch(
  () => [props.open, props.editingProxy] as const,
  ([open, editingProxy]) => {
    if (!open) return
    if (editingProxy) {
      populateEditForm(editingProxy)
    } else {
      resetCreateForm()
    }
  },
  { immediate: true }
)

// Parse proxy URL: protocol://user:pass@host:port or protocol://host:port
// Host may be a domain, IPv4, or bracketed IPv6 ([2001:db8::1]).
const parseProxyUrl = (
  line: string
): {
  protocol: ProxyProtocol
  host: string
  port: number
  username: string
  password: string
} | null => {
  const trimmed = line.trim()
  if (!trimmed) return null

  const regex =
    /^(https?|socks5h?):\/\/(?:([^:@\[\]]+):([^@\[\]]+)@)?(\[[0-9a-f:.]+\]|[^:\[\]]+):(\d+)$/i
  const match = trimmed.match(regex)
  if (!match) return null

  const [, protocol, username, password, rawHost, port] = match
  const portNum = parseInt(port, 10)
  if (portNum < 1 || portNum > 65535) return null

  const host = rawHost.replace(/^\[|\]$/g, '').trim()

  return {
    protocol: protocol.toLowerCase() as ProxyProtocol,
    host,
    port: portNum,
    username: username?.trim() || '',
    password: password?.trim() || ''
  }
}

const parseBatchInput = () => {
  const lines = batchInput.value.split('\n').filter((l) => l.trim())
  const seen = new Set<string>()
  const proxies: typeof batchParseResult.proxies = []
  let invalid = 0
  let duplicate = 0

  for (const line of lines) {
    const parsed = parseProxyUrl(line)
    if (!parsed) {
      invalid++
      continue
    }
    const key = `${parsed.host}:${parsed.port}:${parsed.username}:${parsed.password}`
    if (seen.has(key)) {
      duplicate++
      continue
    }
    seen.add(key)
    proxies.push(parsed)
  }

  batchParseResult.total = lines.length
  batchParseResult.valid = proxies.length
  batchParseResult.invalid = invalid
  batchParseResult.duplicate = duplicate
  batchParseResult.proxies = proxies
}

const handleBatchCreate = async () => {
  if (batchParseResult.valid === 0) return
  submitting.value = true
  try {
    const result = await adminAPI.proxies.batchCreate(batchParseResult.proxies)
    const created = result.created || 0
    const skipped = result.skipped || 0
    if (created > 0) {
      appStore.showSuccess(t('admin.proxies.batchImportSuccess', { created, skipped }))
    } else {
      appStore.showInfo(t('admin.proxies.batchImportAllSkipped', { skipped }))
    }
    emit('saved')
    emit('close')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToImport'))
    console.error('Error batch creating proxies:', error)
  } finally {
    submitting.value = false
  }
}

const handleCreateProxy = async () => {
  if (!form.name.trim()) {
    appStore.showError(t('admin.proxies.nameRequired'))
    return
  }
  if (!form.host.trim()) {
    appStore.showError(t('admin.proxies.hostRequired'))
    return
  }
  if (form.port < 1 || form.port > 65535) {
    appStore.showError(t('admin.proxies.portInvalid'))
    return
  }
  submitting.value = true
  try {
    await adminAPI.proxies.create({
      name: form.name.trim(),
      protocol: form.protocol,
      host: form.host.trim(),
      port: form.port,
      username: form.username.trim() || null,
      password: form.password.trim() || null,
      expires_at: form.expires_at ? Math.floor(new Date(form.expires_at).getTime() / 1000) : null,
      fallback_mode: form.fallback_mode,
      backup_proxy_id: form.fallback_mode === 'proxy' ? form.backup_proxy_id : null,
      expiry_warn_days: form.expiry_warn_days
    })
    appStore.showSuccess(t('admin.proxies.proxyCreated'))
    emit('saved')
    emit('close')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToCreate'))
    console.error('Error creating proxy:', error)
  } finally {
    submitting.value = false
  }
}

const handleUpdateProxy = async () => {
  if (!props.editingProxy) return
  if (!form.name.trim()) {
    appStore.showError(t('admin.proxies.nameRequired'))
    return
  }
  if (!form.host.trim()) {
    appStore.showError(t('admin.proxies.hostRequired'))
    return
  }
  if (form.port < 1 || form.port > 65535) {
    appStore.showError(t('admin.proxies.portInvalid'))
    return
  }

  submitting.value = true
  try {
    const updateData: any = {
      name: form.name.trim(),
      protocol: form.protocol,
      host: form.host.trim(),
      port: form.port,
      username: form.username.trim() || null,
      status: form.status,
      expires_at: form.expires_at ? Math.floor(new Date(form.expires_at).getTime() / 1000) : null,
      fallback_mode: form.fallback_mode,
      backup_proxy_id: form.fallback_mode === 'proxy' ? form.backup_proxy_id : null,
      expiry_warn_days: form.expiry_warn_days
    }
    if (passwordDirty.value) {
      updateData.password = form.password.trim() || null
    }

    await adminAPI.proxies.update(props.editingProxy.id, updateData)
    appStore.showSuccess(t('admin.proxies.proxyUpdated'))
    emit('saved')
    emit('close')
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToUpdate'))
    console.error('Error updating proxy:', error)
  } finally {
    submitting.value = false
  }
}
</script>
