<template>
  <AppLayout>
    <div v-if="loading" class="rounded-2xl border bg-white p-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="ticket" class="grid h-[calc(100vh-10rem)] min-h-[calc(100vh-10rem)] min-w-0 gap-6 overflow-hidden xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,0.95fr)]">
      <div class="min-h-0">
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
            <div
              ref="templateMenuRef"
              class="relative"
              @focusin="showTemplateMenu = true"
              @focusout="handleTemplateFocusOut"
              @mouseenter="showTemplateMenu = true"
              @mouseleave="handleTemplateMouseLeave"
            >
              <button
                ref="templateTriggerRef"
                type="button"
                class="btn btn-secondary"
                aria-haspopup="menu"
                :aria-expanded="showTemplateMenu ? 'true' : 'false'"
                @click="toggleTemplateMenu"
                @keydown.enter.prevent="openTemplateMenuAndFocusFirst"
                @keydown.space.prevent="openTemplateMenuAndFocusFirst"
                @keydown.down.prevent="openTemplateMenuAndFocusFirst"
              >
                {{ t('tickets.templates.button') }}
              </button>

              <div
                v-if="showTemplateMenu"
                ref="templateMenuListRef"
                role="menu"
                tabindex="-1"
                class="absolute bottom-full left-0 z-20 mb-2 w-72 overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-xl dark:border-dark-700 dark:bg-dark-800"
                @keydown.esc.prevent="closeTemplateMenuAndRestoreFocus"
              >
                <div v-if="replyTemplates.length > 0" class="max-h-80 overflow-y-auto py-2">
                  <button
                    v-for="template in replyTemplates"
                    :key="template.id"
                    type="button"
                    role="menuitem"
                    class="flex w-full flex-col items-start px-4 py-3 text-left transition-colors hover:bg-gray-50 dark:hover:bg-dark-700"
                    @mousedown.prevent
                    @click="applyTemplate(template.content)"
                  >
                    <span class="text-sm font-medium text-gray-900 dark:text-white">{{ template.title }}</span>
                    <span class="mt-1 line-clamp-2 text-xs text-gray-500 dark:text-gray-400">{{ template.content }}</span>
                  </button>
                </div>
                <div v-else class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
                  {{ t('tickets.templates.empty') }}
                </div>
                <div class="border-t border-gray-100 p-2 dark:border-dark-700">
                  <button
                    type="button"
                    role="menuitem"
                    class="btn btn-secondary btn-sm w-full"
                    @mousedown.prevent
                    @click="openTemplateDialog"
                  >
                    {{ t('tickets.templates.manage') }}
                  </button>
                </div>
              </div>
            </div>
          </template>
        </TicketConversationPane>
      </div>

      <div class="min-h-0 h-full">
        <TicketDetailPane :ticket="ticket" show-user-meta>
          <template #actions>
            <div v-if="canUpdateStatus" class="space-y-3">
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('tickets.adminActions') }}</p>
              <div class="flex flex-wrap gap-3">
                <button v-for="status in availableAdminStatuses" :key="status" class="btn btn-secondary btn-sm" :disabled="actionLoading || ticket.status === status" @click="updateStatus(status)">
                  {{ t(`tickets.statuses.${status}`) }}
                </button>
              </div>
            </div>
          </template>
        </TicketDetailPane>
      </div>
    </div>

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
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
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
const appStore = useAppStore()
const loading = ref(false)
const actionLoading = ref(false)
const sendingReply = ref(false)
const ticket = ref<SupportTicket | null>(null)
const messages = ref<SupportTicketMessage[]>([])
const clearComposerKey = ref(0)
const replyDraft = ref('')
const replyTemplates = ref<TicketReplyTemplate[]>([])
const showTemplateMenu = ref(false)
const showTemplateDialog = ref(false)
const savingTemplates = ref(false)
const templateMenuRef = ref<HTMLElement | null>(null)
const templateTriggerRef = ref<HTMLButtonElement | null>(null)
const templateMenuListRef = ref<HTMLElement | null>(null)
const adminStatuses: TicketStatus[] = ['processing', 'waiting_user', 'waiting_admin', 'resolved', 'closed']
const canReply = computed(() => !['resolved', 'closed', 'withdrawn'].includes(ticket.value?.status || ''))
const availableAdminStatuses = computed(() => {
  if (!ticket.value || ticket.value.status === 'withdrawn') {
    return []
  }

  return adminStatuses.filter((status) => isAdminStatusActionAllowed(ticket.value!.status, ticket.value!.last_reply_role, status))
})
const canUpdateStatus = computed(() => availableAdminStatuses.value.length > 0)
const ticketID = computed(() => Number(route.params.id))
let loadDetailRequestID = 0
let replyRequestID = 0
let statusRequestID = 0
let saveTemplatesRequestID = 0
let templateDialogSessionID = 0

function ticketError(err: unknown) {
  return extractI18nErrorMessage(err, t, 'tickets.errors', t('common.unknownError'))
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
    messages.value = messageData.data || []
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
  showTemplateMenu.value = false
  nextTick(() => {
    templateTriggerRef.value?.focus()
  })
}

function openTemplateDialog() {
  showTemplateMenu.value = false
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

function toggleTemplateMenu() {
  if (showTemplateMenu.value) {
    showTemplateMenu.value = false
    return
  }
  showTemplateMenu.value = true
}

async function openTemplateMenuAndFocusFirst() {
  showTemplateMenu.value = true
  await nextTick()
  const firstItem = templateMenuListRef.value?.querySelector<HTMLElement>('[role="menuitem"]')
  firstItem?.focus()
}

function closeTemplateMenuAndRestoreFocus() {
  showTemplateMenu.value = false
  templateTriggerRef.value?.focus()
}

function handleTemplateFocusOut(event: FocusEvent) {
  const nextTarget = event.relatedTarget
  if (nextTarget instanceof Node && templateMenuRef.value?.contains(nextTarget)) {
    return
  }
  showTemplateMenu.value = false
}

function handleTemplateMouseLeave() {
  if (templateMenuRef.value?.contains(document.activeElement)) {
    return
  }
  showTemplateMenu.value = false
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
