<template>
  <div class="space-y-6">
    <section
      data-testid="profile-overview-hero"
      class="card overflow-hidden border border-brand-100/80 bg-gradient-to-br from-brand-50 via-white to-amber-50/70 dark:border-brand-900/40 dark:from-brand-950/40 dark:via-dark-900 dark:to-dark-950"
    >
      <div class="px-6 py-6 md:px-8">
        <div class="flex flex-col gap-6 lg:flex-row lg:items-start">
          <div
            class="flex h-20 w-20 shrink-0 items-center justify-center overflow-hidden rounded-[1.75rem] bg-gradient-to-br from-brand-500 to-brand-600 text-2xl font-bold text-white shadow-lg shadow-brand-500/20"
          >
            <img
              v-if="avatarUrl"
              :src="avatarUrl"
              :alt="displayName"
              class="h-full w-full object-cover"
              referrerpolicy="no-referrer"
              loading="lazy"
            >
            <span v-else>{{ avatarInitial }}</span>
          </div>

          <div class="min-w-0 flex-1 space-y-5">
            <div class="space-y-3">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="truncate text-2xl font-semibold text-ink dark:text-white">
                  {{ displayName }}
                </h2>
                <span :class="['badge', user?.role === 'admin' ? 'badge-primary' : 'badge-gray']">
                  {{ user?.role === 'admin' ? t('profile.administrator') : t('profile.user') }}
                </span>
                <span
                  :class="['badge', user?.status === 'active' ? 'badge-success' : 'badge-danger']"
                >
                  {{
                    user?.status === 'active'
                      ? t('common.active')
                      : t('common.disabled')
                  }}
                </span>
              </div>

              <div class="space-y-1">
                <p class="truncate text-sm text-ink-body">
                  {{ primaryEmailDisplay }}
                </p>
                <div v-if="profileSourceHints.length" data-testid="profile-source-hints" class="flex flex-wrap gap-2 pt-1">
                  <span
                    v-for="hint in profileSourceHints"
                    :key="hint"
                    class="rounded-full bg-brand-50 px-2.5 py-1 text-xs font-medium text-brand-700 ring-1 ring-brand-100 dark:bg-brand-950/40 dark:text-brand-300 dark:ring-brand-900/50"
                  >
                    {{ hint }}
                  </span>
                </div>
              </div>
            </div>

            <div class="grid gap-3 sm:grid-cols-3">
              <button
                data-testid="profile-overview-metric-balance"
                type="button"
                class="rounded-card bg-white/85 px-4 py-3 text-left shadow-xs ring-1 ring-white/70 transition-colors hover:ring-brand-200 dark:bg-dark-900/60 dark:ring-dark-700 dark:hover:ring-brand-800/50"
                @click="emit('balance-history')"
              >
                <p class="text-xs font-medium uppercase tracking-[0.16em] text-ink-faint">
                  {{ t('profile.accountBalance') }}
                </p>
                <p class="mt-1 text-lg font-semibold text-ink underline decoration-dashed decoration-line underline-offset-4 transition-colors hover:text-brand-600 dark:text-white dark:decoration-dark-500 dark:hover:text-brand-400">
                  {{ formatCurrency(user?.balance || 0) }}
                </p>
              </button>
              <div
                data-testid="profile-overview-metric-concurrency"
                class="rounded-card bg-white/85 px-4 py-3 shadow-xs ring-1 ring-white/70 dark:bg-dark-900/60 dark:ring-dark-700"
              >
                <p class="text-xs font-medium uppercase tracking-[0.16em] text-ink-faint">
                  {{ t('profile.concurrencyLimit') }}
                </p>
                <p class="mt-1 text-lg font-semibold text-ink dark:text-white">
                  {{ user?.concurrency || 0 }}
                </p>
              </div>
              <div
                data-testid="profile-overview-metric-member-since"
                class="rounded-card bg-white/85 px-4 py-3 shadow-xs ring-1 ring-white/70 dark:bg-dark-900/60 dark:ring-dark-700"
              >
                <p class="text-xs font-medium uppercase tracking-[0.16em] text-ink-faint">
                  {{ t('profile.memberSince') }}
                </p>
                <p class="mt-1 text-lg font-semibold text-ink dark:text-white">
                  {{ memberSinceLabel }}
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <div data-testid="profile-main-column" class="space-y-6">
      <section
        data-testid="profile-basics-panel"
        class="card border border-line bg-white/90 p-6 dark:border-dark-700 dark:bg-dark-900/50"
      >
        <div class="mb-5 flex items-start justify-between gap-4">
          <div>
            <h3 class="text-lg font-semibold text-ink dark:text-white">
              {{ t('profile.basicsTitle') }}
            </h3>
            <p class="mt-1 text-sm text-ink-soft">
              {{ t('profile.basicsDescription') }}
            </p>
          </div>
        </div>

        <div class="grid gap-6 sm:grid-cols-1 md:grid-cols-2">
          <div class="rounded-3xl border border-line bg-page/80 p-5 dark:border-dark-700 dark:bg-dark-900/30">
            <ProfileAvatarCard
              :user="user"
              embedded
            />
          </div>

          <div class="rounded-3xl border border-line bg-page/80 p-5 dark:border-dark-700 dark:bg-dark-900/30">
            <ProfileEditForm
              :initial-username="user?.username || ''"
              embedded
            />
          </div>
        </div>
      </section>

      <section
        data-testid="profile-auth-bindings-panel"
        class="card border border-line bg-white/90 p-6 dark:border-dark-700 dark:bg-dark-900/50"
      >
        <ProfileIdentityBindingsSection
          :user="user"
          :linuxdo-enabled="linuxdoEnabled"
          :dingtalk-enabled="dingtalkEnabled"
          :oidc-enabled="oidcEnabled"
          :oidc-provider-name="oidcProviderName"
          :wechat-enabled="wechatEnabled"
          :wechat-open-enabled="wechatOpenEnabled"
          :wechat-mp-enabled="wechatMpEnabled"
          embedded
          compact
        />
      </section>

      <section
        v-if="contactInfoDisplay || supportQRCodeItems.length > 0"
        data-testid="profile-support-panel"
        class="card border border-line bg-white/90 p-6 dark:border-dark-700 dark:bg-dark-900/50"
      >
        <div class="space-y-4">
          <div>
            <h3 class="text-lg font-semibold text-ink dark:text-white">
              {{ t('common.contactSupport') }}
            </h3>
            <p
              v-if="contactInfoDisplay"
              data-testid="profile-support-contact"
              class="mt-1 whitespace-pre-wrap text-sm text-ink-body"
            >
              {{ contactInfoDisplay }}
            </p>
          </div>

          <div
            v-if="supportQRCodeItems.length > 0"
            data-testid="profile-support-qr-grid"
            class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
          >
            <div
              v-for="(qrCode, index) in supportQRCodeItems"
              :key="`${qrCode.image_url}-${index}`"
              :aria-label="supportQRCodeAlt(qrCode)"
              class="overflow-hidden rounded-3xl border border-line bg-page/80 p-4 dark:border-dark-700 dark:bg-dark-900/30"
              role="group"
            >
              <img
                :src="qrCode.image_url"
                :alt="supportQRCodeAlt(qrCode)"
                :aria-label="supportQRCodeAlt(qrCode)"
                class="aspect-square w-full rounded-card object-cover"
              />
              <p
                v-if="qrCode.note"
                class="mt-2 text-xs text-ink-soft"
              >
                {{ qrCode.note }}
              </p>
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ProfileAvatarCard from '@/components/user/profile/ProfileAvatarCard.vue'
import { safeImageUrl } from '@/utils/safeImageUrl'
import ProfileEditForm from '@/components/user/profile/ProfileEditForm.vue'
import ProfileIdentityBindingsSection from '@/components/user/profile/ProfileIdentityBindingsSection.vue'
import type { SupportQRCodeEntry, User, UserAuthBindingStatus, UserAuthProvider, UserProfileSourceContext } from '@/types'

const emit = defineEmits<{
  'balance-history': []
}>()

const props = withDefaults(defineProps<{
  user: User | null
  linuxdoEnabled?: boolean
  dingtalkEnabled?: boolean
  oidcEnabled?: boolean
  oidcProviderName?: string
  contactInfo?: string
  supportQRCodes?: SupportQRCodeEntry[]
  wechatEnabled?: boolean
  wechatOpenEnabled?: boolean
  wechatMpEnabled?: boolean
}>(), {
  linuxdoEnabled: false,
  dingtalkEnabled: false,
  oidcEnabled: false,
  oidcProviderName: 'OIDC',
  contactInfo: '',
  supportQRCodes: () => [],
  wechatEnabled: false,
  wechatOpenEnabled: undefined,
  wechatMpEnabled: undefined,
})

const { t } = useI18n()

function normalizeBindingStatus(binding: boolean | UserAuthBindingStatus | undefined): boolean | null {
  if (typeof binding === 'boolean') {
    return binding
  }
  if (!binding) {
    return null
  }
  if (typeof binding.bound === 'boolean') {
    return binding.bound
  }
  return Boolean(binding.provider_subject || binding.issuer || binding.provider_key)
}

function isEmailBound(user: User | null | undefined): boolean {
  if (typeof user?.email_bound === 'boolean') {
    return user.email_bound
  }

  const nested = user?.auth_bindings?.email ?? user?.identity_bindings?.email
  const normalized = normalizeBindingStatus(nested)
  return normalized ?? false
}

const avatarUrl = computed(() => safeImageUrl(props.user?.avatar_url))
const displayName = computed(() => props.user?.username?.trim() || props.user?.email?.trim() || t('profile.user'))
const contactInfoDisplay = computed(() => props.contactInfo?.trim() || '')
const supportQRCodeItems = computed(() => (props.supportQRCodes || []).filter((entry) => entry?.image_url?.trim()))
const primaryEmailDisplay = computed(() => {
  const email = props.user?.email?.trim() || ''
  if (!email) {
    return ''
  }
  if (email.endsWith('.invalid') && !isEmailBound(props.user)) {
    return ''
  }
  return email
})
const avatarInitial = computed(() => displayName.value.charAt(0).toUpperCase() || 'U')
const profileSourceHints = computed(() => {
  const hints: string[] = []
  const avatarProvider = resolveProfileSourceProvider(props.user?.profile_sources?.avatar ?? props.user?.avatar_source)
  const usernameProvider = resolveProfileSourceProvider(
    props.user?.profile_sources?.username
      ?? props.user?.profile_sources?.display_name
      ?? props.user?.profile_sources?.nickname
      ?? props.user?.username_source
      ?? props.user?.display_name_source
      ?? props.user?.nickname_source,
  )
  if (avatarProvider) {
    hints.push(t('profile.identity.source.avatar', { providerName: avatarProvider }))
  }
  if (usernameProvider) {
    hints.push(t('profile.identity.source.username', { providerName: usernameProvider }))
  }
  return hints
})
const memberSinceLabel = computed(() => {
  const raw = props.user?.created_at?.trim()
  if (!raw) {
    return '-'
  }

  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }

  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
  }).format(date)
})

const providerLabels = computed<Record<UserAuthProvider, string>>(() => ({
  email: t('profile.authBindings.providers.email'),
  linuxdo: t('profile.authBindings.providers.linuxdo'),
  oidc: t('profile.authBindings.providers.oidc', { providerName: props.oidcProviderName }),
  wechat: t('profile.authBindings.providers.wechat'),
  dingtalk: t('profile.authBindings.providers.dingtalk'),
  github: t('profile.authBindings.providers.github'),
  google: t('profile.authBindings.providers.google')
}))
function formatCurrency(value: number): string {
  return `$${value.toFixed(2)}`
}

function resolveProfileSourceProvider(source: string | UserProfileSourceContext | null | undefined): string {
  if (!source) return ''
  if (typeof source === 'string') {
    const normalized = normalizeProvider(source)
    if (normalized === 'email') {
      return ''
    }
    return normalized ? providerLabels.value[normalized] : formatProviderLabel(source)
  }
  const normalized = normalizeProvider(source.provider || source.source || '')
  if (normalized === 'email') {
    return ''
  }
  const explicitLabel =
    formatProviderLabel(source.provider_label || '') ||
    formatProviderLabel(source.label || '')
  if (explicitLabel) {
    return explicitLabel
  }
  if (normalized) {
    return providerLabels.value[normalized]
  }

  return formatProviderLabel(source.provider || source.source || '')
}

function normalizeProvider(value: string): UserAuthProvider | null {
  const trimmed = value.trim().toLowerCase()
  if (trimmed.startsWith('oidc:') || trimmed.startsWith('oidc/')) {
    return 'oidc'
  }

  const normalized = trimmed.replace(/[\s-]+/g, '_')
  if (normalized === 'oidc_connect' || normalized === 'oidcconnect') {
    return 'oidc'
  }
  if (
    normalized === 'email' ||
    normalized === 'linuxdo' ||
    normalized === 'oidc' ||
    normalized === 'wechat' ||
    normalized === 'dingtalk' ||
    normalized === 'github' ||
    normalized === 'google'
  ) {
    return normalized
  }
  return null
}

function formatProviderLabel(provider: string): string {
  const normalized = provider.trim().toLowerCase().replace(/[\s-]+/g, '_')
  if (!normalized) return ''
  const providerAlias = normalizeProvider(provider)
  if (providerAlias) return providerLabels.value[providerAlias]
  if (isInternalProfileSourceSentinel(normalized)) return ''
  if (normalized === 'oidc' || normalized === 'oidc_connect' || normalized === 'oidcconnect') return props.oidcProviderName || 'OIDC'
  if (normalized === 'linuxdo') return 'LinuxDo'
  if (normalized === 'wechat') return 'WeChat'
  if (normalized === 'email') return ''
  return provider.trim()
}

function isInternalProfileSourceSentinel(normalized: string): boolean {
  return normalized === 'remote_url' || normalized === 'media'
}

function supportQRCodeAlt(qrCode: SupportQRCodeEntry): string {
  const note = qrCode.note?.trim()
  const label = note || t('common.contactSupport')
  return `${label} QR code`
}
</script>
