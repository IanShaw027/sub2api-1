<template>
  <Button variant="secondary" @click="emit('bulk-edit-header')">
    {{ t('admin.accounts.bulkEditHeader') }}
  </Button>

  <!-- Import / Export -->
  <div class="acct-menu" ref="importExportDropdownRef">
    <Button variant="secondary" :aria-expanded="showImportExportDropdown" @click="toggleImportExportDropdown">
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3" /></svg>
      <span>{{ t('admin.accounts.importExport') }}</span>
    </Button>
    <div v-if="showImportExportDropdown" class="dropdown acct-dropdown">
      <div class="dropdown-label">{{ t('admin.accounts.dataActions') }}</div>
      <button type="button" class="dropdown-item" @click="openSyncFromCrsFromMenu">
        <Icon name="sync" size="sm" />
        <span>{{ t('admin.accounts.syncFromCrs') }}</span>
      </button>
      <button type="button" class="dropdown-item" @click="openImportDataFromMenu">
        <Icon name="upload" size="sm" />
        <span>{{ t('admin.accounts.dataImport') }}</span>
      </button>
      <button type="button" class="dropdown-item" @click="openExportDataFromMenu">
        <Icon name="download" size="sm" />
        <span>{{ selIds.length ? t('admin.accounts.dataExportSelected') : t('admin.accounts.dataExport') }}</span>
        <span v-if="selIds.length" class="acct-dropdown-count">{{ selIds.length }}</span>
      </button>
    </div>
  </div>

  <!-- More tools -->
  <div class="acct-menu" ref="accountToolsDropdownRef">
    <Button
      ref="accountToolsTriggerRef"
      variant="secondary"
      :aria-expanded="showAccountToolsDropdown"
      @click="toggleAccountToolsDropdown"
    >
      <span>{{ t('common.more') }}</span>
      <Icon name="chevronDown" size="xs" />
    </Button>
    <Teleport to="body">
      <div
        v-if="showAccountToolsDropdown"
        class="dropdown acct-tools-dropdown"
        :style="accountToolsDropdownStyle"
        @click.stop
      >
        <div class="acct-tools-scroll" :style="{ maxHeight: `${accountToolsDropdownPosition.maxHeight}px` }">
          <div class="dropdown-label">{{ t('admin.accounts.toolActions') }}</div>
          <button type="button" class="dropdown-item" @click="openCapacityForecast">
            <Icon name="chartBar" size="sm" />
            <span>{{ t('admin.accounts.capacityForecast.action') }}</span>
          </button>
          <button type="button" class="dropdown-item" @click="openErrorPassthrough">
            <Icon name="shield" size="sm" />
            <span>{{ t('admin.errorPassthrough.title') }}</span>
          </button>
          <button type="button" class="dropdown-item" @click="openTLSFingerprintProfiles">
            <Icon name="lock" size="sm" />
            <span>{{ t('admin.tlsFingerprintProfiles.title') }}</span>
          </button>
          <button type="button" class="dropdown-item" @click="openTLSFingerprintRouters">
            <Icon name="lock" size="sm" />
            <span>{{ t('admin.tlsFingerprintRouters.title') }}</span>
          </button>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
// Header-actions "after" slot for AccountsView.vue (frontend-health-cleanup 6.4
// split): bulk-edit-header button, the import/export dropdown, and the
// Teleported "more tools" dropdown. Toolbar dropdown visibility/position state
// is owned by useAccountToolbarMenus in the parent and passed in by reference
// so click-outside/scroll handling there keeps working unmodified.
import { useI18n } from 'vue-i18n'
import Button from '@/components/ui/Button.vue'
import Icon from '@/components/icons/Icon.vue'
import type { AccountToolbarMenusState } from './useAccountToolbarMenus'

/* eslint-disable vue/no-mutating-props -- `toolbarMenus` is a composable's returned state
 * object (refs + methods), passed down as a single prop by design so this component and the
 * parent share one source of truth; the assignments below set a Ref's `.value`, not the prop
 * binding itself (mirrors how the parent view mutated the same refs directly pre-split). */

const { t } = useI18n()

const props = defineProps<{
  toolbarMenus: AccountToolbarMenusState
  selIds: number[]
}>()

const emit = defineEmits<{
  (e: 'bulk-edit-header'): void
  (e: 'open-sync'): void
  (e: 'open-import'): void
  (e: 'open-export'): void
  (e: 'open-capacity-forecast'): void
  (e: 'open-error-passthrough'): void
  (e: 'open-tls-routers'): void
  (e: 'open-tls-profiles'): void
}>()

const {
  importExportDropdownRef,
  accountToolsDropdownRef,
  accountToolsTriggerRef,
  showImportExportDropdown,
  toggleImportExportDropdown,
  showAccountToolsDropdown,
  toggleAccountToolsDropdown,
  accountToolsDropdownStyle,
  accountToolsDropdownPosition,
  closeAccountToolsDropdown
} = props.toolbarMenus

const openSyncFromCrsFromMenu = () => {
  showImportExportDropdown.value = false
  emit('open-sync')
}

const openImportDataFromMenu = () => {
  showImportExportDropdown.value = false
  emit('open-import')
}

const openExportDataFromMenu = () => {
  showImportExportDropdown.value = false
  emit('open-export')
}

const openCapacityForecast = () => {
  closeAccountToolsDropdown()
  emit('open-capacity-forecast')
}

const openErrorPassthrough = () => {
  closeAccountToolsDropdown()
  emit('open-error-passthrough')
}

const openTLSFingerprintRouters = () => {
  closeAccountToolsDropdown()
  emit('open-tls-routers')
}

const openTLSFingerprintProfiles = () => {
  closeAccountToolsDropdown()
  emit('open-tls-profiles')
}
</script>

<style scoped>
.acct-menu {
  position: relative;
  display: inline-flex;
  flex: none;
}

.acct-dropdown {
  top: 100%;
  right: 0;
  margin-top: 6px;
  min-width: 200px;
}

.acct-dropdown-count {
  margin-left: auto;
  border-radius: var(--radius-pill);
  padding: 1px 7px;
  font-size: var(--fs-11);
  font-weight: var(--fw-semibold);
  background: color-mix(in oklch, var(--accent) 12%, transparent);
  color: var(--accent);
  font-variant-numeric: tabular-nums;
}

.acct-tools-dropdown {
  position: fixed;
  z-index: 9999;
}

.acct-tools-scroll {
  overflow-y: auto;
}
</style>
