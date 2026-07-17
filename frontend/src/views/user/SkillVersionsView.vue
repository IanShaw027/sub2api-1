<template>
  <AppLayout>
    <div v-if="showVersionPage" class="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <SkillCenterNav
        active="versions"
        :skill-id="skillId"
        :can-edit-skill="Boolean(skill?.editable)"
        :can-view-runs="Boolean(skill?.owned)"
        :can-view-revenue="Boolean(skill?.owned)"
      />

      <div class="grid gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
        <aside class="space-y-6">
          <section class="card p-5">
            <h2 class="text-lg font-semibold text-ink dark:text-white">{{ t('skills.versions.createVersion', '创建版本') }}</h2>
            <p class="mt-1 text-sm text-ink-soft dark:text-dark-400">{{ t('skills.versions.createVersionHint', '默认基于当前技能内容与变量 schema 生成版本快照。') }}</p>
            <div class="mt-4 space-y-4">
              <Input v-model="draft.version" :label="t('skills.versions.version', '版本号')" :placeholder="'v1.0.0'" />
              <div>
                <label class="input-label mb-1.5 block">{{ t('common.status', '状态') }}</label>
                <Select
                  :model-value="draft.status"
                  :options="statusOptions"
                  @update:model-value="(value) => (draft.status = normalizeStatus(value))"
                />
              </div>
              <label class="flex items-start gap-3 rounded-card border border-line bg-page px-4 py-4 text-sm text-ink-body dark:border-dark-700 dark:bg-dark-950 dark:text-dark-200">
                <input
                  v-model="draft.source_locked"
                  type="checkbox"
                  class="mt-0.5 h-4 w-4 rounded border-line text-brand-600 focus:ring-accent/25"
                />
                <span>
                  <span class="block font-medium">{{ t('skills.editor.sourceLocked', '隐藏源内容') }}</span>
                  <span class="mt-1 block text-xs text-ink-soft dark:text-dark-400">{{ t('skills.versions.sourceLockedHint', '版本级别也可以单独决定是否暴露源内容。') }}</span>
                </span>
              </label>
              <TextArea
                v-model="draft.changelog"
                :rows="5"
                :label="t('skills.versions.changelog', '变更说明')"
                :placeholder="t('skills.versions.changelogPlaceholder', '记录本次版本更新了什么。')"
              />
              <button class="btn btn-primary w-full" :disabled="skillsStore.creatingVersion" @click="createVersion">
                <Icon name="plus" size="sm" class="mr-2" />
                {{ skillsStore.creatingVersion ? t('common.processing', '处理中') : t('skills.versions.publishVersion', '创建版本') }}
              </button>
            </div>
          </section>

          <section v-if="skill" class="card p-5">
            <h2 class="text-lg font-semibold text-ink dark:text-white">{{ t('skills.detail.meta', '元信息') }}</h2>
            <dl class="mt-4 space-y-3 text-sm">
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('common.name', '名称') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ skill.name }}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('skills.editor.type', '类型') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ skill.type }}</dd>
              </div>
              <div class="flex items-center justify-between gap-3">
                <dt class="text-ink-soft dark:text-dark-400">{{ t('skills.versions.currentVersion', '当前版本') }}</dt>
                <dd class="text-right font-medium text-ink dark:text-white">{{ skill.latest_version?.version || '-' }}</dd>
              </div>
            </dl>
          </section>
        </aside>

        <section class="space-y-4">
          <article
            v-for="version in skillsStore.versionsPagination.items"
            :key="version.id"
            class="card p-5"
          >
            <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="text-xl font-semibold text-ink dark:text-white">{{ version.version }}</h2>
                  <span class="rounded-full px-2.5 py-1 text-[11px] font-medium" :class="skillVersionReviewBadgeClass(version.review_status)">
                    {{ skillVersionReviewStatusLabel(version.review_status) }}
                  </span>
                  <span class="rounded-full px-2.5 py-1 text-[11px] font-medium" :class="skillVersionBadgeClass(version.status)">
                    {{ skillVersionStatusLabel(version.status) }}
                  </span>
                  <span
                    v-if="version.is_current"
                    class="rounded-full bg-ink dark:bg-dark-700 px-2.5 py-1 text-[11px] font-medium text-white"
                  >
                    {{ t('skills.versions.currentVersion', '当前版本') }}
                  </span>
                </div>
                <p class="mt-2 text-sm text-ink-soft dark:text-dark-400">{{ version.created_at }}</p>
              </div>
              <div class="text-sm text-ink-soft dark:text-dark-400">
                {{ version.variable_schema.length }} {{ t('skills.editor.variableSchema', '变量 Schema') }}
              </div>
            </div>

            <div class="mt-4 grid gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
              <div>
                <label class="input-label mb-1.5 block">{{ t('common.status', '状态') }}</label>
                <Select
                  :model-value="versionEdits[version.id]?.status || version.status"
                  :options="statusOptions"
                  @update:model-value="(value) => ensureVersionEdit(version.id, version).status = normalizeStatus(value)"
                />
              </div>
              <TextArea
                :model-value="versionEdits[version.id]?.changelog || version.changelog"
                :rows="4"
                :label="t('skills.versions.changelog', '变更说明')"
                @update:model-value="(value) => ensureVersionEdit(version.id, version).changelog = value"
              />
            </div>

            <div class="mt-4 flex flex-wrap gap-3">
              <label class="inline-flex items-center gap-2 rounded-full bg-line px-3 py-2 text-sm text-ink-body dark:bg-dark-800 dark:text-dark-200">
                <input
                  :checked="versionEdits[version.id]?.source_locked ?? version.source_locked"
                  type="checkbox"
                  class="h-4 w-4 rounded border-line text-brand-600 focus:ring-accent/25"
                  @change="ensureVersionEdit(version.id, version).source_locked = ($event.target as HTMLInputElement).checked"
                />
                {{ t('skills.editor.sourceLocked', '隐藏源内容') }}
              </label>
              <button class="btn btn-primary btn-sm" :disabled="skillsStore.updatingVersion" @click="saveVersion(version.id)">
                {{ t('common.save', '保存') }}
              </button>
            </div>

            <div class="mt-3 flex flex-wrap gap-3">
              <button
                class="btn btn-secondary btn-sm"
                :disabled="!version.can_submit_review || skillsStore.submittingVersionId === version.id"
                @click="submitVersionForReview(version)"
              >
                {{ skillsStore.submittingVersionId === version.id ? t('common.processing', '处理中') : t('skills.actions.submitReview', '提交审核') }}
              </button>
              <button
                class="btn btn-secondary btn-sm"
                :disabled="!version.can_test || isRunningVersionAction(version.id, 'test')"
                @click="triggerVersionRun(version, 'test')"
              >
                {{ isRunningVersionAction(version.id, 'test') ? t('common.processing', '处理中') : t('skills.actions.test', '测试') }}
              </button>
              <button
                class="btn btn-secondary btn-sm"
                :disabled="!version.can_use || isRunningVersionAction(version.id, 'use')"
                @click="triggerVersionRun(version, 'use')"
              >
                {{ isRunningVersionAction(version.id, 'use') ? t('common.processing', '处理中') : t('skills.actions.use', '使用') }}
              </button>
              <button
                class="btn btn-primary btn-sm"
                :disabled="!version.can_publish || version.is_current || skillsStore.publishingVersionId === version.id"
                @click="publishVersionRecord(version)"
              >
                {{ skillsStore.publishingVersionId === version.id ? t('common.processing', '处理中') : t('skills.actions.publish', '发布') }}
              </button>
            </div>

            <p
              v-if="versionActionHint(version)"
              class="mt-3 text-xs leading-5 text-amber-700 dark:text-amber-200"
            >
              {{ versionActionHint(version) }}
            </p>
          </article>

          <div v-if="!skillsStore.loadingVersions && skillsStore.versionsPagination.items.length === 0" class="card p-12">
            <EmptyState
              :title="t('skills.versions.emptyTitle', '还没有版本')"
              :description="t('skills.versions.emptyDescription', '基于当前技能快照创建第一个版本。')"
            />
          </div>

          <Pagination
            v-if="skillsStore.versionsPagination.total > skillsStore.versionsPagination.page_size"
            :page="skillsStore.versionsPagination.page"
            :total="skillsStore.versionsPagination.total"
            :page-size="skillsStore.versionsPagination.page_size"
            @update:page="(page) => void handlePageChange(page)"
            @update:pageSize="(pageSize) => void handlePageSizeChange(pageSize)"
          />
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, computed, watch, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import SkillCenterNav from '@/components/skills/SkillCenterNav.vue'
import { skillPaths } from '@/components/skills/paths'
import {
  skillVersionBadgeClass,
  skillVersionReviewBadgeClass,
  skillVersionReviewStatusLabel,
  skillVersionStatusLabel
} from '@/components/skills/presentation'
import { useSkillsCenterStore } from '@/stores/skillsCenter'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { SkillRunMode, SkillVersionRecord, SkillVersionStatus } from '@/types/skills'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const appStore = useAppStore()
const skillsStore = useSkillsCenterStore()

const skillId = computed(() => {
  const value = Number(route.params.id)
  return Number.isFinite(value) && value > 0 ? value : 0
})

const skill = computed(() => skillsStore.detail)
const loadedSkillId = ref<number | null>(null)
const showVersionPage = computed(() => loadedSkillId.value === skillId.value)
let suppressRouteVersionsReload = false
const draft = reactive<{
  version: string
  status: SkillVersionStatus
  changelog: string
  source_locked: boolean
}>({
  version: '',
  status: 'draft',
  changelog: '',
  source_locked: false
})
const versionEdits = reactive<Record<number, { status: SkillVersionStatus; changelog: string; source_locked: boolean }>>({})

const statusOptions = [
  { value: 'draft', label: 'draft' },
  { value: 'published', label: 'published' },
  { value: 'deprecated', label: 'deprecated' },
  { value: 'archived', label: 'archived' }
]

function normalizeStatus(value: string | number | boolean | null): SkillVersionStatus {
  return value === 'published' || value === 'deprecated' || value === 'archived' ? value : 'draft'
}

function extractQueryString(value: unknown): string | null {
  if (typeof value === 'string') return value
  if (Array.isArray(value) && typeof value[0] === 'string') return value[0]
  return null
}

function extractPositiveQueryNumber(value: unknown, fallback: number): number {
  const raw = extractQueryString(value)
  if (!raw) return fallback
  const parsed = Number.parseInt(raw, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function currentRoutePage(): number {
  return extractPositiveQueryNumber(route.query.page, 1)
}

function currentRoutePageSize(): number {
  return extractPositiveQueryNumber(route.query.page_size, skillsStore.versionsPagination.page_size)
}

async function replaceVersionsQuery(page = currentRoutePage(), pageSize = currentRoutePageSize()): Promise<boolean> {
  const nextQuery = { ...route.query }
  const nextPage = page > 1 ? String(page) : null
  const nextPageSize = pageSize !== 12 ? String(pageSize) : null
  const currentPage = extractQueryString(route.query.page)
  const currentPageSize = extractQueryString(route.query.page_size)

  if (nextPage) nextQuery.page = nextPage
  else delete nextQuery.page

  if (nextPageSize) nextQuery.page_size = nextPageSize
  else delete nextQuery.page_size

  if (currentPage === nextPage && currentPageSize === nextPageSize) {
    return false
  }

  suppressRouteVersionsReload = true
  await router.replace({ query: nextQuery })
  return true
}

function actionErrorMessage(error: unknown): string {
  return extractApiErrorMessage(error, t('common.error'), {
    AI_SKILL_VERSION_NOT_APPROVED: t('skills.actions.requiresApprovedVersion', '当前版本未审核通过，暂不可测试、使用或发布。')
  })
}

function versionActionHint(version: SkillVersionRecord): string {
  if (version.is_current && version.review_status === 'approved') {
    return t('skills.actions.currentPublishedHint', '当前版本已生效，可继续测试或使用。')
  }
  if (version.review_status === 'pending') {
    return t('skills.actions.pendingReviewHint', '当前版本审核中，暂不可测试、使用或发布。')
  }
  if (version.review_status === 'approved') {
    return t('skills.actions.publishReadyHint', '当前版本已审核通过，可以直接发布。')
  }
  return t('skills.actions.submitRequiredHint', '请先提交审核，审核通过后才能测试、使用或发布。')
}

function isRunningVersionAction(versionId: number, mode: SkillRunMode): boolean {
  return skillsStore.runningMode === mode && skillsStore.runningVersionId === versionId
}

function ensureVersionEdit(versionId: number, version: SkillVersionRecord) {
  if (!versionEdits[versionId]) {
    versionEdits[versionId] = {
      status: version.status,
      changelog: version.changelog,
      source_locked: version.source_locked
    }
  }
  return versionEdits[versionId]
}

function resetDraft(): void {
  draft.version = ''
  draft.status = 'draft'
  draft.changelog = ''
  draft.source_locked = skill.value?.source_locked ?? false
}

async function loadPage(force = false): Promise<void> {
  if (!skillId.value) return
  loadedSkillId.value = null
  try {
    await skillsStore.loadSkillDetail(skillId.value, force)
    if (!skill.value?.editable) {
      await router.replace(skillPaths.detail(skillId.value))
      return
    }
    await skillsStore.loadVersions(skillId.value, currentRoutePage(), currentRoutePageSize())
    resetDraft()
    loadedSkillId.value = skillId.value
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function createVersion(): Promise<void> {
  if (!skill.value) return
  if (!draft.version.trim()) {
    appStore.showError(t('skills.versions.versionRequired', '请先填写版本号'))
    return
  }

  try {
    await skillsStore.addVersion(skill.value.id, {
      version: draft.version.trim(),
      status: draft.status,
      changelog: draft.changelog.trim(),
      source_locked: draft.source_locked,
      variable_schema: skill.value.variable_schema,
      content: skill.value.content
    })
    appStore.showSuccess(t('common.saved', '保存成功'))
    await loadPage(true)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function saveVersion(versionId: number): Promise<void> {
  const version = skillsStore.versionsPagination.items.find((item) => item.id === versionId)
  const edit = version ? ensureVersionEdit(versionId, version) : null
  if (!version || !edit) return

  try {
    await skillsStore.saveVersion(skillId.value, versionId, {
      status: edit.status,
      changelog: edit.changelog,
      source_locked: edit.source_locked
    })
    appStore.showSuccess(t('common.saved', '保存成功'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function submitVersionForReview(version: SkillVersionRecord): Promise<void> {
  if (!skill.value) return
  if (!version.can_submit_review) {
    appStore.showError(versionActionHint(version))
    return
  }

  try {
    await skillsStore.submitVersion(skill.value.id, version.id)
    appStore.showSuccess(t('skills.actions.submitReviewSuccess', '已提交审核'))
  } catch (error) {
    appStore.showError(actionErrorMessage(error))
  }
}

async function publishVersionRecord(version: SkillVersionRecord): Promise<void> {
  if (!skill.value) return
  if (!version.can_publish || version.is_current) {
    appStore.showError(
      version.is_current
        ? t('skills.actions.alreadyCurrent', '当前版本已是生效版本')
        : versionActionHint(version)
    )
    return
  }

  try {
    await skillsStore.publishVersion(skill.value.id, version.id)
    appStore.showSuccess(t('skills.actions.publishSuccess', '版本已发布'))
  } catch (error) {
    appStore.showError(actionErrorMessage(error))
  }
}

async function triggerVersionRun(version: SkillVersionRecord, mode: SkillRunMode): Promise<void> {
  if (!skill.value) return
  if ((mode === 'test' && !version.can_test) || (mode === 'use' && !version.can_use)) {
    appStore.showError(versionActionHint(version))
    return
  }

  try {
    if (mode === 'test') {
      await skillsStore.testSkillVersion(skill.value.id, version.id)
      appStore.showSuccess(t('skills.actions.testSuccess', '已发起测试'))
      return
    }

    await skillsStore.useSkillVersion(skill.value.id, version.id)
    appStore.showSuccess(t('skills.actions.useSuccess', '已发起使用'))
  } catch (error) {
    appStore.showError(actionErrorMessage(error))
  }
}

async function reloadVersions(page = currentRoutePage(), pageSize = currentRoutePageSize()): Promise<void> {
  try {
    await skillsStore.loadVersions(skillId.value, page, pageSize)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function handlePageChange(page: number): Promise<void> {
  await replaceVersionsQuery(page, skillsStore.versionsPagination.page_size)
  await reloadVersions(page, skillsStore.versionsPagination.page_size)
}

async function handlePageSizeChange(pageSize: number): Promise<void> {
  await replaceVersionsQuery(1, pageSize)
  await reloadVersions(1, pageSize)
}

watch(skillId, () => {
  void loadPage(true)
})

watch(
  () => [route.query.page, route.query.page_size],
  async () => {
    if (suppressRouteVersionsReload) {
      suppressRouteVersionsReload = false
      return
    }
    if (!skillId.value || !skill.value?.editable) return
    await reloadVersions()
  }
)

onMounted(() => {
  void loadPage()
})
</script>
