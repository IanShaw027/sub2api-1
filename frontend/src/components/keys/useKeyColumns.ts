import { reactive, computed, type ComputedRef } from 'vue'
import type { Column } from '@/components/common/types'

// Column-visibility persistence for the API keys table, extracted out of
// KeysView.vue to keep the view under the frontend-health-cleanup 6.4 line cap.

const ALWAYS_VISIBLE_COLUMNS = new Set(['name', 'actions'])
// Prototype default visibility: show 速率限制 (rate_limit) and 最近使用
// (last_used_at) by default, hide 创建时间 (created_at) instead — deviation
// "prototype wins" over the previous default, logged in deviations.md.
const DEFAULT_HIDDEN_COLUMNS = ['id', 'last_used_ip', 'created_at']
const HIDDEN_COLUMNS_KEY = 'api-key-hidden-columns'
const COLUMN_SETTINGS_VERSION_KEY = 'api-key-column-settings-version'
const COLUMN_SETTINGS_VERSION = 4
const VERSION_NEW_HIDDEN_COLUMNS: Record<number, string[]> = {
  2: ['last_used_ip'],
  3: ['id'],
  4: ['created_at']
}
// v4 also flips rate_limit/last_used_at to visible-by-default (prototype
// wins); the generic "add to hidden" migration above can't express an
// unhide, so it's handled as an explicit removal step keyed by version.
const VERSION_UNHIDE_COLUMNS: Record<number, string[]> = {
  4: ['rate_limit', 'last_used_at']
}

export function useKeyColumns(allColumns: ComputedRef<Column[]>) {
  const hiddenColumns = reactive<Set<string>>(new Set())

  const toggleableColumns = computed(() =>
    allColumns.value.filter((col) => !ALWAYS_VISIBLE_COLUMNS.has(col.key))
  )

  const columns = computed<Column[]>(() =>
    allColumns.value.filter((col) => ALWAYS_VISIBLE_COLUMNS.has(col.key) || !hiddenColumns.has(col.key))
  )

  const saveColumnsToStorage = () => {
    try {
      localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
      localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
    } catch (error) {
      console.error('Failed to save API key table columns:', error)
    }
  }

  const loadSavedColumns = () => {
    hiddenColumns.clear()
    try {
      const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
      if (saved) {
        const parsed = JSON.parse(saved) as string[]
        const validColumnKeys = new Set(allColumns.value.map((col) => col.key))
        parsed
          .filter((key) =>
            typeof key === 'string' &&
            validColumnKeys.has(key) &&
            !ALWAYS_VISIBLE_COLUMNS.has(key)
          )
          .forEach((key) => hiddenColumns.add(key))
        const storedVersion = Number(localStorage.getItem(COLUMN_SETTINGS_VERSION_KEY) ?? '1')
        if (storedVersion < COLUMN_SETTINGS_VERSION) {
          for (let v = storedVersion + 1; v <= COLUMN_SETTINGS_VERSION; v++) {
            for (const key of VERSION_NEW_HIDDEN_COLUMNS[v] ?? []) {
              if (validColumnKeys.has(key) && !ALWAYS_VISIBLE_COLUMNS.has(key)) {
                hiddenColumns.add(key)
              }
            }
            for (const key of VERSION_UNHIDE_COLUMNS[v] ?? []) {
              hiddenColumns.delete(key)
            }
          }
          saveColumnsToStorage()
        } else {
          localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
        }
      } else {
        DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
        localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
      }
    } catch (error) {
      console.error('Failed to load API key table columns:', error)
      DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
    }
  }

  const toggleColumn = (key: string) => {
    if (ALWAYS_VISIBLE_COLUMNS.has(key)) return
    if (hiddenColumns.has(key)) {
      hiddenColumns.delete(key)
    } else {
      hiddenColumns.add(key)
    }
    saveColumnsToStorage()
  }

  const isColumnVisible = (key: string) => !hiddenColumns.has(key)

  return {
    toggleableColumns,
    columns,
    loadSavedColumns,
    toggleColumn,
    isColumnVisible
  }
}
