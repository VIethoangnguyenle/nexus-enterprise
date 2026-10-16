import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, UUID_RE } from '../../test/chat-fixtures'
import { createChatStateServer } from '../../test/chat-state-server'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { prefetchChatParts } from '../chat/lazy-parts'
import { SpaceView } from './SpaceView'

// The composer, picker and editor load on first use; fetch them once so each test sees them at once.
beforeAll(() => prefetchChatParts(), 60_000)

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

let server: ReturnType<typeof createChatStateServer>
/** `hold` makes the next PUT to a path fragment wait until `release()`. */
let release: () => void = () => {}
const serve = (initial: Parameters<typeof createChatStateServer>[0] = {}, hold?: string) => {
  server = createChatStateServer(initial)
  vi.mocked(apiFetch).mockImplementation(async (path: string, init?: RequestInit) => {
    if (hold && path.includes(hold) && init?.method === 'PUT') {
      await new Promise<void>((r) => { release = r })
    }
    return server.api(path, init) as never
  })
}

function viewport(phone: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: phone && query.includes('max-width'),
    media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
}

beforeEach(() => {
  routerState.params = { channelId: CH.doisoat }
  routerState.pathname = `/channels/${CH.doisoat}`
  serve()
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
  resetClient()
  useToastStore.getState().clear()
})

// The first render of a cold file can outlast the default second.
const title = () => screen.findByRole('button', { name: /^Đối soát giao dịch/ }, { timeout: 4000 })

describe('the group header menu', () => {
  it('chooses a level: the menu closes, the header shows it, the server hears it, a toast offers Hoàn tác', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await title())
    await user.click(await screen.findByRole('menuitemradio', { name: /^Tắt/ }))
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
    await waitFor(() => expect(server.calls).toContainEqual(
      expect.objectContaining({ method: 'PUT', path: `/channels/${CH.doisoat}/notification-level`, body: { level: 'off' } })))
    const t = await title()
    expect(within(t).getByText('đã tắt thông báo')).toBeInTheDocument()
    expect(t.querySelector('svg.lucide-bell-off')).not.toBeNull()
    await waitFor(() => expect(useToastStore.getState().toasts[0]?.message).toBe('Đã tắt thông báo của “Đối soát giao dịch”'))
  })

  it('shows the level the unread list holds without being opened, and checks it in the menu', async () => {
    const user = userEvent.setup()
    serve({ [CH.doisoat]: { notification_level: 'all' } })
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    const t = await title()
    await waitFor(() => expect(within(t).getByText('thông báo mọi tin')).toBeInTheDocument())
    await user.click(t)
    expect(await screen.findByRole('menuitemradio', { name: /^Tất cả/ })).toHaveAttribute('aria-checked', 'true')
    expect(server.calls.some((c) => c.path.endsWith('/notification-level') && c.method === 'GET')).toBe(false)
  })

  it('says "Đang lưu…" in the sub line only when the save takes longer than 300ms', async () => {
    const user = userEvent.setup()
    serve({}, 'notification-level')
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await title())
    await user.click(await screen.findByRole('menuitemradio', { name: /^Tắt/ }))
    expect(screen.queryByText(/Đang lưu/)).toBeNull()
    expect(await screen.findByText(/Đang lưu…/, undefined, { timeout: 1500 })).toBeInTheDocument()
    await act(async () => release())
    await waitFor(() => expect(screen.queryByText(/Đang lưu/)).toBeNull())
  })

  it('stars the group from the menu, and shows the star after the name', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await title())
    await user.click(await screen.findByRole('menuitem', { name: 'Gắn sao' }))
    await waitFor(() => expect(server.calls).toContainEqual(
      expect.objectContaining({ method: 'PUT', path: `/channels/${CH.doisoat}/star` })))
    const t = await title()
    await waitFor(() => expect(within(t).getByText('Đã gắn sao')).toBeInTheDocument())
    await user.click(t)
    expect(await screen.findByRole('menuitem', { name: 'Bỏ gắn sao' })).toBeInTheDocument()
  })

  it('opens the info panel from the menu', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await title())
    await user.click(await screen.findByRole('menuitem', { name: 'Thông tin nhóm' }))
    expect(await screen.findByRole('complementary', { name: 'Thông tin' })).toBeInTheDocument()
  })

  it('a direct message offers Bật and Tắt, never Tất cả', async () => {
    const user = userEvent.setup()
    routerState.params = { channelId: CH.dmYen }
    renderWithClient(<SpaceView channelId={CH.dmYen} />)
    await user.click(await screen.findByRole('button', { name: /^Phạm Hải Yến/ }))
    const menu = await screen.findByRole('menu')
    expect(within(menu).getAllByRole('menuitemradio').map((r) => r.querySelector('b')?.textContent)).toEqual(['Bật', 'Tắt'])
    expect(within(menu).queryByText('Tất cả')).toBeNull()
  })

  it('writes no identifier on the screen', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await title())
    await screen.findByRole('menu')
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })
})

describe('the group info screen on a phone', () => {
  beforeEach(() => viewport(true))

  it('opens from the name, with the Thông báo row naming the current level', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await title())
    expect(screen.queryByRole('menu')).toBeNull()
    const panel = await screen.findByRole('dialog', { name: 'Thông tin' })
    const row = within(panel).getByRole('button', { name: /^Thông báo/ })
    expect(row).toHaveTextContent('Tin chính và thread đang theo dõi')
    expect(row.className).toContain('min-h-14')
  })

  it('the row opens a sheet of radios; choosing one sets the level and closes the sheet', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await title())
    const panel = await screen.findByRole('dialog', { name: 'Thông tin' })
    await user.click(within(panel).getByRole('button', { name: /^Thông báo/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Thông báo' })
    expect(within(sheet).getByText('Đối soát giao dịch')).toBeInTheDocument()
    const group = within(sheet).getByRole('radiogroup', { name: 'Thông báo' })
    const radios = within(group).getAllByRole('radio')
    expect(radios.map((r) => r.getAttribute('aria-checked'))).toEqual(['false', 'true', 'false'])
    expect(radios[0]!.className).toContain('min-h-16')
    await user.click(within(group).getByRole('radio', { name: /^Tắt/ }))
    await waitFor(() => expect(server.calls).toContainEqual(
      expect.objectContaining({ method: 'PUT', body: { level: 'off' } })))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thông báo' })).toBeNull(), { timeout: 1500 })
    await waitFor(() => expect(within(panel).getByRole('button', { name: /^Thông báo/ })).toHaveTextContent('Tắt'))
  })

  it('a direct message offers Bật and Tắt in the sheet', async () => {
    const user = userEvent.setup()
    routerState.params = { channelId: CH.dmYen }
    renderWithClient(<SpaceView channelId={CH.dmYen} />)
    await user.click(await screen.findByRole('button', { name: /^Phạm Hải Yến/ }))
    const panel = await screen.findByRole('dialog', { name: 'Thông tin' })
    await user.click(within(panel).getByRole('button', { name: /^Thông báo/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Thông báo' })
    expect(within(sheet).getAllByRole('radio')).toHaveLength(2)
    expect(within(sheet).queryByText('Tất cả')).toBeNull()
  })

  it('shows a muted group with a bell-off after its name', async () => {
    serve({ [CH.doisoat]: { notification_level: 'off' } })
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    const t = await title()
    await waitFor(() => expect(t.querySelector('svg.lucide-bell-off')).not.toBeNull())
  })
})
