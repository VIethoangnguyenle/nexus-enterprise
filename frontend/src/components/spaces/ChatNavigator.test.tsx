import { screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, UUID_RE } from '../../test/chat-fixtures'
import { ChatNavigator } from './ChatNavigator'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => {
  const { fixtureApi } = await import('../../test/chat-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(fixtureApi) }
})

beforeEach(() => {
  routerState.params = {}
  routerState.pathname = '/channels'
})
afterEach(resetClient)

describe('ChatNavigator', () => {
  it('files DMs under "Tin nhắn trực tiếp" and spaces under "Nhóm"', async () => {
    renderWithClient(<ChatNavigator />)
    const dms = await screen.findByRole('list', { name: 'Tin nhắn trực tiếp' })
    const spaces = screen.getByRole('list', { name: 'Nhóm' })

    // DMs are titled after the other person, by display name.
    await waitFor(() => expect(within(dms).getByText('Phạm Hải Yến')).toBeInTheDocument())
    expect(within(dms).getByText('Nguyễn Thu Lan')).toBeInTheDocument()
    expect(within(dms).queryByText('Đối soát giao dịch')).toBeNull()

    expect(within(spaces).getByText('Đối soát giao dịch')).toBeInTheDocument()
    expect(within(spaces).getByText('Vận hành chung')).toBeInTheDocument()
    expect(within(spaces).getByText('Hợp đồng đối tác')).toBeInTheDocument()
    expect(within(spaces).queryByText('Phạm Hải Yến')).toBeNull()
  })

  it('marks unread conversations bold with a count, and totals them on Trang chủ', async () => {
    renderWithClient(<ChatNavigator />)
    const spaces = await screen.findByRole('list', { name: 'Nhóm' })
    const row = await within(spaces).findByRole('link', { name: /Đối soát giao dịch/ })
    await waitFor(() => expect(row).toHaveAttribute('data-unread', 'true'))
    expect(within(row).getByText('3')).toBeInTheDocument()
    expect(row.className).toContain('font-semibold')

    const quiet = within(spaces).getByRole('link', { name: /Vận hành chung/ })
    expect(quiet).not.toHaveAttribute('data-unread', 'true')

    const home = screen.getByRole('link', { name: /Trang chủ/ })
    await waitFor(() => expect(within(home).getByText('4')).toBeInTheDocument())
  })

  it('collapses a section from its header', async () => {
    const user = userEvent.setup()
    renderWithClient(<ChatNavigator />)
    const spaces = await screen.findByRole('list', { name: 'Nhóm' })
    await within(spaces).findByText('Đối soát giao dịch')
    const toggle = screen.getByRole('button', { name: 'Nhóm' })
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await waitFor(() => expect(screen.queryByText('Đối soát giao dịch')).toBeNull())
  })

  it('marks the open conversation as the current page', async () => {
    routerState.params = { channelId: CH.vanhanh }
    routerState.pathname = `/channels/${CH.vanhanh}`
    renderWithClient(<ChatNavigator />)
    const row = await screen.findByRole('link', { name: /Vận hành chung/ })
    expect(row).toHaveAttribute('aria-current', 'page')
  })

  it('offers "Nhắn tin trực tiếp" and "Tạo nhóm" from "Trò chuyện mới"', async () => {
    const user = userEvent.setup()
    renderWithClient(<ChatNavigator />)
    await user.click(screen.getByRole('button', { name: 'Trò chuyện mới' }))
    const menu = await screen.findByRole('menu')
    expect(within(menu).getByRole('menuitem', { name: 'Nhắn tin trực tiếp' })).toBeInTheDocument()
    expect(within(menu).getByRole('menuitem', { name: 'Tạo nhóm' })).toBeInTheDocument()
    // Browse spaces needs a backend that does not exist yet: hidden, not stubbed.
    expect(within(menu).queryByText('Duyệt nhóm')).toBeNull()
  })

  it('never renders an identifier', async () => {
    renderWithClient(<ChatNavigator />)
    await screen.findByText('Phạm Hải Yến')
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })
})
