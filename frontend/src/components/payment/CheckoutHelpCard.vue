<template>
  <div v-if="helpText || helpImageUrl" class="glass-card p-4">
    <div class="flex flex-col items-center gap-3">
      <img v-if="helpImageUrl" :src="helpImageUrl" alt=""
        class="h-40 max-w-full cursor-pointer rounded-lg object-contain transition-opacity hover:opacity-80"
        @click="$emit('preview', helpImageUrl)" />
      <p v-if="helpText" class="text-center text-sm text-muted">{{ helpText }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  helpText: string
  helpImageUrl: string
}>()

defineEmits<{
  preview: [url: string]
}>()
</script>
