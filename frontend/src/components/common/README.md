# Common Components

This directory contains reusable Vue 3 components built with Composition API, TypeScript, and TailwindCSS.

## Components

### DataTable.vue

A generic data table component with sorting, loading states, and custom cell rendering.

**Props:**

- `columns: Column[]` - Array of column definitions with key, label, sortable, and formatter
- `data: any[]` - Array of data objects to display
- `loading?: boolean` - Show loading skeleton
- `defaultSortKey?: string` - Default sort key (only used if no persisted sort state)
- `defaultSortOrder?: 'asc' | 'desc'` - Default sort order (default: `asc`)
- `sortStorageKey?: string` - Persist sort state (key + order) to localStorage
- `rowKey?: string | (row: any) => string | number` - Row key field or resolver (defaults to `row.id`, falls back to index)

**Slots:**

- `empty` - Custom empty state content
- `cell-{key}` - Custom cell renderer for specific column (receives `row` and `value`)

**Usage:**

```vue
<DataTable
  :columns="[
    { key: 'name', label: 'Name', sortable: true },
    { key: 'email', label: 'Email' },
    { key: 'status', label: 'Status', formatter: (val) => val.toUpperCase() }
  ]"
  :data="users"
  :loading="isLoading"
>
  <template #cell-actions="{ row }">
    <button @click="editUser(row)">Edit</button>
  </template>
</DataTable>
```

---

### Pagination.vue

Pagination component with page numbers, navigation, and page size selector.

**Props:**

- `total: number` - Total number of items
- `page: number` - Current page (1-indexed)
- `pageSize: number` - Items per page
- `pageSizeOptions?: number[]` - Available page size options (default: [10, 20, 50, 100])

**Events:**

- `update:page` - Emitted when page changes
- `update:pageSize` - Emitted when page size changes

**Usage:**

```vue
<Pagination
  :total="totalUsers"
  :page="currentPage"
  :pageSize="pageSize"
  @update:page="currentPage = $event"
  @update:pageSize="pageSize = $event"
/>
```

---

### Modal.vue

Modal dialog with customizable size and close behavior.

**Props:**

- `show: boolean` - Control modal visibility
- `title: string` - Modal title
- `size?: 'sm' | 'md' | 'lg' | 'xl' | 'full'` - Modal size (default: 'md')
- `closeOnEscape?: boolean` - Close on Escape key (default: true)
- `closeOnClickOutside?: boolean` - Close on backdrop click (default: true)

**Events:**

- `close` - Emitted when modal should close

**Slots:**

- `default` - Modal body content
- `footer` - Modal footer content

**Usage:**

```vue
<Modal :show="showModal" title="Edit User" size="lg" @close="showModal = false">
  <form @submit.prevent="saveUser">
    <!-- Form content -->
  </form>

  <template #footer>
    <button @click="showModal = false">Cancel</button>
    <button @click="saveUser">Save</button>
  </template>
</Modal>
```

---

### ConfirmDialog.vue

Confirmation dialog built on top of Modal component.

**Props:**

- `show: boolean` - Control dialog visibility
- `title: string` - Dialog title
- `message: string` - Confirmation message
- `confirmText?: string` - Confirm button text (default: 'Confirm')
- `cancelText?: string` - Cancel button text (default: 'Cancel')
- `danger?: boolean` - Use danger/red styling (default: false)

**Events:**

- `confirm` - Emitted when user confirms
- `cancel` - Emitted when user cancels

**Usage:**

```vue
<ConfirmDialog
  :show="showDeleteConfirm"
  title="Delete User"
  message="Are you sure you want to delete this user? This action cannot be undone."
  confirm-text="Delete"
  cancel-text="Cancel"
  danger
  @confirm="deleteUser"
  @cancel="showDeleteConfirm = false"
/>
```

---

### StatCard.vue

> Superseded by `components/ui/StatCard.vue` (delta tones up / down / warn / neutral); kept for legacy call sites.

Statistics card component for displaying metrics with optional change indicators.

**Props:**

- `title: string` - Card title
- `value: number | string` - Main value to display
- `icon?: Component` - Icon component
- `change?: number` - Percentage change value
- `changeType?: 'up' | 'down' | 'neutral'` - Change direction (default: 'neutral')
- `formatValue?: (value) => string` - Custom value formatter

**Usage:**

```vue
<StatCard title="Total Users" :value="1234" :icon="UserIcon" :change="12.5" change-type="up" />
```

---

### Toast.vue

Toast notification component that automatically displays toasts from the app store.

**Usage:**

```vue
<!-- Add once in App.vue or layout -->
<Toast />
```

```typescript
// Trigger toasts from anywhere using the app store
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()

appStore.addToast({
  type: 'success',
  title: 'Success!',
  message: 'User created successfully',
  duration: 3000
})

appStore.addToast({
  type: 'error',
  message: 'Failed to delete user'
})
```

---

### LoadingSpinner.vue

Simple animated loading spinner. Renders the global `.spinner` recipe: a 2px ring in `currentColor`.

**Props:**

- `size?: 'sm' | 'md' | 'lg' | 'xl'` - Spinner size: 16px / 20px / 24px / 48px (default: 'md')
- `color?: 'primary' | 'secondary' | 'white' | 'gray'` - Spinner color (default: 'primary')

**Usage:**

```vue
<LoadingSpinner size="lg" color="primary" />
```

---

### EmptyState.vue

Empty state placeholder with icon, message, and optional action button.

**Props:**

- `icon?: Component` - Icon component
- `title: string` - Empty state title
- `description: string` - Empty state description
- `actionText?: string` - Action button text
- `actionTo?: string | object` - Router link destination
- `actionIcon?: boolean` - Show plus icon in button (default: true)

**Slots:**

- `icon` - Custom icon content
- `action` - Custom action button/link

**Usage:**

```vue
<EmptyState
  title="No users found"
  description="Get started by creating your first user account."
  action-text="Add User"
  :action-to="{ name: 'users-create' }"
/>
```

## Glass UI recipes (glass-ui-redesign)

These `common/` components follow the token-based recipes from
`openspec/changes/glass-ui-redesign/ui-standards.md`. Key sizes:

- **Fields** (`Select`, `SearchInput`, `Input`, `TextArea`, `DateRangePicker`
  trigger, `ProxySelector`/`ProxyRotationSelector` trigger): `.field` — 36px
  tall, 12px radius, surface at 85% opacity + `--field-shadow`; focus adds an
  accent border and a 3px 18%-accent ring; `.field-error` swaps to a
  14%-danger ring. Labels are 12.5px/600 with a 6px gap under them; hints are
  12px muted; errors are 12px danger text; the required star is danger text.
- **Filter pills / segmented triggers** (`AutoRefreshButton`,
  `SubscriptionProgressMini`, `LocaleSwitcher`): `.filter-pill` — 36px tall,
  muted label + 600-weight value + 14px chevron; `.is-active` swaps to an
  accent border and 10%-accent background.
- **Dropdown / popover panels** (`Select` panel, `DateRangePicker` calendar
  panel, `ProxySelector` panel, `AutoRefreshButton`/`VersionBadge` menus):
  12px radius, 6px padding, surface at 92% + 20px blur, `--shadow-pop`,
  max-height ~320px. Options are 36px/9px-radius rows: 5% foreground on
  hover, 10%-accent + accent text + a 14px check mark when selected. Group
  labels are 11px/600 uppercase muted.
- **Badges/pills** (`StatusBadge`, `GroupBadge`, `GroupCapacityBadge`,
  `PlatformTypeBadge`, `.tag`, `.count-badge`): 20-22px tall pills with an
  `inset 0 0 0 1px color-mix(in oklch, currentColor 22%, transparent)` ring
  and a 6px status dot. Group hues use
  `color-mix(in oklch, <hue> 16%, transparent)` for the background and
  `color-mix(in oklch, <hue> 70%, var(--foreground))` for the text.
  `PlatformTypeBadge` renders a 20px brand tile from `@/utils/platformTile`.
- **HelpTooltip**: a 15px `?` circle (1.5px muted stroke, 10px/700 glyph)
  that opens a `.tooltip-bubble` (dark, `--shadow-pop`).
- **Toast**: `.toast` card — 12px/14px padding, 12px radius, surface + border
  + `--shadow-hover`; a 22px tone circle (16% tone background), 13px/600
  title, 12px muted message, 28px close button, and a 2px bottom progress
  bar. No left color bar. At most 4 toasts stack.
- **EmptyState**: dashed border, 12px radius, 24px padding, 24px icon,
  12.5px/600 title, an 11.5px/600 accent action link; `size="lg"` bumps the
  icon to 40px and the title to 14px/600 (description stays 12.5px muted).
- **Skeleton**: `.skeleton` shimmer with a 5px radius.
- **LoadingSpinner**: 2px ring at 16 / 20 / 24 / 48px (`sm`/`md`/`lg`/`xl`).
- **NavigationProgress**: 3px bar, `accent → accent-60%-white` gradient, with
  an `0 0 8px accent` glow.
- **AnnouncementBell / AnnouncementPopup / ExportProgressDialog**: modal look
  — `.modal-overlay` (foreground 40% + 6px blur), `.modal-content` (16px
  radius, surface, `--shadow-pop`), `.modal-header` (`16px 20px 12px`,
  16px/800 title), `.modal-footer` (`.btn` actions). The bell's unread count
  uses the global `.count-badge` (18px pill) positioned over a 34px
  `.header-icon-btn`.
- **ImageUpload**: dashed 12px-radius drop zone, 44px `.brand-mark-xl`
  preview, a 32px secondary "upload" button and a ghost "remove" button.
- **IpGeoBatchToolbar / IpGeoCell / MonitorQuotaView /
  SubscriptionProgressMini**: 5–6px `.progress` bars with 70/90% tone
  thresholds (success → warning → danger) and 11px muted labels.
- **SupportQRCodesButton / AnnouncementBell / AnnouncementPopup icon
  triggers**: 34px `.header-icon-btn`.
- **VersionBadge**: renders as a mono `.tag` (11.5px); the admin dropdown
  reuses the global `.dropdown` panel recipe.

## Import

You can import components individually:

```typescript
import { DataTable, Pagination, Modal } from '@/components/common'
```

Or import specific components:

```typescript
import DataTable from '@/components/common/DataTable.vue'
```

## Features

All components include:

- **TypeScript support** with proper type definitions
- **Accessibility** with ARIA attributes and keyboard navigation
- **Responsive design** with mobile-friendly layouts
- **TailwindCSS styling** for consistent design
- **Vue 3 Composition API** with `<script setup>`
- **Slot support** for customization
