<template>
  <header :class="['ui-page-header', variant === 'hero' ? 'ui-page-header-hero' : 'ui-page-header-compact']">
    <div class="ui-page-header-main">
      <p v-if="eyebrow || $slots.eyebrow" class="ui-page-header-eyebrow">
        <slot name="eyebrow">{{ eyebrow }}</slot>
      </p>
      <h1 class="ui-page-header-title">
        <slot name="title">{{ title }}</slot>
      </h1>
      <p v-if="descriptionText || $slots.description" class="ui-page-header-description">
        <slot name="description">{{ descriptionText }}</slot>
      </p>
    </div>
    <div v-if="$slots.actions" class="ui-page-header-actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PageHeaderVariant } from './types'

const props = withDefaults(
  defineProps<{
    title?: string
    description?: string
    subtitle?: string
    eyebrow?: string
    variant?: PageHeaderVariant
  }>(),
  {
    variant: 'compact'
  }
)

const descriptionText = computed(() => props.description || props.subtitle)
</script>

<style scoped>
.ui-page-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.ui-page-header-hero {
  align-items: flex-start;
}

.ui-page-header-compact .ui-page-header-title {
  margin: 0;
  font-family: var(--display);
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 35px; /* prototype: line-height normal → 35px box */
  color: var(--foreground);
}

.ui-page-header-hero .ui-page-header-title {
  margin: 0;
  font-family: var(--display);
  font-size: 30px;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1.15;
  color: var(--foreground);
}

.ui-page-header-eyebrow {
  margin: 0 0 6px;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--muted);
}

.ui-page-header-description {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 19px; /* prototype: line-height normal → 19px box */
  color: var(--muted);
}

.ui-page-header-hero .ui-page-header-description {
  font-size: 13.5px;
  max-width: 540px;
  line-height: 1.55;
}

.ui-page-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
}

/* <768px: a long description must not be squeezed into a narrow column beside the
   actions — let the actions wrap under the title block when both cannot share the row. */
@media (max-width: 767px) {
  .ui-page-header {
    flex-wrap: wrap;
  }

  .ui-page-header-main {
    flex: 1 1 180px;
    min-width: 0;
  }

  .ui-page-header-actions {
    flex-wrap: wrap;
  }
}
</style>
