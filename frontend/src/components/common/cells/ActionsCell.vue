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
const menuStyle = ref<{ top: string; left: string }>({ top: '0px', left: '0px' })

const positionMenu = () => {
  const trigger = triggerRef.value
  if (!trigger) return
  const rect = trigger.getBoundingClientRect()
  const menuWidth = menuRef.value?.offsetWidth ?? 180
  const left = Math.min(rect.right - menuWidth, window.innerWidth - menuWidth - 8)
  menuStyle.value = {
    top: `${rect.bottom + 4}px`,
    left: `${Math.max(8, left)}px`
  }
}

const close = () => {
  open.value = false
  window.removeEventListener('scroll', close, true)
  window.removeEventListener('resize', positionMenu)
}

const toggleOpen = async () => {
  if (open.value) {
    close()
    return
  }
  open.value = true
  await nextTick()
  positionMenu()
  window.addEventListener('scroll', close, true)
  window.addEventListener('resize', positionMenu)
}

const handleItemClick = (item: ActionsCellItem) => {
  if (item.disabled) return
  item.onClick?.()
  close()
}

onClickOutside(menuRef, () => close(), { ignore: [triggerRef] })

onBeforeUnmount(() => {
  window.removeEventListener('scroll', close, true)
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
}
</style>
