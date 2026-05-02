export type SkillType = 'prompt_chat' | 'prompt_image' | 'script'
export type SkillVisibility = 'public' | 'private'
export type SkillStatus = 'draft' | 'published' | 'archived' | 'hidden'
export type SkillPriceMode = 'free' | 'paid'
export type SkillVariableType = 'string' | 'text' | 'number' | 'boolean' | 'select' | 'json' | 'image' | 'file'
export type SkillRunStatus = 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
export type SkillVersionStatus = 'draft' | 'published' | 'deprecated' | 'archived'
export type SkillVersionReviewStatus = 'draft' | 'pending' | 'approved' | 'rejected'
export type SkillRevenueOrderStatus = 'pending' | 'paid' | 'refunded' | 'settled'
export type SkillSortKey = 'latest' | 'popular' | 'revenue' | 'runs' | 'price_low' | 'price_high'
export type SkillRunMode = 'test' | 'use'
export type SkillInstallFilter = 'all' | 'installed' | 'not_installed'

export interface SkillVariableOption {
  label: string
  value: string
  description?: string | null
}

export interface SkillVariableSchemaItem {
  key: string
  label: string
  type: SkillVariableType
  required: boolean
  description?: string | null
  placeholder?: string | null
  default_value?: string | number | boolean | null
  options?: SkillVariableOption[]
}

export interface SkillPromptChatContent {
  type: 'prompt_chat'
  system_prompt: string
  user_prompt_template: string
  assistant_prefill?: string | null
  model?: string | null
  temperature?: number | null
  max_tokens?: number | null
}

export interface SkillPromptImageContent {
  type: 'prompt_image'
  prompt_template: string
  negative_prompt_template?: string | null
  style?: string | null
  size?: string | null
  quality?: string | null
  image_count?: number | null
}

export interface SkillScriptContent {
  type: 'script'
  language: string
  runtime?: string | null
  entrypoint?: string | null
  source_code: string
  dependencies: string[]
  timeout_seconds?: number | null
}

export type SkillContent = SkillPromptChatContent | SkillPromptImageContent | SkillScriptContent

export interface SkillAuthor {
  id: number | null
  name: string | null
  avatar_url?: string | null
}

export interface SkillPricing {
  mode: SkillPriceMode
  amount: number
  currency: string
  settlement_ratio?: number | null
}

export interface SkillStats {
  installs: number
  runs: number
  revenue: number
  rating: number | null
  versions: number
}

export interface SkillVersionSummary {
  id: number
  skill_id: number | null
  version: string
  status: SkillVersionStatus
  review_status: SkillVersionReviewStatus
  changelog: string
  source_locked: boolean
  is_current: boolean
  created_at: string
  published_at?: string | null
  submitted_at?: string | null
  reviewed_at?: string | null
  review_note?: string | null
  can_submit_review: boolean
  can_publish: boolean
  can_test: boolean
  can_use: boolean
}

export interface SkillSummary {
  id: number
  slug: string
  name: string
  tagline: string
  description: string
  type: SkillType
  visibility: SkillVisibility
  status: SkillStatus
  category: string | null
  tags: string[]
  cover_image_url: string | null
  pricing: SkillPricing
  source_locked: boolean
  can_view_source: boolean
  installed: boolean
  owned: boolean
  editable: boolean
  author: SkillAuthor
  stats: SkillStats
  latest_version: SkillVersionSummary | null
  current_version: SkillVersionSummary | null
  created_at: string
  updated_at: string
}

export interface SkillDetail extends SkillSummary {
  variable_schema: SkillVariableSchemaItem[]
  content: SkillContent | null
  metadata: Record<string, unknown>
  examples: string[]
  readme: string | null
  install_note: string | null
  can_install: boolean
  can_run: boolean
}

export interface SkillMarketFilters {
  search: string
  type: SkillType | 'all'
  price_mode: SkillPriceMode | 'all'
  installed: SkillInstallFilter
  category: string | 'all'
  sort: SkillSortKey
}

export interface SkillMineFilters {
  search: string
  type: SkillType | 'all'
  visibility: SkillVisibility | 'all'
  status: SkillStatus | 'all'
  sort: SkillSortKey
}

export interface SkillRunFilters {
  search: string
  status: SkillRunStatus | 'all'
  version_id: number | 'all'
}

export interface SkillEditorDraft {
  id?: number
  slug: string
  name: string
  tagline: string
  description: string
  type: SkillType
  visibility: SkillVisibility
  status: SkillStatus
  category: string
  cover_image_url: string
  tags: string[]
  pricing: SkillPricing
  source_locked: boolean
  variable_schema: SkillVariableSchemaItem[]
  content: SkillContent
  readme: string
  install_note: string
}

export interface CreateSkillRequest {
  slug: string
  name: string
  tagline?: string
  description?: string
  type: SkillType
  visibility: SkillVisibility
  status?: SkillStatus
  category?: string | null
  cover_image_url?: string | null
  tags?: string[]
  pricing?: Partial<SkillPricing>
  source_locked?: boolean
  variable_schema?: SkillVariableSchemaItem[]
  content: SkillContent
  readme?: string | null
  install_note?: string | null
}

export type UpdateSkillRequest = Partial<CreateSkillRequest>

export interface CreateSkillVersionRequest {
  version: string
  status?: SkillVersionStatus
  changelog?: string
  source_locked?: boolean
  variable_schema?: SkillVariableSchemaItem[]
  content?: SkillContent | null
}

export interface SkillVersionRecord extends SkillVersionSummary {
  variable_schema: SkillVariableSchemaItem[]
  content: SkillContent | null
  metadata: Record<string, unknown>
}

export interface SkillRunActionResult {
  mode: SkillRunMode
  skill_id: number
  version_id: number | null
  run_id: number | null
  status: string | null
  raw: Record<string, unknown>
}

export interface SkillVersionPublishResult {
  published: boolean
  skill_id: number | null
  version_id: number
  version: SkillVersionRecord | null
  raw: Record<string, unknown>
}

export interface SkillRunRecord {
  id: number
  skill_id: number
  skill_name: string
  version_id: number | null
  version: string | null
  status: SkillRunStatus
  trigger: string | null
  input_preview: string | null
  output_preview: string | null
  error_message: string | null
  duration_ms: number | null
  cost: number | null
  currency: string
  created_at: string
  started_at: string | null
  finished_at: string | null
}

export interface SkillRevenueSummary {
  total_revenue: number
  total_sales: number
  total_runs: number
  pending_amount: number
  settled_amount: number
  refunded_amount: number
  currency: string
}

export interface SkillRevenuePoint {
  date: string
  revenue: number
  sales: number
  runs: number
}

export interface SkillRevenueOrder {
  id: number
  buyer_name: string | null
  version: string | null
  amount: number
  currency: string
  status: SkillRevenueOrderStatus
  created_at: string
}

export interface SkillRevenueDetail {
  summary: SkillRevenueSummary
  trend: SkillRevenuePoint[]
  orders: SkillRevenueOrder[]
}

export function createDefaultSkillContent(type: SkillType): SkillContent {
  switch (type) {
    case 'prompt_image':
      return {
        type,
        prompt_template: '',
        negative_prompt_template: '',
        style: '',
        size: '1024x1024',
        quality: '',
        image_count: 1
      }
    case 'script':
      return {
        type,
        language: 'javascript',
        runtime: 'node20',
        entrypoint: 'main.mjs',
        source_code: '',
        dependencies: [],
        timeout_seconds: 30
      }
    case 'prompt_chat':
    default:
      return {
        type: 'prompt_chat',
        system_prompt: '',
        user_prompt_template: '',
        assistant_prefill: '',
        model: '',
        temperature: 0.7,
        max_tokens: 2048
      }
  }
}

export function createEmptySkillDraft(type: SkillType = 'prompt_chat'): SkillEditorDraft {
  return {
    slug: '',
    name: '',
    tagline: '',
    description: '',
    type,
    visibility: 'private',
    status: 'draft',
    category: '',
    cover_image_url: '',
    tags: [],
    pricing: {
      mode: 'free',
      amount: 0,
      currency: 'CNY',
      settlement_ratio: null
    },
    source_locked: false,
    variable_schema: [],
    content: createDefaultSkillContent(type),
    readme: '',
    install_note: ''
  }
}
