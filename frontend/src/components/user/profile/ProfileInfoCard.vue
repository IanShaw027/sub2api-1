<template>
 <SettingsSection
 :title="t('profile.basicsTitle')"
 :description="t('profile.basicsDescription')"
 >
 <SettingRow
 :label="t('profile.avatar.title')"
 :description="t('profile.avatar.uploadHint')"
 >
 <ProfileAvatarCard
 :user="user"
 embedded
 />
 </SettingRow>

 <SettingRow :label="t('profile.username')">
 <ProfileEditForm
 :initial-username="user?.username || ''"
 embedded
 />
 </SettingRow>

 <SettingRow :label="t('profile.email')">
 <div class="space-y-2">
 <p class="text-sm text-foreground">
 {{ primaryEmailDisplay || t('profile.authBindings.status.notBound') }}
 </p>
 <div
 v-if="sourceHints.length"
 class="flex flex-wrap gap-2 text-xs text-muted"
 >
 <span
 v-for="hint in sourceHints"
 :key="hint.key"
 class="inline-flex items-center gap-1 rounded-full bg-surface-2 px-3 py-1"
 >
 <Icon name="link" size="sm" />
 {{ hint.text }}
 </span>
 </div>
 </div>
 </SettingRow>
 </SettingsSection>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import SettingsSection from '@/components/ui/SettingsSection.vue'
import SettingRow from '@/components/ui/SettingRow.vue'
import ProfileAvatarCard from '@/components/user/profile/ProfileAvatarCard.vue'
import ProfileEditForm from '@/components/user/profile/ProfileEditForm.vue'
import type { User, UserAuthBindingStatus, UserAuthProvider, UserProfileSourceContext } from '@/types'

const props = withDefaults(defineProps<{
 user: User | null
 oidcProviderName?: string
}>(), {
 oidcProviderName: 'OIDC',
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

const providerLabels = computed<Record<UserAuthProvider, string>>(() => ({
 email: t('profile.authBindings.providers.email'),
 linuxdo: t('profile.authBindings.providers.linuxdo'),
 dingtalk: t('profile.authBindings.providers.dingtalk'),
 oidc: t('profile.authBindings.providers.oidc', { providerName: props.oidcProviderName }),
 wechat: t('profile.authBindings.providers.wechat'),
 github: 'GitHub',
 google: 'Google'
}))

function normalizeProvider(value: string): UserAuthProvider | null {
 const normalized = value.trim().toLowerCase()
 if (
 normalized === 'email' ||
 normalized === 'linuxdo' ||
 normalized === 'wechat' ||
 normalized === 'github' ||
 normalized === 'google'
 ) {
 return normalized
 }
 if (normalized === 'oidc' || normalized.startsWith('oidc:') || normalized.startsWith('oidc/')) {
 return 'oidc'
 }
 return null
}

function readObjectString(source: Record<string, unknown>, ...keys: string[]): string {
 for (const key of keys) {
 const value = source[key]
 if (typeof value === 'string' && value.trim()) {
 return value.trim()
 }
 }
 return ''
}

function resolveThirdPartySource(
 rawSource: string | UserProfileSourceContext | null | undefined
): { provider: UserAuthProvider; label: string } | null {
 if (!rawSource) {
 return null
 }

 if (typeof rawSource === 'string') {
 const provider = normalizeProvider(rawSource)
 if (!provider || provider === 'email') {
 return null
 }
 return {
 provider,
 label: providerLabels.value[provider]
 }
 }

 const sourceRecord = rawSource as Record<string, unknown>
 const provider = normalizeProvider(
 readObjectString(sourceRecord, 'provider', 'source', 'provider_type', 'auth_provider')
 )
 if (!provider || provider === 'email') {
 return null
 }

 const explicitLabel = readObjectString(
 sourceRecord,
 'provider_label',
 'label',
 'provider_name',
 'providerName'
 )

 return {
 provider,
 label: explicitLabel || providerLabels.value[provider]
 }
}

const sourceHints = computed(() => {
 const currentUser = props.user
 if (!currentUser) {
 return []
 }

 const hints: Array<{ key: string; text: string }> = []
 const avatarSource = resolveThirdPartySource(
 currentUser.profile_sources?.avatar ?? currentUser.avatar_source
 )
 const usernameSource = resolveThirdPartySource(
 currentUser.profile_sources?.username ??
 currentUser.profile_sources?.display_name ??
 currentUser.profile_sources?.nickname ??
 currentUser.display_name_source ??
 currentUser.username_source ??
 currentUser.nickname_source
 )

 if (avatarSource) {
 hints.push({
 key: 'avatar',
 text: t('profile.authBindings.source.avatar', { providerName: avatarSource.label })
 })
 }

 if (usernameSource) {
 hints.push({
 key: 'username',
 text: t('profile.authBindings.source.username', { providerName: usernameSource.label })
 })
 }

 return hints
})
</script>
