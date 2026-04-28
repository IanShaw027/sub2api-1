import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UsageCleanupDialog from '../UsageCleanupDialog.vue'

const { createCleanupTaskMock, listCleanupTasksMock, cancelCleanupTaskMock, showErrorMock, showSuccessMock } = vi.hoisted(() => ({
  createCleanupTaskMock: vi.fn(),
  listCleanupTasksMock: vi.fn(),
  cancelCleanupTaskMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
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

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock,
  }),
}))

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: {
    listCleanupTasks: listCleanupTasksMock,
    createCleanupTask: createCleanupTaskMock,
    cancelCleanupTask: cancelCleanupTaskMock,
  },
  default: {
    listCleanupTasks: listCleanupTasksMock,
    createCleanupTask: createCleanupTaskMock,
    cancelCleanupTask: cancelCleanupTaskMock,
  },
}))

const BaseDialogStub = {
  props: ['show', 'title', 'width'],
  emits: ['close'],
  template: '<div><slot /><slot name="footer" /></div>',
}

const ConfirmDialogStub = {
  name: 'ConfirmDialog',
  props: ['show', 'title', 'message', 'confirmText', 'danger'],
  emits: ['confirm', 'cancel'],
  template: '<div />',
}

const UsageFiltersStub = {
  props: ['modelValue', 'startDate', 'endDate', 'exporting', 'showActions'],
  emits: ['update:modelValue', 'update:startDate', 'update:endDate', 'change'],
  template: '<div />',
}

describe('admin UsageCleanupDialog', () => {
  beforeEach(() => {
    createCleanupTaskMock.mockReset()
    listCleanupTasksMock.mockReset()
    cancelCleanupTaskMock.mockReset()
    showErrorMock.mockReset()
    showSuccessMock.mockReset()

    listCleanupTasksMock.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 5,
    })
    createCleanupTaskMock.mockResolvedValue({})
    cancelCleanupTaskMock.mockResolvedValue({})
  })

  it('includes exclude_admin when creating cleanup task', async () => {
    const wrapper = mount(UsageCleanupDialog, {
      props: {
        show: false,
        filters: {
          exclude_admin: true,
        },
        startDate: '2026-04-01',
        endDate: '2026-04-24',
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: ConfirmDialogStub,
          Pagination: true,
          UsageFilters: UsageFiltersStub,
        },
      },
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    const confirmDialogs = wrapper.findAllComponents(ConfirmDialogStub)
    confirmDialogs[0].vm.$emit('confirm')
    await flushPromises()

    expect(createCleanupTaskMock).toHaveBeenCalledTimes(1)
    expect(createCleanupTaskMock).toHaveBeenCalledWith(
      expect.objectContaining({
        exclude_admin: true,
      })
    )
  })
})
