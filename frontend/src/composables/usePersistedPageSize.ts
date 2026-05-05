import { getConfiguredTableDefaultPageSize, normalizeTablePageSize } from '@/utils/tablePreferences'

const STORAGE_KEY = 'table-page-size'

/**
 * 读取当前系统配置的表格默认每页条数。
 * 不再使用本地持久化缓存，所有页面统一以通用表格设置为准。
 */
export function getPersistedPageSize(fallback = getConfiguredTableDefaultPageSize()): number {
  return normalizeTablePageSize(getConfiguredTableDefaultPageSize() || fallback)
}

export function setPersistedPageSize(size: number): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, String(size))
  } catch (error) {
    console.warn('Failed to persist page size:', error)
  }
}
