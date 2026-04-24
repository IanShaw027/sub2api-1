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
        :composer-placeholder="t('tickets.replyPlaceholder')"
        :submit-text="t('tickets.reply')"
        :sending-text="t('common.submitting')"
        @reply="reply"
      />

      <div class="space-y-4">
        <TicketEditorCard
          v-if="editing"
          :category="ticket.category"
          :title="ticket.title"
          :payload="ticket.current_form_payload || {}"
          :disable-category="true"
          :submit-label="t('tickets.resubmit')"
          :submitting="submittingEdit"
          :show-cancel="true"
          @submit="saveAndSubmit"
          @cancel="editing = false"
        />

        <TicketDetailPane v-else :ticket="ticket">
          <template #actions>
            <div class="flex flex-wrap gap-3">
              <button v-if="canWithdraw" class="btn btn-secondary" :disabled="actionLoading" @click="withdrawAndEdit">{{ t('tickets.actions.withdrawEdit') }}</button>
              <button v-if="ticket.status === 'withdrawn'" class="btn btn-secondary" @click="editing = true">{{ t('tickets.actions.edit') }}</button>
              <button class="btn btn-secondary" :disabled="ticket.status === 'closed' || actionLoading" @click="closeCurrentTicket">{{ t('tickets.actions.close') }}</button>
            </div>
          </template>
        </TicketDetailPane>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores'
import ticketsAPI from '@/api/tickets'
import TicketConversationPane from '@/components/tickets/TicketConversationPane.vue'
import TicketDetailPane from '@/components/tickets/TicketDetailPane.vue'
import TicketEditorCard from '@/components/tickets/TicketEditorCard.vue'
import { validateTicketPayload } from '@/utils/tickets'
import type { SupportTicket, SupportTicketMessage, TicketCategory } from '@/types'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()

const loading = ref(false)
const actionLoading = ref(false)
const sendingReply = ref(false)
const submittingEdit = ref(false)
const ticket = ref<SupportTicket | null>(null)
const messages = ref<SupportTicketMessage[]>([])
const editing = ref(route.query.edit === '1')

const ticketID = computed(() => Number(route.params.id))
const canWithdraw = computed(() => ['submitted', 'processing', 'waiting_admin'].includes(ticket.value?.status || ''))
const canReply = computed(() => !['resolved', 'closed'].includes(ticket.value?.status || ''))

async function loadDetail() {
  try {
    loading.value = true
    const [ticketData, messageData] = await Promise.all([
      ticketsAPI.getTicket(ticketID.value),
      ticketsAPI.listTicketMessages(ticketID.value),
    ])
    ticket.value = ticketData
    editing.value = route.query.edit === '1' && ticketData.status === 'withdrawn'
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
    await ticketsAPI.replyTicket(ticketID.value, content)
    await loadDetail()
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    sendingReply.value = false
  }
}

async function withdrawAndEdit() {
  try {
    actionLoading.value = true
    await ticketsAPI.withdrawTicket(ticketID.value)
    await loadDetail()
    editing.value = true
    appStore.showSuccess(t('tickets.messages.withdrawn'))
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    actionLoading.value = false
  }
}

async function saveAndSubmit(form: { category: TicketCategory; title: string; form_payload: Record<string, unknown> }) {
  const validationKey = validateTicketPayload(form.category, form.title, form.form_payload)
  if (validationKey) {
    appStore.showError(t(validationKey))
    return
  }
  try {
    submittingEdit.value = true
    await ticketsAPI.updateTicket(ticketID.value, form)
    await ticketsAPI.submitTicket(ticketID.value, form)
    editing.value = false
    await loadDetail()
    appStore.showSuccess(t('tickets.messages.resubmitted'))
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    submittingEdit.value = false
  }
}

async function closeCurrentTicket() {
  try {
    actionLoading.value = true
    await ticketsAPI.closeTicket(ticketID.value)
    await loadDetail()
    appStore.showSuccess(t('tickets.messages.closed'))
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    actionLoading.value = false
  }
}

onMounted(loadDetail)
</script>
