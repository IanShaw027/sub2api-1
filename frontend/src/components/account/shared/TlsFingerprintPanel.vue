<template>
  <div class="border-t border-line pt-4 space-y-4">
    <div class="rounded-lg border border-line p-4">
      <div class="flex items-center justify-between">
        <div>
          <label class="input-label mb-0">{{ t('admin.accounts.quotaControl.tlsFingerprint.label') }}</label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.quotaControl.tlsFingerprint.hint') }}
          </p>
        </div>
        <InlineToggleSwitch v-model="tlsFingerprintEnabled" />
      </div>
      <div v-if="tlsFingerprintEnabled" class="mt-3 space-y-3">
        <select v-model="tlsFingerprintProfileId" class="input">
          <option :value="null">{{ t('admin.accounts.quotaControl.tlsFingerprint.defaultProfile') }}</option>
          <option v-if="tlsFingerprintProfiles.length > 0" :value="-1">{{ t('admin.accounts.quotaControl.tlsFingerprint.randomProfile') }}</option>
          <option v-for="p in tlsFingerprintProfiles" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
        <select v-model="tlsFingerprintRouterId" class="input">
          <option :value="null">{{ t('admin.accounts.quotaControl.tlsFingerprint.noRouter') }}</option>
          <option v-for="router in tlsFingerprintRouters" :key="router.id" :value="router.id">{{ router.name }}</option>
        </select>
        <select v-model="tlsFingerprintDefaultOS" class="input">
          <option value="">{{ t('admin.accounts.quotaControl.tlsFingerprint.anyDefaultOS') }}</option>
          <option v-for="os in tlsFingerprintOSOptions" :key="os" :value="os">{{ os }}</option>
        </select>
        <div class="space-y-2">
          <div v-for="(row, index) in tlsFingerprintBindingRows" :key="index" class="grid grid-cols-4 gap-2">
            <select v-model="row.os" class="input">
              <option value="">os</option>
              <option v-for="os in tlsFingerprintOSOptions" :key="os" :value="os">{{ os }}</option>
            </select>
            <input v-model="row.client" class="input" placeholder="client" />
            <select v-model="row.protocol" class="input">
              <option value="">protocol</option>
              <option v-for="protocol in tlsFingerprintProtocolOptions" :key="protocol" :value="protocol">{{ protocol }}</option>
            </select>
            <select v-model.number="row.profileId" class="input">
              <option :value="0">—</option>
              <option v-for="p in tlsFingerprintProfiles" :key="p.id" :value="p.id">{{ p.name }}</option>
            </select>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="addTLSFingerprintBindingRow">
            {{ t('admin.accounts.quotaControl.tlsFingerprint.addBinding') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// Shared TLS fingerprint section, used by both CreateAccountModal.vue and
// EditAccountModal.vue. The outer v-if="supportsTLSFingerprint(...)" condition
// stays in each host template, wrapping this component.
import { useI18n } from 'vue-i18n'
import InlineToggleSwitch from '@/components/account/shared/InlineToggleSwitch.vue'

interface TLSFingerprintProfile {
  id: number
  name: string
}

interface TLSFingerprintRouter {
  id: number
  name: string
}

interface TLSFingerprintBindingRow {
  os: string
  client: string
  protocol: string
  profileId: number
}

const { t } = useI18n()

defineProps<{
  tlsFingerprintProfiles: TLSFingerprintProfile[]
  tlsFingerprintRouters: TLSFingerprintRouter[]
}>()

const tlsFingerprintEnabled = defineModel<boolean>('tlsFingerprintEnabled', { required: true })
const tlsFingerprintProfileId = defineModel<number | null>('tlsFingerprintProfileId', { required: true })
const tlsFingerprintRouterId = defineModel<number | null>('tlsFingerprintRouterId', { required: true })
const tlsFingerprintDefaultOS = defineModel<string>('tlsFingerprintDefaultOS', { required: true })
const tlsFingerprintBindingRows = defineModel<TLSFingerprintBindingRow[]>('tlsFingerprintBindingRows', { required: true })

const tlsFingerprintOSOptions = ['windows', 'macos', 'linux', 'ios', 'android']
const tlsFingerprintProtocolOptions = [
  'messages',
  'responses',
  'chat_completions',
  'images',
  'embeddings',
  'gemini',
  'antigravity',
  'kiro'
]

function addTLSFingerprintBindingRow() {
  tlsFingerprintBindingRows.value.push({ os: '', client: '', protocol: '', profileId: 0 })
}
</script>
