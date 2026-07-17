<template>
  <div>
    <label class="input-label">{{ label }}</label>
    <div v-if="rows.length > 0" class="space-y-2">
      <div
        v-for="(row, index) in rows"
        :key="row.id"
        class="flex items-center gap-2"
      >
        <input
          v-model="row.from"
          type="text"
          class="input flex-1"
          :placeholder="fromPlaceholder || t('admin.settings.modelMapping.fromPlaceholder')"
          @input="emitRows"
        />
        <span class="text-ink-faint">→</span>
        <input
          v-model="row.to"
          type="text"
          class="input flex-1"
          :placeholder="toPlaceholder || t('admin.settings.modelMapping.toPlaceholder')"
          @input="emitRows"
        />
        <button
          type="button"
          @click="removeRow(index)"
          class="rounded p-1 text-danger transition-colors hover:text-danger/80"
        >
          <Icon name="x" size="sm" :stroke-width="2" />
        </button>
      </div>
    </div>

    <button
      type="button"
      @click="addRow"
      class="mt-2 w-full rounded-control border-2 border-dashed border-line px-4 py-2 text-sm text-ink-body transition-colors hover:border-ink-faint hover:text-ink-body dark:border-dark-500 dark:text-ink-soft dark:hover:border-dark-400 dark:hover:text-ink-faint"
    >
      <svg class="mr-1 inline h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
      </svg>
      {{ t('admin.settings.modelMapping.addRow') }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import Icon from '@/components/icons/Icon.vue'

interface MappingRow {
  id: number
  from: string
  to: string
}

const props = defineProps<{
  modelValue: Record<string, string>
  label: string
  fromPlaceholder?: string
  toPlaceholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: Record<string, string>]
}>()

const { t } = useI18n()

let rowSeq = 0
const rows = ref<MappingRow[]>([])

const syncFromModel = (value: Record<string, string>) => {
  const entries = Object.entries(value || {})
  // 仅当与当前行表示的映射不同才重建，避免编辑时光标跳动
  const currentMap = rowsToMap(rows.value)
  if (JSON.stringify(currentMap) === JSON.stringify(value || {})) {
    return
  }
  rows.value = entries.map(([from, to]) => ({ id: rowSeq++, from, to }))
}

const rowsToMap = (list: MappingRow[]): Record<string, string> => {
  const out: Record<string, string> = {}
  for (const row of list) {
    const from = row.from.trim()
    const to = row.to.trim()
    if (from && to) {
      out[from] = to
    }
  }
  return out
}

const emitRows = () => {
  emit('update:modelValue', rowsToMap(rows.value))
}

const addRow = () => {
  rows.value.push({ id: rowSeq++, from: '', to: '' })
}

const removeRow = (index: number) => {
  rows.value.splice(index, 1)
  emitRows()
}

watch(() => props.modelValue, syncFromModel, { immediate: true, deep: true })
</script>
