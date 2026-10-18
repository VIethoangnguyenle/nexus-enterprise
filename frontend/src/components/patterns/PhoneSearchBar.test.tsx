import '@testing-library/jest-dom/vitest'
import { useState, type ReactNode } from 'react'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { WS_ID } from '../../test/chat-fixtures'
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
  it('draws the workspace tile, the hint of the module and the avatar on a list screen, and no ☰', async () => {
    await renderShell(<Shell body={<h1>Phê duyệt</h1>} />, '/approval')
    const header = await bar()
    const tile = await within(header).findByRole('button', { name: /^Đổi workspace, đang ở/ })
    expect(tile).toHaveAttribute('aria-haspopup', 'dialog')
    expect(within(header).queryByRole('button', { name: /menu chính/i })).toBeNull()
    expect(within(header).getByRole('button', { name: 'Tìm đề nghị' })).toBeInTheDocument()
    expect(within(header).getByRole('button', { name: 'Tôi' })).toBeInTheDocument()
  })

  it('steps aside on the settings pages, which are full screens with their own way back', async () => {
    await renderShell(<Shell body={<h1>Tôi</h1>} />, '/settings')
    await screen.findByRole('heading', { name: 'Tôi' })
    expect(screen.queryByRole('banner')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Tôi' })).toBeNull()
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
    ['/contacts', 'Tìm người'], ['/admin', 'Tìm cài đặt'],
  ])('on %s says "%s" once the screen has a search', async (url, hint) => {
    await renderShell(<Shell body={<Button data-module-search>Tìm</Button>} />, url)
    expect(await within(await bar()).findByRole('button', { name: hint })).toBeInTheDocument()
  })

  it('takes focus to the screen\'s own search box', async () => {
    const user = userEvent.setup()
    await renderShell(<Shell body={<SearchField label="Tìm cài đặt" moduleSearch />} />, '/admin')
    await user.click(await within(await bar()).findByRole('button', { name: 'Tìm cài đặt' }))
    expect(screen.getByRole('searchbox', { name: 'Tìm cài đặt' })).toHaveFocus()
  })

  it('presses the search button of a screen whose search is a panel', async () => {
    const user = userEvent.setup()
    const open = vi.fn()
    await renderShell(<Shell body={<Button data-module-search onClick={open}>Tìm trong nhóm</Button>} />, '/contacts')
    await user.click(await within(await bar()).findByRole('button', { name: 'Tìm người' }))
    expect(open).toHaveBeenCalledTimes(1)
  })

  describe('in Chat', () => {
    it('opens the Tìm trong chat screen as a new history entry, whatever the list below draws', async () => {
      const user = userEvent.setup()
      const screenSearch = vi.fn()
      const router = await renderShell(<Shell body={<Button data-module-search onClick={screenSearch}>Tìm trong nhóm</Button>} />, '/channels?tab=direct')
      await user.click(await within(await bar()).findByRole('button', { name: 'Tìm trong chat' }))
      expect(screenSearch).not.toHaveBeenCalled()
      await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'direct', search: 1 }))
      expect(router.state.location.state.chatOverlay).toBe(true)
      await act(async () => router.history.back())
      await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'direct' }))
    })

    it('is a button even when the list has no search of its own', async () => {
      await renderShell(<Shell body={<p>Trang chủ</p>} />, '/channels')
      expect(await within(await bar()).findByRole('button', { name: 'Tìm trong chat' })).toBeInTheDocument()
    })
  })

  it('names the module, with no button, on a screen without a search of its own', async () => {
    await renderShell(<Shell body={<SearchField label="Tìm vai trò" />} />, '/admin')
    const header = await bar()
    expect(within(header).queryByRole('button', { name: 'Tìm cài đặt' })).toBeNull()
    expect(within(header).getByText('Quản trị')).toBeInTheDocument()
  })

  describe('in Tài liệu, Phê duyệt and Tài sản', () => {
    it.each([
      ['/drive', 'Tìm tài liệu', 'Tài liệu'], ['/approval', 'Tìm đề nghị', 'Phê duyệt'], ['/assets', 'Tìm tài sản', 'Tài sản'],
    ])('on %s opens the one search, scoped to the module, whatever the screen draws', async (url, hint, scope) => {
      const user = userEvent.setup()
      const screenSearch = vi.fn()
      await renderShell(<Shell body={<Button data-module-search onClick={screenSearch}>Tìm</Button>} />, url)
      const pill = await within(await bar()).findByRole('button', { name: hint })
      expect(pill).toHaveAttribute('aria-haspopup', 'dialog')
      await user.click(pill)
      const search = await dialogOf('Tìm kiếm')
      expect(screenSearch).not.toHaveBeenCalled()
      expect(within(search).getByRole('searchbox', { name: `Tìm trong ${scope}` })).toHaveFocus()
      expect(within(search).getByRole('button', { name: scope, pressed: true })).toBeInTheDocument()
      await user.click(within(search).getByRole('button', { name: 'Đóng tìm kiếm' }))
      await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Tìm kiếm' })).toBeNull())
    })

    it('offers no Phê duyệt or Tài sản scope in a personal workspace', async () => {
      server.personal = true
      const user = userEvent.setup()
      await renderShell(<Shell body={<p>x</p>} />, '/drive')
      await user.click(await within(await bar()).findByRole('button', { name: 'Tìm tài liệu' }))
      const search = await dialogOf('Tìm kiếm')
      await waitFor(() => expect(within(search).queryByRole('button', { name: 'Phê duyệt' })).toBeNull())
      expect(within(search).queryByRole('button', { name: 'Tài sản' })).toBeNull()
      expect(within(search).getByRole('button', { name: 'Người' })).toBeInTheDocument()
    })
  })

  it('follows the screen: the hint becomes a button when a search appears, and a name when it goes', async () => {
    const user = userEvent.setup()
    function Swapper() {
      const [on, setOn] = useState(false)
      return (
        <>
          <Pressable onClick={() => setOn(!on)}>Đổi màn</Pressable>
          {on && <SearchField label="Tìm cài đặt" moduleSearch />}
        </>
      )
    }
    await renderShell(<Shell body={<Swapper />} />, '/admin')
    const header = await bar()
    expect(within(header).queryByRole('button', { name: 'Tìm cài đặt' })).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Đổi màn' }))
    await within(header).findByRole('button', { name: 'Tìm cài đặt' })
    await user.click(screen.getByRole('button', { name: 'Đổi màn' }))
    await waitFor(() => expect(within(header).queryByRole('button', { name: 'Tìm cài đặt' })).toBeNull())
  })
})

describe('the workspace tile', () => {
  it('shows the first letter of the workspace, named for assistive tech with where you are', async () => {
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    const tile = await screen.findByRole('button', { name: 'Đổi workspace, đang ở Khối Vận hành' })
    expect(tile).toHaveTextContent('K')
  })

  it('is the accent in an organization and the person\'s own hue in a personal workspace', async () => {
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    const org = (await screen.findByRole('button', { name: /^Đổi workspace/ })).firstElementChild as HTMLElement
    expect(org.className).toContain('bg-accent')
    expect(org.getAttribute('style') ?? '').not.toContain('--p')
    cleanup()
    resetClient()
    server.personal = true
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    const own = (await screen.findByRole('button', { name: /^Đổi workspace/ })).firstElementChild as HTMLElement
    await waitFor(() => expect(own.className).toContain('bg-(--p)'))
    expect(own.getAttribute('style')).toContain('--p')
  })

  it('has a dot and says so when an invitation is waiting', async () => {
    server.offers = 2
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    expect(await screen.findByRole('button', { name: 'Đổi workspace, đang ở Khối Vận hành, có lời mời' })).toBeInTheDocument()
    expect(screen.getByTestId('invitation-dot')).toBeInTheDocument()
  })

  it('has no dot when nothing waits', async () => {
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    await screen.findByRole('button', { name: /^Đổi workspace/ })
    expect(screen.queryByTestId('invitation-dot')).toBeNull()
  })

  it('opens the workspace sheet, and returns focus to the tile on Esc', async () => {
    const user = userEvent.setup()
    await renderShell(<Shell body={<p>x</p>} />, '/drive')
    const tile = await screen.findByRole('button', { name: /^Đổi workspace/ })
    await user.click(tile)
    const sheet = await dialogOf('Đổi workspace')
    expect(await within(sheet).findByRole('option', { name: /NovaPay Kế toán/ })).toBeInTheDocument()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(tile).toHaveFocus()
  })
})

describe('the layers it opens', () => {
  it('the avatar opens Tôi (Cài đặt), keeping the workspace, and no account sheet', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<Shell body={<p>x</p>} />, `/drive?ws=${WS_ID}`)
    const avatar = await screen.findByRole('button', { name: 'Tôi' })
    expect(avatar).not.toHaveAttribute('aria-haspopup')
    await user.click(avatar)
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
    expect(router.state.location.search).toEqual({ ws: WS_ID })
    expect(screen.queryByRole('dialog')).toBeNull()
  })
})

