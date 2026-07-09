<template>
  <BaseDialog
    :show="show"
    :title="t('admin.tlsFingerprintRouters.title')"
    width="wide"
    @close="$emit('close')"
  >
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.tlsFingerprintRouters.description') }}
        </p>
        <button @click="openCreate" class="btn btn-primary btn-sm">
          <Icon name="plus" size="sm" class="mr-1" />
          {{ t('admin.tlsFingerprintRouters.createRouter') }}
        </button>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-8">
        <Icon name="refresh" size="lg" class="animate-spin text-gray-400" />
      </div>

      <div v-else-if="routers.length === 0" class="py-8 text-center">
        <div class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-gray-100 dark:bg-dark-700">
          <Icon name="shield" size="lg" class="text-gray-400" />
        </div>
        <h4 class="mb-1 text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.tlsFingerprintRouters.noRouters') }}
        </h4>
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.tlsFingerprintRouters.createFirstRouter') }}
        </p>
      </div>

      <div v-else class="max-h-96 overflow-auto rounded-lg border border-gray-200 dark:border-dark-600">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
          <thead class="sticky top-0 bg-gray-50 dark:bg-dark-700">
            <tr>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-gray-500 dark:text-gray-400">
                {{ t('admin.tlsFingerprintRouters.columns.name') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-gray-500 dark:text-gray-400">
                {{ t('admin.tlsFingerprintRouters.columns.description') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-gray-500 dark:text-gray-400">
                {{ t('admin.tlsFingerprintRouters.columns.enabled') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-gray-500 dark:text-gray-400">
                {{ t('admin.tlsFingerprintRouters.columns.rules') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-gray-500 dark:text-gray-400">
                {{ t('admin.tlsFingerprintRouters.columns.actions') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-800">
            <tr v-for="router in routers" :key="router.id" class="hover:bg-gray-50 dark:hover:bg-dark-700">
              <td class="px-3 py-2">
                <div class="font-medium text-gray-900 dark:text-white text-sm">{{ router.name }}</div>
              </td>
              <td class="px-3 py-2">
                <div v-if="router.description" class="text-sm text-gray-500 dark:text-gray-400 max-w-xs truncate">
                  {{ router.description }}
                </div>
                <div v-else class="text-xs text-gray-400 dark:text-gray-600">—</div>
              </td>
              <td class="px-3 py-2">
                <button
                  type="button"
                  @click="toggleRouter(router)"
                  :disabled="togglingId === router.id"
                  :class="[
                    'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:opacity-60',
                    router.enabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
                  ]"
                >
                  <span
                    :class="[
                      'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                      router.enabled ? 'translate-x-4' : 'translate-x-0'
                    ]"
                  />
                </button>
              </td>
              <td class="px-3 py-2">
                <span class="badge badge-primary text-xs">
                  {{ t('admin.tlsFingerprintRouters.ruleCount', { count: router.rules?.length || 0 }) }}
                </span>
              </td>
              <td class="px-3 py-2">
                <div class="flex items-center gap-1">
                  <button
                    @click="handleEdit(router)"
                    class="p-1 text-gray-500 hover:text-primary-600 dark:hover:text-primary-400"
                    :title="t('common.edit')"
                  >
                    <Icon name="edit" size="sm" />
                  </button>
                  <button
                    @click="handleDelete(router)"
                    class="p-1 text-gray-500 hover:text-red-600 dark:hover:text-red-400"
                    :title="t('common.delete')"
                  >
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button @click="$emit('close')" class="btn btn-secondary">
          {{ t('common.close') }}
        </button>
      </div>
    </template>

    <BaseDialog
      :show="showCreateModal || showEditModal"
      :title="showEditModal ? t('admin.tlsFingerprintRouters.editRouter') : t('admin.tlsFingerprintRouters.createRouter')"
      width="wide"
      :z-index="60"
      @close="closeFormModal"
    >
      <form @submit.prevent="handleSubmit" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="input-label">{{ t('admin.tlsFingerprintRouters.form.name') }}</label>
            <input v-model="form.name" type="text" required class="input" :placeholder="t('admin.tlsFingerprintRouters.form.namePlaceholder')" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.tlsFingerprintRouters.form.description') }}</label>
            <input v-model="form.description" type="text" class="input" :placeholder="t('admin.tlsFingerprintRouters.form.descriptionPlaceholder')" />
          </div>
        </div>

        <div class="flex items-center gap-3">
          <button
            type="button"
            @click="form.enabled = !form.enabled"
            :class="[
              'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
              form.enabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                form.enabled ? 'translate-x-4' : 'translate-x-0'
              ]"
            />
          </button>
          <div>
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.tlsFingerprintRouters.form.enabled') }}
            </span>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.tlsFingerprintRouters.form.enabledHint') }}
            </p>
          </div>
        </div>

        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('admin.tlsFingerprintRouters.form.rules') }}</label>
            <button type="button" @click="addRule" :disabled="profiles.length === 0" class="btn btn-secondary btn-sm">
              <Icon name="plus" size="sm" class="mr-1" />
              {{ t('admin.tlsFingerprintRouters.form.addRule') }}
            </button>
          </div>

          <div v-if="profiles.length === 0" class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-900/40 dark:bg-amber-900/20 dark:text-amber-300">
            {{ t('admin.tlsFingerprintRouters.form.noProfilesHint') }}
          </div>

          <div
            v-for="(rule, index) in form.rules"
            :key="index"
            class="space-y-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600"
          >
            <div class="flex items-center gap-3">
              <button
                type="button"
                @click="rule.enabled = !rule.enabled"
                :class="[
                  'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
                  rule.enabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
                ]"
              >
                <span
                  :class="[
                    'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                    rule.enabled ? 'translate-x-4' : 'translate-x-0'
                  ]"
                />
              </button>
              <input v-model="rule.name" type="text" class="input min-w-0 flex-1" :placeholder="t('admin.tlsFingerprintRouters.form.ruleNamePlaceholder')" />
              <label class="flex flex-shrink-0 items-center gap-1.5 text-xs text-gray-700 dark:text-gray-300">
                <input v-model="rule.case_sensitive" type="checkbox" class="h-3.5 w-3.5 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                <span>{{ t('admin.tlsFingerprintRouters.form.caseSensitive') }}</span>
              </label>
              <button
                type="button"
                @click="removeRule(index)"
                class="flex-shrink-0 p-1 text-gray-500 hover:text-red-600 dark:hover:text-red-400"
                :title="t('common.delete')"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>

            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="input-label text-xs">{{ t('admin.tlsFingerprintRouters.form.transport') }}</label>
                <select v-model="rule.transport" class="input">
                  <option value="">{{ t('admin.tlsFingerprintRouters.form.transportAny') }}</option>
                  <option v-if="isLegacyRouterTransport(rule.transport)" :value="rule.transport">
                    {{ rule.transport }} (legacy)
                  </option>
                  <option value="http1">HTTP/1.1</option>
                  <option value="h2">{{ t('admin.tlsFingerprintRouters.form.transportH2CaptureOnly') }}</option>
                  <option value="websocket-http1">WebSocket HTTP/1.1</option>
                  <option value="websocket-h2">{{ t('admin.tlsFingerprintRouters.form.transportWebsocketH2CaptureOnly') }}</option>
                </select>
                <p class="mt-1 text-[11px] text-amber-700 dark:text-amber-300">{{ t('admin.tlsFingerprintRouters.form.transportReplayHint') }}</p>
              </div>
              <div>
                <label class="input-label text-xs">{{ t('admin.tlsFingerprintRouters.form.matchType') }}</label>
                <select v-model="rule.match_type" class="input">
                  <option value="contains">{{ t('admin.tlsFingerprintRouters.matchTypes.contains') }}</option>
                  <option value="prefix">{{ t('admin.tlsFingerprintRouters.matchTypes.prefix') }}</option>
                  <option value="exact">{{ t('admin.tlsFingerprintRouters.matchTypes.exact') }}</option>
                  <option value="regex">{{ t('admin.tlsFingerprintRouters.matchTypes.regex') }}</option>
                </select>
              </div>
              <div>
                <label class="input-label text-xs">{{ t('admin.tlsFingerprintRouters.form.pattern') }}</label>
                <input v-model="rule.pattern" type="text" class="input" :placeholder="t('admin.tlsFingerprintRouters.form.patternPlaceholder')" />
              </div>
              <div>
                <label class="input-label text-xs">{{ t('admin.tlsFingerprintRouters.form.profile') }}</label>
                <select v-model.number="rule.tls_fingerprint_profile_id" class="input">
                  <option :value="0">{{ t('admin.tlsFingerprintRouters.form.selectProfile') }}</option>
                  <option v-for="profile in profiles" :key="profile.id" :value="profile.id">{{ profile.name }}</option>
                </select>
              </div>
              <div>
                <label class="input-label text-xs">{{ t('admin.tlsFingerprintRouters.form.dimensionOS') }}</label>
                <select v-model="rule.os" class="input">
                  <option value="">{{ t('admin.tlsFingerprintRouters.form.dimensionNone') }}</option>
                  <option value="windows">Windows</option>
                  <option value="macos">macOS</option>
                  <option value="linux">Linux</option>
                </select>
              </div>
              <div>
                <label class="input-label text-xs">{{ t('admin.tlsFingerprintRouters.form.dimensionClientType') }}</label>
                <input v-model="rule.client_type" type="text" class="input" :placeholder="t('admin.tlsFingerprintRouters.form.dimensionClientTypePlaceholder')" />
              </div>
              <div>
                <label class="input-label text-xs">{{ t('admin.tlsFingerprintRouters.form.upstreamUserAgent') }}</label>
                <input v-model="rule.upstream_user_agent" type="text" class="input" :placeholder="t('admin.tlsFingerprintRouters.form.upstreamUserAgentPlaceholder')" />
              </div>
              <div>
                <label class="input-label text-xs">{{ t('admin.tlsFingerprintRouters.form.upstreamOriginator') }}</label>
                <input v-model="rule.upstream_originator" type="text" class="input" :placeholder="t('admin.tlsFingerprintRouters.form.upstreamOriginatorPlaceholder')" />
              </div>
            </div>
          </div>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeFormModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button @click="handleSubmit" :disabled="submitting" class="btn btn-primary">
            <Icon v-if="submitting" name="refresh" size="sm" class="mr-1 animate-spin" />
            {{ showEditModal ? t('common.update') : t('common.create') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.tlsFingerprintRouters.deleteRouter')"
      :message="t('admin.tlsFingerprintRouters.deleteConfirmMessage', { name: deletingRouter?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { TLSFingerprintProfile } from '@/api/admin/tlsFingerprintProfile'
import type { TLSFingerprintRouter, TLSFingerprintRouterRule } from '@/api/admin/tlsFingerprintRouter'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

void emit

const { t } = useI18n()
const appStore = useAppStore()

const routers = ref<TLSFingerprintRouter[]>([])
const profiles = ref<TLSFingerprintProfile[]>([])
const loading = ref(false)
const submitting = ref(false)
const togglingId = ref<number | null>(null)
const showCreateModal = ref(false)
const showEditModal = ref(false)
const showDeleteDialog = ref(false)
const editingRouter = ref<TLSFingerprintRouter | null>(null)
const deletingRouter = ref<TLSFingerprintRouter | null>(null)

const form = reactive<{
  name: string
  description: string | null
  enabled: boolean
  rules: TLSFingerprintRouterRule[]
}>({
  name: '',
  description: null,
  enabled: true,
  rules: []
})

const legacyRouterTransports = new Set(['http', 'websocket'])
const isLegacyRouterTransport = (transport?: string) => legacyRouterTransports.has(transport || '')

const loadData = async () => {
  loading.value = true
  try {
    const [routerList, profileList] = await Promise.all([
      adminAPI.tlsFingerprintRouters.list(),
      adminAPI.tlsFingerprintProfiles.list()
    ])
    routers.value = routerList
    profiles.value = profileList
  } catch (error) {
    appStore.showError(t('admin.tlsFingerprintRouters.loadFailed'))
    console.error('Error loading TLS fingerprint routers:', error)
  } finally {
    loading.value = false
  }
}

watch(() => props.show, (newVal) => {
  if (newVal) {
    loadData()
  }
}, { immediate: true })

const resetForm = () => {
  form.name = ''
  form.description = null
  form.enabled = true
  form.rules = []
}

const newRule = (): TLSFingerprintRouterRule => ({
  name: '',
  enabled: true,
  transport: '',
  match_type: 'contains',
  pattern: '',
  case_sensitive: false,
  tls_fingerprint_profile_id: profiles.value[0]?.id || 0,
  os: '',
  client_type: '',
  upstream_user_agent: '',
  upstream_originator: ''
})

const openCreate = () => {
  resetForm()
  form.rules = profiles.value.length > 0 ? [newRule()] : []
  showCreateModal.value = true
}

const closeFormModal = () => {
  showCreateModal.value = false
  showEditModal.value = false
  editingRouter.value = null
  resetForm()
}

const addRule = () => {
  form.rules.push(newRule())
}

const removeRule = (index: number) => {
  form.rules.splice(index, 1)
}

const handleEdit = (router: TLSFingerprintRouter) => {
  editingRouter.value = router
  form.name = router.name
  form.description = router.description
  form.enabled = router.enabled
  form.rules = (router.rules || []).map(rule => ({
    name: rule.name,
    enabled: rule.enabled,
    transport: rule.transport || '',
    match_type: rule.match_type,
    pattern: rule.pattern,
    case_sensitive: rule.case_sensitive,
    tls_fingerprint_profile_id: rule.tls_fingerprint_profile_id,
    os: rule.os || '',
    client_type: rule.client_type || '',
    upstream_user_agent: rule.upstream_user_agent || '',
    upstream_originator: rule.upstream_originator || ''
  }))
  showEditModal.value = true
}

const handleDelete = (router: TLSFingerprintRouter) => {
  deletingRouter.value = router
  showDeleteDialog.value = true
}

const normalizeRules = (): TLSFingerprintRouterRule[] => form.rules.map(rule => ({
  name: rule.name.trim(),
  enabled: rule.enabled,
  transport: rule.transport || '',
  match_type: rule.match_type,
  pattern: rule.pattern.trim(),
  case_sensitive: rule.case_sensitive,
  tls_fingerprint_profile_id: Number(rule.tls_fingerprint_profile_id) || 0,
  os: (rule.os || '').trim(),
  client_type: (rule.client_type || '').trim(),
  upstream_user_agent: rule.upstream_user_agent?.trim() || '',
  upstream_originator: rule.upstream_originator?.trim() || ''
}))

const handleSubmit = async () => {
  if (!form.name.trim()) {
    appStore.showError(t('admin.tlsFingerprintRouters.form.name') + ' ' + t('common.required'))
    return
  }

  submitting.value = true
  try {
    const data = {
      name: form.name.trim(),
      description: form.description?.trim() || null,
      enabled: form.enabled,
      rules: normalizeRules()
    }

    if (showEditModal.value && editingRouter.value) {
      await adminAPI.tlsFingerprintRouters.update(editingRouter.value.id, data)
      appStore.showSuccess(t('admin.tlsFingerprintRouters.updateSuccess'))
    } else {
      await adminAPI.tlsFingerprintRouters.create(data)
      appStore.showSuccess(t('admin.tlsFingerprintRouters.createSuccess'))
    }

    closeFormModal()
    loadData()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.tlsFingerprintRouters.saveFailed'))
    console.error('Error saving TLS fingerprint router:', error)
  } finally {
    submitting.value = false
  }
}

const toggleRouter = async (router: TLSFingerprintRouter) => {
  togglingId.value = router.id
  try {
    await adminAPI.tlsFingerprintRouters.toggle(router.id, !router.enabled)
    await loadData()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.tlsFingerprintRouters.toggleFailed'))
    console.error('Error toggling TLS fingerprint router:', error)
  } finally {
    togglingId.value = null
  }
}

const confirmDelete = async () => {
  if (!deletingRouter.value) return

  try {
    await adminAPI.tlsFingerprintRouters.delete(deletingRouter.value.id)
    appStore.showSuccess(t('admin.tlsFingerprintRouters.deleteSuccess'))
    showDeleteDialog.value = false
    deletingRouter.value = null
    loadData()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.tlsFingerprintRouters.deleteFailed'))
    console.error('Error deleting TLS fingerprint router:', error)
  }
}
</script>
