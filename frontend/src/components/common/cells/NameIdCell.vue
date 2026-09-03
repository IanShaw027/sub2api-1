<template>
  <div class="cell-name-id">
    <component
      :is="href ? 'a' : to ? 'router-link' : 'span'"
      :href="href"
      :to="to"
      :target="href ? '_blank' : undefined"
      :rel="href ? 'noopener noreferrer' : undefined"
      class="cell-name-id-name"
      :class="{ 'is-link': href || to }"
    >
      <slot name="name">{{ name }}</slot>
    </component>
    <span v-if="email" class="cell-name-id-email">{{ email }}</span>
    <span v-if="id !== undefined && id !== null" class="cell-name-id-meta">
      #{{ id }}<template v-if="meta"> · {{ meta }}</template>
    </span>
    <span v-else-if="meta" class="cell-name-id-meta">{{ meta }}</span>
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    name: string
    id?: string | number | null
    /** Extra text appended after the id, e.g. a group name. */
    meta?: string | null
    /** Optional secondary line (e.g. email) rendered between name and id/meta. */
    email?: string | null
    /** Render the name as an external link. */
    href?: string | null
    /** Render the name as a router-link. */
    to?: string | Record<string, unknown> | null
  }>(),
  {
    id: null,
    meta: null,
    email: null,
    href: null,
    to: null
  }
)
</script>

<style scoped>
.cell-name-id {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
}

.cell-name-id-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cell-name-id-name.is-link {
  color: var(--accent);
  cursor: pointer;
}

.cell-name-id-name.is-link:hover {
  text-decoration: underline;
}

.cell-name-id-email {
  font-size: 12px;
  font-weight: 500;
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cell-name-id-meta {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
