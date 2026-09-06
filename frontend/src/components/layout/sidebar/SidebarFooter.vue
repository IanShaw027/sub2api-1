<template>
 <div class="mt-auto flex flex-none flex-col gap-1.5 border-t border-[var(--border)] pt-2">
 <div
 class="flex gap-1"
 :class="collapsed ? 'flex-col items-center justify-center' : 'justify-start'"
 >
 <button
 type="button"
 class="sidebar-item"
 :class="collapsed ? 'justify-center px-0' : 'flex-1'"
 :title="isDark ? t('nav.lightMode') : t('nav.darkMode')"
 :aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')"
 @click="toggleTheme"
 >
 <svg
 v-if="isDark"
 class="h-4 w-4 flex-none text-warning-text"
 viewBox="0 0 24 24"
 fill="none"
 stroke="currentColor"
 stroke-width="1.8"
 stroke-linecap="round"
 stroke-linejoin="round"
 >
 <path
 d="M12 3v2.25m6.364.386-1.591 1.591M21 12h-2.25m-.386 6.364-1.591-1.591M12 18.75V21m-4.773-4.227-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z"
 />
 </svg>
 <svg
 v-else
 class="h-4 w-4 flex-none"
 viewBox="0 0 24 24"
 fill="none"
 stroke="currentColor"
 stroke-width="1.8"
 stroke-linecap="round"
 stroke-linejoin="round"
 >
 <path d="M21.752 15.002A9.72 9.72 0 0 1 18 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 0 0 3 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 0 0 9.002-5.998Z" />
 </svg>
 <span v-if="!collapsed" class="truncate">{{ isDark ? t('nav.lightMode') : t('nav.darkMode') }}</span>
 </button>
 <button
 v-if="isDesktop"
 type="button"
 class="sidebar-item h-[30px] w-[30px] flex-none justify-center px-0"
 :title="collapsed ? t('nav.expand') : t('nav.collapse')"
 :aria-label="collapsed ? t('nav.expand') : t('nav.collapse')"
 @click="appStore.toggleSidebar()"
 >
 <svg
 class="h-[15px] w-[15px] transition-transform duration-200"
 :class="collapsed ? 'rotate-180' : ''"
 viewBox="0 0 24 24"
 fill="none"
 stroke="currentColor"
 stroke-width="1.8"
 stroke-linecap="round"
 stroke-linejoin="round"
 >
 <path d="m11 17-5-5 5-5M18 17l-5-5 5-5" />
 </svg>
 </button>
 </div>

 <div
 v-if="user"
 class="flex items-center gap-2 px-1 pt-1"
 :class="collapsed ? 'justify-center' : 'justify-start'"
 >
 <div
 class="avatar-accent flex h-7 w-7 flex-none items-center justify-center overflow-hidden rounded-full text-xs font-bold"
 >
 <img
 v-if="avatarUrl"
 :src="avatarUrl"
 :alt="displayName"
 class="h-full w-full object-cover"
 >
 <span v-else>{{ userInitial }}</span>
 </div>
 <div v-if="!collapsed" class="flex min-w-0 flex-1 flex-col leading-tight">
 <span class="truncate text-xs font-semibold">{{ user.email }}</span>
 <span
 class="text-[10.5px] font-semibold"
 :class="isAdmin ? 'text-info-text' : 'text-muted'"
 >{{ roleLabel }}</span>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/composables/useTheme'
import { useIsMobile } from '@/composables/useIsMobile'

defineProps<{
 collapsed: boolean
}>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { isDark, toggleTheme } = useTheme()
const { isDesktop } = useIsMobile()

const user = computed(() => authStore.user)
const isAdmin = computed(() => authStore.isAdmin)
const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')

const userInitial = computed(() => {
 if (!user.value) return ''
 if (user.value.username) return user.value.username.charAt(0).toUpperCase()
 if (user.value.email) return user.value.email.charAt(0).toUpperCase()
 return ''
})

const displayName = computed(() => {
 if (!user.value) return ''
 return user.value.username || user.value.email?.split('@')[0] || ''
})

const roleLabel = computed(() => {
 if (!user.value) return ''
 if (isAdmin.value) {
 return t('admin.users.roles.admin')
 }
 const role = t('admin.users.roles.user')
 if (authStore.isSimpleMode) return role
 const balance = Number(user.value.balance || 0)
 if (!Number.isFinite(balance)) return role
 return `${role} · $${balance.toFixed(2)}`
})
</script>
