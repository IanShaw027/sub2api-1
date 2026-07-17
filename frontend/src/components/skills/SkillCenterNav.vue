<template>
  <section class="overflow-hidden rounded-[28px] border border-line bg-gradient-to-r from-dark-900 via-brand-950 to-accent-950 text-white shadow-xs dark:border-dark-700">
    <div class="flex flex-col gap-6 px-6 py-6 lg:flex-row lg:items-end lg:justify-between">
      <div class="max-w-3xl">
        <p class="text-xs uppercase tracking-[0.35em] text-white/55">{{ t('skills.center.label', 'Skill Center') }}</p>
        <h1 class="mt-2 text-2xl font-bold">{{ headerTitle }}</h1>
        <p class="mt-2 text-sm text-white/70">{{ headerSubtitle }}</p>
      </div>
      <div class="flex flex-wrap gap-3">
        <RouterLink :to="skillPaths.market" class="btn btn-secondary border-white/15 bg-white/10 text-white hover:bg-white/15">
          <Icon name="grid" size="sm" class="mr-2" />
          {{ t('skills.market.title', '技能市场') }}
        </RouterLink>
        <RouterLink :to="skillPaths.create" class="btn btn-primary">
          <Icon name="plus" size="sm" class="mr-2" />
          {{ t('skills.editor.create', '创建技能') }}
        </RouterLink>
      </div>
    </div>

    <div class="border-t border-white/10 px-3 py-3">
      <div class="flex flex-wrap gap-2">
        <RouterLink
          v-for="item in navItems"
          :key="item.key"
          :to="item.to"
          class="inline-flex items-center rounded-full border px-3 py-2 text-sm font-medium transition-colors"
          :class="item.key === active
            ? 'border-white/20 bg-card text-ink'
            : 'border-white/10 bg-white/5 text-white/80 hover:bg-white/10 hover:text-white'"
        >
          <Icon :name="item.icon" size="sm" class="mr-2" />
          {{ item.label }}
        </RouterLink>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { skillPaths } from './paths'

type SkillNavKey = 'market' | 'installed' | 'my' | 'editor' | 'detail' | 'versions' | 'runs' | 'revenue'

interface Props {
  active: SkillNavKey
  skillId?: number | null
  canEditSkill?: boolean
  canViewRuns?: boolean
  canViewRevenue?: boolean
  showRevenue?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  skillId: null,
  canEditSkill: false,
  canViewRuns: false,
  canViewRevenue: false,
  showRevenue: false
})

const { t } = useI18n()

const headerTitle = computed(() => {
  switch (props.active) {
    case 'my':
      return t('skills.my.title', '我的技能')
    case 'installed':
      return t('skills.installed.title', '已安装技能')
    case 'editor':
      return t('skills.editor.title', '技能编辑器')
    case 'detail':
      return t('skills.detail.title', '技能详情')
    case 'versions':
      return t('skills.versions.title', '版本管理')
    case 'runs':
      return t('skills.runs.title', '运行记录')
    case 'revenue':
      return t('skills.revenue.title', '收益页')
    case 'market':
    default:
      return t('skills.market.title', '技能市场')
  }
})

const headerSubtitle = computed(() => {
  switch (props.active) {
    case 'my':
      return t('skills.my.subtitle', '查看你创建、维护或已安装的技能，快速进入编辑、版本和收益管理。')
    case 'installed':
      return t('skills.installed.subtitle', '集中查看你已经安装的技能，便于继续进入详情、版本和运行操作。')
    case 'editor':
      return t('skills.editor.subtitle', '支持 prompt_chat、prompt_image、script 三种类型，并可编辑变量 schema。')
    case 'detail':
      return t('skills.detail.subtitle', '收费技能默认隐藏源内容，只展示变量表单和必要元信息。')
    case 'versions':
      return t('skills.versions.subtitle', '管理发布节奏、版本说明、当前版本和可见性。')
    case 'runs':
      return t('skills.runs.subtitle', '按技能查看最近执行历史、运行状态、耗时和产出摘要。')
    case 'revenue':
      return t('skills.revenue.subtitle', '查看技能销售、执行带来的收益，以及订单明细。')
    case 'market':
    default:
      return t('skills.market.subtitle', '浏览技能市场、安装技能、查看详情，并为后续运行准备变量输入。')
  }
})

const navItems = computed(() => {
  const canViewRevenue = props.canViewRevenue || props.showRevenue
  const items: Array<{ key: SkillNavKey; to: string; label: string; icon: 'grid' | 'user' | 'edit' | 'book' | 'sync' | 'clock' | 'dollar' }> = [
    { key: 'market', to: skillPaths.market, label: t('skills.market.title', '技能市场'), icon: 'grid' },
    { key: 'installed', to: skillPaths.installed, label: t('skills.installed.title', '已安装技能'), icon: 'grid' },
    { key: 'my', to: skillPaths.my, label: t('skills.my.title', '我的技能'), icon: 'user' },
  ]

  if (!props.skillId || props.canEditSkill) {
    items.push({ key: 'editor', to: props.skillId ? skillPaths.edit(props.skillId) : skillPaths.create, label: t('skills.editor.title', '技能编辑器'), icon: 'edit' })
  }

  if (props.skillId) {
    items.push(
      { key: 'detail', to: skillPaths.detail(props.skillId), label: t('skills.detail.title', '技能详情'), icon: 'book' }
    )

    if (props.canEditSkill) {
      items.push(
        { key: 'versions', to: skillPaths.versions(props.skillId), label: t('skills.versions.title', '版本管理'), icon: 'sync' }
      )
    }

    if (props.canViewRuns) {
      items.push({ key: 'runs', to: skillPaths.runs(props.skillId), label: t('skills.runs.title', '运行记录'), icon: 'clock' })
    }

    if (canViewRevenue) {
      items.push({ key: 'revenue', to: skillPaths.revenue(props.skillId), label: t('skills.revenue.title', '收益页'), icon: 'dollar' })
    }
  }

  return items
})
</script>
