<template>
  <div class="card mb-4 flex gap-1 p-1">
    <button :class="tabClass('/orders')" @click="go('/orders')">
      {{ t('nav.myOrders') }}
    </button>
    <button :class="tabClass('/orders/invoices')" @click="go('/orders/invoices')">
      {{ t('nav.myInvoices') }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()

const activePath = computed(() => route.path)

function isActive(path: string): boolean {
  if (path === '/orders') {
    return activePath.value === '/orders'
  }
  return activePath.value.startsWith(path)
}

function tabClass(path: string): string {
  const base = 'flex-1 rounded-lg px-4 py-2 text-sm font-medium transition'
  return isActive(path)
    ? `${base} tab-active bg-accent-600 text-white shadow`
    : `${base} text-ink-body hover:bg-page dark:text-ink-body dark:hover:bg-dark-800`
}

function go(path: string) {
  if (activePath.value !== path) router.push(path)
}
</script>
