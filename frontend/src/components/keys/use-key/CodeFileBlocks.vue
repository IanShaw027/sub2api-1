<template>
 <div class="space-y-4">
 <div
 v-for="(file, index) in files"
 :key="index"
 class="relative"
 >
 <!-- File Hint (if exists) -->
 <p v-if="file.hint" class="text-xs text-warning-text mb-1.5 flex items-center gap-1">
 <Icon name="exclamationCircle" size="sm" class="flex-shrink-0" />
 {{ file.hint }}
 </p>
 <div class="bg-[var(--code-bg)] rounded-xl overflow-hidden">
 <!-- Code Header -->
 <div class="flex items-center justify-between px-4 py-2 bg-[var(--code-bg)] border-b border-white/10">
 <span class="min-w-0 truncate text-xs text-muted font-mono">{{ file.path }}</span>
 <button
 type="button"
 @click="emit('copy', file.content, index)"
 class="flex flex-shrink-0 items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded-lg transition-colors"
 :class="copiedIndex === index
 ? 'bg-success-500/20 text-success-400'
 : 'bg-[color-mix(in_oklch,white_5%,transparent)] hover:bg-[color-mix(in_oklch,white_10%,transparent)] text-muted hover:text-white'"
 >
 <svg v-if="copiedIndex === index" class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
 <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
 </svg>
 <svg v-else class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
 <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184" />
 </svg>
 {{ copiedIndex === index ? t('keys.useKeyModal.copied') : t('keys.useKeyModal.copy') }}
 </button>
 </div>
 <!-- Code Content -->
 <pre class="p-4 text-sm font-mono text-muted overflow-x-auto"><code v-if="file.highlighted" v-html="file.highlighted"></code><code v-else v-text="file.content"></code></pre>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { FileConfig } from './types'

defineProps<{
 files: FileConfig[]
 copiedIndex: number | null
}>()

const emit = defineEmits<{
 (e: 'copy', content: string, index: number): void
}>()

const { t } = useI18n()
</script>
