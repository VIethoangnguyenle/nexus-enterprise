import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, UUID_RE } from '../../test/chat-fixtures'
import { createChatStateServer } from '../../test/chat-state-server'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { useNavCollapseStore } from '../../stores/nav-collapse.store'
import { ChatNavigator } from './ChatNavigator'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

let server: ReturnType<typeof createChatStateServer>
const serve = (initial: Parameters<typeof createChatStateServer>[0] = {}, mentionsUnread = 0) => {
  server = createChatStateServer(initial, mentionsUnread)
  vi.mocked(apiFetch).mockImplementation(server.api as never)
}

beforeEach(() => {
  routerState.params = {}
  routerState.pathname = '/channels'
  serve()
})
afterEach(() => {
  resetClient()
  useToastStore.getState().clear()
  useNavCollapseStore.setState({ collapsed: {} })
})

const list = (name: string) => screen.queryByRole('list', { name })
const names = (name: string) => within(list(name)!).getAllByRole('link').map((a) => a.querySelector('.truncate')?.textContent)
const ready = async () => {
  renderWithClient(<ChatNavigator />)
  await screen.findByRole('list', { name: 'Nhóm' })
  await waitFor(() => expect(within(list('Nhóm')!).getByText('Đối soát giao dịch')).toBeInTheDocument())
}
const rowOf = (title: string) => screen.getByRole('link', { name: new RegExp(title) })

describe('the two shortcuts under Trang chủ', () => {
  it('lists Được nhắc tới and Đã gắn sao as links to their screens', async () => {
    await ready()
    expect(screen.getByRole('link', { name: /Được nhắc tới/ })).toHaveAttribute('href', '/channels/mentions')
    expect(screen.getByRole('link', { name: /Đã gắn sao/ })).toHaveAttribute('href', '/channels/starred')
  })

  it('puts the unread mentions in a pill, and nothing when there are none', async () => {
    serve({}, 4)
    await ready()
    const link = screen.getByRole('link', { name: /Được nhắc tới/ })
    await waitFor(() => expect(within(link).getByLabelText('4 tin nhắc tên bạn chưa đọc')).toHaveTextContent('4'))
  })

  it('has no pill when nobody mentioned me', async () => {
    await ready()
    const link = screen.getByRole('link', { name: /Được nhắc tới/ })
    expect(within(link).queryByLabelText(/nhắc tên bạn chưa đọc/)).toBeNull()
  })

  it('caps the pill at 99+, and gives Đã gắn sao no number at all', async () => {
    serve({}, 140)
    await ready()
    const link = screen.getByRole('link', { name: /Được nhắc tới/ })
    await waitFor(() => expect(within(link).getByLabelText('140 tin nhắc tên bạn chưa đọc')).toHaveTextContent('99+'))
    expect(within(screen.getByRole('link', { name: /Đã gắn sao/ })).queryByLabelText(/chưa đọc/)).toBeNull()
  })

  it('marks the open shortcut as the current page', async () => {
    routerState.pathname = '/channels/starred'
    await ready()
    expect(screen.getByRole('link', { name: /Đã gắn sao/ })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: /Được nhắc tới/ })).not.toHaveAttribute('aria-current')
    expect(screen.getByRole('link', { name: /Trang chủ/ })).not.toHaveAttribute('aria-current')
  })
})

describe('the Đã gắn sao section', () => {
  it('is not there while nothing is starred', async () => {
    await ready()
    expect(screen.queryByRole('button', { name: 'Đã gắn sao' })).toBeNull()
    expect(list('Đã gắn sao')).toBeNull()
  })

  it('lists starred conversations above the others, and only there', async () => {
    serve({ [CH.hopdong]: { starred: true }, [CH.dmLan]: { starred: true } })
    renderWithClient(<ChatNavigator />)
    const starred = await screen.findByRole('list', { name: 'Đã gắn sao' })
    await waitFor(() => expect(within(starred).getAllByRole('link')).toHaveLength(2))
    expect(names('Đã gắn sao').sort()).toEqual(['Hợp đồng đối tác', 'Nguyễn Thu Lan'].sort())
    const order = screen.getAllByRole('list').map((l) => l.getAttribute('aria-label'))
    expect(order).toEqual(['Đã gắn sao', 'Tin nhắn trực tiếp', 'Nhóm'])
    expect(names('Nhóm')).not.toContain('Hợp đồng đối tác')
    expect(names('Tin nhắn trực tiếp')).not.toContain('Nguyễn Thu Lan')
  })

  it('has no ⋯ on the header and no +, and its rows carry no star', async () => {
    serve({ [CH.hopdong]: { starred: true } })
    renderWithClient(<ChatNavigator />)
    await screen.findByRole('list', { name: 'Đã gắn sao' })
    expect(screen.queryByRole('button', { name: /Tuỳ chọn cho mục Đã gắn sao/ })).toBeNull()
    expect(screen.queryByRole('button', { name: /Đã gắn sao$/ })?.parentElement?.querySelector('[aria-label="Thêm"]')).toBeFalsy()
    const header = screen.getByRole('button', { name: 'Đã gắn sao' })
    expect(header.parentElement!.querySelectorAll('button')).toHaveLength(1)
    expect(within(rowOf('Hợp đồng đối tác')).queryByLabelText(/Đã gắn sao/)).toBeNull()
  })

  it('folds like the other sections and totals its unread while folded', async () => {
    const user = userEvent.setup()
    serve({ [CH.doisoat]: { starred: true } })
    renderWithClient(<ChatNavigator />)
    const header = await screen.findByRole('button', { name: 'Đã gắn sao' })
    await waitFor(() => expect(within(screen.getByRole('list', { name: 'Đã gắn sao' })).getAllByRole('link')).toHaveLength(1))
    await user.click(header)
    expect(header).toHaveAttribute('aria-expanded', 'false')
    expect(within(header).getByLabelText('3 tin chưa đọc')).toBeInTheDocument()
  })

  it('keeps a starred conversation out of the section it is filed under', async () => {
    serve({ [CH.hopdong]: { starred: true } })
    renderWithClient(<ChatNavigator />)
    await screen.findByRole('list', { name: 'Đã gắn sao' })
    expect(screen.getAllByText('Hợp đồng đối tác')).toHaveLength(1)
  })
})

describe('the row menu: star', () => {
  const openMenu = async (user: ReturnType<typeof userEvent.setup>, title: string) => {
    await user.click(await screen.findByRole('button', { name: `Tuỳ chọn cho ${title}` }))
  }

  it('offers Gắn sao before Tắt thông báo on a conversation that is not starred', async () => {
    const user = userEvent.setup()
    await ready()
    await openMenu(user, 'Vận hành chung')
    const items = screen.getAllByRole('menuitem').map((i) => i.textContent)
    expect(items.indexOf('Gắn sao')).toBeGreaterThanOrEqual(0)
    expect(items.indexOf('Gắn sao')).toBeLessThan(items.indexOf('Tắt thông báo'))
  })

  it('stars the conversation: the row leaves its list for Đã gắn sao and a toast offers Hoàn tác', async () => {
    const user = userEvent.setup()
    await ready()
    await openMenu(user, 'Vận hành chung')
    await user.click(screen.getByRole('menuitem', { name: 'Gắn sao' }))
    const starred = await screen.findByRole('list', { name: 'Đã gắn sao' })
    expect(within(starred).getByText('Vận hành chung')).toBeInTheDocument()
    expect(names('Nhóm')).not.toContain('Vận hành chung')
    expect(server.calls).toContainEqual(expect.objectContaining({ method: 'PUT', path: `/channels/${CH.vanhanh}/star` }))
    const toast = useToastStore.getState().toasts[0]!
    expect(toast.message).toBe('Đã gắn sao “Vận hành chung”')
    expect(toast.action!.label).toBe('Hoàn tác')
  })

  it('a starred conversation offers Bỏ gắn sao and the mute, and no way to move it to a section', async () => {
    const user = userEvent.setup()
    serve({ [CH.hopdong]: { starred: true } })
    renderWithClient(<ChatNavigator />)
    await screen.findByRole('list', { name: 'Đã gắn sao' })
    await openMenu(user, 'Hợp đồng đối tác')
    expect(screen.getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Bỏ gắn sao', 'Tắt thông báo'])
    expect(screen.queryByText('Chuyển tới mục')).toBeNull()
    expect(screen.queryByRole('menuitem', { name: /Mục mới/ })).toBeNull()
  })

  it('unstarring sends the row back to its list', async () => {
    const user = userEvent.setup()
    serve({ [CH.hopdong]: { starred: true } })
    renderWithClient(<ChatNavigator />)
    await screen.findByRole('list', { name: 'Đã gắn sao' })
    await openMenu(user, 'Hợp đồng đối tác')
    await user.click(screen.getByRole('menuitem', { name: 'Bỏ gắn sao' }))
    await waitFor(() => expect(names('Nhóm')).toContain('Hợp đồng đối tác'))
    expect(list('Đã gắn sao')).toBeNull()
    expect(useToastStore.getState().toasts[0]!.message).toBe('Đã bỏ gắn sao “Hợp đồng đối tác”')
  })

  it('puts the row back and says so when the server refuses the star', async () => {
    const user = userEvent.setup()
    await ready()
    server.failNext('/star')
    await openMenu(user, 'Vận hành chung')
    await user.click(screen.getByRole('menuitem', { name: 'Gắn sao' }))
    await waitFor(() => expect(useToastStore.getState().toasts.some((t) => t.tone === 'error')).toBe(true))
    expect(names('Nhóm')).toContain('Vận hành chung')
    expect(list('Đã gắn sao')).toBeNull()
  })
})

describe('muted conversations', () => {
  const mutedHopdong = { [CH.hopdong]: { notification_level: 'off', unread_count: 0, mention_count: 0 } as const }

  it('looks quiet: a bell-off at the end, no count, spoken as muted', async () => {
    serve({ [CH.hopdong]: { notification_level: 'off', unread_count: 0 } })
    await ready()
    const row = rowOf('Hợp đồng đối tác')
    expect(row).toHaveAttribute('data-muted', 'true')
    expect(row).not.toHaveAttribute('data-unread')
    expect(row.className).not.toContain('font-semibold')
    expect(within(row).getByText('đã tắt thông báo')).toHaveClass('sr-only')
    expect(row.querySelector('svg.lucide-bell-off')).not.toBeNull()
  })

  it('shows an @ pill when someone named me, and only that: a muted conversation counts no messages', async () => {
    serve({ [CH.hopdong]: { notification_level: 'off', unread_count: 0, mention_count: 1 } })
    await ready()
    const row = rowOf('Hợp đồng đối tác')
    expect(within(row).getByLabelText('1 lần nhắc tên bạn')).toHaveTextContent('@')
    expect(within(row).queryByLabelText(/tin chưa đọc/)).toBeNull()
  })

  it('stays quiet when even a stale count says it has unread messages', async () => {
    serve({ [CH.hopdong]: { notification_level: 'off', unread_count: 9 } })
    await ready()
    const row = rowOf('Hợp đồng đối tác')
    expect(row).not.toHaveAttribute('data-unread')
    expect(within(row).queryByText('9')).toBeNull()
  })

  it('totals Trang chủ by level: the muted conversation adds only its mentions', async () => {
    serve({ [CH.hopdong]: { notification_level: 'off', unread_count: 0, mention_count: 2 } })
    await ready()
    const home = screen.getByRole('link', { name: /Trang chủ/ })
    // doisoat 3 + dmYen 1 + 2 mentions of the muted one
    await waitFor(() => expect(within(home).getByLabelText('6 tin chưa đọc')).toBeInTheDocument())
  })

  it('counts only mentions on the header of a folded section too', async () => {
    const user = userEvent.setup()
    serve({
      [CH.doisoat]: { notification_level: 'off', unread_count: 0, mention_count: 1 },
    })
    renderWithClient(<ChatNavigator />)
    await screen.findByRole('list', { name: 'Nhóm' })
    const header = screen.getByRole('button', { name: 'Nhóm' })
    await user.click(header)
    await waitFor(() => expect(within(header).getByLabelText('1 tin chưa đọc')).toBeInTheDocument())
  })

  it('offers Tắt thông báo, then Bật thông báo once muted, each calling the level endpoint', async () => {
    const user = userEvent.setup()
    await ready()
    await user.click(screen.getByRole('button', { name: 'Tuỳ chọn cho Vận hành chung' }))
    await user.click(screen.getByRole('menuitem', { name: 'Tắt thông báo' }))
    await waitFor(() => expect(server.calls).toContainEqual(
      expect.objectContaining({ method: 'PUT', path: `/channels/${CH.vanhanh}/notification-level`, body: { level: 'off' } })))
    await waitFor(() => expect(rowOf('Vận hành chung')).toHaveAttribute('data-muted', 'true'))

    await user.click(screen.getByRole('button', { name: 'Tuỳ chọn cho Vận hành chung' }))
    expect(screen.queryByRole('menuitem', { name: 'Tắt thông báo' })).toBeNull()
    await user.click(screen.getByRole('menuitem', { name: 'Bật thông báo' }))
    await waitFor(() => expect(server.calls).toContainEqual(
      expect.objectContaining({ method: 'PUT', body: { level: 'main' } })))
    await waitFor(() => expect(rowOf('Vận hành chung')).not.toHaveAttribute('data-muted'))
  })

  it('never writes an identifier on the screen', async () => {
    serve({ ...mutedHopdong, [CH.dmLan]: { starred: true } }, 3)
    await ready()
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })
})
