import '@testing-library/jest-dom/vitest'
import { useState, type ReactNode } from 'react'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { apiFetch, logoutSession } from '../../api/client'
import { Button, Pressable, SearchField } from '../primitives'
import { PhoneSearchBar } from './PhoneSearchBar'
import { answer, dialogOf, renderShell, resetServer, server, viewport } from './phone-shell-harness'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  logoutSession: vi.fn(),
}))
const api = vi.mocked(apiFetch)

/** The shell's arrangement: the bar above the screen it watches. */
function Shell({ body }: { body: ReactNode }) {
  const [el, setEl] = useState<HTMLElement | null>(null)
  return (
    <>
      <PhoneSearchBar scope={el} />
      <div ref={setEl}>{body}</div>
    </>
  )
}

const bar = () => screen.findByRole('banner')

beforeEach(() => {
  viewport(true)
  resetServer()
  api.mockReset()
  api.mockImplementation(answer as typeof apiFetch)
  vi.mocked(logoutSession).mockClear()
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

describe('where it shows', () => {
  it('draws the menu, the hint of the module and the avatar on a list screen', async () => {
    await renderShell(<Shell body={<h1>Phê duyệt</h1>} />, '/approval')
    const header = await bar()
    expect(within(header).getByRole('button', { name: /Mở menu chính/ })).toHaveAttribute('aria-haspopup', 'dialog')
    expect(within(header).getByText('Phê duyệt')).toBeInTheDocument()
    expect(within(header).getByRole('button', { name: 'Tài khoản của bạn' })).toBeInTheDocument()
  })

  it('steps aside inside a conversation and a document, which carry their own header', async () => {
    await renderShell(<Shell body={<h1>Đối soát</h1>} />, '/channels/99999999-aaaa-4bbb-8ccc-000000000002')
    await screen.findByRole('heading', { name: 'Đối soát' })
    expect(screen.queryByRole('banner')).toBeNull()
    cleanup()
    await renderShell(<Shell body={<h1>Báo cáo</h1>} />, '/documents/99999999-aaaa-4bbb-8ccc-000000000003')
    await screen.findByRole('heading', { name: 'Báo cáo' })
    expect(screen.queryByRole('banner')).toBeNull()
  })

  it('is not drawn at desktop widths, and asks for nothing', async () => {
    viewport(false)
    await renderShell(<Shell body={<p>x</p>} />, '/approval')
    await screen.findByText('x')
    expect(screen.queryByRole('banner')).toBeNull()
    expect(api).not.toHaveBeenCalledWith('/approval/pending')
  })
})

describe('the search hint', () => {
  it.each([
    ['/drive', 'Tìm tài liệu'], ['/approval', 'Tìm đề nghị'], ['/assets', 'Tìm tài sản'],
    ['/contacts', 'Tìm người'], ['/settings', 'Tìm cài đặt'], ['/admin', 'Tìm cài đặt'], ['/channels', 'Tìm trong chat'],
  ])('on %s says "%s" once the screen has a search', async (url, hint) => {
    await renderShell(<Shell body={<Button data-module-search>Tìm</Button>} />, url)
    expect(await within(await bar()).findByRole('button', { name: hint })).toBeInTheDocument()
  })

  it('takes focus to the screen\'s own search box', async () => {
    const user = userEvent.setup()
    await renderShell(<Shell body={<SearchField label="Tìm tài sản" moduleSearch />} />, '/assets')
    await user.click(await within(await bar()).findByRole('button', { name: 'Tìm tài sản' }))
    expect(screen.getByRole('searchbox', { name: 'Tìm tài sản' })).toHaveFocus()
  })

  it('presses the search button of a screen whose search is a panel', async () => {
    const user = userEvent.setup()
    const open = vi.fn()
    await renderShell(<Shell body={<Button data-module-search onClick={open}>Tìm trong nhóm</Button>} />, '/channels')
    await user.click(await within(await bar()).findByRole('button', { name: 'Tìm trong chat' }))
    expect(open).toHaveBeenCalledTimes(1)
  })

  it('names the module, with no button, on a screen without a search of its own', async () => {
    await renderShell(<Shell body={<SearchField label="Tìm vai trò" />} />, '/approval')
    const header = await bar()
    expect(within(header).queryByRole('button', { name: 'Tìm đề nghị' })).toBeNull()
    expect(within(header).getByText('Phê duyệt')).toBeInTheDocument()
  })

  it('follows the screen: the hint becomes a button when a search appears, and a name when it goes', async () => {
    const user = userEvent.setup()
    function Swapper() {
      const [on, setOn] = useState(false)
      return (
        <>
          <Pressable onClick={() => setOn(!on)}>Đổi màn</Pressable>
          {on && <SearchField label="Tìm tài sản" moduleSearch />}
        </>
      )
    }
    await renderShell(<Shell body={<Swapper />} />, '/assets')
    const header = await bar()
    expect(within(header).queryByRole('button', { name: 'Tìm tài sản' })).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Đổi màn' }))
    await within(header).findByRole('button', { name: 'Tìm tài sản' })
    await user.click(screen.getByRole('button', { name: 'Đổi màn' }))
    await waitFor(() => expect(within(header).queryByRole('button', { name: 'Tìm tài sản' })).toBeNull())
  })
})

describe('the news dot on the menu button', () => {
  it('shows when a request waits for the person', async () => {
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    expect(await screen.findByRole('button', { name: 'Mở menu chính, có việc mới' })).toBeInTheDocument()
  })

  it('shows when a notification is unread', async () => {
    server.pending = 0
    server.notificationsUnread = 3
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    expect(await screen.findByRole('button', { name: 'Mở menu chính, có việc mới' })).toBeInTheDocument()
  })

  it('is off when nothing is new, and never counts approvals in a personal workspace', async () => {
    server.pending = 5
    server.personal = true
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    await waitFor(() => expect(api).toHaveBeenCalledWith('/workspaces'))
    expect(await screen.findByRole('button', { name: 'Mở menu chính' })).toBeInTheDocument()
    expect(api).not.toHaveBeenCalledWith('/approval/pending')
  })
})

describe('the layers it opens', () => {
  it('opens the menu from ☰ and returns focus to it on Esc', async () => {
    server.pending = 0
    const user = userEvent.setup()
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    const menu = await screen.findByRole('button', { name: 'Mở menu chính' })
    await user.click(menu)
    await dialogOf('Menu chính')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(menu).toHaveFocus()
  })

  it('opens the account sheet from the avatar, and signs out through it', async () => {
    const user = userEvent.setup()
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    await user.click(await screen.findByRole('button', { name: 'Tài khoản của bạn' }))
    const sheet = await dialogOf('Tài khoản')
    await user.click(within(sheet).getByRole('button', { name: 'Đăng xuất' }))
    expect(logoutSession).toHaveBeenCalledTimes(1)
  })

  it('opens the notifications screen from the menu, and Quay lại closes it', async () => {
    server.notificationsUnread = 1
    const user = userEvent.setup()
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    await user.click(await screen.findByRole('button', { name: /Mở menu chính/ }))
    const menu = await dialogOf('Menu chính')
    await user.click(within(menu).getByRole('button', { name: /Thông báo/ }))
    const notifications = await dialogOf('Thông báo')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Menu chính' })).toBeNull())
    await user.click(within(notifications).getByRole('button', { name: 'Quay lại' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })
})
