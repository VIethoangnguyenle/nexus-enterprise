import '@testing-library/jest-dom/vitest'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { createRef } from 'react'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { U, WS_ID } from '../../test/chat-fixtures'
import { N_ID, T_ID, notification, notificationServer, seed, type NotificationServer } from '../../test/notification-fixtures'
import { apiFetch } from '../../api/client'
import { queryClient } from '../../lib/query-client'
import { UNAVAILABLE, receiveNotification, registerNavigator, resetArrivals } from '../../lib/notification-arrivals'
import { MARK_FAILED } from '../../hooks/useNotifications'
import { keys } from '../../hooks/keys'
import { useAuthStore } from '../../stores/auth.store'
import { useNotificationUi } from '../../stores/notification.store'
import { useToastStore } from '../primitives/Toast'
import { Pressable } from '../primitives'
import { NotificationPanel } from './NotificationPanel'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

let server: NotificationServer
let closed = 0
let router: ReturnType<typeof createRouter>

async function renderPanel() {
  const anchor = createRef<HTMLButtonElement>()
  const root = createRootRoute({
    component: () => (
      <>
        <Pressable ref={anchor}>Thông báo</Pressable>
        <NotificationPanel open onClose={() => { closed++ }} anchorRef={anchor} rail={false} />
      </>
    ),
  })
  const pages = ['/approval', '/assets', '/workspace-select'].map((path) =>
    createRoute({ getParentRoute: () => root, path, validateSearch: (s: Record<string, unknown>) => s, component: () => null }))
  router = createRouter({ routeTree: root.addChildren(pages), history: createMemoryHistory({ initialEntries: ['/approval'] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return screen.findByRole('dialog', { name: 'Thông báo' })
}

beforeEach(() => {
  server = notificationServer()
  closed = 0
  api.mockReset()
  api.mockImplementation(((path: string, init?: RequestInit) => server.handler(path, init)) as typeof apiFetch)
  useAuthStore.setState({ tenantId: WS_ID })
  useNotificationUi.setState({ listVisible: false, fresh: {}, burst: null, announcement: '', openRequest: 0 })
  window.history.replaceState({}, '', '/approval')
})
afterEach(() => {
  cleanup()
  resetArrivals()
  registerNavigator(null)
  useToastStore.getState().clear()
  resetClient()
})

describe('the notification panel', () => {
  it('lists today and earlier apart, each saying who did what to which thing', async () => {
    const panel = await renderPanel()
    await within(panel).findByText('Hôm nay')
    expect(within(panel).getByText('Trước đó')).toBeInTheDocument()
    const today = within(panel).getByRole('region', { name: 'Hôm nay' })
    expect(within(today).getAllByRole('listitem')).toHaveLength(3)
    expect(within(today).getByText('Lê Quang Vinh')).toBeInTheDocument()
    expect(within(today).getByText('đã duyệt', { exact: false })).toBeInTheDocument()
    expect(within(today).getByText('Tạm ứng công tác phí tháng 10')).toBeInTheDocument()
    expect(within(today).getByText('Đề nghị đã hoàn tất.', { exact: false })).toBeInTheDocument()
    // The reason of a rejection sits in its own block.
    expect(within(today).getByText('Đã có màn hình dự phòng ở kho.')).toBeInTheDocument()
    // Domain and time on every row.
    expect(within(today).getAllByText('Phê duyệt').length).toBeGreaterThan(0)
    expect(within(today).getAllByText('Tài sản').length).toBeGreaterThan(0)
    expect(within(panel).getByText('3 chưa đọc')).toBeInTheDocument()
    expect(within(panel).getByText('Đã hiện 4 trong 4')).toBeInTheDocument()
  })

  it('never shows an identifier, a UUID being what every fixture id is', async () => {
    const panel = await renderPanel()
    await within(panel).findByText('Hôm nay')
    // Even the links the rows are carry the subject by query, never rendered text.
    expectNoIds(panel)
    expectNoIds(document.body)
  })

  it('does not show the server\'s text, and names an unnamed subject with a plain word', async () => {
    server.rows = [notification({ id: N_ID.approved, target_name: '', actor_name: '', actor_user_id: '' })]
    const panel = await renderPanel()
    expect(await within(panel).findByText('Một đề nghị'.toLowerCase(), { exact: false })).toBeInTheDocument()
    expectNoIds(document.body)
  })

  it('marks unread rows for everyone, with a dot and hidden text', async () => {
    const panel = await renderPanel()
    await within(panel).findByText('Hôm nay')
    expect(within(panel).getAllByText('Chưa đọc')).toHaveLength(3)
    const rows = within(panel).getAllByRole('listitem')
    expect(rows.filter((r) => r.hasAttribute('data-unread'))).toHaveLength(3)
  })

  it('marks one read at once, and the count drops', async () => {
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText('Hôm nay')
    await user.click(within(panel).getAllByRole('button', { name: 'Đánh dấu đã đọc' })[0]!)
    await within(panel).findByText('2 chưa đọc')
    expect(within(panel).getAllByText('Chưa đọc')).toHaveLength(2)
    expect(server.calls).toContain(`POST /notifications/${N_ID.approved}/read`)
  })

  it('puts a failed mark back and says what to do', async () => {
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText('Hôm nay')
    server.markFails = true
    await user.click(within(panel).getAllByRole('button', { name: 'Đánh dấu đã đọc' })[0]!)
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message)).toContain(MARK_FAILED))
    await within(panel).findByText('3 chưa đọc')
    expect(within(panel).getAllByText('Chưa đọc')).toHaveLength(3)
    expect(useToastStore.getState().toasts[0]?.action?.label).toBe('Thử lại')
  })

  it('marks all read, says how many, and then has nothing left to mark', async () => {
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText('Hôm nay')
    await user.click(within(panel).getByRole('button', { name: 'Đánh dấu tất cả đã đọc' }))
    await waitFor(() => expect(within(panel).queryByText('Chưa đọc')).toBeNull())
    expect(useToastStore.getState().toasts.map((t) => t.message)).toContain('Đã đánh dấu 3 thông báo là đã đọc')
    expect(within(panel).getByRole('button', { name: 'Đánh dấu tất cả đã đọc' })).toBeDisabled()
    expect(server.calls).toContain('POST /notifications/read-all')
  })

  it('puts everything back when marking all fails', async () => {
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText('Hôm nay')
    server.markFails = true
    await user.click(within(panel).getByRole('button', { name: 'Đánh dấu tất cả đã đọc' }))
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message)).toContain(MARK_FAILED))
    await within(panel).findByText('3 chưa đọc')
  })

  it('shows skeleton rows while loading', async () => {
    let release!: () => void
    api.mockImplementation(((path: string, init?: RequestInit) =>
      path.startsWith('/notifications?') ? new Promise<unknown>((r) => { release = () => r(server.handler(path, init)) }) : server.handler(path, init)) as typeof apiFetch)
    const panel = await renderPanel()
    expect(await within(panel).findByLabelText('Đang tải thông báo')).toHaveAttribute('aria-busy', 'true')
    expect(panel.querySelectorAll('[data-skeleton]')).toHaveLength(4)
    await act(async () => release())
    await within(panel).findByText('Hôm nay')
  })

  it('says it could not load, and tries again', async () => {
    server.listFails = true
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText('Không tải được thông báo. Kiểm tra kết nối rồi thử lại.')
    expect(within(panel).queryByText('Đã đọc hết')).toBeNull()
    expect(panel.textContent).not.toMatch(/500|request_id|boom/)
    server.listFails = false
    await user.click(within(panel).getByRole('button', { name: 'Thử lại' }))
    await within(panel).findByText('Hôm nay')
  })

  it('has an empty state that leads to the person\'s own requests', async () => {
    server.rows = []
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText(/Chưa có thông báo nào/)
    expect(within(panel).queryByText('Đã đọc hết')).toBeNull()
    await user.click(within(panel).getByRole('button', { name: 'Xem đề nghị của bạn' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/approval'))
    expect(router.state.location.search).toMatchObject({ tab: 'mine' })
    expect(closed).toBe(1)
  })

  it('pages with "Tải thêm", never infinite scroll', async () => {
    server.rows = Array.from({ length: 30 }, (_, i) => notification({ id: `88888888-aaaa-4bbb-8ccc-0000000001${String(i).padStart(2, '0')}`, read: true, target_name: `Đề nghị ${i + 1}` }))
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText('Đã hiện 25 trong 30')
    await user.click(within(panel).getByRole('button', { name: 'Tải thêm' }))
    await within(panel).findByText('Đã hiện 30 trong 30')
    expect(within(panel).queryByRole('button', { name: 'Tải thêm' })).toBeNull()
    expect(server.calls).toContain('GET /notifications?limit=25&offset=25')
  })
})

describe('opening a notification', () => {
  it('reads it, goes to its subject and closes the panel', async () => {
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText('Hôm nay')
    await user.click(within(panel).getByRole('link', { name: /Lê Quang Vinh đã duyệt/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/approval'))
    expect(router.state.location.search).toMatchObject({ tab: 'mine', request: T_ID.request })
    await waitFor(() => expect(server.calls).toContain(`POST /notifications/${N_ID.approved}/read`))
    expect(closed).toBe(1)
  })

  it('sends an asset to Tài sản', async () => {
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText('Hôm nay')
    await user.click(within(panel).getByRole('link', { name: /MacBook Pro 14 inch/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/assets'))
    expect(router.state.location.search).toMatchObject({ section: 'list', asset: T_ID.asset })
  })

  it('stays, still reads it, and explains when the subject is gone', async () => {
    server.gone.add(T_ID.asset)
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText('Hôm nay')
    await user.click(within(panel).getByRole('link', { name: /MacBook Pro 14 inch/ }))
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message)).toEqual([UNAVAILABLE]))
    expect(router.state.location.pathname).toBe('/approval')
    expect(closed).toBe(0)
    await waitFor(() => expect(server.calls).toContain(`POST /notifications/${N_ID.assigned}/read`))
  })

  it('takes an invitation to the workspace picker, which shows the offer', async () => {
    server.rows = [notification({
      id: N_ID.approved, type: 'workspace_invitation', target_type: 'workspace_invitation', target_id: T_ID.request,
      target_name: 'NovaPay Kế toán', workspace_id: '0b9a7d2f-3c4e-4f60-9bac-2d3e4f5a6b7c',
    })]
    const panel = await renderPanel()
    const user = userEvent.setup()
    const link = await within(panel).findByRole('link', { name: /Lê Quang Vinh đã mời bạn vào NovaPay Kế toán/ })
    await user.click(link)
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace-select'))
    // Nothing to fetch first: an offer has no detail record to check.
    expect(server.calls.filter((c) => c.startsWith('GET /approval') || c.startsWith('GET /assets'))).toEqual([])
    expect(server.calls).toContain(`POST /notifications/${N_ID.approved}/read`)
  })

  it('is a real link, so it can be opened in a new tab', async () => {
    const panel = await renderPanel()
    await within(panel).findByText('Hôm nay')
    const link = within(panel).getByRole('link', { name: /MacBook Pro 14 inch/ })
    expect(link.getAttribute('href')).toContain('/assets')
  })
})

describe('a notification arriving while the panel is open', () => {
  it('is inserted at the top with the author\'s wash and a "vừa …" tag, and stays unread', async () => {
    const panel = await renderPanel()
    await within(panel).findByText('Hôm nay')
    const id = '88888888-aaaa-4bbb-8ccc-0000000009aa'
    // The server stores it first; the push and the refetch follow.
    server.rows = [notification({
      id, type: 'approval_rejected', created_at: new Date().toISOString(), actor_user_id: U.yen.id,
      actor_name: 'Phạm Hải Yến', target_name: 'Mua bàn phím',
    }), ...server.rows]
    await act(async () => {
      receiveNotification({
        id, type: 'approval_rejected', targetType: 'approval', targetId: T_ID.request, actorUserId: U.yen.id,
        actorName: 'Phạm Hải Yến', targetName: 'Mua bàn phím', workspaceId: WS_ID, params: {},
      })
      await queryClient.invalidateQueries({ queryKey: keys.notifications.all() })
    })

    const today = await within(panel).findByRole('region', { name: 'Hôm nay' })
    const rows = within(today).getAllByRole('listitem')
    expect(rows[0]).toHaveTextContent('Phạm Hải Yến')
    expect(rows[0]).toHaveTextContent('vừa trả lại')
    expect(rows[0]!.className).toContain('rt-wash')
    expect(rows[0]).toHaveAttribute('data-unread')
    expect(useToastStore.getState().toasts).toHaveLength(0)
    expect(within(panel).getByRole('status')).toHaveTextContent('Thông báo mới: Phạm Hải Yến đã trả lại Mua bàn phím.')
  })

  it('replaces the tags with one summary line when three come within two seconds', async () => {
    const panel = await renderPanel()
    await within(panel).findByText('Hôm nay')
    const names = ['Lê Quang Vinh', 'Phạm Hải Yến', 'Đỗ Văn Khải']
    const ids = names.map((_, i) => `88888888-aaaa-4bbb-8ccc-0000000008a${i}`)
    server.rows = [
      ...names.map((name, i) => notification({ id: ids[i]!, actor_name: name, created_at: new Date().toISOString(), target_name: `Việc ${i + 1}` })),
      ...server.rows,
    ]
    await act(async () => {
      names.forEach((name, i) => receiveNotification({
        id: ids[i]!, type: 'approval_approved', targetType: 'approval', targetId: T_ID.request, actorUserId: U.vinh.id,
        actorName: name, targetName: `Việc ${i + 1}`, workspaceId: WS_ID, params: {},
      }))
      await queryClient.invalidateQueries({ queryKey: keys.notifications.all() })
    })
    await within(panel).findByText('3 thông báo mới · Lê Quang Vinh và 2 người khác')
    expect(within(panel).queryByText('vừa duyệt')).toBeNull()
    expect(useToastStore.getState().toasts).toHaveLength(0)
  })
})

describe('keyboard', () => {
  it('moves focus in, and Esc closes the panel and nothing else', async () => {
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText('Hôm nay')
    await waitFor(() => expect(within(panel).getByRole('button', { name: 'Đóng thông báo' })).toHaveFocus())
    await user.keyboard('{Escape}')
    expect(closed).toBe(1)
  })

  it('lets a person reach the mark button and the rows by Tab', async () => {
    const panel = await renderPanel()
    const user = userEvent.setup()
    await within(panel).findByText('Hôm nay')
    const seen = new Set<string>()
    for (let i = 0; i < 12; i++) {
      await user.tab()
      seen.add(document.activeElement?.getAttribute('aria-label') ?? document.activeElement?.textContent ?? '')
    }
    expect([...seen].some((t) => t === 'Đánh dấu đã đọc')).toBe(true)
    expect([...seen].some((t) => t.includes('Lê Quang Vinh'))).toBe(true)
    expect(panel.contains(document.activeElement)).toBe(true)
  })
})

describe('the seed', () => {
  it('is what the tests above assume', () => {
    expect(seed().filter((n) => !n.read)).toHaveLength(3)
  })
})
