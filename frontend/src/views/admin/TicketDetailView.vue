<template>
  <AppLayout>
    <div v-if="loading" class="rounded-2xl border bg-white p-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="ticket" class="grid gap-6 xl:grid-cols-[1.35fr_0.95fr]">
      <TicketConversationPane
        :title="t('tickets.detailConversationTitle')"
        :subtitle="ticket.ticket_no"
        :messages="messages"
        :empty-text="t('tickets.emptyConversation')"
        :show-composer="canReply"
        :sending="sendingReply"
        :composer-placeholder="t('tickets.replyPlaceholderAdmin')"
        :submit-text="t('tickets.reply')"
        :sending-text="t('common.submitting')"
        @reply="reply"
      />

      <TicketDetailPane :ticket="ticket" show-user-meta>
        <template #actions>
          <div class="space-y-3">
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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores'
import adminTicketsAPI from '@/api/adminTickets'
import TicketConversationPane from '@/components/tickets/TicketConversationPane.vue'
import TicketDetailPane from '@/components/tickets/TicketDetailPane.vue'
import type { SupportTicket, SupportTicketMessage, TicketStatus } from '@/types'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const loading = ref(false)
const actionLoading = ref(false)
const sendingReply = ref(false)
const ticket = ref<SupportTicket | null>(null)
const messages = ref<SupportTicketMessage[]>([])
const adminStatuses: TicketStatus[] = ['processing', 'waiting_user', 'waiting_admin', 'resolved', 'closed']
const canReply = computed(() => !['resolved', 'closed'].includes(ticket.value?.status || ''))

function isStatusLocked(target: TicketStatus) {
  if (!ticket.value) return false
  if (ticket.value.status === 'closed') return target !== 'closed'
  if (ticket.value.status === 'resolved') return target !== 'resolved' && target !== 'closed'
  return false
}

async function loadDetail() {
  try {
    loading.value = true
    const id = Number(route.params.id)
    const [ticketData, messageData] = await Promise.all([
      adminTicketsAPI.getAdminTicket(id),
      adminTicketsAPI.listAdminTicketMessages(id),
    ])
    ticket.value = ticketData
    messages.value = messageData
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    loading.value = false
  }
}

async function reply(content: string) {
  try {
    sendingReply.value = true
    await adminTicketsAPI.replyAdminTicket(Number(route.params.id), content)
    await loadDetail()
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    sendingReply.value = false
  }
}

async function updateStatus(status: TicketStatus) {
  try {
    actionLoading.value = true
    await adminTicketsAPI.updateAdminTicketStatus(Number(route.params.id), status)
    await loadDetail()
    appStore.showSuccess(t('tickets.messages.statusUpdated'))
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    actionLoading.value = false
  }
}

onMounted(loadDetail)
</script>
