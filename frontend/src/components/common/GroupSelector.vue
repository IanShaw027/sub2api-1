<template>
 <div>
 <label class="input-label">
 {{ t('admin.users.groups') }}
 <span class="font-normal text-muted">{{ t('common.selectedCount', { count: modelValue.length }) }}</span>
 </label>
 <div
 v-if="isSearchable"
 class="flex h-9 items-center gap-2 rounded-t-[12px] border border-b-0 border-line bg-surface-2 px-3"
 >
 <Icon name="search" size="sm" class="shrink-0 text-muted" />
 <input
 v-model="searchText"
 type="text"
 :placeholder="t('common.searchPlaceholder')"
 class="flex-1 bg-transparent text-[13px] text-foreground placeholder:text-muted focus:outline-none"
 />
 </div>
 <div
 :class="[
 'grid max-h-32 grid-cols-2 gap-1 overflow-y-auto p-2',
 isSearchable
 ? 'rounded-b-[12px] border border-t-0 border-line bg-surface-2'
 : 'rounded-[12px] border border-line bg-surface-2'
 ]"
 >
 <label
 v-for="group in filteredGroups"
 :key="group.id"
 class="flex cursor-pointer items-center gap-2 rounded-[9px] px-2 py-1.5 transition-colors hover:bg-surface"
 :title="t('admin.groups.rateAndAccounts', { rate: group.rate_multiplier, count: group.account_count || 0 })"
 >
 <input
 type="checkbox"
 :value="group.id"
 :checked="modelValue.includes(group.id)"
 @change="handleChange(group.id, ($event.target as HTMLInputElement).checked)"
 class="h-4 w-4 shrink-0 rounded-[5px] border-line accent-[var(--accent)]"
 />
 <GroupBadge
 :name="group.name"
 :platform="group.platform"
 :subscription-type="group.subscription_type"
 :rate-multiplier="group.rate_multiplier"
 class="min-w-0 flex-1"
 />
 <span class="shrink-0 text-[11.5px] text-muted">{{ group.account_count || 0 }}</span>
 </label>
 <div
 v-if="filteredGroups.length === 0"
 class="col-span-2 py-2 text-center text-[12.5px] text-muted"
 >
 {{ t('common.noGroupsAvailable') }}
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import GroupBadge from './GroupBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import type { AdminGroup, GroupPlatform } from '@/types'

const { t } = useI18n()

interface Props {
 modelValue: number[]
 groups: AdminGroup[]
 platform?: GroupPlatform // Optional platform filter
 mixedScheduling?: boolean // For antigravity accounts: allow anthropic/gemini groups
 searchable?: boolean | 'auto'
}

const props = withDefaults(defineProps<Props>(), {
 searchable: 'auto'
})
const emit = defineEmits<{
 'update:modelValue': [value: number[]]
}>()

const searchText = ref('')

const isSearchable = computed(() => {
 if (props.searchable === 'auto') return props.groups.length > 5
 return props.searchable
})

// Filter groups by platform if specified
const filteredGroups = computed(() => {
 let result: AdminGroup[] = props.groups
 if (props.platform) {
 // antigravity 账户启用混合调度后，可选择 anthropic/gemini 分组
 if (props.platform === 'antigravity' && props.mixedScheduling) {
 result = result.filter(
 (g) => g.platform === 'antigravity' || g.platform === 'anthropic' || g.platform === 'gemini' || g.platform === 'composite'
 )
 } else {
 // 默认：只能选择同 platform 的分组；composite 分组可接收任意具体平台账号
 result = result.filter((g) => g.platform === props.platform || g.platform === 'composite')
 }
 }
 if (isSearchable.value && searchText.value) {
 const q = searchText.value.toLowerCase()
 result = result.filter(
 (g) => g.name.toLowerCase().includes(q) || g.description?.toLowerCase().includes(q)
 )
 }
 return result
})

const handleChange = (groupId: number, checked: boolean) => {
 const newValue = checked
 ? [...props.modelValue, groupId]
 : props.modelValue.filter((id) => id !== groupId)
 emit('update:modelValue', newValue)
}
</script>
