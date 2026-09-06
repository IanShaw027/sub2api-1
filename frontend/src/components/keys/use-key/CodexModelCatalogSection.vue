<template>
 <section
 data-testid="codex-model-catalog"
 class="overflow-hidden rounded-lg border border-line bg-surface-2"
 >
 <div class="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
 <div class="min-w-0">
 <h3 class="text-sm font-medium text-foreground">
 {{ t('keys.useKeyModal.codexModelCatalog.title') }}
 </h3>
 <p class="mt-1 text-xs text-muted">
 {{ t('keys.useKeyModal.codexModelCatalog.description') }}
 </p>
 <p class="mt-1 truncate font-mono text-xs text-foreground">
 {{ path }}
 </p>
 </div>
 <button
 v-if="state === 'ready'"
 type="button"
 class="btn btn-primary min-h-9 flex-shrink-0 px-3 text-xs"
 @click="emit('download')"
 >
 <Icon name="download" size="sm" class="mr-1.5" />
 {{ t('keys.useKeyModal.codexModelCatalog.download') }}
 </button>
 <button
 v-else
 type="button"
 data-testid="codex-model-catalog-fetch"
 class="btn btn-primary min-h-9 flex-shrink-0 px-3 text-xs"
 :disabled="state === 'loading' || !apiKey"
 @click="emit('fetch')"
 >
 <Icon
 name="refresh"
 size="sm"
 class="mr-1.5"
 :class="state === 'loading' ? 'animate-spin' : ''"
 />
 {{ state === 'error'
 ? t('keys.useKeyModal.codexModelCatalog.retry')
 : t('keys.useKeyModal.codexModelCatalog.fetch') }}
 </button>
 </div>
 <p
 v-if="state === 'ready'"
 class="border-t border-line px-4 py-2 text-xs text-success-text"
 >
 {{ t('keys.useKeyModal.codexModelCatalog.modelsCount', { count: modelCount }) }}
 </p>
 <p
 v-else-if="state === 'error'"
 class="border-t border-danger-200 px-4 py-2 text-xs text-danger-text"
 >
 {{ t('keys.useKeyModal.codexModelCatalog.errorDescription') }}
 </p>
 </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { CodexModelManifestState } from './types'

defineProps<{
 apiKey: string
 path: string
 state: CodexModelManifestState
 modelCount: number
}>()

const emit = defineEmits<{
 (e: 'fetch'): void
 (e: 'download'): void
}>()

const { t } = useI18n()
</script>
