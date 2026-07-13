import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import type { BasePaginationResponse } from '@/types'
import { apiClient } from '@/api/client'
import {
  createSkill,
  createSkillVersion,
  getSkillDetail,
  getSkillRevenue,
  installSkill,
  listMySkills,
  listSkillMarket,
  listSkillRuns,
  listSkillVersions,
  publishSkillVersion,
  submitSkillVersion,
  uninstallSkill,
  updateSkill,
  updateSkillVersion
} from '@/api/skills'
import {
  createDefaultSkillContent,
  createEmptySkillDraft,
  type CreateSkillRequest,
  type CreateSkillVersionRequest,
  type SkillContent,
  type SkillDetail,
  type SkillEditorDraft,
  type SkillMarketFilters,
  type SkillMineFilters,
  type SkillRevenueDetail,
  type SkillRunActionResult,
  type SkillRunFilters,
  type SkillRunMode,
  type SkillRunRecord,
  type SkillSortKey,
  type SkillSummary,
  type SkillType,
  type SkillVariableSchemaItem,
  type SkillVersionPublishResult,
  type SkillVersionRecord,
  type SkillVersionSummary
} from '@/types/skills'

function createEmptyPagination<T>(pageSize = 18): BasePaginationResponse<T> {
  return {
    items: [],
    total: 0,
    page: 1,
    page_size: pageSize,
    pages: 1
  }
}

function cloneOptions<T>(value: T): T {
  return JSON.parse(JSON.stringify(value)) as T
}

function cloneContent(content: SkillContent): SkillContent {
  return cloneOptions(content)
}

function cloneVariableSchema(schema: SkillVariableSchemaItem[]): SkillVariableSchemaItem[] {
  return cloneOptions(schema)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function buildDraftFromDetail(detail: SkillDetail): SkillEditorDraft {
  return {
    id: detail.id,
    slug: detail.slug,
    name: detail.name,
    tagline: detail.tagline,
    description: detail.description,
    type: detail.type,
    visibility: detail.visibility,
    status: detail.status,
    category: detail.category ?? '',
    cover_image_url: detail.cover_image_url ?? '',
    tags: [...detail.tags],
    pricing: { ...detail.pricing },
    source_locked: detail.source_locked,
    variable_schema: cloneVariableSchema(detail.variable_schema),
    content: detail.content ? cloneContent(detail.content) : createDefaultSkillContent(detail.type),
    readme: detail.readme ?? '',
    install_note: detail.install_note ?? ''
  }
}

function assignPagination<T>(
  target: BasePaginationResponse<T>,
  next: BasePaginationResponse<T>
): void {
  target.items = next.items
  target.total = next.total
  target.page = next.page
  target.page_size = next.page_size
  target.pages = next.pages
}

function toVersionSummary(version: SkillVersionRecord): SkillVersionSummary {
  return {
    id: version.id,
    skill_id: version.skill_id,
    version: version.version,
    status: version.status,
    review_status: version.review_status,
    changelog: version.changelog,
    source_locked: version.source_locked,
    is_current: version.is_current,
    created_at: version.created_at,
    published_at: version.published_at ?? null,
    submitted_at: version.submitted_at ?? null,
    reviewed_at: version.reviewed_at ?? null,
    review_note: version.review_note ?? null,
    can_submit_review: version.can_submit_review,
    can_publish: version.can_publish,
    can_test: version.can_test,
    can_use: version.can_use
  }
}

export const useSkillsCenterStore = defineStore('skillsCenter', () => {
  const marketFilters = reactive<SkillMarketFilters>({
    search: '',
    type: 'all',
    price_mode: 'all',
    installed: 'all',
    category: 'all',
    sort: 'latest'
  })
  const mySkillFilters = reactive<SkillMineFilters>({
    search: '',
    type: 'all',
    visibility: 'all',
    status: 'all',
    sort: 'latest'
  })
  const runFilters = reactive<SkillRunFilters>({
    search: '',
    status: 'all',
    version_id: 'all'
  })

  const marketPagination = reactive<BasePaginationResponse<SkillSummary>>(createEmptyPagination<SkillSummary>(18))
  const mySkillsPagination = reactive<BasePaginationResponse<SkillSummary>>(createEmptyPagination<SkillSummary>(18))
  const versionsPagination = reactive<BasePaginationResponse<SkillVersionRecord>>(createEmptyPagination<SkillVersionRecord>(12))
  const runsPagination = reactive<BasePaginationResponse<SkillRunRecord>>(createEmptyPagination<SkillRunRecord>(20))

  const detail = ref<SkillDetail | null>(null)
  const revenue = ref<SkillRevenueDetail | null>(null)
  const editorDraft = ref<SkillEditorDraft>(createEmptySkillDraft())

  const loadingMarket = ref(false)
  const loadingMySkills = ref(false)
  const loadingDetail = ref(false)
  const loadingVersions = ref(false)
  const loadingRuns = ref(false)
  const loadingRevenue = ref(false)
  const loadingEditor = ref(false)
  const savingEditor = ref(false)
  const creatingVersion = ref(false)
  const updatingVersion = ref(false)
  const togglingInstall = ref(false)
  const submittingVersionId = ref<number | null>(null)
  const publishingVersionId = ref<number | null>(null)
  const runningVersionId = ref<number | null>(null)
  const runningMode = ref<SkillRunMode | null>(null)

  const detailCache = ref<Record<number, SkillDetail>>({})
  const revenueSkillId = ref<number | null>(null)
  const editorSkillId = ref<number | null>(null)
  const activeSkillId = ref<number | null>(null)

  let marketRequestId = 0
  let mySkillsRequestId = 0
  let detailRequestId = 0
  let versionsRequestId = 0
  let runsRequestId = 0
  let revenueRequestId = 0

  const allKnownSkills = computed(() => {
    const map = new Map<number, SkillSummary>()
    marketPagination.items.forEach((item) => map.set(item.id, item))
    mySkillsPagination.items.forEach((item) => map.set(item.id, item))
    return Array.from(map.values())
  })

  const availableCategories = computed(() => {
    const values = new Set<string>()
    allKnownSkills.value.forEach((item) => {
      if (item.category) values.add(item.category)
    })
    return Array.from(values).sort((a, b) => a.localeCompare(b))
  })

  const versionOptions = computed(() => [
    { value: 'all', label: 'All Versions' },
    ...versionsPagination.items.map((item) => ({
      value: item.id,
      label: item.version
    }))
  ])

  function upsertSkillCollections(next: SkillDetail | SkillSummary): void {
    const currentMarketIndex = marketPagination.items.findIndex((item) => item.id === next.id)
    if (currentMarketIndex >= 0) {
      marketPagination.items.splice(currentMarketIndex, 1, {
        ...marketPagination.items[currentMarketIndex],
        ...next
      })
    }

    const currentMineIndex = mySkillsPagination.items.findIndex((item) => item.id === next.id)
    if (currentMineIndex >= 0) {
      mySkillsPagination.items.splice(currentMineIndex, 1, {
        ...mySkillsPagination.items[currentMineIndex],
        ...next
      })
    } else if ('owned' in next && next.owned) {
      mySkillsPagination.items.unshift(next)
      mySkillsPagination.total += 1
    }
  }

  function writeCachedDetail(skillId: number, updater: (detail: SkillDetail) => SkillDetail): void {
    const cached = detailCache.value[skillId]
    if (!cached) return
    const next = updater(cached)
    detailCache.value = {
      ...detailCache.value,
      [skillId]: next
    }
    if (detail.value?.id === skillId) {
      detail.value = next
    }
    upsertSkillCollections(next)
  }

  function syncVersionIntoCache(skillId: number, version: SkillVersionRecord, forceLatest = false): void {
    const summary = toVersionSummary(version)
    writeCachedDetail(skillId, (cached) => ({
      ...cached,
      latest_version:
        forceLatest || cached.latest_version?.id === version.id || !cached.latest_version
          ? summary
          : cached.latest_version,
      current_version:
        version.is_current || cached.current_version?.id === version.id
          ? summary
          : cached.current_version
    }))
  }

  function replaceVersionRecord(versionId: number, next: SkillVersionRecord): void {
    versionsPagination.items = versionsPagination.items.map((item) => (item.id === versionId ? next : item))
  }

  function resetEditor(type: SkillType = 'prompt_chat'): void {
    editorSkillId.value = null
    editorDraft.value = createEmptySkillDraft(type)
  }

  function setEditorType(type: SkillType): void {
    if (editorDraft.value.type === type) return
    editorDraft.value = {
      ...editorDraft.value,
      type,
      content: createDefaultSkillContent(type)
    }
  }

  function setEditorTags(tags: string[]): void {
    editorDraft.value = {
      ...editorDraft.value,
      tags
    }
  }

  async function loadMarket(page = marketPagination.page, pageSize = marketPagination.page_size): Promise<void> {
    const requestId = ++marketRequestId
    loadingMarket.value = true
    try {
      const response = await listSkillMarket(page, pageSize, marketFilters)
      if (requestId !== marketRequestId) return
      assignPagination(marketPagination, response)
    } finally {
      if (requestId === marketRequestId) {
        loadingMarket.value = false
      }
    }
  }

  async function loadMySkills(page = mySkillsPagination.page, pageSize = mySkillsPagination.page_size): Promise<void> {
    const requestId = ++mySkillsRequestId
    loadingMySkills.value = true
    try {
      const response = await listMySkills(page, pageSize, mySkillFilters)
      if (requestId !== mySkillsRequestId) return
      assignPagination(mySkillsPagination, response)
    } finally {
      if (requestId === mySkillsRequestId) {
        loadingMySkills.value = false
      }
    }
  }

  async function loadSkillDetail(skillId: number, force = false): Promise<SkillDetail> {
    activeSkillId.value = skillId
    if (!force && detailCache.value[skillId]) {
      detail.value = detailCache.value[skillId]
      return detailCache.value[skillId]
    }

    const requestId = ++detailRequestId
    loadingDetail.value = true
    try {
      const response = await getSkillDetail(skillId)
      if (requestId !== detailRequestId) return response
      detailCache.value = {
        ...detailCache.value,
        [skillId]: response
      }
      detail.value = response
      upsertSkillCollections(response)
      return response
    } finally {
      if (requestId === detailRequestId) {
        loadingDetail.value = false
      }
    }
  }

  async function loadEditor(skillId?: number | null, force = false): Promise<SkillEditorDraft> {
    loadingEditor.value = true
    try {
      if (!skillId) {
        resetEditor()
        return editorDraft.value
      }

      const response = await loadSkillDetail(skillId, force)
      editorSkillId.value = response.id
      editorDraft.value = buildDraftFromDetail(response)
      return editorDraft.value
    } finally {
      loadingEditor.value = false
    }
  }

  async function saveEditor(): Promise<SkillDetail> {
    savingEditor.value = true
    try {
      const payload: CreateSkillRequest = {
        slug: editorDraft.value.slug.trim(),
        name: editorDraft.value.name.trim(),
        tagline: editorDraft.value.tagline.trim(),
        description: editorDraft.value.description.trim(),
        type: editorDraft.value.type,
        visibility: editorDraft.value.visibility,
        status: editorDraft.value.status,
        category: editorDraft.value.category.trim() || null,
        cover_image_url: editorDraft.value.cover_image_url.trim() || null,
        tags: editorDraft.value.tags,
        pricing: { ...editorDraft.value.pricing },
        source_locked: editorDraft.value.source_locked,
        variable_schema: cloneVariableSchema(editorDraft.value.variable_schema),
        content: cloneContent(editorDraft.value.content),
        readme: editorDraft.value.readme.trim() || null,
        install_note: editorDraft.value.install_note.trim() || null
      }

      const response = editorSkillId.value
        ? await updateSkill(editorSkillId.value, payload)
        : await createSkill(payload)

      editorSkillId.value = response.id
      activeSkillId.value = response.id
      detail.value = response
      detailCache.value = {
        ...detailCache.value,
        [response.id]: response
      }
      editorDraft.value = buildDraftFromDetail(response)
      upsertSkillCollections(response)
      return response
    } finally {
      savingEditor.value = false
    }
  }

  async function toggleInstall(skillId: number, installed: boolean): Promise<void> {
    togglingInstall.value = true
    try {
      const result = installed
        ? await uninstallSkill(skillId)
        : await installSkill(skillId)

      const nextInstalled = result.installed
      const nextInstallCount = result.install_count

      const cached = detailCache.value[skillId]
      if (cached) {
        const next = {
          ...cached,
          installed: nextInstalled,
          stats: {
            ...cached.stats,
            installs: nextInstallCount
          }
        }
        detailCache.value = {
          ...detailCache.value,
          [skillId]: next
        }
        if (detail.value?.id === skillId) {
          detail.value = next
        }
      }

      marketPagination.items = marketPagination.items.map((item) =>
        item.id === skillId
          ? {
              ...item,
              installed: nextInstalled,
              stats: {
                ...item.stats,
                installs: nextInstallCount
              }
            }
          : item
      )
      mySkillsPagination.items = mySkillsPagination.items.map((item) =>
        item.id === skillId
          ? {
              ...item,
              installed: nextInstalled,
              stats: {
                ...item.stats,
                installs: nextInstallCount
              }
            }
          : item
      )
    } finally {
      togglingInstall.value = false
    }
  }

  async function loadVersions(skillId: number, page = versionsPagination.page, pageSize = versionsPagination.page_size): Promise<void> {
    const requestId = ++versionsRequestId
    loadingVersions.value = true
    try {
      const response = await listSkillVersions(skillId, page, pageSize)
      if (requestId !== versionsRequestId) return
      assignPagination(versionsPagination, response)
    } finally {
      if (requestId === versionsRequestId) {
        loadingVersions.value = false
      }
    }
  }

  async function addVersion(skillId: number, payload: CreateSkillVersionRequest): Promise<SkillVersionRecord> {
    creatingVersion.value = true
    try {
      const response = await createSkillVersion(skillId, payload)
      versionsPagination.items.unshift(response)
      versionsPagination.total += 1
      syncVersionIntoCache(skillId, response, true)
      return response
    } finally {
      creatingVersion.value = false
    }
  }

  async function saveVersion(skillId: number, versionId: number, payload: Partial<CreateSkillVersionRequest>): Promise<SkillVersionRecord> {
    updatingVersion.value = true
    try {
      const response = await updateSkillVersion(skillId, versionId, payload)
      replaceVersionRecord(versionId, response)
      syncVersionIntoCache(skillId, response)
      return response
    } finally {
      updatingVersion.value = false
    }
  }

  async function submitVersion(skillId: number, versionId: number): Promise<SkillVersionRecord> {
    submittingVersionId.value = versionId
    try {
      const response = await submitSkillVersion(skillId, versionId)
      replaceVersionRecord(versionId, response)
      syncVersionIntoCache(skillId, response)
      await Promise.all([
        loadSkillDetail(skillId, true),
        loadVersions(skillId, versionsPagination.page, versionsPagination.page_size)
      ])
      return response
    } finally {
      submittingVersionId.value = null
    }
  }

  async function publishVersion(skillId: number, versionId: number): Promise<SkillVersionPublishResult> {
    publishingVersionId.value = versionId
    try {
      const response = await publishSkillVersion(versionId)
      if (response.version) {
        replaceVersionRecord(versionId, response.version)
        syncVersionIntoCache(skillId, response.version, true)
      }
      await Promise.all([
        loadSkillDetail(skillId, true),
        loadVersions(skillId, versionsPagination.page, versionsPagination.page_size)
      ])
      return response
    } finally {
      publishingVersionId.value = null
    }
  }

  async function resolveDefaultApiKeyId(): Promise<number | null> {
    try {
      const { data } = await apiClient.get('/keys', { params: { page: 1, page_size: 50 } })
      const items = Array.isArray(data?.items) ? data.items : Array.isArray(data) ? data : []
      const active = items.find((item: any) => item && (item.status === 'active' || item.status === 1 || item.status == null) && Number(item.id) > 0)
      const first = active || items.find((item: any) => Number(item?.id) > 0)
      return first ? Number(first.id) : null
    } catch {
      return null
    }
  }

  async function runSkillAction(
    mode: SkillRunMode,
    skillId: number,
    versionId?: number | null,
    parameters: Record<string, unknown> = {}
  ): Promise<SkillRunActionResult> {
    runningVersionId.value = versionId ?? 0
    runningMode.value = mode
    try {
      const apiKeyId = await resolveDefaultApiKeyId()
      if (mode === 'use' && !(apiKeyId && apiKeyId > 0)) {
        throw new Error('请先创建 API Key，再使用 Skill（用于上游 token 计费）')
      }
      const config = typeof versionId === 'number' && versionId > 0 ? { params: { version_id: versionId } } : undefined
      const { data } = await apiClient.post(
        mode === 'test' ? `/user/skills/${skillId}/test` : `/user/skills/${skillId}/use`,
        {
          parameters: cloneOptions(parameters),
          trace: apiKeyId && apiKeyId > 0 ? { api_key_id: apiKeyId } : undefined
        },
        config
      )
      await loadSkillDetail(skillId, true)

      const canLoadRuns = detailCache.value[skillId]?.owned ?? (detail.value?.id === skillId ? detail.value.owned : false)
      if (canLoadRuns) {
        await loadRuns(skillId, 1, runsPagination.page_size)
      }

      const source = isRecord(data) ? data : {}
      return {
        mode,
        skill_id: typeof source.skill_id === 'number' ? source.skill_id : skillId,
        version_id:
          typeof source.version_id === 'number'
            ? source.version_id
            : versionId ?? null,
        run_id:
          typeof source.run_id === 'number'
            ? source.run_id
            : typeof source.id === 'number'
              ? source.id
              : null,
        status: typeof source.status === 'string' ? source.status : null,
        raw: source
      }
    } finally {
      runningVersionId.value = null
      runningMode.value = null
    }
  }

  async function testSkillVersion(
    skillId: number,
    versionId?: number | null,
    parameters?: Record<string, unknown>
  ): Promise<SkillRunActionResult> {
    return runSkillAction('test', skillId, versionId, parameters)
  }

  async function useSkillVersion(
    skillId: number,
    versionId?: number | null,
    parameters?: Record<string, unknown>
  ): Promise<SkillRunActionResult> {
    return runSkillAction('use', skillId, versionId, parameters)
  }

  async function loadRuns(skillId: number, page = runsPagination.page, pageSize = runsPagination.page_size): Promise<void> {
    const requestId = ++runsRequestId
    loadingRuns.value = true
    try {
      const response = await listSkillRuns(skillId, page, pageSize, runFilters)
      if (requestId !== runsRequestId) return
      assignPagination(runsPagination, response)
    } finally {
      if (requestId === runsRequestId) {
        loadingRuns.value = false
      }
    }
  }

  async function loadRevenue(skillId: number, force = false): Promise<SkillRevenueDetail> {
    if (!force && revenueSkillId.value === skillId && revenue.value) {
      return revenue.value
    }
    const requestId = ++revenueRequestId
    revenueSkillId.value = skillId
    loadingRevenue.value = true
    try {
      const response = await getSkillRevenue(skillId)
      if (requestId !== revenueRequestId) return response
      revenue.value = response
      return response
    } finally {
      if (requestId === revenueRequestId) {
        loadingRevenue.value = false
      }
    }
  }

  function resetMarketFilters(): void {
    marketFilters.search = ''
    marketFilters.type = 'all'
    marketFilters.price_mode = 'all'
    marketFilters.installed = 'all'
    marketFilters.category = 'all'
    marketFilters.sort = 'latest'
  }

  function resetMySkillFilters(): void {
    mySkillFilters.search = ''
    mySkillFilters.type = 'all'
    mySkillFilters.visibility = 'all'
    mySkillFilters.status = 'all'
    mySkillFilters.sort = 'latest'
  }

  function resetRunFilters(): void {
    runFilters.search = ''
    runFilters.status = 'all'
    runFilters.version_id = 'all'
  }

  function updateEditorSort(sort: SkillSortKey): void {
    marketFilters.sort = sort
    mySkillFilters.sort = sort
  }

  return {
    marketFilters,
    mySkillFilters,
    runFilters,
    marketPagination,
    mySkillsPagination,
    versionsPagination,
    runsPagination,
    detail,
    revenue,
    editorDraft,
    loadingMarket,
    loadingMySkills,
    loadingDetail,
    loadingVersions,
    loadingRuns,
    loadingRevenue,
    loadingEditor,
    savingEditor,
    creatingVersion,
    updatingVersion,
    togglingInstall,
    submittingVersionId,
    publishingVersionId,
    runningVersionId,
    runningMode,
    editorSkillId,
    activeSkillId,
    allKnownSkills,
    availableCategories,
    versionOptions,
    loadMarket,
    loadMySkills,
    loadSkillDetail,
    loadEditor,
    saveEditor,
    toggleInstall,
    loadVersions,
    addVersion,
    saveVersion,
    submitVersion,
    publishVersion,
    testSkillVersion,
    useSkillVersion,
    loadRuns,
    loadRevenue,
    resetMarketFilters,
    resetMySkillFilters,
    resetRunFilters,
    resetEditor,
    setEditorType,
    setEditorTags,
    updateEditorSort
  }
})
