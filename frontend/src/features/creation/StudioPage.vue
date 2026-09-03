<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import ComposerBar from './components/ComposerBar.vue'
import MessageStream from './components/MessageStream.vue'
import ModelMenu from './components/ModelMenu.vue'
import PreviewDialog from './components/PreviewDialog.vue'
import SessionList from './components/SessionList.vue'
import TaskGrid from './components/TaskGrid.vue'
import { useCreationStore } from './stores/creation'

const { t } = useI18n()
const store = useCreationStore()

const previewOpen = ref(false)
const previewUrl = ref('')

const groupOptions = computed(() =>
  store.groups.map((group) => ({ value: group.id, label: group.name })),
)

onMounted(() => {
  store.initialize().catch((error) => {
    console.error('Failed to initialize creation studio', error)
  })
})

async function onGroupChange(value: string | number | boolean | null) {
  if (typeof value === 'number') {
    await store.setGroupId(value)
  }
}

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

      <div class="studio-layout">
        <aside class="studio-column studio-column-side">
          <SessionList />
        </aside>

        <section class="studio-column studio-column-main">
          <GlassCard padding="sm" class="studio-main-card">
            <MessageStream v-if="!store.isImageSession" />
            <TaskGrid v-else @preview="openPreview" />
          </GlassCard>
        </section>

        <aside class="studio-column studio-column-controls">
          <GlassCard padding="sm" class="studio-controls-card">
            <div class="studio-control-block">
              <label class="text-xs font-medium text-muted">{{ t('studio.groups') }}</label>
              <UiSelect
                :model-value="store.groupId"
                :options="groupOptions"
                :placeholder="t('studio.selectGroup')"
                :disabled="Boolean(store.selectedSessionId)"
                searchable="auto"
                @update:model-value="onGroupChange"
              />
            </div>
            <ModelMenu />
            <ComposerBar />
          </GlassCard>
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
  gap: 16px;
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

.studio-main-card,
.studio-controls-card {
  min-height: min(68vh, 720px);
}

.studio-controls-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.studio-control-block {
  display: flex;
  flex-direction: column;
  gap: 6px;
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

  .studio-main-card,
  .studio-controls-card {
    min-height: auto;
  }
}

@media (max-width: 767px) {
  .studio-page {
    gap: 12px;
    padding-bottom: calc(12px + env(safe-area-inset-bottom, 0px));
  }

  .studio-layout {
    gap: 10px;
  }

  .studio-main-card {
    min-height: min(42vh, 420px);
  }

  .studio-controls-card {
    position: sticky;
    bottom: 0;
    z-index: 2;
    background: color-mix(in oklch, var(--surface) 92%, transparent);
    backdrop-filter: blur(12px);
  }
}
</style>
