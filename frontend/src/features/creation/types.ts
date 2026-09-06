import type { GroupPlatform } from '@/types'

export type CreationSessionMode = 'chat' | 'image'
export type CreationSessionStatus = 'active' | 'archived'
export type CreationMessageRole = 'user' | 'assistant' | 'system'
export type CreationImageJobStatus = 'pending' | 'processing' | 'completed' | 'failed'

export interface CreationSession {
  id: number
  user_id: number
  group_id: number
  title: string
  model: string
  mode: CreationSessionMode
  status: CreationSessionStatus
  metadata?: unknown
  created_at: string
  updated_at: string
}

export interface CreationMessage {
  id: number
  session_id: number
  role: CreationMessageRole
  content: unknown
  model?: string | null
  input_tokens?: number | null
  output_tokens?: number | null
  exchange_request_id?: string
  created_at: string
}

export interface CreationTokenUsage {
  input_tokens?: number
  output_tokens?: number
}

export interface CreationExchangeRequest extends CreationTokenUsage {
  request_id: string
  user_content: string
  assistant_content: string
  model: string
}

export interface CreationExchange {
  user: CreationMessage
  assistant: CreationMessage
}

export interface CreationImageJob {
  id: number
  session_id?: number | null
  user_id: number
  group_id: number
  status: CreationImageJobStatus
  model: string
  prompt: string
  media_asset_id?: number | null
  provider_task_id?: string | null
  error?: string | null
  created_at: string
  updated_at: string
  media_url?: string
}

export interface CreationSessionListResponse {
  items: CreationSession[]
  total: number
  page: number
  page_size: number
}

export interface CreationImageListResponse {
  items: CreationImageJob[]
  total: number
  page: number
  page_size: number
}

export interface GatewayModelItem {
  id: string
  object?: string
  created?: number
  owned_by?: string
}

export interface GatewayModelList {
  object?: string
  data: GatewayModelItem[]
}

export interface AsyncImageTask {
  id?: string
  task_id: string
  object?: string
  status: string
  http_status?: number
  image_url?: string
  result?: unknown
  error?: unknown
  created_at?: number
  completed_at?: number | null
  expires_at?: number
}

export interface PendingSendRequest {
  sessionId: number
  text: string
}

export const ANTHROPIC_STYLE_PLATFORMS: ReadonlySet<GroupPlatform> = new Set([
  'anthropic',
  'antigravity',
  'kiro',
  'gemini',
])
