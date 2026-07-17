<template>
  <AppLayout>
    <div class="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <SkillCenterNav
        active="detail"
        :skill-id="skillId"
        :can-edit-skill="Boolean(skill?.editable)"
        :can-view-runs="Boolean(skill?.owned)"
        :can-view-revenue="Boolean(skill?.owned)"
      />

      <div v-if="skill" class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_360px]">
        <section class="space-y-6">
          <div class="card overflow-hidden">
            <div class="border-b border-line bg-gradient-to-br from-white via-cyan-50 to-emerald-50 px-6 py-6 dark:border-dark-700 dark:from-dark-900 dark:via-dark-950 dark:to-emerald-950/20">
              <div class="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="rounded-full bg-ink dark:bg-dark-700 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.25em] text-white">
                      {{ skillTypeLabel(skill.type) }}
                    </span>
                    <span class="rounded-full px-2.5 py-1 text-[11px] font-medium" :class="skillStatusBadgeClass(skill.status)">
                      {{ skillStatusLabel(skill.status) }}
                    </span>
                    <span class="rounded-full bg-cyan-50 px-2.5 py-1 text-[11px] font-medium text-cyan-700 dark:bg-cyan-900/20 dark:text-cyan-200">
                      {{ priceText }}
                    </span>
                  </div>
                  <h1 class="mt-3 text-3xl font-bold text-ink dark:text-white">{{ skill.name }}</h1>
                  <p class="mt-2 text-base text-ink-body dark:text-dark-300">{{ skill.tagline || skill.description }}</p>
                </div>
                <div class="flex flex-col gap-3 lg:items-end">
                  <div
                    v-if="actionVersion"
                    class="rounded-card border border-white/70 bg-white/80 px-4 py-3 shadow-xs backdrop-blur dark:border-dark-700 dark:bg-dark-900/75"
                  >
                    <p class="text-[11px] font-medium uppercase tracking-[0.25em] text-ink-soft dark:text-dark-400">
                      {{ t('skills.detail.actionVersion', '操作版本') }}
                    </p>
                    <div class="mt-2 flex flex-wrap items-center gap-2">
                      <span class="text-sm font-semibold text-ink dark:text-white">{{ actionVersion.version }}</span>
                      <span class="rounded-full px-2.5 py-1 text-[11px] font-medium" :class="skillVersionReviewBadgeClass(actionVersion.review_status)">
                        {{ skillVersionReviewStatusLabel(actionVersion.review_status) }}
                      </span>
                      <span class="rounded-full px-2.5 py-1 text-[11px] font-medium" :class="skillVersionBadgeClass(actionVersion.status)">
                        {{ skillVersionStatusLabel(actionVersion.status) }}
                      </span>
                    </div>
                  </div>

                  <div class="flex flex-wrap gap-3 lg:justify-end">
                    <button
                      v-if="showInstallAction"
                      class="btn btn-primary"
                      :disabled="skillsStore.togglingInstall"
                      @click="toggleInstall"
                    >
                      <Icon :name="skill.installed ? 'x' : 'plus'" size="sm" class="mr-2" />
                      {{ skill.installed ? t('skills.market.uninstall', '卸载') : t('skills.market.install', '安装') }}
                    </button>
                    <button
                      v-if="canShowRunActions"
                      class="btn btn-secondary"
                      :disabled="!actionVersion || !actionVersion.can_test || isRunningAction('test')"
                      @click="triggerRun('test')"
                    >
                      {{ isRunningAction('test') ? t('common.processing', '处理中') : t('skills.actions.test', '测试') }}
                    </button>
                    <button
                      v-if="canShowRunActions"
                      class="btn btn-secondary"
                      :disabled="!actionVersion || !actionVersion.can_use || isRunningAction('use')"
                      @click="triggerRun('use')"
                    >
                      {{ isRunningAction('use') ? t('common.processing', '处理中') : t('skills.actions.use', '使用') }}
                    </button>
                    <button
                      v-if="skill.owned && actionVersion"
                      class="btn btn-secondary"
                      :disabled="!actionVersion.can_submit_review || skillsStore.submittingVersionId === actionVersion.id"
                      @click="submitActionVersion"
                    >
                      {{ skillsStore.submittingVersionId === actionVersion.id ? t('common.processing', '处理中') : t('skills.actions.submitReview', '提交审核') }}
                    </button>
                    <button
                      v-if="skill.owned && actionVersion"
                      class="btn btn-primary"
                      :disabled="!actionVersion.can_publish || actionVersion.is_current || skillsStore.publishingVersionId === actionVersion.id"
                      @click="publishActionVersion"
                    >
                      {{ skillsStore.publishingVersionId === actionVersion.id ? t('common.processing', '处理中') : t('skills.actions.publish', '发布') }}
                    </button>
                    <RouterLink
                      v-if="skill.editable"
                      :to="skillPaths.edit(skill.id)"
                      class="btn btn-secondary"
                    >
                      <Icon name="edit" size="sm" class="mr-2" />
                      {{ t('common.edit', '编辑') }}
                    </RouterLink>
                  </div>

                  <p
                    v-if="detailActionHint"
                    class="max-w-md text-xs leading-5 text-amber-700 dark:text-amber-200"
                  >
                    {{ detailActionHint }}
                  </p>
                </div>
              </div>
            </div>

            <div class="grid gap-6 px-6 py-6 lg:grid-cols-2">
              <div class="space-y-4">
                <div class="rounded-card border border-line bg-card p-4 dark:border-dark-700 dark:bg-dark-900">
                  <h2 class="text-sm font-semibold text-ink dark:text-white">{{ t('skills.detail.about', '技能说明') }}</h2>
                  <p class="mt-3 whitespace-pre-wrap text-sm leading-6 text-ink-body dark:text-dark-400">{{ skill.description }}</p>
                </div>

                <div v-if="skill.readme" class="rounded-card border border-line bg-card p-4 dark:border-dark-700 dark:bg-dark-900">
                  <h2 class="text-sm font-semibold text-ink dark:text-white">{{ t('skills.detail.readme', '使用说明') }}</h2>
                  <p class="mt-3 whitespace-pre-wrap text-sm leading-6 text-ink-body dark:text-dark-400">{{ skill.readme }}</p>
                </div>

                <div v-if="showSource" class="rounded-card border border-line bg-card p-4 dark:border-dark-700 dark:bg-dark-900">
                  <h2 class="text-sm font-semibold text-ink dark:text-white">{{ t('skills.detail.sourcePreview', '源内容预览') }}</h2>
                  <div class="mt-3 space-y-3">
                    <template v-if="skill.content?.type === 'prompt_chat'">
                      <pre class="overflow-x-auto rounded-xl bg-page p-4 text-xs leading-6 text-ink-body dark:bg-dark-950 dark:text-dark-200">{{ skill.content.system_prompt }}</pre>
                      <pre class="overflow-x-auto rounded-xl bg-page p-4 text-xs leading-6 text-ink-body dark:bg-dark-950 dark:text-dark-200">{{ skill.content.user_prompt_template }}</pre>
                    </template>
                    <template v-else-if="skill.content?.type === 'prompt_image'">
                      <pre class="overflow-x-auto rounded-xl bg-page p-4 text-xs leading-6 text-ink-body dark:bg-dark-950 dark:text-dark-200">{{ skill.content.prompt_template }}</pre>
                      <pre
                        v-if="skill.content.negative_prompt_template"
                        class="overflow-x-auto rounded-xl bg-page p-4 text-xs leading-6 text-ink-body dark:bg-dark-950 dark:text-dark-200"
                      >{{ skill.content.negative_prompt_template }}</pre>
                    </template>
                    <template v-else-if="skill.content?.type === 'script'">
                      <pre class="overflow-x-auto rounded-xl bg-page p-4 text-xs leading-6 text-ink-body dark:bg-dark-950 dark:text-dark-200">{{ skill.content.source_code }}</pre>
                    </template>
                  </div>
                </div>

                <div
                  v-else
                  class="rounded-card border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900/40 dark:bg-amber-900/20 dark:text-amber-100"
                >
                  {{ t('skills.detail.sourceLockedNotice', '当前技能为收费或受保护技能，源内容默认隐藏。这里只展示变量表单与元信息。') }}
                </div>
              </div>

              <div class="space-y-4">
                <div class="rounded-card border border-line bg-card p-4 dark:border-dark-700 dark:bg-dark-900">
                  <h2 class="text-sm font-semibold text-ink dark:text-white">{{ t('skills.detail.exposedVariables', '可配置变量') }}</h2>
                  <p class="mt-1 text-xs text-ink-soft dark:text-dark-400">{{ t('skills.detail.exposedVariablesHint', '这些变量会在运行或调用前展示给用户填写。') }}</p>
                  <div class="mt-4">
                    <SkillVariableForm v-model="variableValues" :schema="skill.variable_schema" />
                  </div>
                </div>

                <div v-if="skill.install_note" class="rounded-card border border-line bg-card p-4 dark:border-dark-700 dark:bg-dark-900">
                  <h2 class="text-sm font-semibold text-ink dark:text-white">{{ t('skills.detail.installNote', '安装说明') }}</h2>
                  <p class="mt-3 whitespace-pre-wrap text-sm leading-6 text-ink-body dark:text-dark-400">{{ skill.install_note }}</p>
                </div>
              </div>
            </div>
          </div>
        </section>

        <aside class="space-y-4">
          <div class="card p-5">
            <h2 class="text-sm font-semibold text-ink dark:text-white">{{ t('skills.detail.meta', '元信息') }}</h2>
            <dl class="mt-4 space-y-3 text-sm">
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('common.name', '名称') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ skill.author.name || '-' }}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('skills.market.category', '分类') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ skill.category || '-' }}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('skills.detail.latestVersion', '当前版本') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ skill.latest_version?.version || '-' }}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('skills.card.installs', '安装') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ skill.stats.installs }}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('skills.card.runs', '运行') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ skill.stats.runs }}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('skills.revenue.title', '收益') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ formatCurrency(skill.stats.revenue, skill.pricing.currency) }}</dd>
              </div>
            </dl>
          </div>

          <div class="card p-5">
            <div class="mb-4 flex items-center justify-between">
              <h2 class="text-sm font-semibold text-ink dark:text-white">{{ t('skills.versions.recent', '最近版本') }}</h2>
              <RouterLink
                v-if="skill.editable"
                :to="skillPaths.versions(skill.id)"
                class="text-sm font-medium text-brand-600 hover:text-brand-700 dark:text-brand-400"
              >
                {{ t('skills.versions.title', '版本管理') }}
              </RouterLink>
            </div>
            <div class="space-y-3">
              <div
                v-for="version in skillsStore.versionsPagination.items.slice(0, 3)"
                :key="version.id"
                class="rounded-card border border-line p-3 dark:border-dark-700"
              >
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <p class="font-medium text-ink dark:text-white">{{ version.version }}</p>
                    <p class="mt-1 text-xs text-ink-soft dark:text-dark-400">{{ version.changelog || t('skills.versions.noChangelog', '暂无变更说明') }}</p>
                  </div>
                  <div class="flex flex-wrap items-center justify-end gap-2">
                    <span class="rounded-full px-2 py-1 text-[11px] font-medium" :class="skillVersionReviewBadgeClass(version.review_status)">
                      {{ skillVersionReviewStatusLabel(version.review_status) }}
                    </span>
                    <span class="rounded-full px-2 py-1 text-[11px] font-medium" :class="skillVersionBadgeClass(version.status)">
                      {{ skillVersionStatusLabel(version.status) }}
                    </span>
                  </div>
                </div>
              </div>
              <EmptyState
                v-if="!skillsStore.loadingVersions && skillsStore.versionsPagination.items.length === 0"
                :title="t('skills.versions.emptyTitle', '还没有版本')"
                :description="t('skills.versions.emptyDescription', '创建第一个版本后会显示在这里。')"
              />
            </div>
          </div>

          <div class="card p-5">
            <div class="grid gap-3">
              <RouterLink v-if="skill.owned" :to="skillPaths.runs(skill.id)" class="btn btn-secondary">
                <Icon name="clock" size="sm" class="mr-2" />
                {{ t('skills.runs.title', '运行记录') }}
              </RouterLink>
              <RouterLink
                v-if="skill.owned"
                :to="skillPaths.revenue(skill.id)"
                class="btn btn-secondary"
              >
                <Icon name="dollar" size="sm" class="mr-2" />
                {{ t('skills.revenue.title', '收益页') }}
              </RouterLink>
            </div>
          </div>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import SkillCenterNav from '@/components/skills/SkillCenterNav.vue'
import SkillVariableForm from '@/components/skills/SkillVariableForm.vue'
import { skillPaths } from '@/components/skills/paths'
import {
  formatCurrency,
  skillStatusBadgeClass,
  skillStatusLabel,
  skillTypeLabel,
  skillVersionBadgeClass,
  skillVersionReviewBadgeClass,
  skillVersionReviewStatusLabel,
  skillVersionStatusLabel
} from '@/components/skills/presentation'
import { useSkillsCenterStore } from '@/stores/skillsCenter'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { SkillRunMode, SkillVersionSummary } from '@/types/skills'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const skillsStore = useSkillsCenterStore()

const variableValues = ref<Record<string, unknown>>({})

const skillId = computed(() => {
  const value = Number(route.params.id)
  return Number.isFinite(value) && value > 0 ? value : 0
})

const skill = computed(() => skillsStore.detail)
const showSource = computed(() => {
  const current = skill.value
  if (!current) return false
  return Boolean(current.content) && (current.can_view_source || current.owned || !current.source_locked)
})
const priceText = computed(() => {
  if (!skill.value) return '-'
  return skill.value.pricing.mode === 'paid'
    ? formatCurrency(skill.value.pricing.amount, skill.value.pricing.currency)
    : t('skills.market.free', '免费')
})
const showInstallAction = computed(() => {
  const currentSkill = skill.value
  if (!currentSkill || currentSkill.owned) return false
  return currentSkill.installed || currentSkill.can_install
})
const actionVersion = computed<SkillVersionSummary | null>(() => {
  const currentSkill = skill.value
  if (!currentSkill) return null

  const preferredIds = currentSkill.owned
    ? [currentSkill.latest_version?.id, currentSkill.current_version?.id]
    : [currentSkill.current_version?.id, currentSkill.latest_version?.id]

  for (const id of preferredIds) {
    if (!id) continue
    const record = skillsStore.versionsPagination.items.find((item) => item.id === id)
    if (record) return record
    if (currentSkill.latest_version?.id === id) return currentSkill.latest_version
    if (currentSkill.current_version?.id === id) return currentSkill.current_version
  }

  return skillsStore.versionsPagination.items[0] ?? currentSkill.latest_version ?? currentSkill.current_version ?? null
})
const canShowRunActions = computed(() => Boolean(skill.value?.can_run && actionVersion.value))
const detailActionHint = computed(() => {
  const currentSkill = skill.value
  const version = actionVersion.value
  if (!currentSkill || !version) return ''
  if (version.is_current && version.review_status === 'approved') {
    return currentSkill.owned
      ? t('skills.actions.currentPublishedHint', '当前版本已生效，可继续测试或使用。')
      : ''
  }
  if (version.review_status === 'pending') {
    return currentSkill.owned
      ? t('skills.actions.pendingReviewHint', '当前版本审核中，暂不可测试、使用或发布。')
      : t('skills.actions.currentUnavailable', '当前版本审核中，暂不可使用。')
  }
  if (version.review_status === 'approved') {
    return currentSkill.owned
      ? t('skills.actions.publishReadyHint', '当前版本已审核通过，可以直接发布。')
      : ''
  }
  return currentSkill.owned
    ? t('skills.actions.submitRequiredHint', '请先提交审核，审核通过后才能测试、使用或发布。')
    : t('skills.actions.currentUnavailable', '当前版本暂不可使用。')
})

function actionErrorMessage(error: unknown): string {
  return extractApiErrorMessage(error, t('common.error'), {
    AI_SKILL_VERSION_NOT_APPROVED: t('skills.actions.requiresApprovedVersion', '当前版本未审核通过，暂不可测试、使用或发布。')
  })
}

function isRunningAction(mode: SkillRunMode): boolean {
  return skillsStore.runningMode === mode && skillsStore.runningVersionId === (actionVersion.value?.id ?? 0)
}

async function loadPage(force = false): Promise<void> {
  if (!skillId.value) return
  try {
    await skillsStore.loadSkillDetail(skillId.value, force)
    await skillsStore.loadVersions(skillId.value, 1, 6)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function toggleInstall(): Promise<void> {
  if (!skill.value) return
  const wasInstalled = skill.value.installed
  try {
    await skillsStore.toggleInstall(skill.value.id, wasInstalled)
    await loadPage(true)
    appStore.showSuccess(wasInstalled ? t('skills.market.uninstallSuccess', '已卸载技能') : t('skills.market.installSuccess', '已安装技能'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function triggerRun(mode: SkillRunMode): Promise<void> {
  if (!skill.value || !actionVersion.value) return
  const version = actionVersion.value

  if ((mode === 'test' && !version.can_test) || (mode === 'use' && !version.can_use)) {
    appStore.showError(detailActionHint.value || t('common.error'))
    return
  }

  try {
    if (mode === 'test') {
      await skillsStore.testSkillVersion(skill.value.id, version.id, { ...variableValues.value })
      appStore.showSuccess(t('skills.actions.testSuccess', '已发起测试'))
      return
    }

    await skillsStore.useSkillVersion(skill.value.id, version.id, { ...variableValues.value })
    appStore.showSuccess(t('skills.actions.useSuccess', '已发起使用'))
  } catch (error) {
    appStore.showError(actionErrorMessage(error))
  }
}

async function submitActionVersion(): Promise<void> {
  if (!skill.value || !actionVersion.value) return
  if (!actionVersion.value.can_submit_review) {
    appStore.showError(detailActionHint.value || t('common.error'))
    return
  }

  try {
    await skillsStore.submitVersion(skill.value.id, actionVersion.value.id)
    appStore.showSuccess(t('skills.actions.submitReviewSuccess', '已提交审核'))
  } catch (error) {
    appStore.showError(actionErrorMessage(error))
  }
}

async function publishActionVersion(): Promise<void> {
  if (!skill.value || !actionVersion.value) return
  if (!actionVersion.value.can_publish || actionVersion.value.is_current) {
    appStore.showError(
      actionVersion.value.is_current
        ? t('skills.actions.alreadyCurrent', '当前版本已是生效版本')
        : detailActionHint.value || t('common.error')
    )
    return
  }

  try {
    await skillsStore.publishVersion(skill.value.id, actionVersion.value.id)
    appStore.showSuccess(t('skills.actions.publishSuccess', '版本已发布'))
  } catch (error) {
    appStore.showError(actionErrorMessage(error))
  }
}

watch(skillId, () => {
  variableValues.value = {}
  void loadPage(true)
})

onMounted(() => {
  void loadPage()
})
</script>
