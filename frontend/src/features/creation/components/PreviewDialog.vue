<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import UiModal from '@/components/ui/UiModal.vue'

defineProps<{
  open: boolean
  imageUrl?: string
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
</script>

<template>
  <UiModal :open="open" :title="t('studio.preview')" @close="emit('close')">
    <div v-if="imageUrl" class="studio-preview-wrap">
      <img :src="imageUrl" alt="" class="studio-preview-image" />
    </div>
  </UiModal>
</template>

<style scoped>
.studio-preview-wrap {
  display: flex;
  justify-content: center;
  align-items: center;
  max-height: 70vh;
}

.studio-preview-image {
  max-width: 100%;
  max-height: 70vh;
  border-radius: 12px;
}
</style>
