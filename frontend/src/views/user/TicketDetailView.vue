<template>
  <AppLayout>
    <div v-if="ticket" class="mx-auto max-w-3xl space-y-4">
      <div class="card space-y-3 p-6">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p class="font-mono text-xs text-gray-500">{{ ticket.ticket_no }}</p>
            <h1 class="text-lg font-semibold">{{ ticket.title }}</h1>
            <p class="text-sm text-gray-500">{{ t('tickets.category.' + ticket.category) }} · {{ t('tickets.status.' + ticket.status) }}</p>
          </div>
          <div class="flex gap-2">
            <button v-if="canWithdraw" class="btn btn-secondary" :disabled="actionLoading" @click="withdraw">{{ t('tickets.withdraw') }}</button>
            <button v-if="canClose" class="btn btn-danger" :disabled="actionLoading" @click="closeTicket">{{ t('tickets.close') }}</button>
          </div>
        </div>
        <input v-if="ticket.status === 'withdrawn'" v-model="editTitle" class="input w-full" />
        <TicketCategoryForm
          v-model:form="editForm"
          v-model:selected-group-ids="selectedGroupIds"
          :category="ticket.category"
          :rate-groups="rateGroups"
          :disabled="ticket.status !== 'withdrawn'"
        />
        <div v-if="ticket.status === 'withdrawn'" class="flex justify-end gap-2">
          <button class="btn btn-secondary" :disabled="actionLoading" @click="saveEdit">{{ t('common.save') }}</button>
          <button class="btn btn-primary" :disabled="actionLoading" @click="resubmit">{{ t('tickets.resubmit') }}</button>
        </div>
      </div>
      <div class="card space-y-3 p-6">
        <div v-for="msg in messages" :key="msg.id" class="rounded border border-gray-100 p-3 text-sm dark:border-dark-700">
          <p class="text-xs text-gray-500">{{ msg.sender_name_snapshot }} · {{ t('tickets.role.' + msg.sender_role) }} · {{ new Date(msg.created_at).toLocaleString() }}</p>
          <p class="mt-1 whitespace-pre-wrap">{{ msg.content }}</p>
          <div v-if="msg.attachments?.length" class="mt-2 space-y-1">
            <button
              v-for="file in msg.attachments"
              :key="file.media_id"
              class="block text-xs text-blue-600 hover:underline"
              @click="download(file.media_id)"
            >{{ file.file_name }} ({{ file.size_bytes }})</button>
          </div>
        </div>
        <div v-if="canReply" class="space-y-2">
          <textarea v-model="reply" rows="3" class="input w-full" :placeholder="t('tickets.replyPlaceholder')" />
          <input type="file" multiple @change="onFiles" />
          <button class="btn btn-primary" :disabled="actionLoading" @click="sendReply">{{ t('tickets.reply') }}</button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { ticketsAPI } from '@/api/tickets'
import { mediaAPI } from '@/api/media'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { emptyTicketForm, notifyTicketUnreadChanged, ticketFormFromPayload, ticketPayloadFromForm } from '@/utils/ticketForm'
import { useAppStore } from '@/stores'
import type { SupportTicket, SupportTicketMessage, TicketRateGroupOption } from '@/types/ticket'
import AppLayout from '@/components/layout/AppLayout.vue'
import TicketCategoryForm from '@/components/ticket/TicketCategoryForm.vue'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const ticket = ref<SupportTicket | null>(null)
const messages = ref<SupportTicketMessage[]>([])
const reply = ref('')
const files = ref<File[]>([])
const editTitle = ref('')
const editForm = ref(emptyTicketForm())
const selectedGroupIds = ref<number[]>([])
const rateGroups = ref<TicketRateGroupOption[]>([])
const actionLoading = ref(false)
const id = computed(() => Number(route.params.id))
const canWithdraw = computed(() => ['submitted', 'processing', 'waiting_admin'].includes(ticket.value?.status || ''))
const canClose = computed(() => ticket.value != null && !['closed', 'withdrawn'].includes(ticket.value.status))
const canReply = computed(() => ticket.value != null && !['closed', 'withdrawn', 'resolved'].includes(ticket.value.status))

async function load() {
  const [detail, msgs] = await Promise.all([ticketsAPI.get(id.value), ticketsAPI.messages(id.value)])
  ticket.value = detail.data
  messages.value = msgs.data || []
  editTitle.value = detail.data.title
  const parsed = ticketFormFromPayload(detail.data.current_form_payload)
  editForm.value = parsed.form
  selectedGroupIds.value = parsed.selectedGroupIds
  notifyTicketUnreadChanged()
}

function onFiles(event: Event) {
  const input = event.target as HTMLInputElement
  files.value = input.files ? Array.from(input.files) : []
}

async function withdraw() {
  actionLoading.value = true
  try {
    await ticketsAPI.withdraw(id.value)
    await load()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function closeTicket() {
  actionLoading.value = true
  try {
    await ticketsAPI.close(id.value)
    await load()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function saveEdit() {
  if (!ticket.value) return
  actionLoading.value = true
  try {
    await ticketsAPI.update(id.value, {
      title: editTitle.value,
      form_payload: ticketPayloadFromForm(ticket.value.category, editForm.value, selectedGroupIds.value),
      expected_revision_no: ticket.value.current_revision_no,
    })
    await load()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function resubmit() {
  if (!ticket.value) return
  actionLoading.value = true
  try {
    await ticketsAPI.resubmit(id.value, {
      title: editTitle.value,
      form_payload: ticketPayloadFromForm(ticket.value.category, editForm.value, selectedGroupIds.value),
      expected_revision_no: ticket.value.current_revision_no,
    })
    await load()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function sendReply() {
  actionLoading.value = true
  try {
    const mediaIds: number[] = []
    for (const file of files.value) {
      const uploaded = await mediaAPI.upload(file, { biz_type: 'ticket', biz_id: String(id.value) })
      mediaIds.push(uploaded.data.id)
    }
    await ticketsAPI.reply(id.value, { content: reply.value, media_ids: mediaIds })
    reply.value = ''
    files.value = []
    await load()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function download(mediaId: number) {
  try {
    const res = await ticketsAPI.downloadGrant(id.value, mediaId)
    window.open(res.data.url, '_blank')
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  }
}

onMounted(async () => {
  try {
    const [_, groups] = await Promise.all([
      load(),
      ticketsAPI.rateGroups().catch(() => ({ data: [] as TicketRateGroupOption[] })),
    ])
    rateGroups.value = groups.data || []
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  }
})
</script>
