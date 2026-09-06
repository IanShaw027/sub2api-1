/** Admin UsersView: column definitions + visibility state (persisted to
 * localStorage, with a versioned migration so upgrades can auto-hide newly
 * added default-hidden columns without disturbing a user's existing prefs
 * for older columns). */
import { computed, reactive, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Column } from '@/components/common/types'
import type { UserAttributeDefinition } from '@/types'

// 显式数组取代 Object.keys()：保证迭代顺序（决定列头排序按钮渲染顺序）
// 不会因 JS 引擎差异或 USAGE_COLUMN_PLATFORMS 属性顺序调整而静默变化。
export const USAGE_COLUMN_KEYS: readonly string[] = ['usage', 'usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity']
export const USAGE_COLUMN_PLATFORMS: Record<string, string | null> = {
  usage: null,
  usage_anthropic: 'anthropic',
  usage_openai: 'openai',
  usage_gemini: 'gemini',
  usage_antigravity: 'antigravity'
}
export const PLATFORM_USAGE_COLUMNS = USAGE_COLUMN_KEYS.filter((k) => k !== 'usage')

export interface UseUserColumnsCallbacks {
  /** Fired when a usage/attribute/platform-quota column becomes visible again (needs a refetch of that page's secondary data). */
  onSecondaryDataColumnShown: () => void
  /** Fired when the subscriptions column is toggled (subscriptions are only ever fetched as part of the main user list request). */
  onSubscriptionsToggled: () => void
  /** Fired when the groups column becomes visible again (needs the shared groups list). */
  onGroupsColumnShown: () => void
}

export function useUserColumns(
  attributeDefinitions: Ref<UserAttributeDefinition[]>,
  callbacks: UseUserColumnsCallbacks
) {
  const { t } = useI18n()

  // Generate dynamic attribute columns from enabled definitions
  const attributeColumns = computed<Column[]>(() =>
    attributeDefinitions.value
      .filter(def => def.enabled)
      .map(def => ({
        key: `attr_${def.id}`,
        label: def.name,
        sortable: false
      }))
  )

  // All possible columns (for column settings)
  const allColumns = computed<Column[]>(() => [
    { key: 'email', label: t('admin.users.columns.user'), sortable: true },
    { key: 'id', label: t('admin.users.columns.id'), sortable: true },
    { key: 'username', label: t('admin.users.columns.username'), sortable: true },
    { key: 'notes', label: t('admin.users.columns.notes'), sortable: false },
    // Dynamic attribute columns
    ...attributeColumns.value,
    { key: 'role', label: t('admin.users.columns.role'), sortable: true },
    { key: 'groups', label: t('admin.users.columns.groups'), sortable: false },
    { key: 'subscriptions', label: t('admin.users.columns.subscriptions'), sortable: false },
    { key: 'balance', label: t('admin.users.columns.balance'), sortable: true },
    { key: 'balance_platform_quota', label: t('admin.users.columns.balancePlatformQuota'), sortable: false },
    { key: 'usage', label: t('admin.users.columns.usage'), sortable: false },
    { key: 'usage_anthropic', label: t('admin.users.columns.usageAnthropic'), sortable: false },
    { key: 'usage_openai', label: t('admin.users.columns.usageOpenAI'), sortable: false },
    { key: 'usage_gemini', label: t('admin.users.columns.usageGemini'), sortable: false },
    { key: 'usage_antigravity', label: t('admin.users.columns.usageAntigravity'), sortable: false },
    { key: 'concurrency', label: t('admin.users.columns.concurrency'), sortable: true },
    { key: 'status', label: t('admin.users.columns.status'), sortable: true },
    { key: 'last_active_at', label: t('admin.users.columns.lastActive'), sortable: true },
    { key: 'last_used_at', label: t('admin.users.columns.lastUsed'), sortable: true },
    { key: 'created_at', label: t('admin.users.columns.created'), sortable: true },
    { key: 'actions', label: t('admin.users.columns.actions'), sortable: false }
  ])

  // Columns that can be toggled (exclude email and actions which are always visible)
  const toggleableColumns = computed(() =>
    allColumns.value.filter(col => col.key !== 'email' && col.key !== 'actions')
  )

  // Hidden columns (stored in Set - columns NOT in this set are visible)
  // This way, new columns are visible by default
  const hiddenColumns = reactive<Set<string>>(new Set())

  // Default hidden columns (columns hidden by default on first load)
  const DEFAULT_HIDDEN_COLUMNS = [
    'notes', 'groups', 'subscriptions', 'usage', 'concurrency',
    'usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity',
    'balance_platform_quota'
  ]
  const REMOVED_COLUMNS = new Set(['last_login_at'])
  // 强制可见列：加载时会被强制移出 hiddenColumns，并在列设置 UI 上 disabled。
  // 当前没有列需要强制可见 —— last_active_at 已改为可被用户隐藏。
  const FORCED_VISIBLE_COLUMNS = new Set<string>(['concurrency'])

  // localStorage keys for column settings
  const HIDDEN_COLUMNS_KEY = 'user-hidden-columns'
  // 列设置 schema 版本号。每次给 DEFAULT_HIDDEN_COLUMNS 新增列时 bump 一次，
  // 并在 VERSION_NEW_HIDDEN_COLUMNS 中登记该版本新增的 key。
  // 这样老用户升级后这些新列会被自动隐藏一次，而不会影响他们对其它老列的偏好。
  const COLUMN_SETTINGS_VERSION_KEY = 'user-column-settings-version'
  const COLUMN_SETTINGS_VERSION = 3
  const VERSION_NEW_HIDDEN_COLUMNS: Record<number, string[]> = {
    2: ['usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity'],
    3: ['balance_platform_quota']
  }

  // Save column settings to localStorage
  const saveColumnsToStorage = () => {
    try {
      localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
      localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
    } catch (e) {
      console.error('Failed to save columns:', e)
    }
  }

  // Load saved column settings
  const loadSavedColumns = () => {
    try {
      const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
      if (saved) {
        const parsed = JSON.parse(saved) as string[]
        parsed
          .filter(key => !REMOVED_COLUMNS.has(key) && !FORCED_VISIBLE_COLUMNS.has(key))
          .forEach(key => hiddenColumns.add(key))

        // 老用户升级：把每个未应用过的版本里新增的默认隐藏列自动追加到 hiddenColumns。
        const storedVersion = Number(localStorage.getItem(COLUMN_SETTINGS_VERSION_KEY) ?? '1')
        if (storedVersion < COLUMN_SETTINGS_VERSION) {
          let mutated = false
          for (let v = storedVersion + 1; v <= COLUMN_SETTINGS_VERSION; v++) {
            for (const key of VERSION_NEW_HIDDEN_COLUMNS[v] ?? []) {
              if (REMOVED_COLUMNS.has(key) || FORCED_VISIBLE_COLUMNS.has(key)) continue
              if (!hiddenColumns.has(key)) {
                hiddenColumns.add(key)
                mutated = true
              }
            }
          }
          if (mutated) saveColumnsToStorage()
          else localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
        }
      } else {
        // Use default hidden columns on first load
        DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
        localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
      }
    } catch (e) {
      console.error('Failed to load saved columns:', e)
      DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
    }
  }

  // Toggle column visibility
  const isForcedVisibleColumn = (key: string) => FORCED_VISIBLE_COLUMNS.has(key)
  const toggleColumn = (key: string) => {
    // 强制可见列(如 last_active_at)在加载时会被恢复成可见，
    // 这里阻止用户在当前会话隐藏它，避免"取消勾选 → 刷新又恢复"的反直觉行为。
    if (FORCED_VISIBLE_COLUMNS.has(key)) return
    const wasHidden = hiddenColumns.has(key)
    if (hiddenColumns.has(key)) {
      hiddenColumns.delete(key)
    } else {
      hiddenColumns.add(key)
    }
    saveColumnsToStorage()
    if (wasHidden && (key === 'usage' || key.startsWith('usage_') || key.startsWith('attr_') || key === 'balance_platform_quota')) {
      callbacks.onSecondaryDataColumnShown()
    }
    if (key === 'subscriptions') {
      callbacks.onSubscriptionsToggled()
    }
    if (wasHidden && key === 'groups') {
      callbacks.onGroupsColumnShown()
    }
  }

  // Check if column is visible (not in hidden set)
  const isColumnVisible = (key: string) => !hiddenColumns.has(key)
  // usage 主列或任意 usage_<platform> 子列可见时都需要批量拉取用量数据
  const hasVisibleUsageColumn = computed(
    () => !hiddenColumns.has('usage') || PLATFORM_USAGE_COLUMNS.some((k) => !hiddenColumns.has(k))
  )
  const hasVisibleGroupsColumn = computed(() => !hiddenColumns.has('groups'))
  const hasVisiblePlatformQuotaColumn = computed(() => !hiddenColumns.has('balance_platform_quota'))
  const hasVisibleAttributeColumns = computed(() =>
    attributeDefinitions.value.some((def) => def.enabled && !hiddenColumns.has(`attr_${def.id}`))
  )

  // Filtered columns based on visibility
  const columns = computed<Column[]>(() =>
    allColumns.value.filter(col =>
      col.key === 'email' || col.key === 'actions' || !hiddenColumns.has(col.key)
    )
  )

  // ListPage 配方：固定列宽通过 `usr-col-<key>` class 挂到 th/td 上，宽度在 style（scoped）里用 :deep() 指定。
  const cols = computed<Column[]>(() =>
    columns.value.map((col) => ({ ...col, class: `usr-col-${col.key}` }))
  )

  return {
    attributeColumns,
    allColumns,
    toggleableColumns,
    hiddenColumns,
    loadSavedColumns,
    isForcedVisibleColumn,
    toggleColumn,
    isColumnVisible,
    hasVisibleUsageColumn,
    hasVisibleGroupsColumn,
    hasVisiblePlatformQuotaColumn,
    hasVisibleAttributeColumns,
    columns,
    cols
  }
}
