<script lang="ts">
// Module-level counter (must live outside <script setup>, which re-runs per instance).
// Pairs with useId so SVG gradient ids stay unique across separately mounted app
// roots (vitest) as well as within one app tree.
let logoInstanceSeq = 0
</script>

<script setup lang="ts">
/**
 * Clomio BrandLogo — inline SVG (cloud + gold sparkle).
 * Visual recipe ported from clomio packages/ui brand_logo; Vue-only, no monorepo import.
 */
import { computed, useId } from 'vue'

const props = withDefaults(
  defineProps<{
    /** Icon size in px */
    size?: number
    /** Hide wordmark (collapsed nav / favicon-like) */
    iconOnly?: boolean
    /** Wordmark text; callers may pass siteName */
    wordmark?: string
    /** Extra classes on root */
    class?: string
  }>(),
  {
    size: 28,
    iconOnly: false,
    wordmark: 'Clomio'
  }
)

const vueId = useId()
const instanceSeq = ++logoInstanceSeq
const gradientId = computed(
  () => `clomio-brand-logo-${vueId.replace(/[^a-zA-Z0-9_-]/g, '')}-${instanceSeq}`
)
</script>

<template>
  <div
    class="inline-flex items-center gap-2"
    :class="props.class"
    data-brand="clomio"
  >
    <svg
      :width="size"
      :height="size"
      viewBox="0 0 32 32"
      fill="none"
      aria-hidden="true"
      class="shrink-0"
    >
      <defs>
        <linearGradient :id="gradientId" x1="0" y1="0" x2="32" y2="32">
          <stop offset="0%" stop-color="rgb(var(--c-grad-from))" />
          <stop offset="100%" stop-color="rgb(var(--c-grad-to))" />
        </linearGradient>
      </defs>
      <path
        d="M9 22a6 6 0 0 1-.6-11.97A8 8 0 0 1 24 12a5 5 0 0 1-.5 10H9Z"
        :fill="`url(#${gradientId})`"
        stroke="rgb(255 255 255 / 0.2)"
        stroke-width="0.75"
      />
      <path
        d="M22 6l1.2 2.6L26 9.8l-2.8 1.2L22 13.6l-1.2-2.6L18 9.8l2.8-1.2L22 6Z"
        fill="rgb(var(--c-gold))"
      />
    </svg>
    <span
      v-if="!iconOnly"
      class="text-lg font-[650] tracking-[-0.01em] text-ink dark:text-ink"
    >
      {{ wordmark }}
    </span>
  </div>
</template>
