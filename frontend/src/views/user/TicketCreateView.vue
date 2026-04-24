<template>
  <AppLayout>
    <div class="grid gap-6 xl:grid-cols-[1.35fr_0.95fr]">
      <TicketConversationPane
        :title="t('tickets.createConversationTitle')"
        :subtitle="t('tickets.createConversationSubtitle')"
        :messages="systemMessages"
        :empty-text="t('tickets.emptyConversation')"
        :show-composer="false"
      />

      <TicketEditorCard
        :category="category"
        :title="title"
        :payload="payload"
        :submit-label="t('tickets.submit')"
        :submitting="submitting"
        @submit="submit"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores'
import ticketsAPI from '@/api/tickets'
import TicketConversationPane from '@/components/tickets/TicketConversationPane.vue'
import TicketEditorCard from '@/components/tickets/TicketEditorCard.vue'
import { validateTicketPayload } from '@/utils/tickets'
import type { SupportTicketMessage, TicketCategory } from '@/types'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const category = ref<TicketCategory>('consult')
const title = ref('')
const payload = ref<Record<string, unknown>>({})
const submitting = ref(false)

const systemMessages = ref<SupportTicketMessage[]>([{
  id: 0,
  ticket_id: 0,
  sender_role: 'system',
  sender_name_snapshot: '系统',
  sender_avatar_snapshot: '',
  message_type: 'system',
  content: t('tickets.createSystemMessage'),
  created_at: new Date().toISOString(),
}])

async function submit(form: { category: TicketCategory; title: string; form_payload: Record<string, unknown> }) {
  const validationKey = validateTicketPayload(form.category, form.title, form.form_payload)
  if (validationKey) {
    appStore.showError(t(validationKey))
    return
  }
  try {
    submitting.value = true
    const created = await ticketsAPI.createTicket(form)
    appStore.showSuccess(t('tickets.messages.created'))
    router.replace(`/tickets/${created.id}`)
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    submitting.value = false
  }
}
</script>
