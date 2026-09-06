<template>
  <div class="cell-actions" ref="rootRef">
    <button
      v-if="showEdit"
      type="button"
      class="icon-btn"
      :aria-label="editLabel"
      :title="editLabel"
      @click.stop="emit('edit')"
    >
      <Icon name="edit" size="sm" :stroke-width="1.8" />
    </button>

    <slot name="extra" />

    <template v-if="items.length || $slots.menu">
      <button
        ref="triggerRef"
        type="button"
        class="icon-btn"
        :aria-label="moreLabel"
        :title="moreLabel"
        :aria-expanded="open"
        @click.stop="toggleOpen"
      >
        <Icon name="more" size="sm" :stroke-width="1.8" />
      </button>

      <Teleport to="body">
        <div
          v-if="open"
          ref="menuRef"
          class="dropdown cell-actions-menu"
          :style="menuStyle"
          @click.stop
          @keydown.esc.stop.prevent="closeAndFocusTrigger"
        >
          <slot name="menu" :close="close">
            <button
              v-for="(item, index) in items"
              :key="index"
              type="button"
              class="dropdown-item"
              :class="{ 'dropdown-item-danger': item.danger, 'is-active': item.active }"
              :disabled="item.disabled"
              @click="handleItemClick(item)"
            >
              <Icon v-if="item.icon" :name="(item.icon as any)" size="sm" :stroke-width="1.8" />
              {{ item.label }}
            </button>
          </slot>
        </div>
      </Teleport>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { getFloatingPanelPosition } from '@/utils/floatingPanel'

export interface ActionsCellItem {
  label: string
  icon?: string
  danger?: boolean
  active?: boolean
  disabled?: boolean
  onClick?: () => void
}

const props = withDefaults(
  defineProps<{
    /** Show the 28px edit pencil button (emits 'edit'). */
    showEdit?: boolean
    editLabel?: string
    moreLabel?: string
    /** Dropdown menu entries. Ignored if the `menu` slot is used. */
    items?: ActionsCellItem[]
  }>(),
  {
    showEdit: true,
    editLabel: undefined,
    moreLabel: undefined,
    items: () => []
  }
)

const emit = defineEmits<{
  edit: []
}>()

const { t } = useI18n()
const editLabel = computed(() => props.editLabel ?? t('common.edit'))
const moreLabel = computed(() => props.moreLabel ?? t('common.moreActions'))

const rootRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLElement | null>(null)
const menuRef = ref<HTMLElement | null>(null)
const open = ref(false)
const menuStyle = ref<Record<string, string>>({ top: '0px', left: '0px' })

const positionMenu = () => {
  const trigger = triggerRef.value
  if (!trigger) return
  const rect = trigger.getBoundingClientRect()
  const position = getFloatingPanelPosition(rect, window.innerWidth, window.innerHeight, {
    viewportPadding: 8,
    gap: 4,
    maxWidth: menuRef.value?.offsetWidth || 180,
    mobileBreakpoint: 0,
    minComfortableHeight: menuRef.value?.scrollHeight || 264
  })
  menuStyle.value = {
    top: position.top === null ? '' : `${position.top}px`,
    bottom: position.bottom === null ? '' : `${position.bottom}px`,
    left: `${position.left}px`,
    width: `${position.width}px`,
    maxHeight: `${position.maxHeight}px`
  }
}

const close = () => {
  open.value = false
  window.removeEventListener('scroll', onScroll, true)
  window.removeEventListener('resize', positionMenu)
}

const closeAndFocusTrigger = () => {
  close()
  triggerRef.value?.focus()
}

const onScroll = (event: Event) => {
  // Scrolling the menu must not dismiss actions below its visible area.
  if (event.target instanceof Node && menuRef.value?.contains(event.target)) return
  close()
}

const toggleOpen = async () => {
  if (open.value) {
    close()
    return
  }
  open.value = true
  await nextTick()
  if (!open.value || !menuRef.value) return
  positionMenu()
  window.addEventListener('scroll', onScroll, true)
  window.addEventListener('resize', positionMenu)
}

const handleItemClick = (item: ActionsCellItem) => {
  if (item.disabled) return
  item.onClick?.()
  close()
}

onClickOutside(menuRef, () => close(), { ignore: [triggerRef] })

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll, true)
  window.removeEventListener('resize', positionMenu)
})
</script>

<style scoped>
.cell-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.cell-actions-menu {
  position: fixed;
  min-width: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}
</style>
