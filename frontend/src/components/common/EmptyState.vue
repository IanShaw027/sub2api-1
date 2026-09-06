<template>
  <div class="empty-state" :class="isLarge ? 'empty-state-lg' : 'empty-state-md'">
    <!-- Icon · 24px (md) / 40px (lg), muted, stroke 1.6 -->
    <slot name="icon">
      <component
        v-if="icon"
        :is="icon"
        class="empty-state-icon"
        :class="iconSizeClass"
        aria-hidden="true"
      />
      <svg
        v-else
        class="empty-state-icon"
        :class="iconSizeClass"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        stroke-width="1.6"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path
          d="M22 12h-6l-2 3h-4l-2-3H2M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"
        />
      </svg>
    </slot>

    <!-- Title · 12.5/600 (md) · 14/600 (lg) -->
    <h3 class="empty-state-title">
      {{ displayTitle }}
    </h3>

    <!-- Description · 12.5 muted -->
    <p v-if="description" class="empty-state-description">
      {{ description }}
    </p>

    <!-- Action · accent 11.5/600 link (md) or .btn-primary .btn-sm (lg) -->
    <div v-if="actionText || $slots.action" class="empty-state-action">
      <slot name="action">
        <component
          :is="actionTo ? 'RouterLink' : 'button'"
          v-if="actionText"
          :to="actionTo"
          :type="actionTo ? undefined : 'button'"
          @click="!actionTo && $emit('action')"
          :class="isLarge ? 'btn btn-primary btn-sm' : 'empty-state-link'"
        >
          <Icon v-if="actionIcon && isLarge" name="plus" size="sm" class="mr-1" />
          {{ actionText }}
        </component>
      </slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Component } from 'vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

interface Props {
  icon?: Component | string
  title?: string
  description?: string
  actionText?: string
  actionTo?: string | object
  actionIcon?: boolean
  message?: string
  /** `md` = compact dashed box (prototype 07); `lg` = full-page empty. */
  size?: 'md' | 'lg'
}

const props = withDefaults(defineProps<Props>(), {
  description: '',
  actionIcon: true,
  size: 'md'
})

const isLarge = computed(() => props.size === 'lg')
const iconSizeClass = computed(() => (isLarge.value ? 'h-10 w-10' : 'h-6 w-6'))
const displayTitle = computed(() => props.title || t('common.noData'))

defineEmits(['action'])
</script>

<style scoped>
/* Base geometry (dashed 12px box, centred column, 6px gap) comes from `.empty-state`. */
.empty-state-md {
  padding: 16px 12px;
}

.empty-state-md .empty-state-icon {
  width: 24px;
  height: 24px;
  color: var(--muted);
  stroke-width: 1.6;
}

.empty-state-md .empty-state-title {
  font-size: 12.5px;
  font-weight: 600;
}

.empty-state-md .empty-state-description {
  font-size: 11.5px;
}

.empty-state-lg {
  padding: 40px 16px;
  gap: 8px;
}

.empty-state-action {
  margin-top: 2px;
}

.empty-state-lg .empty-state-action {
  margin-top: 10px;
}

.empty-state-link {
  font-size: 11.5px;
  font-weight: 600;
  color: var(--accent);
  cursor: pointer;
  background: transparent;
  border: 0;
  padding: 0;
}

.empty-state-link:hover {
  text-decoration: underline;
}
</style>
