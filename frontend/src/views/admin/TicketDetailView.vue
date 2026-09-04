<template>
  <AppLayout>
    <PageHeader
      :title="ticket?.title || ticket?.ticket_no || t('nav.ticketManagement')"
      :description="ticket ? `#${ticket.ticket_no} · ${t(`tickets.categories.${ticket.category}`)}` : t('tickets.detailConversationTitle')"
    >
      <template #actions>
        <Button variant="secondary" @click="goBack">
          <Icon name="arrowLeft" size="sm" :stroke-width="1.8" />
          {{ t('common.back') }}
        </Button>
      </template>
    </PageHeader>

    <GlassCard v-if="loading" class="detail-loading">
      {{ t('common.loading') }}
    </GlassCard>

    <DetailPageLayout v-else-if="ticket" class="ticket-detail-layout">
      <template #main>
        <TicketConversationPane
          :title="t('tickets.detailConversationTitle')"
          :subtitle="ticket.ticket_no"
          :messages="messages"
          :empty-text="t('tickets.emptyConversation')"
          :show-composer="canReply"
          :sending="sendingReply"
          :reply-content="replyDraft"
          :clear-composer-key="clearComposerKey"
          :composer-placeholder="t('tickets.replyPlaceholderAdmin')"
          :submit-text="t('tickets.reply')"
          :sending-text="t('common.submitting')"
          :ticket-id="ticketID"
          :upload-fn="uploadAdminTicketMedia"
          :download-fn="downloadAttachment"
          @update:reply-content="replyDraft = $event"
          @reply="reply"
          @upload-error="handleUploadError"
        >
          <template #composer-actions>
            <div class="template-rail" role="group" :aria-label="t('tickets.templates.button')">
              <button
                v-for="template in replyTemplates"
                :key="template.id"
                ref="templateTriggerRef"
                type="button"
                class="filter-pill template-pill"
                :title="template.content"
                @click="applyTemplate(template.content)"
              >
                <span class="filter-pill-value">{{ template.title }}</span>
              </button>
              <span v-if="replyTemplates.length === 0" class="template-empty">{{ t('tickets.templates.empty') }}</span>
              <button type="button" class="filter-pill template-pill" @click="openTemplateDialog">
                <span class="filter-pill-label">{{ t('tickets.templates.manage') }}</span>
              </button>
            </div>
          </template>
        </TicketConversationPane>
      </template>

      <template #side>
        <TicketDetailPane :ticket="ticket" show-user-meta>
          <template #actions>
            <div v-if="canUpdateStatus" class="status-block">
              <p class="status-label">{{ t('tickets.adminActions') }}</p>
              <UiSelect
                :model-value="ticket.status"
                :options="statusActionOptions"
                :disabled="actionLoading"
                :searchable="false"
                :aria-label="t('tickets.adminActions')"
                @update:model-value="handleStatusSelect"
              />
            </div>
          </template>
        </TicketDetailPane>
      </template>
    </DetailPageLayout>

    <TicketReplyTemplatesDialog
      :show="showTemplateDialog"
      :templates="replyTemplates"
      :saving="savingTemplates"
      @close="closeTemplateDialog"
      @save="saveTemplates"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DetailPageLayout from '@/components/layout/DetailPageLayout.vue'
import Button from '@/components/ui/Button.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { adminTicketsAPI } from '@/api/admin/tickets'
import { adminMediaAPI } from '@/api/media'
import TicketConversationPane from '@/components/tickets/TicketConversationPane.vue'
import TicketDetailPane from '@/components/tickets/TicketDetailPane.vue'
import TicketReplyTemplatesDialog from '@/components/tickets/TicketReplyTemplatesDialog.vue'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { notifyTicketUnreadChanged } from '@/utils/ticketForm'
import type { SupportTicket, SupportTicketMessage, TicketReplyTemplate, TicketStatus } from '@/types/ticket'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const loading = ref(false)
const actionLoading = ref(false)
const sendingReply = ref(false)
const ticket = ref<SupportTicket | null>(null)
const messages = ref<SupportTicketMessage[]>([])
const clearComposerKey = ref(0)
const replyDraft = ref('')
const replyTemplates = ref<TicketReplyTemplate[]>([])
const showTemplateDialog = ref(false)
const savingTemplates = ref(false)
const templateTriggerRef = ref<HTMLButtonElement | null>(null)
const adminStatuses: TicketStatus[] = ['processing', 'waiting_user', 'waiting_admin', 'resolved', 'closed']
const canReply = computed(() => !['resolved', 'closed', 'withdrawn'].includes(ticket.value?.status || ''))
const availableAdminStatuses = computed(() => {
  if (!ticket.value || ticket.value.status === 'withdrawn') {
    return []
  }

  return adminStatuses.filter((status) => isAdminStatusActionAllowed(ticket.value!.status, ticket.value!.last_reply_role, status))
})
const canUpdateStatus = computed(() => availableAdminStatuses.value.length > 0)
const statusActionOptions = computed(() =>
  availableAdminStatuses.value.map((status) => ({
    value: status,
    label: t(`tickets.statuses.${status}`),
  })),
)
const ticketID = computed(() => Number(route.params.id))
let loadDetailRequestID = 0
let replyRequestID = 0
let statusRequestID = 0
let saveTemplatesRequestID = 0
let templateDialogSessionID = 0

function ticketError(err: unknown) {
  return extractI18nErrorMessage(err, t, 'tickets.errors', t('common.unknownError'))
}

function goBack() {
  if (window.history.length > 1) {
    router.back()
    return
  }
  router.push('/admin/tickets')
}

function isAdminStatusActionAllowed(currentStatus: TicketStatus, lastReplyRole?: string, nextStatus?: TicketStatus) {
  if (!nextStatus) return false
  if (currentStatus === 'closed') return nextStatus === 'closed'
  if (currentStatus === 'resolved') return nextStatus === 'resolved' || nextStatus === 'closed'
  if (nextStatus === 'waiting_user' || nextStatus === 'resolved') {
    return lastReplyRole === 'admin'
  }
  return true
}

async function loadDetail(targetTicketID = ticketID.value) {
  const requestID = ++loadDetailRequestID
  try {
    loading.value = true
    const [detail, messageData] = await Promise.all([
      adminTicketsAPI.get(targetTicketID),
      adminTicketsAPI.messages(targetTicketID),
    ])
    if (requestID !== loadDetailRequestID) {
      return
    }
    ticket.value = detail.data
    messages.value = Array.isArray(messageData.data) ? messageData.data : []
    notifyTicketUnreadChanged()
  } catch (err: unknown) {
    if (requestID !== loadDetailRequestID) {
      return
    }
    appStore.showError(ticketError(err))
  } finally {
    if (requestID === loadDetailRequestID) {
      loading.value = false
    }
  }
}

async function loadReplyTemplates() {
  try {
    const res = await adminTicketsAPI.listTemplates()
    replyTemplates.value = res.data || []
  } catch (err: unknown) {
    appStore.showError(ticketError(err))
  }
}

async function reply(content: string, attachments?: { media_id: number }[]) {
  const currentTicketID = ticketID.value
  const requestID = ++replyRequestID
  try {
    sendingReply.value = true
    await adminTicketsAPI.reply(currentTicketID, {
      content,
      media_ids: attachments?.map((item) => item.media_id),
    })
    if (requestID !== replyRequestID || currentTicketID !== ticketID.value) {
      return
    }
    replyDraft.value = ''
    clearComposerKey.value += 1
    await loadDetail(currentTicketID)
  } catch (err: unknown) {
    if (requestID !== replyRequestID || currentTicketID !== ticketID.value) {
      return
    }
    appStore.showError(ticketError(err))
  } finally {
    if (requestID === replyRequestID) {
      sendingReply.value = false
    }
  }
}

async function uploadAdminTicketMedia(file: File, ticketId: number | string) {
  if (!ticket.value) {
    throw new Error('ticket missing')
  }
  const uploaded = await adminMediaAPI.upload(file, {
    owner_user_id: ticket.value.user_id,
    biz_type: 'ticket',
    biz_id: String(ticketId),
    visibility: 'private',
  })
  return uploaded.data
}

async function downloadAttachment(mediaId: number) {
  const res = await adminTicketsAPI.downloadGrant(ticketID.value, mediaId)
  return res.data.url
}

function handleUploadError() {
  appStore.showError(t('tickets.uploadFailed'))
}

function applyTemplate(content: string) {
  replyDraft.value = content
  nextTick(() => {
    templateTriggerRef.value?.focus()
  })
}

function openTemplateDialog() {
  templateDialogSessionID += 1
  showTemplateDialog.value = true
}

function closeTemplateDialog() {
  showTemplateDialog.value = false
  savingTemplates.value = false
  templateDialogSessionID += 1
}

async function saveTemplates(templates: TicketReplyTemplate[]) {
  const requestID = ++saveTemplatesRequestID
  const dialogSessionID = templateDialogSessionID
  try {
    savingTemplates.value = true
    const original = replyTemplates.value
    const nextExistingIds = new Set(templates.filter((item) => item.id > 0).map((item) => item.id))
    for (const item of original) {
      if (!nextExistingIds.has(item.id)) {
        await adminTicketsAPI.deleteTemplate(item.id)
      }
    }
    for (const item of templates) {
      const title = item.title.trim()
      const content = item.content.trim()
      if (!title || !content) continue
      if (item.id > 0) {
        await adminTicketsAPI.updateTemplate(item.id, { title, content, sort_order: item.sort_order })
      } else {
        await adminTicketsAPI.createTemplate({ title, content, sort_order: item.sort_order })
      }
    }
    if (requestID !== saveTemplatesRequestID) {
      return
    }
    const latest = await adminTicketsAPI.listTemplates()
    if (requestID !== saveTemplatesRequestID) {
      return
    }
    replyTemplates.value = latest.data || []
    if (showTemplateDialog.value && dialogSessionID === templateDialogSessionID) {
      showTemplateDialog.value = false
      nextTick(() => {
        templateTriggerRef.value?.focus()
      })
    }
    appStore.showSuccess(t('common.saved'))
  } catch (err: unknown) {
    if (requestID !== saveTemplatesRequestID) {
      return
    }
    appStore.showError(ticketError(err))
  } finally {
    if (requestID === saveTemplatesRequestID) {
      savingTemplates.value = false
    }
  }
}

async function updateStatus(status: TicketStatus) {
  const currentTicketID = ticketID.value
  const requestID = ++statusRequestID
  try {
    actionLoading.value = true
    await adminTicketsAPI.updateStatus(currentTicketID, status)
    if (requestID !== statusRequestID || currentTicketID !== ticketID.value) {
      return
    }
    await loadDetail(currentTicketID)
    appStore.showSuccess(t('tickets.messages.statusUpdated'))
  } catch (err: unknown) {
    if (requestID !== statusRequestID || currentTicketID !== ticketID.value) {
      return
    }
    appStore.showError(ticketError(err))
  } finally {
    if (requestID === statusRequestID) {
      actionLoading.value = false
    }
  }
}

function handleStatusSelect(value: string | number | boolean | null) {
  if (typeof value !== 'string' || !value) {
    return
  }
  updateStatus(value as TicketStatus)
}

onMounted(loadReplyTemplates)

onUnmounted(() => {
  loadDetailRequestID += 1
  replyRequestID += 1
  statusRequestID += 1
  saveTemplatesRequestID += 1
})

watch(ticketID, (nextTicketID, previousTicketID) => {
  if (nextTicketID !== previousTicketID && previousTicketID !== undefined) {
    replyDraft.value = ''
    clearComposerKey.value += 1
  }
  loadDetail()
}, { immediate: true })
</script>

<style scoped>
.detail-loading {
  padding: 40px;
  text-align: center;
  font-size: 13px;
  color: var(--muted);
}

/* The conversation pane and side meta card both size themselves to 100% of
   their parent (with their own internal scroll areas), so the layout needs
   an explicit, viewport-relative height here — DetailPageLayout itself stays
   height-agnostic for pages that just want natural page scroll. */
.ticket-detail-layout :deep(.detail-page-layout-grid) {
  height: calc(100vh - 10rem);
  min-height: calc(100vh - 10rem);
  align-items: stretch;
}

.ticket-detail-layout :deep(.detail-page-layout-main),
.ticket-detail-layout :deep(.detail-page-layout-side) {
  min-height: 0;
  height: 100%;
  overflow: hidden;
}

.status-block {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.status-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--foreground);
}

.template-rail {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.template-pill {
  max-width: 220px;
  overflow: hidden;
}

.template-pill .filter-pill-value {
  overflow: hidden;
  text-overflow: ellipsis;
}

.template-empty {
  font-size: 12px;
  color: var(--muted);
}
</style>
