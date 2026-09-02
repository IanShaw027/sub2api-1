<script setup lang="ts">
import Pagination from '@/components/common/Pagination.vue'

defineOptions({ inheritAttrs: false })

withDefaults(
  defineProps<{
    page: number
    pageSize: number
    total: number
    showPageSizeSelector?: boolean
    showJump?: boolean
  }>(),
  {
    showPageSizeSelector: true,
    showJump: false
  }
)

defineEmits<{
  'update:page': [page: number]
  'update:pageSize': [size: number]
}>()
</script>

<template>
  <Pagination
    v-bind="$attrs"
    class="ui-pagination"
    :page="page"
    :page-size="pageSize"
    :total="total"
    :show-page-size-selector="showPageSizeSelector"
    :show-jump="showJump"
    @update:page="$emit('update:page', $event)"
    @update:page-size="$emit('update:pageSize', $event)"
  />
</template>
