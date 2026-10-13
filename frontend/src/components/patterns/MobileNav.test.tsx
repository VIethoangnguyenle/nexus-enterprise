import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { CH, UUID_RE } from '../../test/chat-fixtures'
import { apiFetch } from '../../api/client'
import { useUiStore } from '../../stores/ui.store'
import { MobileNav } from './MobileNav'
import { DIRECT, GROUP, GROUP_2, answer, renderShell, resetServer, server, unreadRow, viewport } from './phone-shell-harness'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

beforeEach(() => {
  viewport(true)
  resetServer()
  api.mockReset()
  api.mockImplementation(answer as typeof apiFetch)
  useUiStore.setState({ chatBarShown: false })
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

const chatBar = () => screen.findByRole('navigation', { name: 'Điều hướng Chat' })

describe('where the Chat bar shows', () => {
  it('shows on the conversation list, with the + button beside it', async () => {
    await renderShell(<MobileNav />, '/channels')
    expect(await chatBar()).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Trò chuyện mới' })).toBeInTheDocument()
  })

  it.each(['/channels/99999999-aaaa-4bbb-8ccc-000000000002', '/channels/mentions', '/drive', '/approval', '/settings'])(
    'is not on %s',
    async (url) => {
      await renderShell(<MobileNav />, url)
      await waitFor(() => expect(api).not.toHaveBeenCalledWith('/dms'))
      expect(screen.queryByRole('navigation', { name: 'Điều hướng Chat' })).toBeNull()
      expect(screen.queryByRole('button', { name: 'Trò chuyện mới' })).toBeNull()
    },
  )

  it('draws nothing and asks for nothing at desktop widths', async () => {
    viewport(false)
    await renderShell(<MobileNav />, '/channels')
    await waitFor(() => expect(screen.queryByRole('navigation', { name: 'Điều hướng Chat' })).toBeNull())
    expect(api).not.toHaveBeenCalledWith('/dms')
    expect(api).not.toHaveBeenCalledWith('/approval/pending')
  })

  it('tells the floating things the bar is there while it is, and that it is gone after', async () => {
    const router = await renderShell(<MobileNav />, '/channels')
    await chatBar()
    expect(useUiStore.getState().chatBarShown).toBe(true)
    await router.navigate({ to: '/drive' })
    await waitFor(() => expect(useUiStore.getState().chatBarShown).toBe(false))
  })
})

describe('the four icons', () => {
  it('are Trang chủ, Trực tiếp, Mục and Thêm in order, named only by their label', async () => {
    await renderShell(<MobileNav />)
    const nav = await chatBar()
    const links = within(nav).getAllByRole('link')
    expect(links.map((a) => a.getAttribute('aria-label'))).toEqual(['Trang chủ', 'Trực tiếp', 'Mục', 'Thêm'])
    for (const a of links) expect(a.textContent).toBe('')
  })

  it('opens Trang chủ without a tab, the others with theirs', async () => {
    await renderShell(<MobileNav />)
    const nav = await chatBar()
    const hrefs = within(nav).getAllByRole('link').map((a) => a.getAttribute('href'))
    expect(hrefs).toEqual(['/channels', '/channels?tab=direct', '/channels?tab=sections', '/channels?tab=more'])
  })

  it('marks Trang chủ when the address names no tab or a tab it does not know', async () => {
    await renderShell(<MobileNav />, '/channels?tab=nope')
    const nav = await chatBar()
    expect(within(nav).getByRole('link', { name: 'Trang chủ' })).toHaveAttribute('aria-current', 'page')
    expect(within(nav).getAllByRole('link').filter((a) => a.hasAttribute('aria-current'))).toHaveLength(1)
  })

  it.each([['direct', 'Trực tiếp'], ['sections', 'Mục'], ['more', 'Thêm']])('marks %s from the address', async (tab, name) => {
    await renderShell(<MobileNav />, `/channels?tab=${tab}`)
    const nav = await chatBar()
    expect(within(nav).getByRole('link', { name })).toHaveAttribute('aria-current', 'page')
    expect(within(nav).getByRole('link', { name: 'Trang chủ' })).not.toHaveAttribute('aria-current')
  })

  it('moves the current mark when another icon is pressed', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<MobileNav />)
    const nav = await chatBar()
    await user.click(within(nav).getByRole('link', { name: 'Mục' }))
    await waitFor(() => expect(router.state.location.search).toMatchObject({ tab: 'sections' }))
    expect(within(nav).getByRole('link', { name: 'Mục' })).toHaveAttribute('aria-current', 'page')
    await user.click(within(nav).getByRole('link', { name: 'Trang chủ' }))
    await waitFor(() => expect(within(nav).getByRole('link', { name: 'Trang chủ' })).toHaveAttribute('aria-current', 'page'))
    expect(router.state.location.search).not.toHaveProperty('tab')
  })
})

describe('what they report', () => {
  it('says nothing when nothing is unread', async () => {
    await renderShell(<MobileNav />)
    const nav = await chatBar()
    await waitFor(() => expect(api).toHaveBeenCalledWith('/channels/unread'))
    expect(within(nav).getAllByRole('link').map((a) => a.getAttribute('aria-label'))).toEqual(['Trang chủ', 'Trực tiếp', 'Mục', 'Thêm'])
    expect(nav).not.toHaveTextContent(/\d/)
  })

  it('dots Trang chủ when anything is unread', async () => {
    server.unread = [unreadRow(GROUP, 3)]
    await renderShell(<MobileNav />)
    expect(await screen.findByRole('link', { name: 'Trang chủ, có tin chưa đọc' })).toBeInTheDocument()
  })

  it('counts unread direct messages on Trực tiếp and the groups with news on Mục', async () => {
    server.unread = [unreadRow(DIRECT, 2), unreadRow(CH.dmLan, 3), unreadRow(GROUP, 5), unreadRow(GROUP_2, 1)]
    await renderShell(<MobileNav />)
    const direct = await screen.findByRole('link', { name: 'Trực tiếp, 5 chưa đọc' })
    expect(direct).toHaveTextContent('5')
    const sections = await screen.findByRole('link', { name: 'Mục, 2 chưa đọc' })
    expect(sections).toHaveTextContent('2')
  })

  it('counts by notification level: a muted group adds only when someone named you, a muted direct only its mentions', async () => {
    server.unread = [
      unreadRow(GROUP, 3, { notification_level: 'off' }),
      unreadRow(GROUP_2, 4, { notification_level: 'off', mention_count: 1 }),
      unreadRow(DIRECT, 9, { notification_level: 'off' }),
    ]
    await renderShell(<MobileNav />)
    expect(await screen.findByRole('link', { name: 'Mục, 1 chưa đọc' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Trực tiếp' })).not.toHaveTextContent(/\d/)
  })

  it('caps a number at 99+', async () => {
    server.unread = [unreadRow(DIRECT, 150)]
    await renderShell(<MobileNav />)
    expect(await screen.findByRole('link', { name: 'Trực tiếp, 150 chưa đọc' })).toHaveTextContent('99+')
  })

  it('dots Thêm when someone has named you', async () => {
    server.mentionsUnread = 2
    await renderShell(<MobileNav />)
    expect(await screen.findByRole('link', { name: 'Thêm, có lượt nhắc tên mới' })).toBeInTheDocument()
  })

  it('never puts an identifier on the screen', async () => {
    server.unread = [unreadRow(GROUP, 3), unreadRow(DIRECT, 1)]
    await renderShell(<MobileNav />)
    await screen.findByRole('link', { name: 'Trực tiếp, 1 chưa đọc' })
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })
})
