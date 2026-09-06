<template>
 <div class="relative" ref="dropdownRef" @keydown.esc="handleEscape">
 <button
 ref="triggerRef"
 type="button"
 @click="toggleDropdown"
 :disabled="switching"
 class="btn btn-ghost btn-sm"
 :class="{ 'locale-menu-trigger': menuMode }"
 :title="currentLocale?.name"
 :aria-label="currentLocale?.name"
 :aria-expanded="isOpen"
 :aria-controls="listId"
 >
 <span class="text-base">{{ currentLocale?.flag }}</span>
 <span :class="menuMode ? '' : 'hidden sm:inline'">{{ menuMode ? currentLocale?.name : currentLocale?.code.toUpperCase() }}</span>
 <Icon
 name="chevronDown"
 size="xs"
 class="text-muted transition-transform duration-200"
 :class="{ 'rotate-180': isOpen }"
 />
 </button>

 <transition name="dropdown">
 <div
 v-if="isOpen"
 :id="listId"
 class="dropdown right-0 mt-1 w-36"
 :class="{ 'locale-menu-options': menuMode }"
 >
 <button
 v-for="locale in availableLocales"
 type="button"
 :key="locale.code"
 :disabled="switching"
 @click="selectLocale(locale.code)"
 class="dropdown-item"
 :class="{ 'is-active': locale.code === currentLocaleCode }"
 >
 <span class="text-base">{{ locale.flag }}</span>
 <span>{{ locale.name }}</span>
 <Icon v-if="locale.code === currentLocaleCode" name="check" size="sm" class="ml-auto text-accent" />
 </button>
 </div>
 </transition>
 </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { setLocale, availableLocales } from '@/i18n'

const { locale } = useI18n()
defineProps<{ menuMode?: boolean }>()
const listId = `locale-options-${useId()}`

const isOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const switching = ref(false)
const triggerRef = ref<HTMLButtonElement | null>(null)

const currentLocaleCode = computed(() => locale.value)
const currentLocale = computed(() => availableLocales.find((l) => l.code === locale.value))

async function toggleDropdown() {
 isOpen.value = !isOpen.value
 if (isOpen.value) {
 await nextTick()
 dropdownRef.value?.querySelector<HTMLElement>('.dropdown-item')?.focus()
 }
}

function closeDropdown(restoreFocus = false) {
 isOpen.value = false
 if (restoreFocus) triggerRef.value?.focus()
}

function handleEscape(event: KeyboardEvent) {
 if (!isOpen.value) return
 event.stopPropagation()
 closeDropdown(true)
}

async function selectLocale(code: string) {
 if (switching.value || code === currentLocaleCode.value) {
 closeDropdown(true)
 return
 }
 switching.value = true
 try {
 await setLocale(code)
 closeDropdown(true)
 } finally {
 switching.value = false
 }
}

function handleClickOutside(event: MouseEvent) {
 if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
 isOpen.value = false
 }
}

onMounted(() => {
 document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
 document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.locale-menu-trigger {
 width: 100%;
 justify-content: flex-start;
 padding-inline: 10px;
 gap: 10px;
}

.locale-menu-options {
 position: static;
 width: 100%;
 box-shadow: none;
}

.dropdown-enter-active,
.dropdown-leave-active {
 transition: all 0.15s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
 opacity: 0;
 transform: scale(0.95) translateY(-4px);
}
</style>
