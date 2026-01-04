/**
 * OpsDashboard 组件共享类型定义
 */

import type { OpsSeverity } from '@/api/admin/ops'

export type ChartState = 'loading' | 'empty' | 'ready'

export interface ErrorFilters {
  platforms: string[]
  statusCodes: number[]
  clientIp: string
  severity: OpsSeverity | ''
  searchText: string
}

export interface ErrorLogsPagination {
  page: number
  pageSize: number
}
