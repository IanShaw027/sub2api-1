<template>
  <BaseDialog
    :show="show"
    :title="t('admin.tlsFingerprintRouters.title')"
    width="wide"
    @close="$emit('close')"
  >
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <p class="text-sm text-muted">
          {{ t('admin.tlsFingerprintRouters.description') }}
        </p>
        <button class="btn btn-primary btn-sm" @click="startCreate">
          <Icon name="plus" size="sm" class="mr-1" />
          {{ t('admin.tlsFingerprintRouters.create') }}
        </button>
      </div>

      <div v-if="loading" class="flex justify-center py-8">
        <Icon name="refresh" size="lg" class="animate-spin text-muted" />
      </div>
      <div v-else-if="routers.length === 0" class="py-8 text-center text-sm text-muted">
        {{ t('admin.tlsFingerprintRouters.empty') }}
      </div>
      <div v-else class="max-h-80 overflow-auto rounded-lg border border-line">
        <table class="min-w-full divide-y divide-line">
          <tbody class="divide-y divide-line">
            <tr v-for="router in routers" :key="router.id">
              <td class="px-3 py-2 text-sm font-medium text-foreground">{{ router.name }}</td>
              <td class="px-3 py-2 text-xs text-muted">{{ router.rules?.length || 0 }} {{ t('admin.tlsFingerprintRouters.rules') }}</td>
              <td class="px-3 py-2 text-right">
                <button class="btn btn-secondary btn-sm mr-2" @click="startEdit(router)">{{ t('common.edit') }}</button>
                <button class="btn btn-danger btn-sm" @click="askRemove(router)">{{ t('common.delete') }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="editing" class="space-y-3 rounded-lg border border-line p-4">
        <input v-model="form.name" class="input" :placeholder="t('admin.tlsFingerprintRouters.name')" />
        <textarea v-model="form.description" class="input" rows="2" :placeholder="t('admin.tlsFingerprintRouters.descriptionField')" />
        <label class="flex items-center gap-2 text-sm">
          <input v-model="form.enabled" type="checkbox" />
          {{ t('admin.tlsFingerprintRouters.enabled') }}
        </label>
        <div v-for="(rule, index) in form.rules" :key="index" class="grid grid-cols-2 gap-2 rounded border border-line p-3">
          <label class="col-span-2 flex items-center gap-2 text-sm">
            <input v-model="rule.enabled" type="checkbox" />
            {{ t('admin.tlsFingerprintRouters.ruleEnabled') }}
          </label>
          <input v-model="rule.name" class="input" :placeholder="t('admin.tlsFingerprintRouters.ruleName')" />
          <select v-model.number="rule.tls_fingerprint_profile_id" class="input">
            <option :value="0">{{ t('admin.tlsFingerprintRouters.noProfile') }}</option>
            <option v-for="profile in profiles" :key="profile.id" :value="profile.id">{{ profile.name }}</option>
          </select>
          <select v-model="rule.os" class="input">
            <option value="">{{ t('admin.tlsFingerprintRouters.anyOS') }}</option>
            <option v-for="os in osOptions" :key="os" :value="os">{{ os }}</option>
          </select>
          <input v-model="rule.client_type" class="input" :placeholder="t('admin.tlsFingerprintRouters.client')" />
          <select v-model="rule.protocol" class="input">
            <option value="">{{ t('admin.tlsFingerprintRouters.protocol') }}</option>
            <option v-for="protocol in protocolOptions" :key="protocol" :value="protocol">{{ protocol }}</option>
          </select>
          <select v-model="rule.transport" class="input">
            <option value="">{{ t('admin.tlsFingerprintRouters.anyTransport') }}</option>
            <option v-for="transport in transportOptions" :key="transport" :value="transport">{{ transport }}</option>
          </select>
          <select v-model="rule.match_type" class="input">
            <option v-for="matchType in matchTypeOptions" :key="matchType" :value="matchType">{{ matchType }}</option>
          </select>
          <input v-model="rule.pattern" class="input" :placeholder="t('admin.tlsFingerprintRouters.uaPattern')" />
          <label class="flex items-center gap-2 text-sm">
            <input v-model="rule.case_sensitive" type="checkbox" />
            {{ t('admin.tlsFingerprintRouters.caseSensitive') }}
          </label>
          <input v-model="rule.upstream_user_agent" class="input" :placeholder="t('admin.tlsFingerprintRouters.upstreamUserAgent')" />
          <input v-model="rule.upstream_originator" class="input" :placeholder="t('admin.tlsFingerprintRouters.upstreamOriginator')" />
          <button class="btn btn-secondary btn-sm col-span-2" @click="form.rules.splice(index, 1)">
            {{ t('admin.tlsFingerprintRouters.removeRule') }}
          </button>
        </div>
        <button class="btn btn-secondary btn-sm" @click="addRule">{{ t('admin.tlsFingerprintRouters.addRule') }}</button>
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" @click="editing = false">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="saving" @click="save">{{ t('common.save') }}</button>
        </div>
      </div>
    </div>
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.tlsFingerprintRouters.deleteRouter')"
      :message="t('admin.tlsFingerprintRouters.deleteConfirmMessage', { name: deletingRouter?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmRemove"
      @cancel="showDeleteDialog = false"
    />
  </BaseDialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { TLSFingerprintRouter, TLSFingerprintRouterRule } from '@/api/admin/tlsFingerprintRouter'
import { useAppStore } from '@/stores'

const props = defineProps<{ show: boolean }>()
defineEmits<{ close: [] }>()

const { t } = useI18n()
const appStore = useAppStore()
const osOptions = ['windows', 'macos', 'linux', 'ios', 'android']
const protocolOptions = ['messages', 'responses', 'chat_completions', 'images', 'embeddings', 'gemini', 'antigravity', 'kiro']
const transportOptions = ['http', 'websocket', 'http1', 'h2', 'websocket-http1', 'websocket-h2']
const matchTypeOptions = ['contains', 'prefix', 'exact', 'regex']
const loading = ref(false)
const showDeleteDialog = ref(false)
const deletingRouter = ref<TLSFingerprintRouter | null>(null)
const saving = ref(false)
const editing = ref(false)
const editingId = ref<number | null>(null)
const routers = ref<TLSFingerprintRouter[]>([])
const profiles = ref<{ id: number; name: string }[]>([])
const form = reactive({
  name: '',
  description: '',
  enabled: true,
  rules: [] as TLSFingerprintRouterRule[]
})

watch(
  () => props.show,
  async (show) => {
    if (!show) return
    loading.value = true
    try {
      const [routerList, profileList] = await Promise.all([
        adminAPI.tlsFingerprintRouters.list(),
        adminAPI.tlsFingerprintProfiles.list()
      ])
      routers.value = routerList
      profiles.value = profileList.map((p) => ({ id: p.id, name: p.name }))
    } catch {
      appStore.showError(t('admin.tlsFingerprintRouters.loadFailed'))
    } finally {
      loading.value = false
    }
  }
)

function emptyRule(): TLSFingerprintRouterRule {
  return {
    name: '',
    enabled: true,
    tls_fingerprint_profile_id: 0,
    os: '',
    client_type: '',
    protocol: '',
    transport: '',
    pattern: '',
    match_type: 'contains',
    case_sensitive: false,
    upstream_user_agent: '',
    upstream_originator: ''
  }
}

function startCreate() {
  editingId.value = null
  form.name = ''
  form.description = ''
  form.enabled = true
  form.rules = [emptyRule()]
  editing.value = true
}

function startEdit(router: TLSFingerprintRouter) {
  editingId.value = router.id
  form.name = router.name
  form.description = router.description || ''
  form.enabled = router.enabled
  form.rules = (router.rules || []).map((rule) => ({ ...rule }))
  editing.value = true
}

function addRule() {
  form.rules.push(emptyRule())
}

async function save() {
  saving.value = true
  try {
    const payload = {
      name: form.name,
      description: form.description || null,
      enabled: form.enabled,
      rules: form.rules.map((rule) => ({ ...rule }))
    }
    if (editingId.value) {
      await adminAPI.tlsFingerprintRouters.update(editingId.value, payload)
    } else {
      await adminAPI.tlsFingerprintRouters.create(payload)
    }
    routers.value = await adminAPI.tlsFingerprintRouters.list()
    editing.value = false
    appStore.showSuccess(t('admin.tlsFingerprintRouters.saveSuccess'))
  } catch (error: unknown) {
    const err = error as { message?: string }
    appStore.showError(err.message || t('admin.tlsFingerprintRouters.saveFailed'))
  } finally {
    saving.value = false
  }
}

function askRemove(router: TLSFingerprintRouter) {
  deletingRouter.value = router
  showDeleteDialog.value = true
}

async function confirmRemove() {
  const router = deletingRouter.value
  showDeleteDialog.value = false
  if (!router) return
  try {
    await adminAPI.tlsFingerprintRouters.delete(router.id)
    routers.value = routers.value.filter((item) => item.id !== router.id)
    appStore.showSuccess(t('admin.tlsFingerprintRouters.deleteSuccess'))
  } catch (error: unknown) {
    const err = error as { message?: string }
    appStore.showError(err.message || t('admin.tlsFingerprintRouters.deleteFailed'))
  } finally {
    deletingRouter.value = null
  }
}
</script>
