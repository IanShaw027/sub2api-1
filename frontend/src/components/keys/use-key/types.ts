import type { Component } from 'vue'

export interface TabConfig {
  id: string
  label: string
  icon: Component
}

export interface FileConfig {
  path: string
  content: string
  hint?: string // Optional hint message for this file
  highlighted?: string
}

export type CodexAuthMode = 'legacy' | 'api-key'

export type CodexModelManifestState = 'idle' | 'loading' | 'ready' | 'error'
