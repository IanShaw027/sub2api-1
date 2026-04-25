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
          @update:reply-content="replyDraft = $event"
          @reply="reply"
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
                <button v-for="status in adminStatuses" :key="status" class="btn btn-secondary btn-sm" :disabled="actionLoading || ticket.status === status || isStatusLocked(status)" @click="updateStatus(status)">
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
      @close="showTemplateDialog = false"
      @save="saveTemplates"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores'
import adminTicketsAPI from '@/api/adminTickets'
import TicketConversationPane from '@/components/tickets/TicketConversationPane.vue'
import TicketDetailPane from '@/components/tickets/TicketDetailPane.vue'
import TicketReplyTemplatesDialog from '@/components/tickets/TicketReplyTemplatesDialog.vue'
import type { SupportTicket, SupportTicketMessage, TicketStatus } from '@/types'
import type { TicketReplyTemplate } from '@/api/adminTickets'

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
const canUpdateStatus = computed(() => ticket.value?.status !== 'withdrawn')
const ticketID = computed(() => Number(route.params.id))
let loadDetailRequestID = 0

function isStatusLocked(target: TicketStatus) {
  if (!ticket.value) return false
  if (ticket.value.status === 'closed') return target !== 'closed'
  if (ticket.value.status === 'resolved') return target !== 'resolved' && target !== 'closed'
  return false
}

async function loadDetail() {
  const requestID = ++loadDetailRequestID
  try {
    loading.value = true
    const [ticketData, messageData] = await Promise.all([
      adminTicketsAPI.getAdminTicket(ticketID.value),
      adminTicketsAPI.listAdminTicketMessages(ticketID.value),
    ])
    if (requestID !== loadDetailRequestID) {
      return
    }
    ticket.value = ticketData
    messages.value = messageData
  } catch (err: any) {
    if (requestID !== loadDetailRequestID) {
      return
    }
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    if (requestID === loadDetailRequestID) {
      loading.value = false
    }
  }
}

async function loadReplyTemplates() {
  try {
    replyTemplates.value = await adminTicketsAPI.listAdminTicketReplyTemplates()
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  }
}

async function reply(content: string) {
  try {
    sendingReply.value = true
    await adminTicketsAPI.replyAdminTicket(ticketID.value, content)
    replyDraft.value = ''
    clearComposerKey.value += 1
    await loadDetail()
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    sendingReply.value = false
  }
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
  showTemplateDialog.value = true
}

async function saveTemplates(templates: TicketReplyTemplate[]) {
  try {
    savingTemplates.value = true
    await adminTicketsAPI.replaceAdminTicketReplyTemplates(templates)
    replyTemplates.value = await adminTicketsAPI.listAdminTicketReplyTemplates()
    showTemplateDialog.value = false
    appStore.showSuccess(t('common.saved'))
    nextTick(() => {
      templateTriggerRef.value?.focus()
    })
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    savingTemplates.value = false
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
  try {
    actionLoading.value = true
    await adminTicketsAPI.updateAdminTicketStatus(ticketID.value, status)
    await loadDetail()
    appStore.showSuccess(t('tickets.messages.statusUpdated'))
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    actionLoading.value = false
  }
}

onMounted(loadReplyTemplates)

watch(ticketID, () => {
  loadDetail()
}, { immediate: true })
</script>
