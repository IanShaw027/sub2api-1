import type { GatewayModelItem } from './types'

export const GEMINI_IMAGE_ASPECT_RATIOS = ['1:1', '3:2', '16:9', '9:16', '4:3', '3:4']

export function isGeminiImageModel(modelId: string): boolean {
  return /^gemini-3[\w.-]*image/i.test(modelId.trim())
}

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
