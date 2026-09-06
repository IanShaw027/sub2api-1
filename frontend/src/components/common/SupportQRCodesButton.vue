<template>
 <div v-if="hasEntries || hasLegacyContactInfo">
 <button
 type="button"
 class="header-icon-btn !w-auto gap-1.5 px-2.5 text-sm font-medium"
 :class="{ 'support-menu-trigger': menuMode }"
 :aria-label="t('common.contactSupport')"
 @click="showDialog = true"
 >
 <Icon name="chat" size="sm" />
 <span :class="menuMode ? '' : 'hidden sm:inline'">{{ t('common.contactSupport') }}</span>
 </button>

 <BaseDialog
 :show="showDialog"
 :title="t('common.contactSupport')"
 width="normal"
 :close-on-click-outside="true"
 @close="showDialog = false"
 >
 <div v-if="hasEntries" :class="['grid gap-4', normalizedEntries.length > 1 ? 'sm:grid-cols-2' : '']">
 <div
 v-for="(entry, index) in normalizedEntries"
 :key="`${entry.image_url}-${index}`"
 class="rounded-xl border border-line bg-surface-2/80 p-4"
 >
 <div class="overflow-hidden rounded-xl bg-surface p-3 shadow-sm ring-1 ring-line">
 <img
 :src="entry.image_url"
 :alt="entry.note || t('common.contactSupport')"
 class="aspect-square w-full object-contain"
 >
 </div>
 <p
 v-if="entry.note"
 class="mt-3 text-center text-sm text-muted"
 >
 {{ entry.note }}
 </p>
 </div>
 </div>
 <p
 v-else
 class="rounded-xl border border-line bg-surface-2/80 p-4 text-sm leading-6 text-foreground whitespace-pre-line break-words"
 >
 {{ normalizedLegacyContactInfo }}
 </p>
 </BaseDialog>
 </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SupportQRCodeEntry } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeSupportQRUrl } from '@/utils/url'

const props = defineProps<{
 entries: SupportQRCodeEntry[]
 legacyContactInfo?: string
 menuMode?: boolean
}>()

const emit = defineEmits<{ 'update:open': [open: boolean] }>()

const { t } = useI18n()
const showDialog = ref(false)
watch(showDialog, open => emit('update:open', open), { flush: 'sync' })

const normalizedEntries = computed(() =>
 (props.entries || [])
 .map((entry) => ({
 image_url: sanitizeSupportQRUrl(typeof entry?.image_url === 'string' ? entry.image_url : ''),
 note: entry.note?.trim() || '',
 }))
 .filter((entry) => entry.image_url)
)

const hasEntries = computed(() => normalizedEntries.value.length > 0)
const normalizedLegacyContactInfo = computed(() => props.legacyContactInfo?.trim() || '')
const hasLegacyContactInfo = computed(() => !hasEntries.value && normalizedLegacyContactInfo.value.length > 0)
</script>

<style scoped>
.support-menu-trigger {
 width: 100% !important;
 justify-content: flex-start;
 gap: 10px;
 font-size: 13px;
}
</style>
