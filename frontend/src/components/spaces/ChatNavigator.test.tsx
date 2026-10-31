import { screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, UUID_RE, WS_ID } from '../../test/chat-fixtures'
import { useNavCollapseStore } from '../../stores/nav-collapse.store'
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
afterEach(() => {
  resetClient()
  // Folded sections are remembered on the device; a test must not inherit the last one's.
  useNavCollapseStore.setState({ collapsed: {} })
})

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

  it('puts the search pill at the head of the column, above "Trò chuyện mới", where "/" also leads', async () => {
    const user = userEvent.setup()
    renderWithClient(<ChatNavigator />)
    const nav = await screen.findByRole('navigation', { name: 'Trò chuyện' })
    const search = within(nav).getByRole('combobox', { name: 'Tìm trong chat' })
    const create = within(nav).getByRole('button', { name: /Trò chuyện mới/ })
    expect(search.compareDocumentPosition(create) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    await user.keyboard('/')
    expect(search).toHaveFocus()
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

  it('on a wide screen a gear beside "Mới" opens Cài đặt Chat in the same workspace', async () => {
    const user = userEvent.setup()
    routerState.navigate.mockClear()
    renderWithClient(<ChatNavigator />)
    const nav = await screen.findByRole('navigation', { name: 'Trò chuyện' })
    const create = within(nav).getByRole('button', { name: 'Trò chuyện mới' })
    const gear = within(nav).getByRole('button', { name: 'Cài đặt Chat' })
    expect(gear).toHaveAttribute('title', 'Cài đặt Chat')
    expect(gear.parentElement).toBe(create.parentElement)
    await user.click(gear)
    expect(routerState.navigate).toHaveBeenCalledTimes(1)
    const call = routerState.navigate.mock.calls[0]?.[0] as {
      to: string; params: { section: string }; search: (prev: Record<string, unknown>) => unknown
    }
    expect(call.to).toBe('/settings/$section')
    expect(call.params).toEqual({ section: 'chat' })
    // Only the workspace follows; the open conversation stays behind.
    expect(call.search({ ws: WS_ID, thread: 'x' })).toEqual({ ws: WS_ID })
  })

  it('a phone has no gear: its ⋯ already leads there', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
    }))
    try {
      renderWithClient(<ChatNavigator />)
      const nav = await screen.findByRole('navigation', { name: 'Trò chuyện' })
      expect(within(nav).getByRole('button', { name: 'Trò chuyện mới' })).toBeInTheDocument()
      expect(within(nav).queryByRole('button', { name: 'Cài đặt Chat' })).toBeNull()
    } finally {
      vi.unstubAllGlobals()
    }
  })

  it('never renders an identifier', async () => {
    renderWithClient(<ChatNavigator />)
    await screen.findByText('Phạm Hải Yến')
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })
})
