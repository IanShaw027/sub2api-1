<template>
  <article class="card flex h-full flex-col overflow-hidden">
    <div class="border-b border-line bg-gradient-to-br from-card via-page to-brand-50 px-5 py-5 dark:border-dark-700 dark:from-dark-900 dark:via-dark-950 dark:to-brand-950/30">
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <span class="rounded-full bg-ink dark:bg-dark-700 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.25em] text-white">
              {{ skillTypeLabel(skill.type) }}
            </span>
            <span class="rounded-full px-2.5 py-1 text-[11px] font-medium" :class="skillStatusBadgeClass(skill.status)">
              {{ skillStatusLabel(skill.status) }}
            </span>
            <span v-if="skill.source_locked" class="rounded-full bg-warning-soft px-2.5 py-1 text-[11px] font-medium text-warning dark:bg-amber-900/20 dark:text-amber-200">
              {{ t('skills.detail.sourceLocked', '源内容隐藏') }}
            </span>
          </div>
          <h2 class="mt-3 line-clamp-2 text-xl font-semibold text-ink dark:text-white">{{ skill.name }}</h2>
          <p class="mt-2 line-clamp-2 text-sm text-ink-soft">{{ skill.tagline || skill.description }}</p>
        </div>
        <div class="text-right">
          <div class="text-sm font-semibold text-ink dark:text-white">{{ priceText }}</div>
          <p class="mt-1 text-xs text-ink-soft">{{ skill.author.name || t('skills.common.anonymous', '匿名作者') }}</p>
        </div>
      </div>
    </div>

    <div class="flex flex-1 flex-col px-5 py-5">
      <p class="line-clamp-4 flex-1 whitespace-pre-wrap text-sm text-ink-soft">{{ skill.description }}</p>

      <div class="mt-4 flex flex-wrap gap-2">
        <span
          v-for="tag in skill.tags"
          :key="tag"
          class="rounded-full bg-line px-2 py-1 text-[11px] text-ink-soft dark:bg-dark-800"
        >
          #{{ tag }}
        </span>
        <span
          v-if="skill.category"
          class="rounded-full bg-accent-50 px-2 py-1 text-[11px] text-accent-700 dark:bg-accent-900/20 dark:text-accent-200"
        >
          {{ skill.category }}
        </span>
      </div>

      <div class="mt-5 grid grid-cols-3 gap-2 rounded-card bg-page p-3 text-center dark:bg-dark-900">
        <div>
          <p class="text-[11px] uppercase tracking-[0.2em] text-ink-faint dark:text-ink-soft">{{ t('skills.card.installs', '安装') }}</p>
          <p class="mt-1 text-sm font-semibold text-ink dark:text-white">{{ skill.stats.installs }}</p>
        </div>
        <div>
          <p class="text-[11px] uppercase tracking-[0.2em] text-ink-faint dark:text-ink-soft">{{ t('skills.card.runs', '运行') }}</p>
          <p class="mt-1 text-sm font-semibold text-ink dark:text-white">{{ skill.stats.runs }}</p>
        </div>
        <div>
          <p class="text-[11px] uppercase tracking-[0.2em] text-ink-faint dark:text-ink-soft">{{ t('skills.card.versions', '版本') }}</p>
          <p class="mt-1 text-sm font-semibold text-ink dark:text-white">{{ skill.stats.versions }}</p>
        </div>
      </div>

      <div class="mt-5 flex flex-wrap gap-2">
        <RouterLink :to="skillPaths.detail(skill.id)" class="btn btn-secondary btn-sm">
          <Icon name="book" size="sm" class="mr-1" />
          {{ t('skills.detail.title', '技能详情') }}
        </RouterLink>
        <button
          v-if="!showOwnerActions && showInstallAction"
          type="button"
          class="btn btn-primary btn-sm"
          :disabled="busy"
          @click="$emit('toggle-install', skill)"
        >
          <Icon :name="skill.installed ? 'x' : 'plus'" size="sm" class="mr-1" />
          {{ skill.installed ? t('skills.market.uninstall', '卸载') : t('skills.market.install', '安装') }}
        </button>
        <RouterLink v-if="showOwnerActions" :to="skillPaths.edit(skill.id)" class="btn btn-secondary btn-sm">
          <Icon name="edit" size="sm" class="mr-1" />
          {{ t('common.edit', '编辑') }}
        </RouterLink>
        <RouterLink v-if="showOwnerActions" :to="skillPaths.versions(skill.id)" class="btn btn-secondary btn-sm">
          <Icon name="sync" size="sm" class="mr-1" />
          {{ t('skills.versions.title', '版本管理') }}
        </RouterLink>
        <RouterLink v-if="showOwnerActions" :to="skillPaths.revenue(skill.id)" class="btn btn-secondary btn-sm">
          <Icon name="dollar" size="sm" class="mr-1" />
          {{ t('skills.revenue.title', '收益页') }}
        </RouterLink>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import type { SkillSummary } from '@/types/skills'
import { skillPaths } from './paths'
import { formatCurrency, skillPriceModeLabel, skillStatusBadgeClass, skillStatusLabel, skillTypeLabel } from './presentation'

interface Props {
  skill: SkillSummary
  showOwnerActions?: boolean
  busy?: boolean
}

defineEmits<{
  (e: 'toggle-install', skill: SkillSummary): void
}>()

const props = withDefaults(defineProps<Props>(), {
  showOwnerActions: false,
  busy: false
})

const { t } = useI18n()

const showInstallAction = computed(() => {
  const installableSkill = props.skill as SkillSummary & { can_install?: boolean }
  return props.skill.installed || Boolean(installableSkill.can_install ?? !props.skill.owned)
})

const priceText = computed(() =>
  props.skill.pricing.mode === 'paid'
    ? `${formatCurrency(props.skill.pricing.amount, props.skill.pricing.currency)} · ${skillPriceModeLabel(props.skill.pricing.mode)}`
    : t('skills.market.free', '免费')
)
</script>
