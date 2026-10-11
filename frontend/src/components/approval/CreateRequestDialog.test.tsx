import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { resetFixtures } from '../../test/approval-fixtures'
import { CreateRequestDialog } from './CreateRequestDialog'

vi.mock('../../api/client', async (orig) => {
  const { approvalFixtureApi } = await import('../../test/approval-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(approvalFixtureApi) }
})

beforeEach(() => {
  resetFixtures()
  resetClient()
})
afterEach(() => vi.clearAllMocks())

describe('CreateRequestDialog', () => {
  // A chat posts a card for the request it just made, so the caller needs the request itself.
  it('hands the created request to onCreated', async () => {
    const user = userEvent.setup()
    const onCreated = vi.fn()
    renderWithClient(<CreateRequestDialog open onClose={() => {}} onCreated={onCreated} />)
    const dialog = await screen.findByRole('dialog', { name: 'Tạo đề nghị' })
    const select = await within(dialog).findByRole('combobox', { name: 'Loại đề nghị' })
    await waitFor(() => expect(within(select).getAllByRole('option').length).toBeGreaterThan(1))
    await user.selectOptions(select, 'Tạm ứng')
    await user.type(await within(dialog).findByRole('textbox', { name: /Lý do/ }), 'Công tác')
    await user.type(within(dialog).getByRole('textbox', { name: /Số tiền/ }), '1000')
    await user.click(within(dialog).getByRole('button', { name: 'Gửi đề nghị' }))

    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1))
    expect(onCreated.mock.calls[0]![0]).toMatchObject({ id: '55555555-aaaa-4bbb-8ccc-0000000000ff' })
  })
})
