<template>
  <BaseDialog
    :show="open"
    :title="t('admin.subscriptions.assignSubscription')"
    width="normal"
    @close="close"
  >
    <form
      id="assign-subscription-form"
      @submit.prevent="handleAssignSubscription"
      class="space-y-5"
    >
      <div>
        <label class="input-label">{{ t('admin.subscriptions.form.user') }}</label>
        <div class="relative" data-assign-user-search>
          <input
            v-model="userSearchKeyword"
            type="text"
            class="input pr-8"
            :placeholder="t('admin.usage.searchUserPlaceholder')"
            @input="debounceSearchUsers"
            @focus="showUserDropdown = true"
          />
          <button
            v-if="selectedUser"
            @click="clearUserSelection"
            type="button"
            class="absolute right-2 top-1/2 -translate-y-1/2 text-muted hover:text-foreground"
          >
            <Icon name="x" size="sm" :stroke-width="2" />
          </button>
          <!-- User Dropdown -->
          <div
            v-if="showUserDropdown && (userSearchResults.length > 0 || userSearchKeyword)"
            class="dropdown subs-assign-user-dropdown"
          >
            <div
              v-if="userSearchLoading"
              class="px-4 py-3 text-sm text-muted"
            >
              {{ t('common.loading') }}
            </div>
            <div
              v-else-if="userSearchResults.length === 0 && userSearchKeyword"
              class="px-4 py-3 text-sm text-muted"
            >
              {{ t('common.noOptionsFound') }}
            </div>
            <button
              v-for="user in userSearchResults"
              :key="user.id"
              type="button"
              @click="selectUser(user)"
              class="dropdown-item"
            >
              <span class="font-medium text-foreground">{{ user.email }}</span>
              <span class="ml-2 text-muted">#{{ user.id }}</span>
            </button>
          </div>
        </div>
      </div>
      <div>
        <label class="input-label">{{ t('admin.subscriptions.form.group') }}</label>
        <Select
          v-model="assignForm.group_id"
          :options="subscriptionGroupOptions"
          :placeholder="t('admin.subscriptions.selectGroup')"
        >
          <template #selected="{ option }">
            <GroupBadge
              v-if="option"
              :name="(option as unknown as GroupOption).label"
              :platform="(option as unknown as GroupOption).platform"
              :subscription-type="(option as unknown as GroupOption).subscriptionType"
              :rate-multiplier="(option as unknown as GroupOption).rate"
            />
            <span v-else class="text-muted">{{ t('admin.subscriptions.selectGroup') }}</span>
          </template>
          <template #option="{ option, selected }">
            <GroupOptionItem
              :name="(option as unknown as GroupOption).label"
              :platform="(option as unknown as GroupOption).platform"
              :subscription-type="(option as unknown as GroupOption).subscriptionType"
              :rate-multiplier="(option as unknown as GroupOption).rate"
              :description="(option as unknown as GroupOption).description"
              :selected="selected"
            />
          </template>
        </Select>
        <p class="input-hint">{{ t('admin.subscriptions.groupHint') }}</p>
      </div>
      <div>
        <label class="input-label">{{ t('admin.subscriptions.form.validityDays') }}</label>
        <input v-model.number="assignForm.validity_days" type="number" min="1" class="input" />
        <p class="input-hint">{{ t('admin.subscriptions.validityHint') }}</p>
      </div>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="close" type="button" class="btn-glass-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="assign-subscription-form"
          :disabled="submitting"
          class="btn-glass-primary"
        >
          <svg
            v-if="submitting"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            ></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          {{ submitting ? t('admin.subscriptions.assigning') : t('admin.subscriptions.assign') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Group, GroupPlatform, SubscriptionType } from '@/types'
import type { SimpleUser } from '@/api/admin/usage'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import Icon from '@/components/icons/Icon.vue'

interface GroupOption {
  value: number
  label: string
  description: string | null
  platform: GroupPlatform
  subscriptionType: SubscriptionType
  rate: number
}

const props = defineProps<{
  open: boolean
  groups: Group[]
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const submitting = ref(false)

const assignForm = reactive({
  user_id: null as number | null,
  group_id: null as number | null,
  validity_days: 30
})

// Group options for assign (only subscription type groups)
const subscriptionGroupOptions = computed(() =>
  props.groups
    .filter((g) => g.subscription_type === 'subscription' && g.status === 'active')
    .map((g) => ({
      value: g.id,
      label: g.name,
      description: g.description,
      platform: g.platform,
      subscriptionType: g.subscription_type,
      rate: g.rate_multiplier
    }))
)

// User search state
const userSearchKeyword = ref('')
const userSearchResults = ref<SimpleUser[]>([])
const userSearchLoading = ref(false)
const showUserDropdown = ref(false)
const selectedUser = ref<SimpleUser | null>(null)
let userSearchTimeout: ReturnType<typeof setTimeout> | null = null

const debounceSearchUsers = () => {
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
  }
  userSearchTimeout = setTimeout(searchUsers, 300)
}

const searchUsers = async () => {
  const keyword = userSearchKeyword.value.trim()

  // Clear selection if user modified the search keyword
  if (selectedUser.value && keyword !== selectedUser.value.email) {
    selectedUser.value = null
    assignForm.user_id = null
  }

  if (!keyword) {
    userSearchResults.value = []
    return
  }

  userSearchLoading.value = true
  try {
    userSearchResults.value = await adminAPI.usage.searchUsers(keyword)
  } catch (error) {
    console.error('Failed to search users:', error)
    userSearchResults.value = []
  } finally {
    userSearchLoading.value = false
  }
}

const selectUser = (user: SimpleUser) => {
  selectedUser.value = user
  userSearchKeyword.value = user.email
  showUserDropdown.value = false
  assignForm.user_id = user.id
}

const clearUserSelection = () => {
  selectedUser.value = null
  userSearchKeyword.value = ''
  userSearchResults.value = []
  assignForm.user_id = null
}

const close = () => {
  emit('close')
  assignForm.user_id = null
  assignForm.group_id = null
  assignForm.validity_days = 30
  // Clear user search state
  selectedUser.value = null
  userSearchKeyword.value = ''
  userSearchResults.value = []
  showUserDropdown.value = false
}

const handleAssignSubscription = async () => {
  if (!assignForm.user_id) {
    appStore.showError(t('admin.subscriptions.pleaseSelectUser'))
    return
  }
  if (!assignForm.group_id) {
    appStore.showError(t('admin.subscriptions.pleaseSelectGroup'))
    return
  }
  if (!assignForm.validity_days || assignForm.validity_days < 1) {
    appStore.showError(t('admin.subscriptions.validityDaysRequired'))
    return
  }

  submitting.value = true
  try {
    await adminAPI.subscriptions.assign({
      user_id: assignForm.user_id,
      group_id: assignForm.group_id,
      validity_days: assignForm.validity_days
    })
    appStore.showSuccess(t('admin.subscriptions.subscriptionAssigned'))
    emit('saved')
    close()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToAssign'))
    console.error('Error assigning subscription:', error)
  } finally {
    submitting.value = false
  }
}

const handleGlobalClick = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (!target.closest('[data-assign-user-search]')) showUserDropdown.value = false
}

onMounted(() => {
  document.addEventListener('click', handleGlobalClick)
})

onUnmounted(() => {
  document.removeEventListener('click', handleGlobalClick)
})
</script>

<style scoped>
.subs-assign-user-dropdown {
  top: 100%;
  left: 0;
  margin-top: 4px;
  width: 100%;
  max-height: 240px;
  overflow-y: auto;
}
</style>
