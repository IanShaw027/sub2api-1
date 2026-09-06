import { reactive } from 'vue'

// Column-visibility persistence for the admin accounts table. Defaults retain
// the pre-Glass readable fields; saved layouts are never rewritten by a theme
// migration.
const DEFAULT_HIDDEN_COLUMNS = [
  'today_stats',
  'proxy',
  'notes',
  'scheduler_score',
  'rate_multiplier',
]

const HIDDEN_COLUMNS_KEY = 'account-hidden-columns'
// One-time migration: hide scheduler score for existing admins too, because showing it opt-ins to heavy backend scoring.
const HIDDEN_COLUMNS_VERSION_KEY = 'account-hidden-columns-version'
const HIDDEN_COLUMNS_CURRENT_VERSION = 'preserve-readable-columns-v2'

export function useAccountColumnVisibility() {
  const hiddenColumns = reactive<Set<string>>(new Set())

  const saveColumnsToStorage = () => {
    try {
      localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
      localStorage.setItem(HIDDEN_COLUMNS_VERSION_KEY, HIDDEN_COLUMNS_CURRENT_VERSION)
    } catch (e) {
      console.error('Failed to save saved columns:', e)
    }
  }

  const loadSavedColumns = () => {
    try {
      const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
      if (saved) {
        const parsed = JSON.parse(saved) as string[]
        parsed.forEach(key => {
          hiddenColumns.add(key)
        })
        // The scheduler score was already opt-in before the Glass migration and
        // remains the one backend-expensive column whose legacy default must be
        // preserved. Other columns are never rewritten from a theme migration.
        const savedVersion = localStorage.getItem(HIDDEN_COLUMNS_VERSION_KEY)
        if (savedVersion !== HIDDEN_COLUMNS_CURRENT_VERSION) {
          if (!savedVersion && !hiddenColumns.has('scheduler_score')) {
            hiddenColumns.add('scheduler_score')
            localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
          }
          localStorage.setItem(HIDDEN_COLUMNS_VERSION_KEY, HIDDEN_COLUMNS_CURRENT_VERSION)
        }
      } else {
        DEFAULT_HIDDEN_COLUMNS.forEach(key => {
          hiddenColumns.add(key)
        })
        localStorage.setItem(HIDDEN_COLUMNS_VERSION_KEY, HIDDEN_COLUMNS_CURRENT_VERSION)
      }
    } catch (e) {
      console.error('Failed to load saved columns:', e)
      DEFAULT_HIDDEN_COLUMNS.forEach(key => {
        hiddenColumns.add(key)
      })
    }
  }

  const isColumnVisible = (key: string) => !hiddenColumns.has(key)

  /** Toggles the raw hidden-columns state + persistence only; callers add their own side effects (reload data, etc). */
  const toggleColumnVisibility = (key: string) => {
    if (hiddenColumns.has(key)) {
      hiddenColumns.delete(key)
    } else {
      hiddenColumns.add(key)
    }
    saveColumnsToStorage()
  }

  if (typeof window !== 'undefined') {
    loadSavedColumns()
  }

  return {
    hiddenColumns,
    isColumnVisible,
    toggleColumnVisibility
  }
}
