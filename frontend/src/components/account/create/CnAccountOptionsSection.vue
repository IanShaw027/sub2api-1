<template>
  <!-- Account Mode Selection (Kimi / Zhipu / DeepSeek) -->
  <div v-if="isCnPlatform">
    <label class="input-label">{{ t('admin.accounts.cnProviders.accountMode.title') }}</label>
    <div class="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2" data-tour="account-form-mode">
      <!-- Pay-as-you-go (token balance) -->
      <button
        type="button"
        @click="accountMode = 'payg'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountMode === 'payg'
 ? cnAccentActiveClass
 : 'border-line hover:border-line'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountMode === 'payg'
 ? cnAccentIconClass
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon name="creditCard" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">{{ t('admin.accounts.cnProviders.accountMode.payg') }}</span>
          <span class="text-xs text-muted">{{ t('admin.accounts.cnProviders.accountMode.paygDesc') }}</span>
        </div>
      </button>
      <!-- Coding Plan (kimi / zhipu only — DeepSeek has no coding plan) -->
      <button
        v-if="platform !== 'deepseek'"
        type="button"
        @click="accountMode = 'coding'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountMode === 'coding'
 ? cnAccentActiveClass
 : 'border-line hover:border-line'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountMode === 'coding'
 ? cnAccentIconClass
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon name="bolt" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">{{ t('admin.accounts.cnProviders.accountMode.coding') }}</span>
          <span class="text-xs text-muted">{{ t('admin.accounts.cnProviders.accountMode.codingDesc') }}</span>
        </div>
      </button>
    </div>
  </div>

  <!-- API Protocol Selection (Kimi / Zhipu / DeepSeek) -->
  <div v-if="isCnPlatform" class="mt-4">
    <label class="input-label">{{ t('admin.accounts.cnProviders.apiProtocol.title') }}</label>
    <div class="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-3">
      <button
        v-for="opt in cnProtocolOptions"
        :key="opt.value"
        type="button"
        @click="apiProtocol = opt.value"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 apiProtocol === opt.value
 ? cnAccentActiveClass
 : 'border-line hover:border-line'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 apiProtocol === opt.value
 ? cnAccentIconClass
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon :name="opt.value === 'adaptive' ? 'swap' : opt.value === 'anthropic' ? 'sparkles' : opt.value === 'responses' ? 'terminal' : 'chat'" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">{{ t(`admin.accounts.cnProviders.apiProtocol.${opt.labelKey}`) }}</span>
          <span class="text-xs text-muted">{{ t(`admin.accounts.cnProviders.apiProtocol.${opt.labelKey}Desc`) }}</span>
        </div>
      </button>
    </div>
  </div>

  <!-- Zhipu 团队版 Coding Plan：组织/项目 ID（可选，填写后额度探测走团队版端点） -->
  <div v-if="platform === 'zhipu' && accountMode === 'coding'" class="mt-4">
    <div class="flex items-center">
      <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.title') }}</label>
      <HelpTooltip trigger="click" width-class="w-80">
        <p class="mb-1 font-medium">{{ t('admin.accounts.cnProviders.zhipuTeam.help.title') }}</p>
        <ol class="list-decimal space-y-1 pl-4">
          <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step1') }}</li>
          <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step2') }}</li>
          <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step3') }}</li>
          <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step4') }}</li>
        </ol>
        <p class="mt-2 break-all rounded bg-black/20 p-1.5 font-mono text-[11px] leading-relaxed">
          {{ t('admin.accounts.cnProviders.zhipuTeam.help.example') }}
        </p>
      </HelpTooltip>
    </div>
    <div class="mt-2 grid gap-4 sm:grid-cols-2">
      <div>
        <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.organization') }}</label>
        <input v-model="zhipuOrganization" type="text" class="input" :placeholder="t('admin.accounts.cnProviders.zhipuTeam.organizationPlaceholder')" />
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.project') }}</label>
        <input v-model="zhipuProject" type="text" class="input" :placeholder="t('admin.accounts.cnProviders.zhipuTeam.projectPlaceholder')" />
      </div>
    </div>
    <p class="input-hint mt-2">{{ t('admin.accounts.cnProviders.zhipuTeam.hint') }}</p>
  </div>
</template>

<script setup lang="ts">
// Shared "CN provider (Kimi / Zhipu / DeepSeek) account mode + API protocol +
// Zhipu team plan" section. Byte-identical markup lifted verbatim out of the
// host to shrink it; the surrounding v-if gating stays in this component
// since all three blocks share the same isCnPlatform/platform/accountMode
// conditions used only here.
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import type { CnApiProtocol, CnAccountMode } from '@/components/account/credentialsBuilder'

const { t } = useI18n()

defineProps<{
  platform: string
  isCnPlatform: boolean
  cnAccentActiveClass: string
  cnAccentIconClass: string
  cnProtocolOptions: Array<{ value: CnApiProtocol; labelKey: string }>
}>()

const accountMode = defineModel<CnAccountMode>('accountMode', { required: true })
const apiProtocol = defineModel<CnApiProtocol>('apiProtocol', { required: true })
const zhipuOrganization = defineModel<string>('zhipuOrganization', { required: true })
const zhipuProject = defineModel<string>('zhipuProject', { required: true })
</script>
