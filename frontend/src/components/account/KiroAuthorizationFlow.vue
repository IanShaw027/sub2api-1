<template>
  <div class="rounded-lg border border-cyan-200 bg-cyan-50/70 p-4 dark:border-cyan-900/40 dark:bg-cyan-950/20">
    <div class="mb-4 flex items-start gap-3">
      <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-cyan-500 text-white">
        <Icon name="link" size="md" />
      </div>
      <div>
        <h4 class="font-semibold text-cyan-950 dark:text-cyan-100">
          {{ title }}
        </h4>
        <p class="mt-1 text-sm text-cyan-800 dark:text-cyan-200">
          {{ description }}
        </p>
      </div>
    </div>

    <div class="space-y-4">
      <p class="text-sm text-cyan-800 dark:text-cyan-300">
        {{ t('admin.accounts.kiro.followSteps') }}
      </p>

      <div class="rounded-lg border border-cyan-300 bg-white/80 p-4 dark:border-cyan-700 dark:bg-gray-800/80">
        <div class="flex items-start gap-3">
          <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-cyan-600 text-xs font-bold text-white">
            1
          </div>
          <div class="flex-1">
            <p class="mb-2 font-medium text-cyan-950 dark:text-cyan-100">
              {{ t('admin.accounts.kiro.step1GenerateUrl') }}
            </p>
            <button
              v-if="!authUrl"
              type="button"
              :disabled="loading"
              class="btn btn-primary text-sm"
              @click="emit('generate-url')"
            >
              <svg
                v-if="loading"
                class="-ml-1 mr-2 h-4 w-4 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <Icon v-else name="link" size="sm" class="mr-2" />
              {{ loading ? t('admin.accounts.oauth.generating') : t('admin.accounts.oauth.generateAuthUrl') }}
            </button>
            <div v-else class="space-y-3">
              <div class="flex items-center gap-2">
                <input
                  :value="authUrl"
                  readonly
                  type="text"
                  class="input flex-1 bg-gray-50 font-mono text-xs dark:bg-gray-700"
                />
                <button
                  type="button"
                  class="btn btn-secondary p-2"
                  title="Copy URL"
                  @click="copyToClipboard(authUrl, 'URL copied to clipboard')"
                >
                  <svg
                    v-if="!copied"
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.5"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184"
                    />
                  </svg>
                  <Icon
                    v-else
                    name="check"
                    size="sm"
                    class="text-green-500"
                    :stroke-width="2"
                  />
                </button>
              </div>
              <p class="text-xs text-cyan-700 dark:text-cyan-300">
                {{ t('admin.accounts.kiro.callbackBaseUrlHint', { value: callbackBaseUrl || t('admin.accounts.kiro.optionalPlaceholder') }) }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <div class="rounded-lg border border-cyan-300 bg-white/80 p-4 dark:border-cyan-700 dark:bg-gray-800/80">
        <div class="flex items-start gap-3">
          <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-cyan-600 text-xs font-bold text-white">
            2
          </div>
          <div class="flex-1">
            <p class="mb-2 font-medium text-cyan-950 dark:text-cyan-100">
              {{ t('admin.accounts.kiro.step2Authorize') }}
            </p>
            <p class="text-sm text-cyan-700 dark:text-cyan-300">
              {{ t('admin.accounts.kiro.step2AuthorizeHint') }}
            </p>
          </div>
        </div>
      </div>

      <div class="rounded-lg border border-cyan-300 bg-white/80 p-4 dark:border-cyan-700 dark:bg-gray-800/80">
        <div class="flex items-start gap-3">
          <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-cyan-600 text-xs font-bold text-white">
            3
          </div>
          <div class="flex-1 space-y-3">
            <div>
              <p class="mb-2 font-medium text-cyan-950 dark:text-cyan-100">
                {{ t('admin.accounts.kiro.step3PasteCallback') }}
              </p>
              <p class="mb-3 text-sm text-cyan-700 dark:text-cyan-300">
                {{ t('admin.accounts.kiro.callbackUrlHint') }}
              </p>
              <textarea
                v-model="callbackUrl"
                rows="4"
                class="input w-full resize-y font-mono text-sm"
                :placeholder="t('admin.accounts.kiro.callbackUrlPlaceholder')"
              />
            </div>

            <details class="rounded-lg border border-cyan-200/80 bg-white/70 p-3 dark:border-cyan-900/50 dark:bg-black/10">
              <summary class="cursor-pointer text-sm font-medium text-cyan-900 dark:text-cyan-100">
                {{ t('admin.accounts.kiro.advancedFieldsTitle') }}
              </summary>
              <div class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
                <p class="md:col-span-2 text-sm text-cyan-700 dark:text-cyan-300">
                  Kiro version、system version、Node.js version 等运行参数由系统配置统一管理。
                </p>
                <div>
                  <label class="input-label">{{ t('admin.accounts.kiro.regionLabel') }}</label>
                  <input
                    v-model="region"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.accounts.kiro.regionPlaceholder')"
                  />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.accounts.kiro.authRegionLabel') }}</label>
                  <input
                    v-model="authRegion"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
                  />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.accounts.kiro.apiRegionLabel') }}</label>
                  <input
                    v-model="apiRegion"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
                  />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.accounts.kiro.profileArnLabel') }}</label>
                  <input
                    v-model="profileARN"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
                  />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.accounts.kiro.machineIdLabel') }}</label>
                  <input
                    v-model="machineID"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
                  />
                </div>
              </div>
            </details>

            <div
              v-if="localError || error"
              class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800/60 dark:bg-red-900/20 dark:text-red-300"
            >
              {{ localError || error }}
            </div>

            <button
              type="button"
              class="btn btn-primary"
              :disabled="loading"
              @click="handleSubmit"
            >
              <svg
                v-if="loading"
                class="-ml-1 mr-2 h-4 w-4 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              {{ loading ? t('admin.accounts.oauth.verifying') : submitLabel }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import type { KiroAccountExtra, KiroCredentials } from '@/types'

interface Props {
  mode?: 'create' | 'reauth'
  loading?: boolean
  error?: string
  authUrl?: string
  callbackBaseUrl?: string
  initialCredentials?: KiroCredentials | null
  initialExtra?: KiroAccountExtra | null
}

const props = withDefaults(defineProps<Props>(), {
  mode: 'create',
  loading: false,
  error: '',
  authUrl: '',
  callbackBaseUrl: '',
  initialCredentials: null,
  initialExtra: null
})

const emit = defineEmits<{
  'generate-url': []
  submit: [payload: {
    callbackUrl: string
    credentials: KiroCredentials & Record<string, unknown>
    extra: KiroAccountExtra & Record<string, unknown>
  }]
}>()

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()

const callbackUrl = ref('')
const profileARN = ref('')
const region = ref('us-east-1')
const authRegion = ref('')
const apiRegion = ref('')
const machineID = ref('')
const localError = ref('')

const title = computed(() => (
  props.mode === 'reauth'
    ? t('admin.accounts.kiro.reauthorizeTitle')
    : t('admin.accounts.kiro.authorizationTitle')
))

const description = computed(() => (
  props.mode === 'reauth'
    ? t('admin.accounts.kiro.reauthorizeDesc')
    : t('admin.accounts.kiro.authorizationDesc')
))

const submitLabel = computed(() => (
  props.mode === 'reauth'
    ? t('admin.accounts.reAuthorize')
    : t('admin.accounts.oauth.completeAuth')
))

const resetForm = () => {
  const credentials = props.initialCredentials || {}
  callbackUrl.value = ''
  profileARN.value = credentials.profile_arn || ''
  region.value = credentials.region || 'us-east-1'
  authRegion.value = credentials.auth_region || ''
  apiRegion.value = credentials.api_region || ''
  machineID.value = credentials.machine_id || ''
  localError.value = ''
}

watch(
  () => [props.initialCredentials, props.mode],
  () => {
    resetForm()
  },
  { immediate: true, deep: true }
)

const handleSubmit = () => {
  localError.value = ''

  if (!props.authUrl.trim()) {
    localError.value = t('admin.accounts.kiro.generateUrlFirst')
    return
  }

  if (!callbackUrl.value.trim()) {
    localError.value = t('admin.accounts.kiro.callbackUrlRequired')
    return
  }

  const credentials: KiroCredentials & Record<string, unknown> = {}
  if (region.value.trim()) credentials.region = region.value.trim()
  if (authRegion.value.trim()) credentials.auth_region = authRegion.value.trim()
  if (apiRegion.value.trim()) credentials.api_region = apiRegion.value.trim()
  if (profileARN.value.trim()) credentials.profile_arn = profileARN.value.trim()
  if (machineID.value.trim()) credentials.machine_id = machineID.value.trim()

  emit('submit', {
    callbackUrl: callbackUrl.value.trim(),
    credentials,
    extra: {}
  })
}
</script>
