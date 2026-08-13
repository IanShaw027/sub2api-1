<template>
  <AppLayout>
    <div v-if="loading" class="rounded-2xl border bg-white p-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="ticket" class="grid h-[calc(100vh-10rem)] min-h-[calc(100vh-10rem)] min-w-0 gap-6 overflow-hidden xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,0.95fr)]">
      <div v-if="editing" class="min-h-0 xl:col-span-2">
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

      <template v-else>
        <div class="min-h-0">
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
        </div>

        <div class="min-h-0 h-full">
          <TicketDetailPane :ticket="ticket">
            <template #actions>
              <div class="flex flex-wrap gap-3">
                <button v-if="canWithdraw" class="btn btn-secondary" :disabled="actionLoading" @click="withdrawAndEdit">{{ t('tickets.actions.withdrawEdit') }}</button>
                <button v-if="ticket.status === 'withdrawn'" class="btn btn-secondary" @click="enterEditMode">{{ t('tickets.actions.edit') }}</button>
                <button v-if="canClose" class="btn btn-secondary" :disabled="actionLoading" @click="closeCurrentTicket">{{ t('tickets.actions.close') }}</button>
              </div>
            </template>
          </TicketDetailPane>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
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
