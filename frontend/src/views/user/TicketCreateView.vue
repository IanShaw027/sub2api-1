<template>
  <AppLayout>
    <div class="mx-auto max-w-2xl space-y-4">
      <div class="card space-y-4 p-6">
        <h1 class="text-lg font-semibold">{{ t('tickets.create') }}</h1>
        <div>
          <label class="input-label">{{ t('tickets.categoryLabel') }}</label>
          <Select v-model="category" :options="categoryOptions" class="mt-1 w-full" />
        </div>
        <div>
          <label class="input-label">{{ t('tickets.title') }}</label>
          <input v-model="title" class="input mt-1 w-full" maxlength="80" />
        </div>
        <TicketCategoryForm v-model:form="form" v-model:selected-group-ids="selectedGroupIds" :category="category" :rate-groups="rateGroups" />
        <div class="flex justify-end gap-2">
          <button class="btn btn-secondary" @click="router.push('/tickets')">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="saving" @click="submit">{{ saving ? t('common.processing') : t('common.submit') }}</button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ticketsAPI } from '@/api/tickets'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { emptyTicketForm, ticketPayloadFromForm } from '@/utils/ticketForm'
import { useAppStore, useAuthStore } from '@/stores'
import type { TicketCategory, TicketRateGroupOption } from '@/types/ticket'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import TicketCategoryForm from '@/components/ticket/TicketCategoryForm.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const saving = ref(false)
const category = ref<TicketCategory>('consult')
const title = ref('')
const selectedGroupIds = ref<number[]>([])
const rateGroups = ref<TicketRateGroupOption[]>([])
const form = ref(emptyTicketForm(String(authStore.user?.concurrency ?? '')))
const categoryOptions = [
  { value: 'consult', label: t('tickets.category.consult') },
  { value: 'refund', label: t('tickets.category.refund') },
  { value: 'concurrency_apply', label: t('tickets.category.concurrency_apply') },
  { value: 'rate_apply', label: t('tickets.category.rate_apply') },
  { value: 'other', label: t('tickets.category.other') },
]

async function submit() {
  if (!title.value.trim()) return
  saving.value = true
  try {
    const res = await ticketsAPI.create({
      category: category.value,
      title: title.value.trim(),
      form_payload: ticketPayloadFromForm(category.value, form.value, selectedGroupIds.value),
    })
    router.push(`/tickets/${res.data.id}`)
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  try {
    const res = await ticketsAPI.rateGroups()
    rateGroups.value = res.data || []
  } catch {
    rateGroups.value = []
  }
})
</script>
