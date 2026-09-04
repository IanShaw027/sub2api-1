<template>
  <div v-if="show" class="border-t border-line pt-4">
    <div class="mb-3 flex items-center justify-between">
      <div class="flex-1 pr-4">
        <label
          id="bulk-edit-openai-compact-model-mapping-label"
          class="input-label mb-0"
          for="bulk-edit-openai-compact-model-mapping-enabled"
        >
          {{ t('admin.accounts.openai.compactModelMapping') }}
        </label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.openai.compactModelMappingDesc') }}
        </p>
      </div>
      <input
        v-model="enabled"
        id="bulk-edit-openai-compact-model-mapping-enabled"
        type="checkbox"
        aria-controls="bulk-edit-openai-compact-model-mapping"
        class="rounded border-line text-accent focus:ring-accent"
      />
    </div>
    <div
      id="bulk-edit-openai-compact-model-mapping"
      :class="!enabled && 'pointer-events-none opacity-50'"
    >
      <div v-if="mappings.length > 0" class="mb-3 space-y-2">
        <div
          v-for="(mapping, index) in mappings"
          :key="index"
          class="flex items-center gap-2"
        >
          <input
            v-model="mapping.from"
            type="text"
            class="input flex-1"
            :placeholder="t('admin.accounts.fromModel')"
            data-testid="bulk-edit-openai-compact-model-mapping-input"
          />
          <span class="text-muted">→</span>
          <input
            v-model="mapping.to"
            type="text"
            class="input flex-1"
            :placeholder="t('admin.accounts.toModel')"
            data-testid="bulk-edit-openai-compact-model-mapping-input"
          />
          <button
            type="button"
            class="rounded-lg p-2 text-danger-text transition-colors hover:bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] hover:text-danger-text"
            @click="removeMapping(index)"
          >
            <Icon name="trash" size="sm" />
          </button>
        </div>
      </div>
      <button
        type="button"
        class="mb-3 w-full rounded-lg border-2 border-dashed border-line px-4 py-2 text-muted transition-colors hover:border-line hover:text-foreground"
        data-testid="bulk-edit-openai-compact-model-mapping-add"
        @click="addMapping"
      >
        + {{ t('admin.accounts.addMapping') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

interface ModelMapping {
  from: string
  to: string
}

interface Props {
  show: boolean
}

defineProps<Props>()

const enabled = defineModel<boolean>('enabled', { required: true })
const mappings = defineModel<ModelMapping[]>('mappings', { required: true })

const { t } = useI18n()

const addMapping = () => {
  mappings.value.push({ from: '', to: '' })
}

const removeMapping = (index: number) => {
  mappings.value.splice(index, 1)
}
</script>
