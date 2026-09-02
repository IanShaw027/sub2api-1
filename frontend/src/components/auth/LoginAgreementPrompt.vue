<template>
 <div
 v-if="mode === 'checkbox' && documents.length > 0"
 class="px-0.5"
 >
 <div class="flex items-start gap-2">
 <input
 id="login-agreement-consent"
 type="checkbox"
 :checked="accepted"
 class="mt-[2px] h-4 w-4 flex-shrink-0 rounded border-line text-accent focus:ring-accent"
 @change="handleCheckboxChange"
 />
 <div class="min-w-0 flex-1">
 <p class="text-[13px] leading-5 text-muted">
 <label
 for="login-agreement-consent"
 class="cursor-pointer text-foreground"
 >
 {{ t('legal.loginAgreementPrompt.checkboxPrefix') }}
 </label>
 <template v-for="(doc, index) in documents" :key="doc.id || doc.title">
 <RouterLink
 :to="documentRoute(doc)"
 target="_blank"
 rel="noopener noreferrer"
 class="font-medium text-accent underline-offset-4 transition hover:text-accent hover:underline"
 >
 {{ doc.title }}
 </RouterLink>
 <span v-if="index < documents.length - 1">{{ t('legal.loginAgreementPrompt.documentSeparator') }}</span>
 </template>
 </p>
 </div>
 </div>
 </div>

 <div
 v-else-if="!accepted && documents.length > 0"
 class="rounded-lg border border-[color-mix(in_oklch,var(--accent)_16%,transparent)] bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] p-3 text-sm text-accent"
 >
 <div class="flex items-start gap-3">
 <Icon name="shield" size="sm" class="mt-0.5 flex-shrink-0 text-accent" />
 <div class="min-w-0 flex-1">
 <p class="font-medium">{{ t('legal.loginAgreementPrompt.noticeTitle') }}</p>
 <p class="mt-1 text-accent">
 {{ t('legal.loginAgreementPrompt.noticeDescription') }}
 </p>
 </div>
 <button
 type="button"
 class="flex-shrink-0 rounded-md bg-accent px-3 py-1.5 text-xs font-medium text-white transition hover:opacity-90"
 @click="emit('open')"
 >
 {{ t('legal.loginAgreementPrompt.viewTerms') }}
 </button>
 </div>
 </div>

 <Teleport to="body">
 <Transition name="agreement-fade">
 <div
 v-if="dialogVisible"
 class="fixed inset-0 z-[140] flex items-center justify-center overflow-y-auto bg-black/60 p-4 backdrop-blur-sm"
 >
 <div class="w-full max-w-[600px] overflow-hidden rounded-2xl bg-surface shadow-2xl ring-1 ring-black/10">
 <div class="border-b border-line bg-surface px-6 py-6">
 <div class="flex items-start gap-4">
 <span class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent ring-1 ring-[color-mix(in_oklch,var(--accent)_16%,transparent)]">
 <Icon name="shield" size="md" />
 </span>
 <div class="min-w-0 flex-1">
 <div class="flex flex-wrap items-center gap-2">
 <h2 class="text-xl font-bold tracking-normal text-foreground">
 {{ t('legal.loginAgreementPrompt.dialogTitle') }}
 </h2>
 <span
 v-if="updatedAt"
 class="rounded-full bg-surface-2 px-2.5 py-1 text-xs font-medium text-muted"
 >
 {{ updatedAt }}
 </span>
 </div>
 <p class="mt-2 text-sm leading-6 text-muted">
 {{
 t('legal.loginAgreementPrompt.dialogDescription', {
 date: updatedAt || t('legal.loginAgreementPrompt.recently'),
 })
 }}
 </p>
 </div>
 </div>
 </div>

 <div class="max-h-[58vh] overflow-y-auto px-6 py-5">
 <div class="mb-3 flex items-center justify-between gap-3">
 <p class="text-sm font-semibold text-foreground">{{ t('legal.loginAgreementPrompt.relatedDocuments') }}</p>
 </div>
 <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
 <RouterLink
 v-for="(doc, index) in documents"
 :key="doc.id || doc.title"
 :to="documentRoute(doc)"
 target="_blank"
 rel="noopener noreferrer"
 class="group flex min-h-[72px] w-full items-center gap-3 rounded-xl border border-line bg-surface-2/70 px-4 py-3 text-left transition hover:-translate-y-0.5 hover:border-[color-mix(in_oklch,var(--accent)_20%,transparent)] hover:bg-surface hover:shadow-sm"
 >
 <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-surface text-foreground ring-1 ring-line transition group-hover:bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] group-hover:text-accent group-hover:ring-[color-mix(in_oklch,var(--accent)_16%,transparent)]">
 <Icon :name="documentIcon(index, doc.title)" size="sm" />
 </span>
 <span class="min-w-0 flex-1">
 <span class="block truncate text-sm font-semibold text-foreground">{{ doc.title }}</span>
 </span>
 <span class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full text-muted transition group-hover:bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] group-hover:text-accent">
 <Icon name="externalLink" size="sm" />
 </span>
 </RouterLink>
 </div>
 </div>

 <div class="border-t border-line bg-surface-2/80 px-6 py-4">
 <div class="grid grid-cols-2 gap-3">
 <button
 type="button"
 class="rounded-xl border border-line bg-surface px-4 py-3 text-sm font-semibold text-foreground transition hover:bg-surface-2"
 @click="emit('reject')"
 >
 {{ t('legal.loginAgreementPrompt.reject') }}
 </button>
 <button
 type="button"
 class="rounded-xl bg-accent px-4 py-3 text-sm font-semibold text-white shadow-sm shadow-[color-mix(in_oklch,var(--accent)_20%,transparent)] transition hover:opacity-90"
 @click="emit('accept')"
 >
 {{ t('legal.loginAgreementPrompt.accept') }}
 </button>
 </div>
 </div>
 </div>
 </div>
 </Transition>
 </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { LoginAgreementDocument } from '@/types'

const { t } = useI18n()

const props = withDefaults(defineProps<{
 accepted: boolean
 documents: LoginAgreementDocument[]
 mode: 'modal' | 'checkbox' | string
 updatedAt?: string
 visible: boolean
}>(), {
 updatedAt: ''
})

const emit = defineEmits<{
 accept: []
 reject: []
 open: []
}>()

const dialogVisible = computed(() => props.visible && documents.value.length > 0)
const documents = computed(() => props.documents.filter((doc) => doc.title.trim()))
const updatedAt = computed(() => props.updatedAt || '')
const accepted = computed(() => props.accepted)
const mode = computed(() => props.mode === 'checkbox' ? 'checkbox' : 'modal')

function documentRoute(doc: LoginAgreementDocument) {
 return {
 name: 'LegalDocument',
 params: {
 documentId: doc.id || doc.title,
 },
 }
}

function handleCheckboxChange(event: Event): void {
 const checked = (event.target as HTMLInputElement).checked
 if (checked) {
 emit('accept')
 } else {
 emit('reject')
 }
}

function documentIcon(index: number, title: string): 'document' | 'shield' | 'globe' | 'cog' {
 const normalizedTitle = title.toLowerCase()
 if (
 normalizedTitle.includes('policy') ||
 normalizedTitle.includes('privacy') ||
 title.includes('政策') ||
 title.includes('隐私')
 ) {
 return 'shield'
 }
 if (
 normalizedTitle.includes('country') ||
 normalizedTitle.includes('region') ||
 title.includes('国家') ||
 title.includes('地区')
 ) {
 return 'globe'
 }
 if (index === 3) {
 return 'cog'
 }
 return 'document'
}
</script>

<style scoped>
.agreement-fade-enter-active,
.agreement-fade-leave-active {
 transition: opacity 0.18s ease;
}

.agreement-fade-enter-from,
.agreement-fade-leave-to {
 opacity: 0;
}

.agreement-fade-enter-active > div,
.agreement-fade-leave-active > div {
 transition: transform 0.18s ease, opacity 0.18s ease;
}

.agreement-fade-enter-from > div,
.agreement-fade-leave-to > div {
 opacity: 0;
 transform: translateY(8px) scale(0.98);
}
</style>
