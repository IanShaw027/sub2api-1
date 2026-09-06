<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import ComposerBar from './components/ComposerBar.vue'
import MessageStream from './components/MessageStream.vue'
import PreviewDialog from './components/PreviewDialog.vue'
import SessionList from './components/SessionList.vue'
import TaskGrid from './components/TaskGrid.vue'
import TokenStats from './components/TokenStats.vue'
import { useCreationStore } from './stores/creation'

const { t } = useI18n()
const store = useCreationStore()

const previewOpen = ref(false)
const previewUrl = ref('')
const keyboardOffset = ref('0px')

type MobilePanel = 'sessions' | 'main' | 'controls'
const mobilePanel = ref<MobilePanel>('main')

const mobilePanelOptions = computed(() => [
  { value: 'sessions' as MobilePanel, label: t('studio.mobileTabs.sessions') },
  { value: 'main' as MobilePanel, label: t('studio.mobileTabs.main') },
  { value: 'controls' as MobilePanel, label: t('studio.mobileTabs.controls') },
])

const chatTokenTotals = computed(() => {
  let input = 0
  let output = 0
  for (const message of store.messages) {
    if (typeof message.input_tokens === 'number') input += message.input_tokens
    if (typeof message.output_tokens === 'number') output += message.output_tokens
  }
  return { input, output }
})

// CreationImageJob does not carry token usage today; sum defensively in case a
// future API response adds it, otherwise this always resolves to zero.
const imageTokenTotals = computed(() => {
  let input = 0
  let output = 0
  for (const task of store.imageTasks) {
    const record = task as unknown as { input_tokens?: number | null; output_tokens?: number | null }
    if (typeof record.input_tokens === 'number') input += record.input_tokens
    if (typeof record.output_tokens === 'number') output += record.output_tokens
  }
  return { input, output }
})

const controlsTokenTotals = computed(() =>
  store.isImageSession ? imageTokenTotals.value : chatTokenTotals.value,
)

const controlsTaskCount = computed(() => (store.isImageSession ? store.imageTasks.length : null))

function updateKeyboardOffset() {
  const viewport = window.visualViewport
  if (!viewport) {
    keyboardOffset.value = '0px'
    return
  }
  const occluded = Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop)
  keyboardOffset.value = `${occluded}px`
}

onMounted(() => {
  store.initialize().catch((error) => {
    console.error('Failed to initialize creation studio', error)
  })
  updateKeyboardOffset()
  window.visualViewport?.addEventListener('resize', updateKeyboardOffset)
  window.visualViewport?.addEventListener('scroll', updateKeyboardOffset)
})

onUnmounted(() => {
  window.visualViewport?.removeEventListener('resize', updateKeyboardOffset)
  window.visualViewport?.removeEventListener('scroll', updateKeyboardOffset)
})

function openPreview(url: string) {
  previewUrl.value = url
  previewOpen.value = true
}

function closePreview() {
  previewOpen.value = false
  previewUrl.value = ''
}
</script>

<template>
  <AppLayout>
    <div class="studio-page">
      <PageHeader
        :title="t('studio.title')"
        :description="t('studio.description')"
        variant="compact"
      />

      <div class="studio-mobile-tabs">
        <SegmentedControl v-model="mobilePanel" :options="mobilePanelOptions" size="sm" />
      </div>

      <div class="studio-layout">
        <aside
          class="studio-column studio-column-side"
          :class="{ 'is-active-mobile': mobilePanel === 'sessions' }"
        >
          <SessionList />
        </aside>

        <section
          class="studio-column studio-column-main"
          :class="{ 'is-active-mobile': mobilePanel === 'main' }"
        >
          <div
            class="glass-card studio-main-card"
            :style="{ '--studio-keyboard-offset': keyboardOffset }"
          >
            <MessageStream v-if="!store.isImageSession" class="studio-main-scroll" />
            <TaskGrid v-else class="studio-main-scroll" @preview="openPreview" />
            <ComposerBar />
          </div>
        </section>

        <aside
          class="studio-column studio-column-controls"
          :class="{ 'is-active-mobile': mobilePanel === 'controls' }"
        >
          <div class="glass-card studio-controls-card">
            <TokenStats
              variant="card"
              :input-tokens="controlsTokenTotals.input"
              :output-tokens="controlsTokenTotals.output"
              :task-count="controlsTaskCount"
            />
          </div>
        </aside>
      </div>
    </div>

    <PreviewDialog :open="previewOpen" :image-url="previewUrl" @close="closePreview" />
  </AppLayout>
</template>

<style scoped>
.studio-page {
  display: flex;
  flex-direction: column;
}

.studio-mobile-tabs {
  display: none;
}

.studio-layout {
  display: grid;
  grid-template-columns: minmax(220px, 280px) minmax(0, 1fr) minmax(240px, 320px);
  gap: 14px;
  align-items: start;
}

.studio-column {
  min-width: 0;
}

.studio-main-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  min-height: min(68vh, 720px);
}

.studio-main-scroll {
  min-height: 0;
}

.studio-controls-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 12px;
  min-height: min(68vh, 720px);
}

/* Desktop 3-column layout: lock the row to the viewport (minus header + page
   padding) so the composer never gets clipped, and let each column scroll
   its own content internally instead of growing the whole page. */
@media (min-width: 1101px) {
  .studio-main-card,
  .studio-controls-card {
    height: calc(100vh - 146px - 24px);
    min-height: 0;
  }

  .studio-main-scroll {
    flex: 1;
    overflow-y: auto;
  }

  .studio-controls-card {
    overflow-y: auto;
  }
}

@media (max-width: 1100px) {
  .studio-layout {
    grid-template-columns: 1fr;
    grid-template-areas:
      'sessions'
      'main'
      'controls';
  }

  .studio-column-side {
    grid-area: sessions;
  }

  .studio-column-main {
    grid-area: main;
  }

  .studio-column-controls {
    grid-area: controls;
  }
}

@media (max-width: 767px) {
  .studio-page {
    padding-bottom: calc(12px + env(safe-area-inset-bottom, 0px));
  }

  .studio-mobile-tabs {
    display: flex;
    margin-bottom: 12px;
  }

  .studio-layout {
    gap: 10px;
  }

  .studio-column {
    display: none;
  }

  .studio-column.is-active-mobile {
    display: flex;
    flex-direction: column;
  }

  .studio-main-card {
    min-height: min(42vh, 420px);
    position: sticky;
    bottom: var(--studio-keyboard-offset, 0px);
    z-index: 2;
    background: color-mix(in oklch, var(--surface) 92%, transparent);
    backdrop-filter: blur(12px);
  }
}
</style>
