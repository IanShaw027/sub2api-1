import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import BackupView from '../BackupView.vue'

const api = vi.hoisted(() => ({ restoreBackup: vi.fn(), listBackups: vi.fn() }))
vi.mock('@/api', () => ({
  adminAPI: {
    backup: {
      getS3Config: vi.fn().mockResolvedValue({}),
      getImageStorageConfig: vi.fn().mockResolvedValue({ config: {}, secret_configured: false }),
      getSchedule: vi.fn().mockResolvedValue({ enabled: false, cron_expr: '', retain_days: 14, retain_count: 10 }),
      listBackups: api.listBackups,
      restoreBackup: api.restoreBackup,
    },
  },
}))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showWarning: vi.fn() }),
}))
vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ run: (action: () => unknown) => action() }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: (error: { code?: string }) => error.code === 'STEP_UP_CANCELLED',
  stepUpBlockReason: () => '',
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

const record = (id: string) => ({
  id,
  status: 'completed',
  backup_type: 'postgres',
  file_name: `${id}.sql.gz`,
  s3_key: `backups/${id}.sql.gz`,
  size_bytes: 10,
  triggered_by: 'manual',
  started_at: '2026-08-09T00:00:00Z',
})
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

let wrapper: VueWrapper
const dialog = () => new DOMWrapper(document.body.querySelector('[role="dialog"]')!)
const confirmButton = () => dialog().findAll('button').find(button => button.text() === 'admin.backup.actions.restore')!
async function openRestore(index = 0) {
  const buttons = wrapper.findAll('button').filter(button => button.text() === 'admin.backup.actions.restore')
  await buttons[index].trigger('click')
  await flushPromises()
}
async function submitPassword(password: string) {
  await dialog().get('input[type="password"]').setValue(password)
  await confirmButton().trigger('click')
  await flushPromises()
}

beforeEach(async () => {
  api.restoreBackup.mockReset().mockRejectedValue({ status: 400, message: 'Invalid password' })
  api.listBackups.mockResolvedValue({ items: [record('original'), record('new-target')] })
  wrapper = mount(BackupView, { global: { stubs: { TotpStepUpDialog: true } } })
  await flushPromises()
})
afterEach(() => {
  wrapper.unmount()
  document.body.innerHTML = ''
})

describe('BackupView restore lifecycle', () => {
  it.each([
    { status: 400, message: 'Invalid password' },
    { code: 'STEP_UP_CANCELLED' },
  ])('retains the backup ID when retrying after %j', async (error) => {
    api.restoreBackup.mockRejectedValueOnce(error)
    await openRestore()
    await submitPassword('wrong-password')
    expect(api.restoreBackup).toHaveBeenNthCalledWith(1, 'original', 'wrong-password')
    await submitPassword('correct-password')
    expect(api.restoreBackup).toHaveBeenNthCalledWith(2, 'original', 'correct-password')
  })

  it('ignores repeated Enter while restore confirmation is pending', async () => {
    const request = deferred<object>()
    api.restoreBackup.mockReturnValueOnce(request.promise)
    await openRestore()
    await submitPassword('password')
    await dialog().get('input').trigger('keydown', { key: 'Enter' })
    await dialog().get('input').trigger('keydown', { key: 'Enter' })
    expect(api.restoreBackup).toHaveBeenCalledTimes(1)
    request.reject(new Error('Request failed'))
    await flushPromises()
  })

  it.each(['success', 'failure'])('does not clear a new dialog after an older restore ends in %s', async (outcome) => {
    const request = deferred<ReturnType<typeof record>>()
    api.restoreBackup.mockReturnValueOnce(request.promise)
    await openRestore()
    await submitPassword('old-password')
    const cancel = dialog().findAll('button').find(button => button.text() === 'common.cancel')!
    await cancel.trigger('click')
    await flushPromises()
    await openRestore(1)
    await dialog().get('input').setValue('new-password')

    if (outcome === 'success') request.resolve(record('original'))
    else request.reject(new Error('Old request failed'))
    await flushPromises()
    expect(dialog().get('input').element.value).toBe('new-password')
    await confirmButton().trigger('click')
    await flushPromises()
    expect(api.restoreBackup).toHaveBeenLastCalledWith('new-target', 'new-password')
  })
})
