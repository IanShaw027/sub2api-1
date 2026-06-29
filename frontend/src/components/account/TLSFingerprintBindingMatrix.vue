<template>
  <div class="space-y-2">
    <div class="flex items-center justify-between">
      <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.tlsFingerprint.bindingMatrix') }}</label>
      <button type="button" class="text-xs text-primary-600 hover:underline dark:text-primary-400" @click="addRow">
        + {{ t('admin.accounts.quotaControl.tlsFingerprint.addBinding') }}
      </button>
    </div>
    <p class="text-xs text-gray-500 dark:text-gray-400">
      {{ t('admin.accounts.quotaControl.tlsFingerprint.bindingMatrixHint') }}
    </p>

    <div v-if="rows.length === 0" class="rounded-md border border-dashed border-gray-300 px-3 py-2 text-xs text-gray-400 dark:border-dark-600">
      {{ t('admin.accounts.quotaControl.tlsFingerprint.noBindings') }}
    </div>

    <div v-for="(row, index) in rows" :key="row.key" class="flex items-center gap-2">
      <select v-model="row.os" class="input w-28 text-sm" @change="emitUpdate">
        <option value="windows">Windows</option>
        <option value="macos">macOS</option>
        <option value="linux">Linux</option>
      </select>
      <input
        v-if="withClientType"
        v-model="row.clientType"
        type="text"
        class="input w-36 text-sm"
        :placeholder="t('admin.accounts.quotaControl.tlsFingerprint.clientTypePlaceholder')"
        @input="emitUpdate"
      />
      <select v-model.number="row.profileId" class="input min-w-0 flex-1 text-sm" @change="emitUpdate">
        <option :value="-1">{{ t('admin.accounts.quotaControl.tlsFingerprint.randomProfile') }}</option>
        <option v-for="p in profilesForRow(row)" :key="p.id" :value="p.id">{{ p.name }}</option>
      </select>
      <button
        type="button"
        class="flex-shrink-0 rounded p-1 text-gray-400 hover:text-red-500"
        :title="t('common.delete')"
        @click="removeRow(index)"
      >
        <Icon name="trash" size="sm" />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { TLSFingerprintProfileOption } from '@/components/account/tlsFingerprintProfileOptions'
import { getTLSFingerprintProfilesForDimension } from '@/components/account/tlsFingerprintProfileOptions'

const { t } = useI18n()

const props = defineProps<{
  modelValue: Record<string, number>
  profiles: TLSFingerprintProfileOption[]
  platform: string
  withClientType: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: Record<string, number>): void
}>()

interface MatrixRow {
  key: string
  os: string
  clientType: string
  profileId: number
}

let rowSeq = 0
const rows = ref<MatrixRow[]>([])

function parseModel(value: Record<string, number>): MatrixRow[] {
  return Object.entries(value || {}).map(([dim, profileId]) => {
    const [os, clientType = ''] = dim.split('/')
    return { key: `r${rowSeq++}`, os: os || 'windows', clientType, profileId }
  })
}

// Initialize from modelValue; re-sync only when the external object identity
// changes (e.g. account switch), not on every keystroke we emit.
watch(
  () => props.modelValue,
  (value) => {
    rows.value = parseModel(value)
  },
  { immediate: true }
)

function profilesForRow(row: MatrixRow): TLSFingerprintProfileOption[] {
  return getTLSFingerprintProfilesForDimension(
    props.profiles,
    { platform: props.platform, os: row.os, clientType: props.withClientType ? row.clientType : '' },
    row.profileId
  )
}

function addRow() {
  rows.value.push({ key: `r${rowSeq++}`, os: 'windows', clientType: '', profileId: -1 })
  emitUpdate()
}

function removeRow(index: number) {
  rows.value.splice(index, 1)
  emitUpdate()
}

function emitUpdate() {
  const out: Record<string, number> = {}
  for (const row of rows.value) {
    const os = row.os.trim().toLowerCase()
    if (!os) continue
    const client = props.withClientType ? row.clientType.trim().toLowerCase() : ''
    const dim = client ? `${os}/${client}` : os
    out[dim] = row.profileId
  }
  emit('update:modelValue', out)
}
</script>
