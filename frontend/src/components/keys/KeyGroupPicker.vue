<template>
  <Teleport to="body">
    <div
      v-if="position"
      ref="rootRef"
      class="dropdown keys-group-dropdown"
      :style="{
        top: position.top !== undefined ? position.top + 'px' : undefined,
        bottom: position.bottom !== undefined ? position.bottom + 'px' : undefined,
        left: position.left + 'px'
      }"
    >
      <div class="keys-group-search">
        <Icon name="search" size="sm" class="text-muted" />
        <input
          v-model="searchQuery"
          type="text"
          class="keys-group-search-input"
          :placeholder="t('keys.searchGroup')"
          @click.stop
        />
      </div>
      <div class="keys-group-options">
        <button
          v-for="option in filteredOptions"
          :key="option.value ?? 'null'"
          type="button"
          class="dropdown-item keys-group-option"
          :class="{ 'is-active': isSelected(option) }"
          :title="option.description || undefined"
          @click="$emit('select', option.value)"
        >
          <GroupOptionItem
            :name="option.label"
            :platform="option.platform"
            :subscription-type="option.subscriptionType"
            :rate-multiplier="option.rate"
            :user-rate-multiplier="option.userRate"
            :peak-rate-enabled="option.peakRateEnabled"
            :peak-start="option.peakStart"
            :peak-end="option.peakEnd"
            :peak-rate-multiplier="option.peakRateMultiplier"
            :description="option.description"
            :selected="isSelected(option)"
          />
        </button>
        <p v-if="filteredOptions.length === 0" class="keys-group-empty">
          {{ t('keys.noGroupFound') }}
        </p>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import type { SubscriptionType, GroupPlatform } from '@/types'

export interface KeyGroupOption {
  value: number | null
  label: string
  description: string | null
  rate: number
  userRate: number | null
  peakRateEnabled: boolean
  peakStart: string
  peakEnd: string
  peakRateMultiplier: number
  subscriptionType: SubscriptionType
  platform: GroupPlatform
}

const props = defineProps<{
  options: KeyGroupOption[]
  selectedValue: number | null
  position: { top?: number; bottom?: number; left: number } | null
}>()

defineEmits<{
  select: [value: number | null]
}>()

const { t } = useI18n()
const rootRef = ref<HTMLElement | null>(null)
const searchQuery = ref('')

watch(
  () => props.position,
  (pos) => {
    if (pos) searchQuery.value = ''
  }
)

const filteredOptions = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return props.options
  return props.options.filter(
    (opt) =>
      opt.label.toLowerCase().includes(query) ||
      (opt.description && opt.description.toLowerCase().includes(query))
  )
})

const isSelected = (option: KeyGroupOption) =>
  props.selectedValue === option.value || (!props.selectedValue && option.value === null)

defineExpose({ rootRef })
</script>

<style scoped>
.keys-group-dropdown {
  position: fixed;
  z-index: 100000020;
  width: max-content;
  min-width: 320px;
  max-width: calc(100vw - 16px);
  padding: 0;
  overflow: hidden;
}

.keys-group-search {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--border);
}

.keys-group-search-input {
  flex: 1;
  min-width: 0;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--foreground);
  font-size: 13px;
}

.keys-group-options {
  max-height: 320px;
  overflow-y: auto;
  padding: 6px;
}

.keys-group-option {
  height: auto;
  padding: 7px 10px;
}

.keys-group-empty {
  padding: 16px 10px;
  text-align: center;
  font-size: 12.5px;
  color: var(--muted);
}
</style>
