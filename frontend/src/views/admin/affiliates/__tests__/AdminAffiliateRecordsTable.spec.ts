import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import { formatPaymentAmount } from '@/components/payment/currency'
import AdminAffiliateRecordsTable from '../AdminAffiliateRecordsTable.vue'

const {
  listInviteRecords,
  listRebateRecords,
  listTransferRecords,
  getUserOverview,
  affiliatesAPI,
} = vi.hoisted(() => {
  const listInviteRecords = vi.fn()
  const listRebateRecords = vi.fn()
  const listTransferRecords = vi.fn()
  const getUserOverview = vi.fn()
  return {
    listInviteRecords,
    listRebateRecords,
    listTransferRecords,
    getUserOverview,
    affiliatesAPI: {
      listInviteRecords,
      listRebateRecords,
      listTransferRecords,
      getUserOverview,
    },
  }
})

vi.mock('@/api/admin/affiliates', () => ({
  affiliatesAPI,
  default: affiliatesAPI,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
}
const IconStub = { template: '<span />' }
const PaginationStub = { template: '<div />' }
const BaseDialogStub = { props: ['show'], template: '<div v-if="show"><slot /></div>' }
const OrderStatusBadgeStub = { props: ['status'], template: '<span>{{ status }}</span>' }
const DataTableStub = {
  props: ['data'],
  template: `
    <div>
      <div v-for="row in data" :key="row.order_id ?? row.user_id" :data-test="'row-' + (row.order_id ?? row.user_id)">
        <slot name="cell-pay_amount" :row="row" />
      </div>
    </div>
  `,
}

function mountView() {
  return mount(AdminAffiliateRecordsTable, {
    props: {
      type: 'rebates',
    },
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: PaginationStub,
        BaseDialog: BaseDialogStub,
        Icon: IconStub,
        OrderStatusBadge: OrderStatusBadgeStub,
      },
    },
  })
}

describe('AdminAffiliateRecordsTable', () => {
  beforeEach(() => {
    listInviteRecords.mockReset()
    listRebateRecords.mockReset()
    listTransferRecords.mockReset()
    getUserOverview.mockReset()
  })

  it('formats rebate pay_amount with the order currency instead of a hardcoded yuan symbol', async () => {
    listRebateRecords.mockResolvedValueOnce({
      items: [
        {
          order_id: 301,
          out_trade_no: 'rebate-hkd-301',
          inviter_id: 11,
          inviter_email: 'inviter@example.com',
          inviter_username: 'inviter',
          invitee_id: 22,
          invitee_email: 'invitee@example.com',
          invitee_username: 'invitee',
          order_amount: 88,
          pay_amount: 103,
          currency: 'HKD',
          rebate_amount: 9,
          payment_type: 'stripe',
          order_status: 'COMPLETED',
          created_at: '2026-07-01T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain(formatPaymentAmount(103, 'HKD'))
    expect(wrapper.text()).not.toContain('¥103.00')
  })
})
