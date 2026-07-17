<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.inviteResetTitle')"
    width="wide"
    @close="handleClose"
  >
    <div v-if="account" class="space-y-5">
      <div class="flex flex-col gap-3 rounded-control border border-line bg-page p-4 dark:border-dark-600 dark:bg-dark-700 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex items-center gap-3">
          <div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-control bg-success text-white">
            <Icon name="gift" size="md" :stroke-width="2" />
          </div>
          <div class="min-w-0">
            <div class="truncate font-semibold text-ink dark:text-white">{{ account.name }}</div>
            <div class="mt-0.5 text-sm text-ink-soft dark:text-ink-soft">
              {{ t('admin.accounts.inviteResetSubtitle') }}
            </div>
          </div>
        </div>
        <button type="button" class="btn btn-secondary shrink-0" :disabled="loading" @click="loadStatus()">
          <Icon name="refresh" size="sm" :class="loading && 'animate-spin'" />
          {{ t('admin.accounts.inviteResetRefresh') }}
        </button>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)]">
          <section class="space-y-4 rounded-control border border-line p-4 dark:border-dark-600">
            <div class="flex items-center justify-between gap-3">
              <div>
                <div class="text-sm font-medium text-ink-soft dark:text-ink-soft">
                  {{ t('admin.accounts.inviteResetAvailable') }}
                </div>
                <div class="mt-2 flex items-end gap-2">
                  <span class="text-4xl font-bold text-ink dark:text-white">{{ availableCount }}</span>
                  <span class="pb-1 text-sm text-ink-soft dark:text-ink-soft">{{ t('admin.accounts.inviteResetAvailableUnit') }}</span>
                </div>
              </div>
              <div class="flex h-12 w-12 items-center justify-center rounded-control bg-brand-100 text-brand-700 dark:bg-brand/15 dark:text-brand-300">
                <Icon name="refresh" size="md" :stroke-width="2" />
              </div>
            </div>

            <div v-if="availableCredits.length > 0" class="space-y-2">
              <div
                v-for="(credit, index) in availableCredits"
                :key="credit.id"
                class="rounded-control bg-page p-3 text-sm text-ink-body dark:bg-dark-800 dark:text-ink-body"
              >
                <div class="font-medium text-ink dark:text-white">{{ creditTitle(credit) }} #{{ index + 1 }}</div>
                <div class="mt-1">{{ creditDescription(credit) }}</div>
              </div>
            </div>

            <div v-else class="rounded-control border border-dashed border-line p-4 text-sm text-ink-soft dark:border-dark-600 dark:text-ink-soft">
              {{ t('admin.accounts.inviteResetNoCredits') }}
            </div>

            <button
              type="button"
              class="flex w-full items-center justify-between rounded-control border border-line px-3 py-2 text-left text-sm font-medium text-ink-body hover:bg-page dark:border-dark-600 dark:text-ink dark:hover:bg-dark-700"
              @click="showRules = !showRules"
            >
              <span>{{ t('admin.accounts.inviteResetRules') }}</span>
              <Icon name="chevronDown" size="sm" :class="['transition-transform', showRules && 'rotate-180']" />
            </button>
            <div v-if="showRules" class="rounded-control bg-page p-3 text-sm text-ink-body dark:bg-dark-800 dark:text-ink-body">
              <ul v-if="rules.length > 0" class="list-disc space-y-1 pl-5">
                <li v-for="rule in rules" :key="rule">{{ rule }}</li>
              </ul>
              <p v-else>{{ t('admin.accounts.inviteResetRulesEmpty') }}</p>
            </div>
          </section>

          <section class="space-y-4 rounded-control border border-line p-4 dark:border-dark-600">
            <div>
              <label class="input-label" for="codex-invite-reset-emails">
                {{ t('admin.accounts.inviteResetInviteEmails') }}
              </label>
              <textarea
                id="codex-invite-reset-emails"
                v-model="emailInput"
                rows="8"
                class="input mt-2 min-h-[180px] resize-y font-mono text-sm leading-6"
                :placeholder="t('admin.accounts.inviteResetPlaceholder')"
              ></textarea>
              <p class="mt-2 text-xs text-ink-soft dark:text-ink-soft">
                {{ t('admin.accounts.inviteResetEmailHint', { max: maxEmails }) }}
              </p>
            </div>

            <label class="flex items-start gap-2 rounded-control border border-line p-3 text-sm text-ink-body dark:border-dark-600 dark:text-ink-body">
              <input
                v-model="consentConfirmed"
                type="checkbox"
                class="mt-0.5 h-4 w-4 rounded border-line text-brand-600 focus:ring-accent/25"
              />
              <span>{{ t('admin.accounts.inviteResetConsent') }}</span>
            </label>

            <div v-if="message" :class="['rounded-control p-3 text-sm', messageClass]">
              {{ message }}
            </div>

            <button
              type="button"
              class="btn btn-primary w-full justify-center"
              :disabled="sendingInvite"
              @click="handleSendInvite"
            >
              <Icon name="mail" size="sm" :class="sendingInvite && 'animate-pulse'" />
              {{ sendingInvite ? t('admin.accounts.inviteResetSending') : t('admin.accounts.inviteResetSendInvite') }}
            </button>
          </section>
        </div>

        <section class="rounded-control border border-line p-4 dark:border-dark-600">
          <button
            type="button"
            class="flex w-full items-center justify-between text-left text-sm font-medium text-ink-body dark:text-ink-body"
            @click="toggleHistory"
          >
            <span>{{ t('admin.accounts.inviteResetHistoryTitle') }}</span>
            <Icon name="chevronDown" size="sm" :class="['transition-transform', showHistory && 'rotate-180']" />
          </button>

          <div v-if="showHistory" class="mt-3">
            <div v-if="historyLoading" class="flex items-center justify-center py-6">
              <LoadingSpinner />
            </div>
            <div v-else-if="history.length === 0" class="rounded-control border border-dashed border-line p-4 text-sm text-ink-soft dark:border-dark-600 dark:text-ink-soft">
              {{ t('admin.accounts.inviteResetHistoryEmpty') }}
            </div>
            <ul v-else class="space-y-2">
              <li
                v-for="entry in history"
                :key="entry.id"
                class="flex items-start gap-3 rounded-control bg-page p-3 text-sm dark:bg-dark-800"
              >
                <span
                  :class="[
                    'mt-0.5 shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium',
                    entry.action_type === 'invite'
                      ? 'bg-accent-100 text-accent-700 dark:bg-accent-900/40 dark:text-accent-300'
                      : 'bg-warning-soft text-warning dark:bg-warning/15 dark:text-warning'
                  ]"
                >
                  {{ entry.action_type === 'invite' ? t('admin.accounts.inviteResetHistoryActionInvite') : t('admin.accounts.inviteResetHistoryActionConsume') }}
                </span>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span
                      :class="[
                        'shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium',
                        entry.success
                          ? 'bg-success-soft text-success dark:bg-success/15 dark:text-success'
                          : 'bg-danger-soft text-danger dark:bg-danger/15 dark:text-danger'
                      ]"
                    >
                      {{ entry.success ? t('admin.accounts.inviteResetHistorySuccess') : t('admin.accounts.inviteResetHistoryFailed') }}
                    </span>
                    <span class="text-xs text-ink-faint">{{ formatHistoryTime(entry.created_at) }}</span>
                  </div>
                  <div v-if="entry.action_type === 'invite' && entry.emails?.length" class="mt-1 break-all text-ink-body dark:text-ink-body">
                    {{ entry.emails.join(', ') }}
                  </div>
                  <div v-if="entry.action_type === 'invite' && entry.failed_emails?.length" class="mt-0.5 break-all text-xs text-danger dark:text-danger">
                    {{ t('admin.accounts.inviteResetHistoryFailedEmails', { emails: entry.failed_emails.join(', ') }) }}
                  </div>
                  <div v-if="entry.action_type === 'consume' && entry.credit_id" class="mt-1 break-all font-mono text-xs text-ink-soft dark:text-ink-soft">
                    {{ entry.credit_id }}
                  </div>
                  <div v-if="entry.message" class="mt-0.5 break-all text-xs text-ink-soft dark:text-ink-soft">
                    {{ entry.message }}
                  </div>
                </div>
              </li>
            </ul>
          </div>
        </section>
      </template>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-secondary" @click="handleClose">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'
import type { CodexInviteResetCredit, CodexInviteResetHistoryEntry, CodexInviteResetStatus } from '@/api/admin/accounts'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { Icon } from '@/components/icons'

const maxEmails = 5
const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

const props = withDefaults(
  defineProps<{
    show: boolean
    account: Account | null
    initialStatus?: CodexInviteResetStatus | null
  }>(),
  {
    initialStatus: null
  }
)

const emit = defineEmits<{
  close: []
  updated: []
}>()

const { t } = useI18n()

const loading = ref(false)
const sendingInvite = ref(false)
const status = ref<CodexInviteResetStatus | null>(null)
const emailInput = ref('')
const consentConfirmed = ref(false)
const message = ref('')
const messageType = ref<'success' | 'error' | ''>('')
const showRules = ref(false)
const showHistory = ref(false)
const history = ref<CodexInviteResetHistoryEntry[]>([])
const historyLoading = ref(false)

const availableCredits = computed(() => {
  return (status.value?.credits ?? []).filter((credit) => {
    const state = credit.status?.toLowerCase()
    return !state || state === 'available'
  })
})

const availableCount = computed(() => status.value?.available_count ?? availableCredits.value.length)

const rules = computed(() => status.value?.eligibility_rules ?? [])

const messageClass = computed(() => {
  if (messageType.value === 'success') {
    return 'bg-success-soft text-success dark:bg-success/10 dark:text-success'
  }
  if (messageType.value === 'error') {
    return 'bg-danger-soft text-danger dark:bg-danger/10 dark:text-danger'
  }
  return 'bg-page text-ink-body dark:bg-dark-700 dark:text-ink-body'
})

const creditTitle = (credit: CodexInviteResetCredit) => {
  return credit.title || t('admin.accounts.inviteResetCreditFallbackTitle')
}

const creditDescription = (credit: CodexInviteResetCredit) => {
  return credit.description || t('admin.accounts.inviteResetCreditFallbackDescription')
}

// 邀请接口支持逗号、分号、空白和换行分隔，这里先在前端做同样的校验。
const parseEmails = () => {
  const emails = emailInput.value
    .split(/[,\s;]+/)
    .map((item) => item.trim())
    .filter(Boolean)
  const unique = [...new Map(emails.map((email) => [email.toLowerCase(), email])).values()]
  if (unique.length === 0) {
    throw new Error(t('admin.accounts.inviteResetEmailsRequired'))
  }
  if (unique.length > maxEmails) {
    throw new Error(t('admin.accounts.inviteResetEmailLimit', { max: maxEmails }))
  }
  const invalid = unique.find((email) => !emailPattern.test(email))
  if (invalid) {
    throw new Error(t('admin.accounts.inviteResetInvalidEmail', { email: invalid }))
  }
  return unique
}

const setMessage = (type: 'success' | 'error', text: string) => {
  messageType.value = type
  message.value = text
}

const loadStatus = async (clearMessage = true) => {
  if (!props.account) return
  loading.value = true
  if (clearMessage) {
    message.value = ''
    messageType.value = ''
  }
  try {
    status.value = await adminAPI.accounts.getCodexInviteResetStatus(props.account.id)
  } catch (error: any) {
    status.value = null
    setMessage('error', error?.message || t('admin.accounts.inviteResetLoadFailed'))
    useAppStore().showError(error?.message || t('admin.accounts.inviteResetLoadFailed'))
  } finally {
    loading.value = false
  }
}

const loadHistory = async () => {
  if (!props.account) return
  historyLoading.value = true
  try {
    const res = await adminAPI.accounts.getCodexInviteResetHistory(props.account.id, { page: 1, page_size: 20 })
    history.value = res.items
  } catch {
    // 历史加载失败不打断主流程，仅留空列表。
    history.value = []
  } finally {
    historyLoading.value = false
  }
}

const toggleHistory = () => {
  showHistory.value = !showHistory.value
  if (showHistory.value && history.value.length === 0) {
    loadHistory()
  }
}

const handleSendInvite = async () => {
  if (!props.account || sendingInvite.value) return
  try {
    const emails = parseEmails()
    if ((status.value?.requires_consent ?? true) && !consentConfirmed.value) {
      throw new Error(t('admin.accounts.inviteResetConsentRequired'))
    }
    sendingInvite.value = true
    const result = await adminAPI.accounts.sendCodexInviteResetInvite(props.account.id, emails)
    const failed = result.failed_emails?.filter(Boolean) ?? []
    if (failed.length > 0) {
      setMessage('error', t('admin.accounts.inviteResetInvitePartialFailed', { emails: failed.join(', ') }))
      return
    }
    emailInput.value = ''
    setMessage('success', result.message || t('admin.accounts.inviteResetInviteSuccess'))
    useAppStore().showSuccess(t('admin.accounts.inviteResetInviteSuccess'))
    emit('updated')
  } catch (error: any) {
    setMessage('error', error?.message || t('admin.accounts.inviteResetInviteFailed'))
  } finally {
    sendingInvite.value = false
    // 邀请已落库（无论成功/部分失败），历史区展开时刷新。
    if (showHistory.value) {
      loadHistory()
    }
  }
}

const formatHistoryTime = (iso: string) => {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

const resetLocalState = () => {
  status.value = null
  emailInput.value = ''
  consentConfirmed.value = false
  message.value = ''
  messageType.value = ''
  showRules.value = false
  showHistory.value = false
  history.value = []
}

const handleClose = () => {
  emit('close')
}

watch(
  () => [props.show, props.account?.id],
  ([visible]) => {
    if (visible && props.account) {
      // 内联区已经查询过则直接复用，避免重复打 ChatGPT 后台 API。
      if (props.initialStatus) {
        status.value = props.initialStatus
      } else {
        loadStatus()
      }
      return
    }
    resetLocalState()
  },
  { immediate: true }
)
</script>
