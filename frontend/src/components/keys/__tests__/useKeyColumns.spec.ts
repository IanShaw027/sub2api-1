import { computed } from 'vue'
import { beforeEach, describe, expect, it } from 'vitest'
import { useKeyColumns } from '../useKeyColumns'

const columns = computed(() => ['name', 'id', 'key', 'created_at', 'rate_limit', 'last_used_at', 'last_used_ip', 'actions']
  .map(key => ({ key, label: key })))

describe('key column preference preservation', () => {
  beforeEach(() => localStorage.clear())

  it('keeps creation time readable by default', () => {
    const preferences = useKeyColumns(columns)
    preferences.loadSavedColumns()
    expect(preferences.isColumnVisible('created_at')).toBe(true)
    expect(preferences.isColumnVisible('rate_limit')).toBe(true)
  })

  it.each(['1', '3', '4', '5'])('preserves explicit hidden choices from version %s', (version) => {
    localStorage.setItem('api-key-hidden-columns', JSON.stringify(['created_at', 'rate_limit', 'last_used_at']))
    localStorage.setItem('api-key-column-settings-version', version)
    const preferences = useKeyColumns(columns)
    preferences.loadSavedColumns()
    for (const key of ['created_at', 'rate_limit', 'last_used_at']) {
      expect(preferences.isColumnVisible(key), key).toBe(false)
    }
    preferences.toggleColumn('created_at')
    const restored = useKeyColumns(columns)
    restored.loadSavedColumns()
    expect(restored.isColumnVisible('created_at')).toBe(true)
    expect(restored.isColumnVisible('rate_limit')).toBe(false)
  })

  it('ignores obsolete and always-visible entries in stored settings', () => {
    localStorage.setItem('api-key-hidden-columns', JSON.stringify(['name', 'actions', 'removed', 'created_at']))
    localStorage.setItem('api-key-column-settings-version', '4')
    const preferences = useKeyColumns(columns)
    preferences.loadSavedColumns()
    expect(preferences.columns.value.map(column => column.key)).toContain('name')
    expect(preferences.columns.value.map(column => column.key)).toContain('actions')
    expect(JSON.parse(localStorage.getItem('api-key-hidden-columns')!)).toEqual(['created_at'])
  })
})
