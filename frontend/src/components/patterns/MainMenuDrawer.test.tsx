import '@testing-library/jest-dom/vitest'
import { useState } from 'react'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { WS_ID } from '../../test/chat-fixtures'
import { apiFetch } from '../../api/client'
import { queryClient } from '../../lib/query-client'
import { useAuthStore } from '../../stores/auth.store'
import { useDriveStore } from '../../stores/drive.store'
import { Pressable } from '../primitives'
import { MainMenuDrawer } from './MainMenuDrawer'
import {
  GROUP, GROUP_2, OTHER_WS, answer, dialogOf, renderShell, resetServer, server, tokenFor, unreadRow, viewport,
} from './phone-shell-harness'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  logoutSession: vi.fn(),
}))
const api = vi.mocked(apiFetch)

const onNotifications = vi.fn()

function Host() {
  const [open, setOpen] = useState(false)
  return (
    <>
      <Pressable onClick={() => setOpen(true)}>Mở menu chính</Pressable>
      <MainMenuDrawer open={open} onClose={() => setOpen(false)} onOpenNotifications={onNotifications} />
    </>
  )
}

async function openDrawer(url = '/channels') {
  const router = await renderShell(<Host />, url)
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: 'Mở menu chính' }))
  const dialog = await dialogOf('Menu chính')
  return { router, user, dialog }
}

beforeEach(() => {
  viewport(true)
  resetServer()
  api.mockReset()
  api.mockImplementation(answer as typeof apiFetch)
  onNotifications.mockClear()
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
  useDriveStore.setState({ selectedItemId: null, expandedFolders: new Set() })
})

const labelsOf = (nav: HTMLElement) => within(nav).getAllByRole('link').map((a) => a.textContent)

describe('destinations', () => {
  it('lists Chat, Tài liệu, Phê duyệt, Tài sản, Danh bạ, then Thông báo, Quản trị, Cài đặt, with the person last', async () => {
    const { dialog } = await openDrawer()
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    const nav = within(dialog).getByRole('navigation', { name: 'Các phần của Nexus' })
    await within(nav).findByRole('link', { name: /Phê duyệt/ })
    expect(labelsOf(nav)).toEqual(['Chat', 'Tài liệu', 'Phê duyệt2 chờ bạn', 'Tài sản', 'Danh bạ', 'Quản trị', 'Cài đặt'])
    expect(within(nav).getByRole('button', { name: 'Thông báo' })).toBeInTheDocument()
    expect(await within(dialog).findByText(/Lê Thị Hoa/)).toBeInTheDocument()
  })

  it('leaves Phê duyệt, Tài sản and Quản trị out of a personal workspace, as the sidebar does', async () => {
    server.personal = true
    const { dialog } = await openDrawer()
    const nav = within(dialog).getByRole('navigation', { name: 'Các phần của Nexus' })
    await waitFor(() => expect(labelsOf(nav)).toEqual(['Chat', 'Tài liệu', 'Danh bạ', 'Cài đặt']))
    expect(api).not.toHaveBeenCalledWith('/approval/pending')
  })

  it('marks the module the person is in', async () => {
    const { dialog } = await openDrawer('/approval')
    expect(await within(dialog).findByRole('link', { name: /Phê duyệt/ })).toHaveAttribute('aria-current', 'page')
    expect(within(dialog).getByRole('link', { name: 'Chat' })).not.toHaveAttribute('aria-current')
  })

  it('lights Tài liệu on a document', async () => {
    const { dialog } = await openDrawer('/documents/99999999-aaaa-4bbb-8ccc-000000000001')
    expect(within(dialog).getByRole('link', { name: 'Tài liệu' })).toHaveAttribute('aria-current', 'page')
  })

  it('counts Chat by notification level, Phê duyệt waiting on you, and unread Thông báo, capped at 99+', async () => {
    server.unread = [
      unreadRow(GROUP, 3),
      unreadRow(GROUP_2, 5, { notification_level: 'off', mention_count: 1 }),
      unreadRow('c3', 7, { notification_level: 'off' }),
    ]
    server.notificationsUnread = 120
    const { dialog } = await openDrawer()
    expect(await within(dialog).findByRole('link', { name: 'Chat, 4 chưa đọc' })).toBeInTheDocument()
    expect(await within(dialog).findByRole('link', { name: 'Phê duyệt, 2 chờ bạn' })).toBeInTheDocument()
    const bell = await within(dialog).findByRole('button', { name: 'Thông báo, 120 chưa đọc' })
    expect(bell).toHaveTextContent('99+')
  })

  it('names the person with their role at the foot', async () => {
    const { dialog } = await openDrawer()
    expect(await within(dialog).findByText(/· Kế toán trưởng/)).toHaveTextContent('Lê Thị Hoa · Kế toán trưởng')
  })

  it('shows the Chat pill beside the Chat row', async () => {
    server.unread = [unreadRow(GROUP, 3)]
    const { dialog } = await openDrawer()
    const chat = await within(dialog).findByRole('link', { name: 'Chat, 3 chưa đọc' })
    expect(chat).toHaveTextContent('3')
  })

  it('shows no number when nothing waits', async () => {
    server.pending = 0
    const { dialog } = await openDrawer()
    await waitFor(() => expect(api).toHaveBeenCalledWith('/approval/pending'))
    expect(within(dialog).getByRole('navigation', { name: 'Các phần của Nexus' })).not.toHaveTextContent(/\d/)
  })

  it('goes to the chosen destination and closes', async () => {
    const { router, user, dialog } = await openDrawer()
    await user.click(within(dialog).getByRole('link', { name: 'Danh bạ' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/contacts'))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('closes and hands over to the notifications screen from Thông báo', async () => {
    const { user, dialog } = await openDrawer()
    await user.click(within(dialog).getByRole('button', { name: 'Thông báo' }))
    expect(onNotifications).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })
})

describe('as a modal layer', () => {
  it('puts focus on the workspace block, and gives it back to the opener on Esc', async () => {
    const { user, dialog } = await openDrawer()
    await waitFor(() => expect(within(dialog).getByRole('button', { name: /Đổi workspace/ })).toHaveFocus())
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('button', { name: 'Mở menu chính' })).toHaveFocus()
  })

  it('keeps Tab inside', async () => {
    const { user, dialog } = await openDrawer()
    await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true))
    for (let i = 0; i < 14; i++) {
      await user.tab()
      expect(dialog.contains(document.activeElement)).toBe(true)
    }
  })

  it('closes from the scrim', async () => {
    const { user } = await openDrawer()
    await user.click(screen.getByTestId('drawer-scrim'))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })
})

describe('switching workspace', () => {
  const openList = async (url?: string) => {
    const opened = await openDrawer(url)
    const entry = await within(opened.dialog).findByRole('button', { name: /Đổi workspace, đang ở/ })
    expect(entry).toHaveAttribute('aria-expanded', 'false')
    await opened.user.click(entry)
    expect(entry).toHaveAttribute('aria-expanded', 'true')
    const list = await within(opened.dialog).findByRole('listbox', { name: 'Workspace của bạn' })
    return { ...opened, list }
  }

  it('names the open workspace and its kind in the block', async () => {
    const { dialog } = await openDrawer()
    const entry = await within(dialog).findByRole('button', { name: /Đổi workspace, đang ở Khối Vận hành/ })
    expect(entry).toHaveTextContent('Khối Vận hành')
    await waitFor(() => expect(entry).toHaveTextContent('64 thành viên'))
  })

  it('says Workspace cá nhân for a personal one', async () => {
    server.personal = true
    const { dialog } = await openDrawer()
    const entry = await within(dialog).findByRole('button', { name: /Đổi workspace, đang ở/ })
    await waitFor(() => expect(entry).toHaveTextContent('Workspace cá nhân'))
  })

  it('offers only the workspaces the person can enter, the open one ticked', async () => {
    const { list } = await openList()
    const options = within(list).getAllByRole('option')
    expect(options.map((o) => o.getAttribute('aria-selected'))).toEqual(['true', 'false'])
    expect(options[0]).toHaveTextContent('Khối Vận hành')
    expect(options[1]).toHaveTextContent('NovaPay Kế toán')
    expect(within(list).queryByText('Chỉ xem được')).toBeNull()
  })

  it('re-scopes the token, drops the old data, resets Tài liệu and lands on Chat of the new workspace', async () => {
    const { router, user, list, dialog } = await openList('/drive')
    queryClient.setQueryData(['drive', 'old'], { items: ['x'] })
    useDriveStore.setState({ selectedItemId: 'item-1', expandedFolders: new Set(['f1']) })

    await user.click(within(list).getByRole('option', { name: /NovaPay Kế toán/ }))

    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(api).toHaveBeenCalledWith('/auth/switch-tenant', expect.objectContaining({ method: 'POST', body: JSON.stringify({ tenant_id: OTHER_WS }) }))
    expect(useAuthStore.getState().accessToken).toBe(tokenFor(OTHER_WS))
    expect(useAuthStore.getState().tenantId).toBe(OTHER_WS)
    expect(queryClient.getQueryData(['drive', 'old'])).toBeUndefined()
    expect(useDriveStore.getState().selectedItemId).toBeNull()
    expect(useDriveStore.getState().expandedFolders.size).toBe(0)
    expect(router.state.location.pathname).toBe('/channels')
    expect(router.state.location.search).toEqual({ ws: OTHER_WS })
    expect(dialog).not.toBeInTheDocument()
  })

  it('leaves everything as it was when the server refuses', async () => {
    server.switchError = Object.assign(new Error('forbidden'), { status: 403 })
    const { user, list } = await openList('/drive')
    queryClient.setQueryData(['drive', 'old'], { items: ['x'] })

    await user.click(within(list).getByRole('option', { name: /NovaPay Kế toán/ }))

    await waitFor(() => expect(api).toHaveBeenCalledWith('/auth/switch-tenant', expect.anything()))
    await waitFor(() => expect(within(list).getAllByRole('option')[1]).not.toBeDisabled())
    expect(useAuthStore.getState().accessToken).toBe(tokenFor(WS_ID))
    expect(queryClient.getQueryData(['drive', 'old'])).toBeDefined()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('does nothing but close when the open workspace is chosen', async () => {
    const { user, list } = await openList()
    await user.click(within(list).getAllByRole('option')[0]!)
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(api).not.toHaveBeenCalledWith('/auth/switch-tenant', expect.anything())
  })

  it('ends the list with Tạo workspace, which remembers the screen to come back to', async () => {
    const { dialog } = await openList()
    const createLink = within(dialog).getByRole('link', { name: /Tạo workspace/ })
    const href = new URL(createLink.getAttribute('href')!, 'http://app.test')
    expect(href.pathname).toBe('/onboarding')
    expect(href.searchParams.get('from')).toBe('/channels')
  })
})

describe('offers waiting', () => {
  it('dots the workspace block and lists Lời mời with the count; tapping it opens Chọn workspace', async () => {
    server.offers = 3
    const { router, user, dialog } = await openDrawer()
    const entry = await within(dialog).findByRole('button', { name: /Đổi workspace, đang ở Khối Vận hành, có lời mời/ })
    expect(within(entry).getByTestId('invitation-dot')).toBeInTheDocument()
    await user.click(entry)
    const row = await within(dialog).findByRole('link', { name: /Lời mời.*3 lời mời đang chờ/ })
    expect(row).toHaveTextContent('3')
    await user.click(row)
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace-select'))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Menu chính' })).toBeNull())
  })

  it('caps the pill at 99+', async () => {
    server.offers = 150
    const { user, dialog } = await openDrawer()
    await user.click(await within(dialog).findByRole('button', { name: /có lời mời/ }))
    expect(await within(dialog).findByText('99+')).toBeInTheDocument()
  })

  it('shows no dot and no row without offers', async () => {
    const { user, dialog } = await openDrawer()
    await user.click(await within(dialog).findByRole('button', { name: /Đổi workspace/ }))
    await waitFor(() => expect(api).toHaveBeenCalledWith('/invitations'))
    expect(within(dialog).queryByRole('link', { name: /Lời mời/ })).toBeNull()
    expect(screen.queryByTestId('invitation-dot')).toBeNull()
  })

  it('shows nothing and never asks when the address is not verified', async () => {
    server.offers = 2
    server.addressVerified = false
    const { user, dialog } = await openDrawer()
    await user.click(await within(dialog).findByRole('button', { name: /Đổi workspace/ }))
    await waitFor(() => expect(api).toHaveBeenCalledWith('/me'))
    expect(within(dialog).queryByRole('link', { name: /Lời mời/ })).toBeNull()
    expect(screen.queryByTestId('invitation-dot')).toBeNull()
    expect(api).not.toHaveBeenCalledWith('/invitations')
  })
})
