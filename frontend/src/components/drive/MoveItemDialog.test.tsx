import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { D, ITEMS, WS_ID, driveFixtureApi, calls } from '../../test/drive-fixtures'
import { apiFetch } from '../../api/client'
import { MoveItemDialog } from './MoveItemDialog'

vi.mock('../../api/client', async (orig) => {
  const { driveFixtureApi } = await import('../../test/drive-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(driveFixtureApi) }
})

beforeEach(() => {
  calls.length = 0
  vi.mocked(apiFetch).mockImplementation(driveFixtureApi)
})
afterEach(() => resetClient())

const props = { workspaceId: WS_ID, pending: false, onConfirm: vi.fn(), onClose: vi.fn() }
const confirm = () => screen.getByRole('button', { name: 'Di chuyển vào đây' })

describe('MoveItemDialog', () => {
  it('forgets the folder picked for one item when it opens for the next', async () => {
    const user = userEvent.setup()
    const { rerender } = renderWithClient(<MoveItemDialog {...props} item={ITEMS[D.xlsx]!} />)
    const tree = await screen.findByRole('tree', { name: 'Chọn thư mục' })
    await user.click(await within(tree).findByText('Hợp đồng'))
    expect(confirm()).toBeEnabled()

    rerender(<MoveItemDialog {...props} item={null} />)
    rerender(<MoveItemDialog {...props} item={ITEMS[D.bienban]!} />)
    expect(confirm()).toBeDisabled()
  })

  it('never confirms a move of a folder into itself, even if it was the stale pick', async () => {
    const user = userEvent.setup()
    const { rerender } = renderWithClient(<MoveItemDialog {...props} item={ITEMS[D.xlsx]!} />)
    const tree = await screen.findByRole('tree', { name: 'Chọn thư mục' })
    await user.click(await within(tree).findByText('Hợp đồng'))

    // Straight to the folder that was picked, without closing in between.
    rerender(<MoveItemDialog {...props} item={ITEMS[D.hopdong]!} />)
    expect(confirm()).toBeDisabled()
    expect(within(screen.getByRole('tree', { name: 'Chọn thư mục' })).queryByText('Hợp đồng')).toBeNull()
  })

  it('does not offer the folder the item is already in', async () => {
    const user = userEvent.setup()
    renderWithClient(<MoveItemDialog {...props} item={ITEMS[D.xlsx]!} />)
    const tree = await screen.findByRole('tree', { name: 'Chọn thư mục' })
    await user.click(await within(tree).findByText('Đối soát'))
    expect(confirm()).toBeDisabled()
  })
})
