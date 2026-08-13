<template>
  <AppLayout>
    <div class="grid h-[calc(100vh-10rem)] min-h-[calc(100vh-10rem)] min-w-0 gap-6 overflow-hidden xl:grid-cols-[minmax(0,1.35fr)_minmax(360px,0.95fr)]">
      <div class="min-h-0">
        <TicketConversationPane
          :title="t('tickets.createConversationTitle')"
          :subtitle="t('tickets.createConversationSubtitle')"
          :messages="systemMessages"
          :empty-text="t('tickets.emptyConversation')"
          :show-composer="false"
        />
      </div>

      <div class="min-h-0">
        <TicketEditorCard
          :category="category"
          :title="title"
          :payload="payload"
          :submit-label="t('tickets.submit')"
          :submitting="submitting"
          :user-concurrency="authStore.user?.concurrency ?? null"
          :rate-groups="rateGroups"
          @submit="submit"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { ticketsAPI } from '@/api/tickets'
import TicketConversationPane from '@/components/tickets/TicketConversationPane.vue'
import TicketEditorCard from '@/components/tickets/TicketEditorCard.vue'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { validateTicketPayload } from '@/utils/tickets'
import type { SupportTicketMessage, TicketCategory, TicketRateGroupOption } from '@/types/ticket'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const category = ref<TicketCategory>('consult')
const title = ref('')
const payload = ref<Record<string, unknown>>({})
const submitting = ref(false)
const rateGroups = ref<TicketRateGroupOption[]>([])

const systemMessages = ref<SupportTicketMessage[]>([{
  id: 0,
  ticket_id: 0,
  sender_role: 'system',
  sender_name_snapshot: t('tickets.role.system'),
  message_type: 'system',
  content: t('tickets.createSystemMessage'),
  created_at: new Date().toISOString(),
}])

function ticketError(err: unknown) {
  return extractI18nErrorMessage(err, t, 'tickets.errors', t('common.unknownError'))
}

async function loadTicketContext() {
  try {
    const res = await ticketsAPI.rateGroups()
    rateGroups.value = res.data || []
  } catch {
    rateGroups.value = []
  }
}

async function submit(form: { category: TicketCategory; title: string; form_payload: Record<string, unknown> }) {
  const validationKey = validateTicketPayload(form.category, form.title, form.form_payload)
  if (validationKey) {
    appStore.showError(t(validationKey))
    return
  }
  try {
    submitting.value = true
    const created = await ticketsAPI.create(form)
    appStore.showSuccess(t('tickets.messages.created'))
    router.replace(`/tickets/${created.data.id}`)
  } catch (err: unknown) {
    appStore.showError(ticketError(err))
  } finally {
    submitting.value = false
  }
}

onMounted(loadTicketContext)
</script>
