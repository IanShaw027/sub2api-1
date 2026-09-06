<template>
 <div class="table-page-layout" :class="{ 'mobile-mode': isMobile }">
 <!-- 固定区域：操作按钮 -->
 <div v-if="$slots.actions" class="layout-section-fixed">
 <slot name="actions" />
 </div>

 <!-- 固定区域：搜索和过滤器 -->
 <div v-if="$slots.filters" class="layout-section-fixed">
 <slot name="filters" />
 </div>

 <!-- 滚动区域：表格 -->
 <div class="layout-section-scrollable">
 <div class="glass-card table-scroll-container">
 <slot name="table" />
 <div v-if="$slots.pagination" class="layout-section-fixed table-pagination-footer">
 <slot name="pagination" />
 </div>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const isMobile = ref(false)

const checkMobile = () => {
 isMobile.value = window.innerWidth < 1024
}

onMounted(() => {
 checkMobile()
 window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
 window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
/* Desktop: fixed header/filters, scrolling table card, fixed pagination (design 04/05) */
.table-page-layout {
 display: flex;
 flex-direction: column;
 gap: 14px;
 /* viewport − topbar 66px − content padding (8px top + 24px bottom) */
 height: calc(100vh - 98px);
}

.layout-section-fixed {
 flex-shrink: 0;
}

.layout-section-scrollable {
 flex: 1;
 min-height: 0;
 min-width: 0;
 display: flex;
 flex-direction: column;
}

.table-scroll-container {
 display: flex;
 flex-direction: column;
 height: 100%;
 overflow: hidden;
 min-width: 0;
}

.table-pagination-footer {
 border-top: 1px solid var(--border);
}

.table-scroll-container :deep(.table-wrapper) {
 flex: 1;
 overflow-x: auto;
 overflow-y: auto;
 scrollbar-gutter: stable;
}

.table-scroll-container :deep(table) {
 width: 100%;
 min-width: max-content;
 display: table;
}

.table-scroll-container :deep(thead) {
 background: color-mix(in oklch, var(--surface-secondary) 45%, transparent);
 backdrop-filter: blur(8px);
 -webkit-backdrop-filter: blur(8px);
}

/* Mobile / tablet: natural page scroll, DataTable card mode */
.table-page-layout.mobile-mode {
 height: auto;
}

.table-page-layout.mobile-mode .table-scroll-container {
 height: auto;
 overflow: visible;
 border: 0;
 box-shadow: none;
 background: transparent;
 backdrop-filter: none;
 -webkit-backdrop-filter: none;
}

.table-page-layout.mobile-mode .layout-section-scrollable {
 flex: none;
 min-height: fit-content;
}

.table-page-layout.mobile-mode .table-scroll-container :deep(table) {
 flex: none;
 display: table;
 min-width: 100%;
}
</style>
