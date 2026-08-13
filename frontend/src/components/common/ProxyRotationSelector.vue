<template>
  <div class="space-y-3">
    <div
      v-if="selectedProxies.length > 0"
      class="divide-y divide-gray-100 overflow-hidden rounded-md border border-gray-200 dark:divide-dark-600 dark:border-dark-600"
    >
      <div
        v-for="(proxy, index) in selectedProxies"
        :key="proxy.id"
        class="flex min-h-12 items-center gap-2 px-3 py-2"
      >
        <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded bg-primary-50 text-xs font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
          {{ index + 1 }}
        </span>
        <div class="min-w-0 flex-1">
          <div class="truncate text-sm font-medium text-gray-800 dark:text-gray-100">{{ proxy.name }}</div>
          <div class="truncate text-xs text-gray-500 dark:text-gray-400">
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
          class="btn btn-ghost h-8 w-8 p-0 text-red-600 dark:text-red-400"
          :title="t('common.remove')"
          @click="toggle(proxy.id, false)"
        >
          <Icon name="x" size="sm" />
        </button>
      </div>
    </div>

    <div class="overflow-hidden rounded-md border border-gray-200 dark:border-dark-600">
      <div class="flex items-center gap-2 border-b border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-600 dark:bg-dark-800">
        <Icon name="search" size="sm" class="shrink-0 text-gray-400" />
        <input
          v-model="search"
          type="text"
          class="min-w-0 flex-1 bg-transparent text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none dark:text-gray-100"
          :placeholder="t('admin.proxies.searchProxies')"
        />
      </div>
      <div class="grid max-h-48 grid-cols-1 gap-1 overflow-y-auto p-2 sm:grid-cols-2">
        <label
          v-for="proxy in filteredProxies"
          :key="proxy.id"
          class="flex cursor-pointer items-start gap-2 rounded px-2 py-2 hover:bg-gray-50 dark:hover:bg-dark-700"
        >
          <input
            type="checkbox"
            class="mt-0.5 h-4 w-4 shrink-0 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
            :checked="modelValue.includes(proxy.id)"
            @change="toggle(proxy.id, ($event.target as HTMLInputElement).checked)"
          />
          <span class="min-w-0">
            <span class="block truncate text-sm font-medium text-gray-800 dark:text-gray-100">{{ proxy.name }}</span>
            <span class="block truncate text-xs text-gray-500 dark:text-gray-400">{{ proxy.host }}:{{ proxy.port }}</span>
          </span>
        </label>
        <div v-if="filteredProxies.length === 0" class="py-3 text-center text-sm text-gray-500 sm:col-span-2">
          {{ t('common.noOptionsFound') }}
        </div>
      </div>
    </div>
    <p class="text-xs text-gray-500 dark:text-gray-400">
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
