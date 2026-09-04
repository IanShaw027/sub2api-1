<template>
  <Teleport to="body">
    <div
      v-if="subscription && position"
      class="dropdown subs-row-menu"
      :style="{ top: position.top + 'px', left: position.left + 'px' }"
    >
      <button
        v-if="subscription.status === 'active'"
        type="button"
        class="dropdown-item"
        @click="$emit('reset-quota')"
      >
        <Icon name="refresh" size="sm" />
        {{ t('admin.subscriptions.resetQuota') }}
      </button>
      <button
        v-if="subscription.status === 'active'"
        type="button"
        class="dropdown-item dropdown-item-danger"
        @click="$emit('revoke')"
      >
        <Icon name="ban" size="sm" />
        {{ t('admin.subscriptions.revoke') }}
      </button>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { UserSubscription } from '@/types'

defineProps<{
  subscription: UserSubscription | null
  position: { top: number; left: number } | null
}>()

defineEmits<{
  'reset-quota': []
  revoke: []
}>()

const { t } = useI18n()
</script>

<style scoped>
.subs-row-menu {
  position: fixed;
  z-index: 100000030;
  min-width: 180px;
}
</style>
