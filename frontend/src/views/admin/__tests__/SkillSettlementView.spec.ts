import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SkillSettlementView from '../SkillSettlementView.vue'

const { listSettlements, replaySettlement } = vi.hoisted(() => ({
  listSettlements: vi.fn(),
  replaySettlement: vi.fn(),
}))

const showError = vi.fn()
const showSuccess = vi.fn()

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh-CN' },
      t: (key: string, fallback?: string | Record<string, unknown>) =>
        typeof fallback === 'string' ? fallback : key,
    }),
  }
})

vi.mock('@/api/admin/skills', () => ({
  __esModule: true,
  default: {
    listSettlements,
    replaySettlement,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

const AppLayoutStub = defineComponent({
  name: 'AppLayoutStub',
  template: '<div><slot /></div>',
})

const TablePageLayoutStub = defineComponent({
  name: 'TablePageLayoutStub',
  template: `
    <div>
      <div data-test="filters"><slot name="filters" /></div>
      <div data-test="actions"><slot name="actions" /></div>
      <div data-test="table"><slot name="table" /></div>
      <div data-test="pagination"><slot name="pagination" /></div>
    </div>
  `,
})

const DataTableStub = defineComponent({
  name: 'DataTableStub',
  props: {
    data: {
      type: Array,
      default: () => [],
    },
  },
  template: `
    <div>
      <template v-for="row in data" :key="String(row.id)">
        <slot name="cell-actions" :row="row" />
      </template>
      <slot v-if="data.length === 0" name="empty" />
    </div>
  `,
})

function mountView() {
  return mount(SkillSettlementView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        EmptyState: true,
        Icon: true,
        Input: true,
        Pagination: true,
        Select: true,
        SkillAdminMetricGrid: true,
        SkillAdminStatusBadge: true,
        SkillAdminTimelineCard: true,
      },
    },
  })
}

describe('SkillSettlementView replay action', () => {
  beforeEach(() => {
    listSettlements.mockReset()
    replaySettlement.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
  })

  it('replays the selected rejected settlement from the detail panel', async () => {
    listSettlements.mockResolvedValue({
      items: [{
        id: 77,
        skill_id: 1,
        skill_name: 'Skill One',
        skill_slug: 'skill-one',
        author_name: 'Author',
        period_label: '2026-07',
        settlement_status: 'rejected',
        gross_amount: 12.5,
        platform_fee_amount: 2.5,
        payout_amount: 10,
        frozen_amount: 0,
        currency: 'CNY',
        note: 'credit down',
        created_at: '2026-07-01T00:00:00Z',
        updated_at: '2026-07-01T01:00:00Z',
      }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
      summary: {
        pending_amount: 0,
        settled_amount: 0,
        frozen_amount: 0,
        pending_skill_count: 0,
        currency: 'CNY',
      },
    })
    replaySettlement.mockResolvedValue({
      action: 'replay',
      message: 'replayed',
      status: 'settled',
      operated_at: '2026-07-01T02:00:00Z',
    })

    const wrapper = mountView()
    await flushPromises()

    const replayButton = wrapper.get('[data-test="settlement-replay-button"]')
    await replayButton.trigger('click')
    await flushPromises()

    expect(replaySettlement).toHaveBeenCalledWith(77, {})
    expect(showSuccess).toHaveBeenCalled()
    expect(listSettlements).toHaveBeenCalledTimes(2)
  })

  it('hides the replay action for non-rejected settlements', async () => {
    listSettlements.mockResolvedValue({
      items: [{
        id: 78,
        skill_id: 2,
        skill_name: 'Skill Two',
        skill_slug: 'skill-two',
        author_name: 'Author',
        period_label: '2026-07',
        settlement_status: 'settled',
        gross_amount: 12.5,
        platform_fee_amount: 2.5,
        payout_amount: 10,
        frozen_amount: 0,
        currency: 'CNY',
        note: '',
        created_at: '2026-07-01T00:00:00Z',
        updated_at: '2026-07-01T01:00:00Z',
      }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
      summary: {
        pending_amount: 0,
        settled_amount: 10,
        frozen_amount: 0,
        pending_skill_count: 0,
        currency: 'CNY',
      },
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-test="settlement-replay-button"]').exists()).toBe(false)
  })
})
