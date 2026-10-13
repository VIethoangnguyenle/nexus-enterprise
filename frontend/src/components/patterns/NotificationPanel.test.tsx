import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createRef } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { Pressable } from '../primitives'
import { apiFetch } from '../../api/client'
import type { NotificationPage } from '../../api/notifications'
import { switchPreferencesTo, updatePreferences } from '../../lib/preferences'
import { NotificationPanel } from './NotificationPanel'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

const at = (daysAgo: number, hour = 9) => new Date(Date.now() - daysAgo * 86_400_000 + (hour - 9) * 3_600_000).toISOString()
const N = (over: Record<string, unknown>) => ({
  id: 'n1', type: 'approval_approved', title: 'Request 6f1c0a4e approved', body: 'raw english body', read: false,
  created_at: at(0, 9), entity_type: 'approval', entity_id: '6f1c0a4e-1111-4222-8333-444455556666', ...over,
})
const PAGE: NotificationPage = {
  notifications: [
    N({ id: 'n1', created_at: new Date(Date.now() - 5 * 60_000).toISOString() }),
    N({ id: 'n2', type: 'asset_assigned', entity_type: 'asset', entity_id: '7a7a7a7a-1111-4222-8333-444455556666', created_at: new Date(Date.now() - 2 * 3_600_000).toISOString(), read: true }),
    N({ id: 'n3', type: 'asset_request_rejected', entity_type: 'asset_request', entity_id: 'q1', created_at: at(3) }),
  ],
  total: 3, unread_count: 2,
}

const onClose = vi.fn()
const anchor = createRef<HTMLButtonElement>()
const at0 = <T,>(xs: T[]) => xs[0] as T
const at1 = <T,>(xs: T[]) => xs[1] as T

async function renderPanel(open = true) {
  const root = createRootRoute({
    component: () => (
      <>
        <Pressable ref={anchor}>Thông báo</Pressable>
        <NotificationPanel open={open} onClose={onClose} anchorRef={anchor} sideRef={anchor} />
      </>
    ),
  })
  const pages = ['/approval', '/assets', '/channels', '/workspace-select'].map((path) => createRoute({ getParentRoute: () => root, path, component: () => null }))
  const router = createRouter({ routeTree: root.addChildren(pages), history: createMemoryHistory({ initialEntries: ['/channels'] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

function serve(page: NotificationPage | Error = PAGE) {
  api.mockImplementation((path: string, opts?: RequestInit) => {
    if (path.startsWith('/notifications?')) return page instanceof Error ? Promise.reject(page) : Promise.resolve(page)
    if (path === '/notifications/unread-count') return Promise.resolve({ count: page instanceof Error ? 0 : page.unread_count })
    if (opts?.method === 'POST') return Promise.resolve({ status: 'ok' })
    return Promise.reject(new Error(`unexpected ${path}`))
  })
}
const posts = () => api.mock.calls.filter(([, o]) => (o as RequestInit | undefined)?.method === 'POST').map(([p]) => p)

beforeEach(() => {
  api.mockReset()
  onClose.mockReset()
  localStorage.clear()
  switchPreferencesTo('user-a')
  updatePreferences({ motion: 'reduce' })
})
afterEach(() => {
  cleanup()
  resetClient()
  switchPreferencesTo(undefined)
})

describe('NotificationPanel', () => {
  it('lists the notifications by day, in words built from their type, with no id and no server text', async () => {
    serve()
    await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    expect(await within(panel).findByText('Một đề nghị của bạn đã được duyệt.')).toBeInTheDocument()
    const lists = within(panel).getAllByRole('list')
    expect(within(panel).getByText('Hôm nay')).toBeInTheDocument()
    expect(within(panel).getByText('Trước đó')).toBeInTheDocument()
    expect(within(at0(lists)).getAllByRole('listitem')).toHaveLength(2)
    expect(within(at1(lists)).getByText('Yêu cầu cấp tài sản của bạn đã bị từ chối.')).toBeInTheDocument()
    expect(within(panel).getAllByText('Phê duyệt').length).toBeGreaterThan(0)
    expect(panel.textContent).not.toMatch(/6f1c0a4e|7a7a7a7a|raw english|Request/)
  })

  it('marks the unread ones for screen readers and counts them in the header', async () => {
    serve()
    await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    await within(panel).findByText('Một đề nghị của bạn đã được duyệt.')
    expect(within(panel).getByText('2 chưa đọc')).toBeInTheDocument()
    expect(within(panel).getAllByText('Chưa đọc')).toHaveLength(2)
  })

  it('counts unread from the list it shows, not from a badge that may be older', async () => {
    api.mockImplementation((path: string) =>
      path.startsWith('/notifications?') ? Promise.resolve(PAGE)
      : path === '/notifications/unread-count' ? Promise.resolve({ count: 0 })
      : Promise.reject(new Error(`unexpected ${path}`)))
    await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    expect(await within(panel).findByText('2 chưa đọc')).toBeInTheDocument()
    expect(within(panel).getByRole('button', { name: 'Đánh dấu tất cả đã đọc' })).toBeEnabled()
  })

  it('opens the object the notification is about', async () => {
    serve()
    const router = await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    const link = (await within(panel).findByText('Một đề nghị của bạn đã được duyệt.')).closest('a')!
    expect(link.getAttribute('href')).toBe('/approval?request=6f1c0a4e-1111-4222-8333-444455556666')
    const asset = within(panel).getByText('Bạn được giao một tài sản.').closest('a')!
    expect(asset.getAttribute('href')).toBe('/assets?asset=7a7a7a7a-1111-4222-8333-444455556666')
    const request = within(panel).getByText('Yêu cầu cấp tài sản của bạn đã bị từ chối.').closest('a')!
    expect(request.getAttribute('href')).toBe('/assets?section=requests&request=q1')

    await userEvent.setup().click(link)
    await waitFor(() => expect(router.state.location.pathname).toBe('/approval'))
  })

  it('shows an invitation in words made of names and opens Chọn workspace', async () => {
    const invitationId = '5b5b5b5b-1111-4222-8333-444455556666'
    serve({
      notifications: [N({
        id: 'n9', type: 'workspace_invitation', title: 'Lời mời vào workspace', body: 'Lê Thị Hoa mời bạn vào NovaPay Kế toán',
        entity_type: 'invitation', entity_id: invitationId,
      })],
      total: 1, unread_count: 1,
    })
    const router = await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    const link = (await within(panel).findByText('Lê Thị Hoa mời bạn vào NovaPay Kế toán')).closest('a')!
    expect(link.getAttribute('href')).toBe('/workspace-select')
    expect(within(panel).getByText('Lời mời')).toBeInTheDocument()
    expect(panel.textContent).not.toContain(invitationId)

    await userEvent.setup().click(link)
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace-select'))
    await waitFor(() => expect(posts()).toEqual(['/notifications/n9/read']))
  })

  it('clicking an unread row marks it read and closes the panel; a read row only closes it', async () => {
    serve()
    await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    const user = userEvent.setup()
    await user.click((await within(panel).findByText('Một đề nghị của bạn đã được duyệt.')).closest('a')!)
    await waitFor(() => expect(posts()).toEqual(['/notifications/n1/read']))
    expect(onClose).toHaveBeenCalled()

    onClose.mockClear()
    await user.click(within(panel).getByText('Bạn được giao một tài sản.').closest('a')!)
    expect(posts()).toEqual(['/notifications/n1/read'])
    expect(onClose).toHaveBeenCalled()
  })

  it('has a button to mark one read without leaving, and it turns the row read at once', async () => {
    serve()
    await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    await within(panel).findByText('Một đề nghị của bạn đã được duyệt.')
    const mark = within(panel).getByRole('button', { name: 'Đánh dấu đã đọc: Một đề nghị của bạn đã được duyệt.' })
    await userEvent.setup().click(mark)
    await waitFor(() => expect(posts()).toEqual(['/notifications/n1/read']))
    expect(onClose).not.toHaveBeenCalled()
    await waitFor(() => expect(within(panel).getByText('1 chưa đọc')).toBeInTheDocument())
    expect(within(panel).queryByRole('button', { name: /Đánh dấu đã đọc: Một đề nghị/ })).toBeNull()
  })

  it('"Đánh dấu tất cả đã đọc" marks everything and is off when nothing is unread', async () => {
    serve()
    await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    await within(panel).findByText('Một đề nghị của bạn đã được duyệt.')
    const all = within(panel).getByRole('button', { name: 'Đánh dấu tất cả đã đọc' })
    expect(all).toBeEnabled()
    await userEvent.setup().click(all)
    await waitFor(() => expect(posts()).toEqual(['/notifications/read-all']))
    await waitFor(() => expect(within(panel).getByText('0 chưa đọc')).toBeInTheDocument())
    expect(all).toBeDisabled()
  })

  it('pages 25 at a time: Tải thêm and "Đã hiện X trong Y"', async () => {
    const more = N({ id: 'n4', created_at: at(5) })
    api.mockImplementation((path: string) => {
      if (path === '/notifications?limit=25&offset=0') return Promise.resolve({ ...PAGE, total: 4 })
      if (path === '/notifications?limit=25&offset=3') return Promise.resolve({ notifications: [more], total: 4, unread_count: 2 })
      if (path === '/notifications/unread-count') return Promise.resolve({ count: 2 })
      return Promise.reject(new Error(`unexpected ${path}`))
    })
    await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    expect(await within(panel).findByText('Đã hiện 3 trong 4')).toBeInTheDocument()
    await userEvent.setup().click(within(panel).getByRole('button', { name: 'Tải thêm' }))
    expect(await within(panel).findByText('Đã hiện 4 trong 4')).toBeInTheDocument()
    expect(within(panel).queryByRole('button', { name: 'Tải thêm' })).toBeNull()
  })

  it('shows a skeleton while loading', async () => {
    api.mockImplementation(() => new Promise(() => {}))
    await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    expect(within(panel).getByLabelText('Đang tải thông báo')).toHaveAttribute('aria-busy', 'true')
  })

  it('says so when there is nothing, and sends the person to their requests', async () => {
    serve({ notifications: [], total: 0, unread_count: 0 })
    const router = await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    expect(await within(panel).findByText('Chưa có thông báo nào')).toBeInTheDocument()
    expect(within(panel).getByRole('button', { name: 'Đánh dấu tất cả đã đọc' })).toBeDisabled()
    await userEvent.setup().click(within(panel).getByRole('button', { name: 'Xem đề nghị của bạn' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/approval'))
    expect(router.state.location.search).toMatchObject({ tab: 'mine' })
    expect(onClose).toHaveBeenCalled()
  })

  it('says what failed and retries', async () => {
    serve(new Error('boom'))
    await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    expect(await within(panel).findByText('Không tải được thông báo')).toBeInTheDocument()
    serve()
    await userEvent.setup().click(within(panel).getByRole('button', { name: 'Thử lại' }))
    expect(await within(panel).findByText('Một đề nghị của bạn đã được duyệt.')).toBeInTheDocument()
  })

  it('a notification that names nothing is not a link, but can still be read', async () => {
    serve({ notifications: [N({ id: 'n9', type: 'asset_lifecycle', entity_type: undefined, entity_id: undefined })], total: 1, unread_count: 1 })
    await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    const text = await within(panel).findByText('Bạn đã đổi trạng thái của một tài sản.')
    expect(text.closest('a')).toBeNull()
    await userEvent.setup().click(text.closest('button')!)
    await waitFor(() => expect(posts()).toEqual(['/notifications/n9/read']))
  })

  it('closes on Esc, on a press outside, and on Đóng; not on a press of its own button', async () => {
    serve()
    await renderPanel()
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onClose).toHaveBeenCalledTimes(1)
    fireEvent.pointerDown(document.body)
    expect(onClose).toHaveBeenCalledTimes(2)
    fireEvent.pointerDown(within(panel).getByRole('heading', { name: 'Thông báo' }))
    expect(onClose).toHaveBeenCalledTimes(2)
    fireEvent.pointerDown(screen.getByRole('button', { name: 'Thông báo' }))
    expect(onClose).toHaveBeenCalledTimes(2)
    await userEvent.setup().click(within(panel).getByRole('button', { name: 'Đóng thông báo' }))
    expect(onClose).toHaveBeenCalledTimes(3)
  })

  it('renders nothing and asks for nothing while closed', async () => {
    serve()
    await renderPanel(false)
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(api.mock.calls.some(([path]) => String(path).startsWith('/notifications?'))).toBe(false)
  })
})
