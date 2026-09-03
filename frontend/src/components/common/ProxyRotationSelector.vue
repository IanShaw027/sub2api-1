<template>
 <div class="space-y-3">
 <div
 v-if="selectedProxies.length > 0"
 class="divide-y divide-line overflow-hidden rounded-[12px] border border-line"
 >
 <div
 v-for="(proxy, index) in selectedProxies"
 :key="proxy.id"
 class="flex min-h-12 items-center gap-2 px-3 py-2"
 >
 <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-[7px] bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-[11.5px] font-semibold text-accent">
 {{ index + 1 }}
 </span>
 <div class="min-w-0 flex-1">
 <div class="truncate text-[13px] font-medium text-foreground">{{ proxy.name }}</div>
 <div class="truncate text-[11.5px] text-muted">
 {{ proxy.protocol }}://{{ proxy.host }}:{{ proxy.port }}
 </div>
 </div>
 <button
 type="button"
 class="btn btn-ghost h-8 w-8 p-0"
 :disabled="index === 0"
 :title="t('admin.accounts.bulkEdit.moveProxyUp')"
 @click="move(index, -1)"
 >
 <Icon name="arrowUp" size="sm" />
 </button>
 <button
 type="button"
 class="btn btn-ghost h-8 w-8 p-0"
 :disabled="index === selectedProxies.length - 1"
 :title="t('admin.accounts.bulkEdit.moveProxyDown')"
 @click="move(index, 1)"
 >
 <Icon name="arrowDown" size="sm" />
 </button>
 <button
 type="button"
 class="btn btn-ghost h-8 w-8 p-0 text-danger-text"
 :title="t('common.remove')"
 @click="toggle(proxy.id, false)"
 >
 <Icon name="x" size="sm" />
 </button>
 </div>
 </div>

 <div class="overflow-hidden rounded-[12px] border border-line">
 <div class="flex h-9 items-center gap-2 border-b border-line bg-surface-2 px-3">
 <Icon name="search" size="sm" class="shrink-0 text-muted" />
 <input
 v-model="search"
 type="text"
 class="min-w-0 flex-1 bg-transparent text-[13px] text-foreground placeholder:text-muted focus:outline-none"
 :placeholder="t('admin.proxies.searchProxies')"
 />
 </div>
 <div class="grid max-h-48 grid-cols-1 gap-1 overflow-y-auto p-2 sm:grid-cols-2">
 <label
 v-for="proxy in filteredProxies"
 :key="proxy.id"
 class="flex cursor-pointer items-start gap-2 rounded-[9px] px-2 py-2 hover:bg-surface-2"
 >
 <input
 type="checkbox"
 class="mt-0.5 h-4 w-4 shrink-0 rounded-[5px] border-line accent-[var(--accent)]"
 :checked="modelValue.includes(proxy.id)"
 @change="toggle(proxy.id, ($event.target as HTMLInputElement).checked)"
 />
 <span class="min-w-0">
 <span class="block truncate text-[13px] font-medium text-foreground">{{ proxy.name }}</span>
 <span class="block truncate text-[11.5px] text-muted">{{ proxy.host }}:{{ proxy.port }}</span>
 </span>
 </label>
 <div v-if="filteredProxies.length === 0" class="py-3 text-center text-[12.5px] text-muted sm:col-span-2">
 {{ t('common.noOptionsFound') }}
 </div>
 </div>
 </div>
 <p class="text-[11.5px] text-muted">
 {{ t('admin.accounts.bulkEdit.proxyRotationHint', { count: modelValue.length }) }}
 </p>
 </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { Proxy } from '@/types'

const props = defineProps<{ modelValue: number[]; proxies: Proxy[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: number[]] }>()
const { t } = useI18n()
const search = ref('')

const selectedProxies = computed(() =>
 props.modelValue.flatMap(id => {
 const proxy = props.proxies.find(item => item.id === id)
 return proxy ? [proxy] : []
 })
)

const filteredProxies = computed(() => {
 const query = search.value.trim().toLowerCase()
 if (!query) return props.proxies
 return props.proxies.filter(proxy =>
 proxy.name.toLowerCase().includes(query) || proxy.host.toLowerCase().includes(query)
 )
})

const toggle = (proxyId: number, checked: boolean) => {
 emit('update:modelValue', checked
 ? [...props.modelValue, proxyId]
 : props.modelValue.filter(id => id !== proxyId))
}

const move = (index: number, offset: number) => {
 const target = index + offset
 if (target < 0 || target >= props.modelValue.length) return
 const next = [...props.modelValue]
 ;[next[index], next[target]] = [next[target], next[index]]
 emit('update:modelValue', next)
}
</script>
