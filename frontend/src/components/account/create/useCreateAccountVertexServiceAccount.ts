import { ref } from 'vue'

export interface CreateAccountVertexServiceAccountDeps {
  appStore: { showError: (message: string) => void }
  t: (key: string, params?: Record<string, unknown>) => string
}

export function useCreateAccountVertexServiceAccount(deps: CreateAccountVertexServiceAccountDeps) {
  const { appStore, t } = deps

  const vertexServiceAccountJson = ref('')
  const vertexProjectId = ref('')
  const vertexClientEmail = ref('')
  const vertexLocation = ref('global')

  const applyVertexServiceAccountJson = (value: string) => {
    const raw = value.trim()
    if (!raw) {
      vertexProjectId.value = ''
      vertexClientEmail.value = ''
      return false
    }
    try {
      const parsed = JSON.parse(raw) as Record<string, unknown>
      const projectId = typeof parsed.project_id === 'string' ? parsed.project_id.trim() : ''
      const clientEmail = typeof parsed.client_email === 'string' ? parsed.client_email.trim() : ''
      const privateKey = typeof parsed.private_key === 'string' ? parsed.private_key.trim() : ''
      if (!projectId || !clientEmail || !privateKey) {
        appStore.showError(t('admin.accounts.vertexSaJsonMissingFields'))
        return false
      }
      vertexProjectId.value = projectId
      vertexClientEmail.value = clientEmail
      vertexServiceAccountJson.value = JSON.stringify(parsed)
      return true
    } catch {
      appStore.showError(t('admin.accounts.vertexSaJsonInvalid'))
      return false
    }
  }

  const parseVertexServiceAccountJson = () => applyVertexServiceAccountJson(vertexServiceAccountJson.value)

  return {
    vertexServiceAccountJson,
    vertexProjectId,
    vertexClientEmail,
    vertexLocation,
    applyVertexServiceAccountJson,
    parseVertexServiceAccountJson
  }
}
