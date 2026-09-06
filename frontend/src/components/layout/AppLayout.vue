<template>
 <div class="app-shell">
 <div class="app-shell-glow" aria-hidden="true"></div>

 <AppSidebar />

 <div class="app-shell-main" :class="{ 'is-collapsed': !isDesktop || sidebarCollapsed, 'is-workspace': fillHeight }">
 <AppHeader />

 <main class="app-shell-content">
 <slot />
 </main>
 </div>
 </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import { useIsMobile } from '@/composables/useIsMobile'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

withDefaults(defineProps<{ fillHeight?: boolean }>(), { fillHeight: false })

const appStore = useAppStore()
const authStore = useAuthStore()
const { isDesktop } = useIsMobile()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour } = useOnboardingTour({
 storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
 autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
 onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>

<style scoped>
.app-shell {
 min-height: 100vh;
 background: var(--bg-workspace);
 color: var(--foreground);
}

.app-shell-glow {
 pointer-events: none;
 position: fixed;
 inset: 0;
 background:
 radial-gradient(circle at 0% 0%, color-mix(in oklch, var(--accent) 22%, transparent) 0%, transparent 30rem),
 radial-gradient(circle at 18% 100%, color-mix(in oklch, var(--accent) 10%, transparent) 0%, transparent 30rem),
 radial-gradient(circle at 100% 0%, color-mix(in oklch, var(--success) 14%, transparent) 0%, transparent 24rem);
}

.app-shell-main {
 position: relative;
 min-height: 100vh;
 transition: margin-left 0.2s ease;
}

@media (min-width: 768px) and (max-width: 1023px) {
 .app-shell-main {
 margin-left: 72px;
 }
}

@media (min-width: 1024px) {
 .app-shell-main {
 margin-left: 224px;
 }

 .app-shell-main.is-collapsed {
 margin-left: 72px;
 }
}

.app-shell-content {
 padding: 1rem;
}

.app-shell-main.is-workspace {
 height: 100vh;
 height: 100dvh;
 min-height: 0;
 display: flex;
 flex-direction: column;
}

.is-workspace > .app-shell-content {
 flex: 1;
 min-height: 0;
 display: flex;
 flex-direction: column;
}

.is-workspace > :deep(.app-header) {
 flex-shrink: 0;
}

@media (min-width: 768px) {
 .app-shell-content {
 padding: 8px 24px 24px 20px;
 }
}
</style>
