<template>
  <div v-if="mode === 'checkbox' && documents.length > 0" class="agreement-consent">
    <Checkbox
      id="login-agreement-consent"
      :model-value="accepted"
      @update:model-value="handleCheckboxChange"
    >
      <span class="agreement-consent-text">
        {{ t('legal.loginAgreementPrompt.checkboxPrefix') }}
        <template v-for="(doc, index) in documents" :key="doc.id || doc.title">
          <RouterLink
            :to="documentRoute(doc)"
            target="_blank"
            rel="noopener noreferrer"
            class="agreement-link"
          >
            {{ doc.title }}
          </RouterLink>
          <span v-if="index < documents.length - 1">{{ t('legal.loginAgreementPrompt.documentSeparator') }}</span>
        </template>
      </span>
    </Checkbox>
  </div>

  <div v-else-if="!accepted && documents.length > 0" class="notice notice-info agreement-notice">
    <Icon name="shield" size="sm" class="agreement-notice-icon" />
    <div class="agreement-notice-body">
      <p class="agreement-notice-title">{{ t('legal.loginAgreementPrompt.noticeTitle') }}</p>
      <p class="agreement-notice-desc">{{ t('legal.loginAgreementPrompt.noticeDescription') }}</p>
    </div>
    <button type="button" class="btn btn-primary btn-xs agreement-notice-action" @click="emit('open')">
      {{ t('legal.loginAgreementPrompt.viewTerms') }}
    </button>
  </div>

  <UiModal
    :open="dialogVisible"
    :title="t('legal.loginAgreementPrompt.dialogTitle')"
    width="lg"
    :close-on-overlay="false"
    :show-close="false"
    @close="emit('reject')"
  >
    <div class="agreement-dialog">
      <div class="agreement-dialog-head">
        <span v-if="updatedAt" class="badge badge-gray">{{ updatedAt }}</span>
        <p class="agreement-dialog-desc">
          {{
            t('legal.loginAgreementPrompt.dialogDescription', {
              date: updatedAt || t('legal.loginAgreementPrompt.recently')
            })
          }}
        </p>
      </div>

      <p class="agreement-dialog-label">{{ t('legal.loginAgreementPrompt.relatedDocuments') }}</p>
      <div class="agreement-doc-grid">
        <RouterLink
          v-for="(doc, index) in documents"
          :key="doc.id || doc.title"
          :to="documentRoute(doc)"
          target="_blank"
          rel="noopener noreferrer"
          class="agreement-doc glass-card-flat"
        >
          <span class="agreement-doc-icon">
            <Icon :name="documentIcon(index, doc.title)" size="sm" />
          </span>
          <span class="agreement-doc-title">{{ doc.title }}</span>
          <Icon name="externalLink" size="sm" class="agreement-doc-out" />
        </RouterLink>
      </div>
    </div>

    <template #footer>
      <button type="button" class="btn btn-secondary btn-md agreement-footer-btn" @click="emit('reject')">
        {{ t('legal.loginAgreementPrompt.reject') }}
      </button>
      <button type="button" class="btn btn-primary btn-md agreement-footer-btn" @click="emit('accept')">
        {{ t('legal.loginAgreementPrompt.accept') }}
      </button>
    </template>
  </UiModal>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Checkbox from '@/components/ui/Checkbox.vue'
import UiModal from '@/components/ui/UiModal.vue'
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

const documents = computed(() => props.documents.filter((doc) => doc.title.trim()))
const dialogVisible = computed(() => props.visible && documents.value.length > 0)
const updatedAt = computed(() => props.updatedAt || '')
const accepted = computed(() => props.accepted)
const mode = computed(() => (props.mode === 'checkbox' ? 'checkbox' : 'modal'))

function documentRoute(doc: LoginAgreementDocument) {
  return {
    name: 'LegalDocument',
    params: {
      documentId: doc.id || doc.title,
    },
  }
}

function handleCheckboxChange(checked: boolean): void {
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
.agreement-consent-text {
  font-size: 12.5px;
  line-height: 1.6;
  color: var(--muted);
}

.agreement-link {
  font-weight: 600;
  color: var(--accent);
  text-decoration: none;
}

.agreement-link:hover {
  text-decoration: underline;
}

.agreement-notice {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.agreement-notice-icon {
  flex: none;
  margin-top: 1px;
}

.agreement-notice-body {
  min-width: 0;
  flex: 1;
}

.agreement-notice-title {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
}

.agreement-notice-desc {
  margin: 4px 0 0;
  font-size: 12.5px;
  line-height: 1.55;
}

.agreement-notice-action {
  flex: none;
}

.agreement-dialog {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.agreement-dialog-head {
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: flex-start;
}

.agreement-dialog-desc {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--muted);
}

.agreement-dialog-label {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--muted);
}

.agreement-doc-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

@media (max-width: 640px) {
  .agreement-doc-grid {
    grid-template-columns: 1fr;
  }
}

.agreement-doc {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 56px;
  padding: 10px 12px;
  border-radius: var(--radius-field);
  text-decoration: none;
  color: var(--foreground);
  transition: border-color 0.15s ease, background 0.15s ease, transform 0.15s ease;
}

.agreement-doc:hover {
  transform: translateY(-1px);
  border-color: color-mix(in oklch, var(--accent) 32%, transparent);
}

.agreement-doc-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  flex: none;
  border-radius: 9px;
  background: color-mix(in oklch, var(--accent) 12%, transparent);
  color: var(--accent);
}

.agreement-doc-title {
  min-width: 0;
  flex: 1;
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agreement-doc-out {
  flex: none;
  color: var(--muted);
}

.agreement-footer-btn {
  flex: 1;
}
</style>
