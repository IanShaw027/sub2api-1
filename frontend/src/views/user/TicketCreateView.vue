<template>
  <AppLayout>
    <PageHeader :title="t('tickets.create')" :description="t('tickets.createDescription')">
      <template #actions>
        <Button variant="secondary" to="/tickets">
          <Icon name="arrowLeft" size="sm" :stroke-width="1.8" />
          {{ t('common.back') }}
        </Button>
      </template>
    </PageHeader>

    <SettingsPageLayout>
      <template #content>
        <SettingsSection :title="t('tickets.createBasicSection')">
          <SettingRow :label="t('tickets.fields.category')">
            <SegmentedControl v-model="category" :options="categorySegmentOptions" />
          </SettingRow>
          <SettingRow :label="t('tickets.fields.title')">
            <TextInput v-model="title" maxlength="80" :placeholder="t('tickets.fields.title')" />
          </SettingRow>
        </SettingsSection>

        <SettingsSection :title="t('tickets.createDetailsSection')">
          <div class="category-form-body">
            <TicketCategoryForm
              :category="category"
              :model-value="payload"
              :user-concurrency="authStore.user?.concurrency ?? null"
              :rate-groups="rateGroups"
              @update:model-value="payload = $event"
            />
          </div>
        </SettingsSection>

        <div class="form-actions">
          <Button variant="secondary" to="/tickets">{{ t('common.cancel') }}</Button>
          <Button :loading="submitting" @click="submit">{{ t('tickets.submit') }}</Button>
        </div>
      </template>
    </SettingsPageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import SettingsPageLayout from '@/components/layout/SettingsPageLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import SettingsSection from '@/components/ui/SettingsSection.vue'
import SettingRow from '@/components/ui/SettingRow.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import TextInput from '@/components/ui/TextInput.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { ticketsAPI } from '@/api/tickets'
import TicketCategoryForm from '@/components/tickets/TicketCategoryForm.vue'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { sanitizeTicketPayload, ticketCategoryOptions, validateTicketPayload } from '@/utils/tickets'
import type { TicketCategory, TicketRateGroupOption } from '@/types/ticket'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const category = ref<TicketCategory>('consult')
const title = ref('')
const payload = ref<Record<string, unknown>>({})
const submitting = ref(false)
const rateGroups = ref<TicketRateGroupOption[]>([])

const categorySegmentOptions = computed(() =>
  ticketCategoryOptions.map((option) => ({
    value: option.value,
    label: t(option.labelKey),
  })),
)

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

async function submit() {
  const form = {
    category: category.value,
    title: title.value.trim(),
    form_payload: sanitizeTicketPayload(category.value, payload.value),
  }
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

<style scoped>
.category-form-body {
  padding: 20px;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>
