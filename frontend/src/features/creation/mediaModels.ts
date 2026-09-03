import type { GatewayModelItem } from './types'

const IMAGE_MODEL_PATTERNS = [
  /^gpt-image/i,
  /^dall-e/i,
  /image/i,
  /^flux/i,
  /^imagen/i,
  /^grok-image/i,
]

export function isImageCapableModel(modelId: string): boolean {
  const id = modelId.trim().toLowerCase()
  if (!id) return false
  return IMAGE_MODEL_PATTERNS.some((pattern) => pattern.test(id))
}

export function isChatCapableModel(modelId: string): boolean {
  const id = modelId.trim().toLowerCase()
  if (!id) return false
  if (isImageCapableModel(id)) return false
  if (id.includes('embedding')) return false
  if (id.includes('whisper')) return false
  if (id.includes('tts')) return false
  if (id.includes('realtime')) return false
  return true
}

export function classifyModels(models: GatewayModelItem[]): { chat: string[]; image: string[] } {
  const chat: string[] = []
  const image: string[] = []
  const seen = new Set<string>()

  for (const model of models) {
    const id = String(model.id || '').trim()
    if (!id || seen.has(id)) continue
    seen.add(id)
    if (isImageCapableModel(id)) {
      image.push(id)
    } else if (isChatCapableModel(id)) {
      chat.push(id)
    }
  }

  return { chat, image }
}

export function pickDefaultModel(models: string[], preferred?: string): string {
  if (preferred && models.includes(preferred)) return preferred
  return models[0] ?? ''
}
