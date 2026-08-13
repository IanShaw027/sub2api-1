<template>
  <AppLayout>
    <div v-if="ticket" class="mx-auto max-w-3xl space-y-4">
      <div class="card space-y-3 p-6">
        <p class="font-mono text-xs text-gray-500">{{ ticket.ticket_no }} · {{ ticket.user_email }}</p>
        <h1 class="text-lg font-semibold">{{ ticket.title }}</h1>
        <p class="text-sm text-gray-500">{{ t('tickets.category.' + ticket.category) }} · {{ t('tickets.status.' + ticket.status) }}</p>
        <pre class="overflow-auto rounded bg-gray-50 p-3 text-xs dark:bg-dark-800">{{ JSON.stringify(ticket.current_form_payload, null, 2) }}</pre>
        <div class="flex flex-wrap gap-2">
          <Select v-model="nextStatus" :options="statusOptions" class="w-48" />
          <button class="btn btn-secondary" :disabled="actionLoading" @click="changeStatus">{{ t('tickets.updateStatus') }}</button>
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
            >{{ file.file_name }}</button>
          </div>
        </div>
        <div v-if="canReply" class="space-y-2">
          <Select v-if="templates.length" v-model="templateId" :options="templateOptions" class="w-full" @change="applyTemplate" />
          <textarea v-model="reply" rows="3" class="input w-full" />
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
import { adminTicketsAPI } from '@/api/admin/tickets'
import { adminMediaAPI } from '@/api/media'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { notifyTicketUnreadChanged } from '@/utils/ticketForm'
import { useAppStore } from '@/stores'
import type { SupportTicket, SupportTicketMessage, TicketReplyTemplate } from '@/types/ticket'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const ticket = ref<SupportTicket | null>(null)
const messages = ref<SupportTicketMessage[]>([])
const templates = ref<TicketReplyTemplate[]>([])
const reply = ref('')
const files = ref<File[]>([])
const nextStatus = ref('processing')
const templateId = ref('')
const actionLoading = ref(false)
const id = computed(() => Number(route.params.id))
const canReply = computed(() => ticket.value != null && !['closed', 'withdrawn', 'resolved'].includes(ticket.value.status))
const statusOptions = computed(() =>
  ['processing', 'waiting_user', 'waiting_admin', 'resolved', 'closed'].map((value) => ({
    value, label: t('tickets.status.' + value),
  }))
)
const templateOptions = computed(() => [
  { value: '', label: t('tickets.pickTemplate') },
  ...templates.value.map((item) => ({ value: String(item.id), label: item.title })),
])

async function load() {
  const [detail, msgs, tpls] = await Promise.all([
    adminTicketsAPI.get(id.value),
    adminTicketsAPI.messages(id.value),
    adminTicketsAPI.listTemplates(),
  ])
  ticket.value = detail.data
  nextStatus.value = detail.data.status
  messages.value = msgs.data || []
  templates.value = tpls.data || []
  notifyTicketUnreadChanged()
}

function applyTemplate() {
  const found = templates.value.find((item) => String(item.id) === templateId.value)
  if (found) reply.value = found.content
}

function onFiles(event: Event) {
  const input = event.target as HTMLInputElement
  files.value = input.files ? Array.from(input.files) : []
}

async function changeStatus() {
  actionLoading.value = true
  try {
    await adminTicketsAPI.updateStatus(id.value, nextStatus.value)
    await load()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function sendReply() {
  if (!ticket.value) return
  actionLoading.value = true
  try {
    const mediaIds: number[] = []
    for (const file of files.value) {
      const uploaded = await adminMediaAPI.upload(file, { owner_user_id: ticket.value.user_id, biz_type: 'ticket', biz_id: String(id.value) })
      mediaIds.push(uploaded.data.id)
    }
    await adminTicketsAPI.reply(id.value, { content: reply.value, media_ids: mediaIds })
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
    const res = await adminTicketsAPI.downloadGrant(id.value, mediaId)
    window.open(res.data.url, '_blank')
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  }
}

onMounted(async () => {
  try {
    await load()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  }
})
</script>
