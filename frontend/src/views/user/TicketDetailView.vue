<template>
  <AppLayout>
    <PageHeader
      :title="ticket?.title || ticket?.ticket_no || t('tickets.title')"
      :description="ticket ? `#${ticket.ticket_no} · ${t(`tickets.categories.${ticket.category}`)}` : t('tickets.detailConversationTitle')"
    >
      <template #actions>
        <Button variant="secondary" @click="goBack">
          <Icon name="arrowLeft" size="sm" :stroke-width="1.8" />
          {{ t('common.back') }}
        </Button>
      </template>
    </PageHeader>

    <GlassCard v-if="loading" class="detail-loading text-[13px]">
      {{ t('common.loading') }}
    </GlassCard>

    <div v-else-if="ticket && editing" class="ticket-edit-wrap">
      <TicketEditorCard
        :category="ticket.category"
        :title="ticket.title"
        :payload="ticket.current_form_payload || {}"
        :disable-category="true"
        :submit-label="t('tickets.resubmit')"
        :submitting="submittingEdit"
        :show-cancel="true"
        :user-concurrency="authStore.user?.concurrency ?? null"
        :rate-groups="rateGroups"
        @submit="saveAndSubmit"
        @cancel="exitEditMode"
      />
    </div>

    <DetailPageLayout v-else-if="ticket" class="ticket-detail-layout">
      <template #main>
        <TicketConversationPane
          :title="t('tickets.detailConversationTitle')"
          :subtitle="ticket.ticket_no"
          :messages="messages"
          :empty-text="t('tickets.emptyConversation')"
          :show-composer="canReply"
          :sending="sendingReply"
          :clear-composer-key="clearComposerKey"
          :composer-placeholder="t('tickets.replyPlaceholder')"
          :submit-text="t('tickets.reply')"
          :sending-text="t('common.submitting')"
          :ticket-id="ticketID"
          :upload-fn="uploadTicketMedia"
          :download-fn="downloadAttachment"
          @reply="reply"
          @upload-error="handleUploadError"
        />
      </template>

      <template #side>
        <TicketDetailPane :ticket="ticket">
          <template #actions>
            <div class="flex flex-wrap gap-3">
              <Button v-if="canWithdraw" variant="secondary" :disabled="actionLoading" @click="withdrawAndEdit">{{ t('tickets.actions.withdrawEdit') }}</Button>
              <Button v-if="ticket.status === 'withdrawn'" variant="secondary" @click="enterEditMode">{{ t('tickets.actions.edit') }}</Button>
              <Button v-if="canClose" variant="secondary" :disabled="actionLoading" @click="closeCurrentTicket">{{ t('tickets.actions.close') }}</Button>
            </div>
          </template>
        </TicketDetailPane>
      </template>
    </DetailPageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DetailPageLayout from '@/components/layout/DetailPageLayout.vue'
import Button from '@/components/ui/Button.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { ticketsAPI } from '@/api/tickets'
import { mediaAPI } from '@/api/media'
import TicketConversationPane from '@/components/tickets/TicketConversationPane.vue'
import TicketDetailPane from '@/components/tickets/TicketDetailPane.vue'
import TicketEditorCard from '@/components/tickets/TicketEditorCard.vue'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { notifyTicketUnreadChanged } from '@/utils/ticketForm'
import { validateTicketPayload } from '@/utils/tickets'
import type { SupportTicket, SupportTicketMessage, TicketCategory, TicketRateGroupOption } from '@/types/ticket'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const loading = ref(false)
const actionLoading = ref(false)
const sendingReply = ref(false)
const submittingEdit = ref(false)
const ticket = ref<SupportTicket | null>(null)
const messages = ref<SupportTicketMessage[]>([])
const rateGroups = ref<TicketRateGroupOption[]>([])
const clearComposerKey = ref(0)
const editing = ref(false)

const ticketID = computed(() => Number(route.params.id))
const canWithdraw = computed(() => ['submitted', 'processing', 'waiting_admin'].includes(ticket.value?.status || ''))
const canReply = computed(() => !['resolved', 'closed', 'withdrawn'].includes(ticket.value?.status || ''))
const canClose = computed(() => !['closed', 'withdrawn'].includes(ticket.value?.status || ''))
let loadDetailRequestID = 0

function ticketError(err: unknown) {
  return extractI18nErrorMessage(err, t, 'tickets.errors', t('common.unknownError'))
}

function goBack() {
  if (window.history.length > 1) {
    router.back()
    return
  }
  router.push('/tickets')
}

function syncEditingWithRoute() {
  editing.value = route.query.edit === '1' && ticket.value?.status === 'withdrawn'
}

async function replaceEditQuery(edit: boolean) {
  const nextQuery = { ...route.query }
  if (edit) {
    nextQuery.edit = '1'
  } else {
    delete nextQuery.edit
  }
  await router.replace({
    path: route.path,
    query: nextQuery,
  })
}

async function enterEditMode() {
  if (ticket.value?.status !== 'withdrawn') {
    return
  }
  editing.value = true
  await replaceEditQuery(true)
}

async function exitEditMode() {
  editing.value = false
  if (route.query.edit === '1') {
    await replaceEditQuery(false)
  }
}

async function loadDetail() {
  const requestID = ++loadDetailRequestID
  try {
    loading.value = true
    const [detail, messageData] = await Promise.all([
      ticketsAPI.get(ticketID.value),
      ticketsAPI.messages(ticketID.value),
    ])
    if (requestID !== loadDetailRequestID) {
      return
    }
    ticket.value = detail.data
    messages.value = messageData.data || []
    notifyTicketUnreadChanged()
    syncEditingWithRoute()
    if (route.query.edit === '1' && detail.data.status !== 'withdrawn') {
      void replaceEditQuery(false)
    }
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

async function loadTicketContext() {
  try {
    const res = await ticketsAPI.rateGroups()
    rateGroups.value = res.data || []
  } catch {
    rateGroups.value = []
  }
}

async function uploadTicketMedia(file: File, ticketId: number | string) {
  const uploaded = await mediaAPI.upload(file, { biz_type: 'ticket', biz_id: String(ticketId), visibility: 'private' })
  return uploaded.data
}

async function downloadAttachment(mediaId: number) {
  const res = await ticketsAPI.downloadGrant(ticketID.value, mediaId)
  return res.data.url
}

async function reply(content: string, attachments?: { media_id: number }[]) {
  try {
    sendingReply.value = true
    await ticketsAPI.reply(ticketID.value, {
      content,
      media_ids: attachments?.map((item) => item.media_id),
    })
    clearComposerKey.value += 1
    await loadDetail()
  } catch (err: unknown) {
    appStore.showError(ticketError(err))
  } finally {
    sendingReply.value = false
  }
}

function handleUploadError() {
  appStore.showError(t('tickets.uploadFailed'))
}

async function withdrawAndEdit() {
  try {
    actionLoading.value = true
    await ticketsAPI.withdraw(ticketID.value)
    await loadDetail()
    await enterEditMode()
    appStore.showSuccess(t('tickets.messages.withdrawn'))
  } catch (err: unknown) {
    appStore.showError(ticketError(err))
  } finally {
    actionLoading.value = false
  }
}

async function saveAndSubmit(form: { category: TicketCategory; title: string; form_payload: Record<string, unknown> }) {
  if (!ticket.value) {
    return
  }
  const validationKey = validateTicketPayload(form.category, form.title, form.form_payload)
  if (validationKey) {
    appStore.showError(t(validationKey))
    return
  }
  try {
    submittingEdit.value = true
    await ticketsAPI.resubmit(ticketID.value, {
      title: form.title,
      form_payload: form.form_payload,
      expected_revision_no: ticket.value.current_revision_no,
    })
    await exitEditMode()
    await loadDetail()
    appStore.showSuccess(t('tickets.messages.resubmitted'))
  } catch (err: unknown) {
    appStore.showError(ticketError(err))
  } finally {
    submittingEdit.value = false
  }
}

async function closeCurrentTicket() {
  try {
    actionLoading.value = true
    await ticketsAPI.close(ticketID.value)
    await loadDetail()
    appStore.showSuccess(t('tickets.messages.closed'))
  } catch (err: unknown) {
    appStore.showError(ticketError(err))
  } finally {
    actionLoading.value = false
  }
}

onMounted(loadTicketContext)

watch(ticketID, () => {
  loadDetail()
}, { immediate: true })

watch(
  [() => route.query.edit, () => ticket.value?.status],
  () => {
    syncEditingWithRoute()
  },
  { immediate: true },
)
</script>

<style scoped>
.detail-loading {
  padding: 40px;
  text-align: center;
  color: var(--muted);
}

.ticket-edit-wrap {
  display: flex;
  min-height: calc(100vh - 10rem);
}

.ticket-edit-wrap :deep(> *) {
  flex: 1;
  min-height: 0;
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
</style>
