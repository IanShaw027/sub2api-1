import { reactive } from 'vue'

// Column-visibility persistence for the admin accounts table (glass-ui-redesign 8.4 / frontend-health-cleanup 6.4).
// Extracted from AccountsView.vue: owns the hidden-columns Set plus its versioned
// localStorage migration so existing admins get the new prototype-04 default
// column set exactly once, without losing any custom column choices they made.
const DEFAULT_HIDDEN_COLUMNS = [
  'id',
  'groups',
  'proxy',
  'notes',
  'scheduler_score',
  'rate_multiplier',
  'upstream_billing_rate',
  'created_at',
  'expires_at'
]

const HIDDEN_COLUMNS_KEY = 'account-hidden-columns'
// One-time migration: hide scheduler score for existing admins too, because showing it opt-ins to heavy backend scoring.
const HIDDEN_COLUMNS_VERSION_KEY = 'account-hidden-columns-version'
const HIDDEN_COLUMNS_CURRENT_VERSION = 'glass-04-default-columns'

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
        // Older saved column layouts may have scheduler_score/upstream_billing_rate/created_at visible;
        // migrate them to the new prototype-04 default once.
        const savedVersion = localStorage.getItem(HIDDEN_COLUMNS_VERSION_KEY)
        if (savedVersion !== HIDDEN_COLUMNS_CURRENT_VERSION) {
          // The older 'scheduler-score-hidden-by-default' marker means this browser already went
          // through the scheduler-score migration once; if the admin explicitly re-showed it since
          // then (it's absent from their saved hidden set), respect that instead of re-hiding it.
          if (savedVersion !== 'scheduler-score-hidden-by-default') {
            hiddenColumns.add('scheduler_score')
          }
          hiddenColumns.add('upstream_billing_rate')
          hiddenColumns.add('created_at')
          localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
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
