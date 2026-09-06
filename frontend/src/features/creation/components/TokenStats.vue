<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import StatCard from '@/components/ui/StatCard.vue'

const props = withDefaults(
  defineProps<{
    inputTokens?: number | null
    outputTokens?: number | null
    variant?: 'inline' | 'card'
    /** Optional task/run count shown alongside tokens (e.g. image generation sessions). */
    taskCount?: number | null
  }>(),
  {
    variant: 'inline',
  },
)

const { t } = useI18n()

const hasInput = computed(() => typeof props.inputTokens === 'number')
const hasOutput = computed(() => typeof props.outputTokens === 'number')
const hasCount = computed(() => typeof props.taskCount === 'number')
const hasData = computed(() => hasInput.value || hasOutput.value || hasCount.value)

const inputValue = computed(() => props.inputTokens ?? t('common.unknown'))
const outputValue = computed(() => props.outputTokens ?? t('common.unknown'))
const totalTokens = computed(() => hasInput.value && hasOutput.value
  ? (props.inputTokens as number) + (props.outputTokens as number)
  : t('common.unknown'))

const cardSubText = computed(() =>
  hasCount.value
    ? t('studio.tokens.cardSubWithCount', {
        input: inputValue.value,
        output: outputValue.value,
        count: props.taskCount ?? 0,
      })
    : t('studio.tokens.cardSub', { input: inputValue.value, output: outputValue.value }),
)

const hovering = ref(false)
const pinned = ref(false)
const expanded = computed(() => hovering.value || pinned.value)

function toggle() {
  pinned.value = !pinned.value
}

function onMouseEnter() {
  hovering.value = true
}

function onMouseLeave() {
  hovering.value = false
}

function onBlur() {
  hovering.value = false
  pinned.value = false
}
</script>

<template>
  <StatCard
    v-if="variant === 'card'"
    class="studio-token-stats-card"
    :label="t('studio.tokens.cardLabel')"
    :value="totalTokens"
    :sub="cardSubText"
    :aria-label="t('studio.a11y.tokenStatsCard')"
    padding="sm"
  />
  <span
    v-else-if="hasData"
    class="studio-token-stats text-xs text-muted"
    :class="{ 'is-expanded': expanded }"
    role="button"
    tabindex="0"
    :aria-expanded="expanded"
    :aria-label="
      t('studio.tokens.ariaLabel', { input: inputValue, output: outputValue, total: totalTokens })
    "
    @click="toggle"
    @keydown.enter.prevent="toggle"
    @keydown.space.prevent="toggle"
    @mouseenter="onMouseEnter"
    @mouseleave="onMouseLeave"
    @blur="onBlur"
  >
    <span class="studio-token-stats-icon" aria-hidden="true">📊</span>
    <span v-if="!expanded" class="studio-token-stats-summary">{{
      t('studio.tokens.total', { total: totalTokens })
    }}</span>
    <span v-else class="studio-token-stats-detail">{{
      t('studio.tokens.detail', { input: inputValue, output: outputValue })
    }}</span>
  </span>
</template>

<style scoped>
.studio-token-stats {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-top: 8px;
  padding: 2px 8px;
  border-radius: 999px;
  border: 1px solid transparent;
  cursor: pointer;
  user-select: none;
  line-height: 1.4;
}

.studio-token-stats:hover,
.studio-token-stats:focus-visible,
.studio-token-stats.is-expanded {
  border-color: color-mix(in oklch, var(--border) 70%, transparent);
  background: color-mix(in oklch, var(--surface-secondary, var(--surface)) 88%, transparent);
  outline: none;
}

.studio-token-stats-icon {
  font-size: 12px;
}
</style>
