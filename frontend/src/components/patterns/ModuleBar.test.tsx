import '@testing-library/jest-dom/vitest'
import { useState, type ReactNode } from 'react'
import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { ME, WS_ID } from '../../test/chat-fixtures'
import { apiFetch } from '../../api/client'
import { updatePreferences } from '../../lib/preferences'
import { useUiStore } from '../../stores/ui.store'
import { ModuleBar } from './ModuleBar'
import { GROUP, OTHER_WS, answer, dialogOf, renderShell, resetServer, server, unreadRow, viewport } from './phone-shell-harness'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

/** The shell's arrangement: the bar beside the screen it watches for scrolling. */
function Shell({ body = null }: { body?: ReactNode }) {
  const [el, setEl] = useState<HTMLElement | null>(null)
  return (
    <>
      <ModuleBar scope={el} />
      <div ref={setEl}>{body}</div>
    </>
  )
}

beforeEach(() => {
  viewport(true)
  resetServer()
  api.mockReset()
  api.mockImplementation(answer as typeof apiFetch)
  useUiStore.setState({ chatBarShown: false })
  localStorage.clear()
})
afterEach(() => {
  vi.unstubAllGlobals()
  updatePreferences({ motion: 'system' })
  cleanup()
  resetClient()
})

const bar = () => screen.findByRole('navigation', { name: 'Phân hệ' })
/** The names of the controls, without the number pill that sits before or after the label. */
const namesIn = (root: HTMLElement) =>
  Array.from(root.querySelectorAll('a, button')).map((el) => el.textContent?.replace(/^\d+\+?|\d+\+?$/g, ''))
const names = async () => namesIn(await bar())

describe('where the bar shows', () => {
  it.each(['/channels', '/drive', '/approval', '/contacts', '/assets'])('shows on the list screen %s', async (url) => {
    await renderShell(<ModuleBar scope={null} />, url)
    expect(await bar()).toBeInTheDocument()
  })

  it.each([
    '/channels/99999999-aaaa-4bbb-8ccc-000000000002', '/channels/mentions', '/documents/d-1', '/settings', '/settings/chat',
    '/admin', '/admin/users',
  ])(
    'is not on %s',
    async (url) => {
      await renderShell(<ModuleBar scope={null} />, url)
      expect(screen.queryByRole('navigation', { name: 'Phân hệ' })).toBeNull()
      expect(screen.queryByRole('button', { name: 'Trò chuyện mới' })).toBeNull()
    },
  )

  it('draws nothing and asks for nothing at desktop widths', async () => {
    viewport(false)
    await renderShell(<ModuleBar scope={null} />, '/channels')
    await waitFor(() => expect(screen.queryByRole('navigation', { name: 'Phân hệ' })).toBeNull())
    for (const path of ['/channels/unread', '/approval/pending', `/workspaces/${WS_ID}/settings`]) {
      expect(api).not.toHaveBeenCalledWith(path)
    }
  })

  it('steps aside while the on-screen keyboard is up', async () => {
    const vv = new EventTarget() as EventTarget & { height: number }
    vv.height = 800
    vi.stubGlobal('visualViewport', vv)
    vi.stubGlobal('innerWidth', 390)
    vi.stubGlobal('innerHeight', 800)
    await renderShell(<ModuleBar scope={null} />, '/drive')
    await bar()
    act(() => { vv.height = 450; vv.dispatchEvent(new Event('resize')) })
    await waitFor(() => expect(screen.queryByRole('navigation', { name: 'Phân hệ' })).toBeNull())
    act(() => { vv.height = 800; vv.dispatchEvent(new Event('resize')) })
    expect(await bar()).toBeInTheDocument()
  })

  it('tells the floating things the bar is there while it is, and that it is gone after', async () => {
    const router = await renderShell(<ModuleBar scope={null} />, '/drive')
    await bar()
    expect(useUiStore.getState().chatBarShown).toBe(true)
    await router.navigate({ to: '/settings' })
    await waitFor(() => expect(useUiStore.getState().chatBarShown).toBe(false))
  })

  it('has the + of Chat beside it in Chat only', async () => {
    const router = await renderShell(<ModuleBar scope={null} />, '/channels')
    await bar()
    expect(screen.getByRole('button', { name: 'Trò chuyện mới' })).toBeInTheDocument()
    await router.navigate({ to: '/drive' })
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Trò chuyện mới' })).toBeNull())
  })
})

describe('the slots', () => {
  it('are Chat, Lịch, Tài liệu and Phê duyệt in an organization, then Thêm, each named by its label', async () => {
    await renderShell(<ModuleBar scope={null} />)
    expect(await names()).toEqual(['Chat', 'Lịch', 'Tài liệu', 'Phê duyệt', 'Thêm'])
    const nav = await bar()
    expect(within(nav).getAllByRole('link').map((a) => a.getAttribute('href'))).toEqual(['/channels', '/calendar', '/drive', '/approval'])
  })

  it('put Danh bạ where Phê duyệt was in a personal workspace, and never ask about approvals', async () => {
    server.personal = true
    await renderShell(<ModuleBar scope={null} />)
    await waitFor(() => expect(api).toHaveBeenCalledWith('/workspaces'))
    await waitFor(async () => expect(await names()).toEqual(['Chat', 'Lịch', 'Tài liệu', 'Danh bạ', 'Thêm']))
    expect(api).not.toHaveBeenCalledWith('/approval/pending')
  })

  it('follow the choice the person saved for this workspace, in its order', async () => {
    server.bars[WS_ID] = ['assets', 'chat', 'notifications', 'drive']
    await renderShell(<ModuleBar scope={null} />)
    await waitFor(async () => expect(await names()).toEqual(['Tài sản', 'Chat', 'Thông báo', 'Tài liệu', 'Thêm']))
  })

  it('keep a choice per workspace', async () => {
    server.bars[WS_ID] = ['assets', 'chat', 'notifications', 'drive']
    server.bars[OTHER_WS] = ['contacts', 'chat', 'calendar', 'drive']
    await renderShell(<ModuleBar scope={null} />, `/drive?ws=${OTHER_WS}`)
    await waitFor(async () => expect(await names()).toEqual(['Danh bạ', 'Chat', 'Lịch', 'Tài liệu', 'Thêm']))
    expect(api).toHaveBeenCalledWith(`/workspaces/${OTHER_WS}/settings`)
  })

  it.each([
    ['Phê duyệt', ['chat', 'calendar', 'drive', 'approval']],
    ['Tài sản', ['chat', 'calendar', 'assets', 'contacts']],
    ['a repeat', ['chat', 'chat', 'drive', 'contacts']],
    ['Quản trị', ['chat', 'calendar', 'drive', 'admin']],
  ])('fall back to the personal defaults when the saved choice has %s', async (_what, saved) => {
    server.personal = true
    server.bars[WS_ID] = saved
    await renderShell(<ModuleBar scope={null} />)
    await waitFor(async () => expect(await names()).toEqual(['Chat', 'Lịch', 'Tài liệu', 'Danh bạ', 'Thêm']))
  })

  it('fall back to the defaults when the settings cannot be read', async () => {
    server.settingsDown = true
    await renderShell(<ModuleBar scope={null} />)
    expect(await names()).toEqual(['Chat', 'Lịch', 'Tài liệu', 'Phê duyệt', 'Thêm'])
  })

  it('mark the module you are in', async () => {
    await renderShell(<ModuleBar scope={null} />, '/drive')
    const nav = await bar()
    expect(within(nav).getByRole('link', { name: 'Tài liệu' })).toHaveAttribute('aria-current', 'page')
    expect(nav.querySelectorAll('[aria-current]')).toHaveLength(1)
  })

  it('mark Chat on Chat\'s own screens too', async () => {
    await renderShell(<ModuleBar scope={null} />, '/channels?tab=direct')
    expect(within(await bar()).getByRole('link', { name: 'Chat' })).toHaveAttribute('aria-current', 'page')
  })
})

describe('what the numbers say', () => {
  it('counts unread conversations in Chat the way the sidebar does: a muted one adds only the times you were named', async () => {
    server.unread = [unreadRow(GROUP, 3), unreadRow('c-muted', 5, { notification_level: 'off', mention_count: 1 }), unreadRow('c-quiet', 9, { notification_level: 'off' })]
    await renderShell(<ModuleBar scope={null} />)
    expect(await within(await bar()).findByRole('link', { name: 'Chat, 4 chưa đọc' })).toBeInTheDocument()
  })

  it('shows 99+ above 99 and keeps the exact figure for assistive tech', async () => {
    server.unread = [unreadRow(GROUP, 120)]
    await renderShell(<ModuleBar scope={null} />)
    const link = await within(await bar()).findByRole('link', { name: 'Chat, 120 chưa đọc' })
    expect(link).toHaveTextContent('99+')
  })

  it('counts the requests waiting for the person on Phê duyệt', async () => {
    await renderShell(<ModuleBar scope={null} />)
    expect(await within(await bar()).findByRole('link', { name: 'Phê duyệt, 2 chờ bạn' })).toBeInTheDocument()
  })

  it('hides a zero', async () => {
    server.pending = 0
    await renderShell(<ModuleBar scope={null} />)
    const nav = await bar()
    await waitFor(() => expect(api).toHaveBeenCalledWith('/approval/pending'))
    expect(within(nav).getByRole('link', { name: 'Phê duyệt' })).toBeInTheDocument()
    expect(nav.textContent).not.toMatch(/\d/)
  })

  it('puts a dot on Thêm when a module inside it has news, and says so', async () => {
    server.pending = 0
    server.notificationsUnread = 3
    await renderShell(<ModuleBar scope={null} />)
    expect(await within(await bar()).findByRole('button', { name: 'Thêm, có việc mới' })).toBeInTheDocument()
    expect(screen.getByTestId('more-dot')).toBeInTheDocument()
  })

  it('has no dot when what is new is already on the bar', async () => {
    server.pending = 0
    server.notificationsUnread = 3
    server.bars[WS_ID] = ['chat', 'calendar', 'drive', 'notifications']
    await renderShell(<ModuleBar scope={null} />)
    expect(await within(await bar()).findByRole('button', { name: 'Thông báo, 3 chưa đọc' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Thêm' })).toBeInTheDocument()
    expect(screen.queryByTestId('more-dot')).toBeNull()
  })

  it('never counts approvals in a personal workspace', async () => {
    server.personal = true
    server.pending = 5
    await renderShell(<ModuleBar scope={null} />)
    await waitFor(async () => expect(await names()).toContain('Danh bạ'))
    expect(screen.getByRole('button', { name: 'Thêm' })).toBeInTheDocument()
    expect(api).not.toHaveBeenCalledWith('/approval/pending')
  })

  it('opens the notifications screen from a bar slot, and Quay lại closes it', async () => {
    server.bars[WS_ID] = ['chat', 'calendar', 'drive', 'notifications']
    const user = userEvent.setup()
    await renderShell(<ModuleBar scope={null} />)
    await user.click(await within(await bar()).findByRole('button', { name: 'Thông báo' }))
    const screenOfNotifications = await dialogOf('Thông báo')
    await user.click(within(screenOfNotifications).getByRole('button', { name: 'Quay lại' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })
})

describe('folding on scroll', () => {
  function scrolling() {
    const scroller = document.createElement('div')
    scroller.className = 'overflow-y-auto'
    scroller.scrollTo = vi.fn() as never
    return scroller
  }
  const Body = ({ scroller }: { scroller: HTMLElement }) => (
    <div ref={(el) => { if (el && !el.contains(scroller)) el.append(scroller) }} />
  )
  const scrollTo = (el: HTMLElement, top: number) => act(() => { el.scrollTop = top; fireEvent.scroll(el) })

  it('folds to icons past 24px down, and opens on a little scrolling up', async () => {
    const scroller = scrolling()
    await renderShell(<Shell body={<Body scroller={scroller} />} />, '/channels')
    const nav = await bar()
    expect(nav.className).toContain('h-16')
    scrollTo(scroller, 200)
    await waitFor(() => expect(nav.className).toContain('h-13'))
    scrollTo(scroller, 180)
    await waitFor(() => expect(nav.className).toContain('h-16'))
  })

  it('does not fold under reduced motion', async () => {
    updatePreferences({ motion: 'reduce' })
    const scroller = scrolling()
    await renderShell(<Shell body={<Body scroller={scroller} />} />, '/channels')
    const nav = await bar()
    scrollTo(scroller, 300)
    expect(nav.className).toContain('h-16')
  })

  it('goes to the top of the list when the slot you are in is pressed, without leaving', async () => {
    const user = userEvent.setup()
    const scroller = scrolling()
    const router = await renderShell(<Shell body={<Body scroller={scroller} />} />, '/drive')
    const nav = await bar()
    scrollTo(scroller, 300)
    await user.click(within(nav).getByRole('link', { name: 'Tài liệu' }))
    expect(scroller.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' })
    expect(router.state.location.pathname).toBe('/drive')
  })

  it('goes to another module with a plain press', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<Shell />, '/drive')
    await user.click(within(await bar()).getByRole('link', { name: 'Lịch' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar'))
  })

  it('drops the Chat filter when Chat is pressed at the top, after the first press took it there', async () => {
    const user = userEvent.setup()
    const scroller = scrolling()
    const router = await renderShell(<Shell body={<Body scroller={scroller} />} />, '/channels?tab=direct')
    const nav = await bar()
    scrollTo(scroller, 300)
    await user.click(within(nav).getByRole('link', { name: 'Chat' }))
    expect(scroller.scrollTo).toHaveBeenCalledTimes(1)
    expect(router.state.location.search).toMatchObject({ tab: 'direct' })
    scrollTo(scroller, 0)
    await user.click(within(nav).getByRole('link', { name: 'Chat' }))
    await waitFor(() => expect(router.state.location.search).not.toHaveProperty('tab'))
    expect(localStorage.getItem(`ngac-chat-chip:${ME.id}`)).toBe('all')
  })
})

describe('long press', () => {
  const hold = (el: HTMLElement) => {
    fireEvent.pointerDown(el, { pointerType: 'touch', clientX: 5, clientY: 5 })
    act(() => { vi.advanceTimersByTime(401) })
    fireEvent.pointerUp(el, { pointerType: 'touch' })
    fireEvent.click(el)
  }
  beforeEach(() => { vi.useFakeTimers({ shouldAdvanceTime: true }) })
  afterEach(() => { vi.useRealTimers() })

  it('on Chat opens Đi nhanh with Trực tiếp, Được nhắc, Thread theo dõi and Đã gắn sao, and does not also open Chat', async () => {
    server.mentionsUnread = 2
    const router = await renderShell(<ModuleBar scope={null} />, '/drive')
    const nav = await bar()
    hold(within(nav).getByRole('link', { name: 'Chat' }))
    const menu = await screen.findByRole('menu', { name: 'Đi nhanh trong Chat' })
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent?.replace(/\d+$/, ''))).toEqual([
      'Trực tiếp', 'Được nhắc', 'Thread theo dõi', 'Đã gắn sao',
    ])
    await waitFor(() => expect(within(menu).getByRole('menuitem', { name: 'Được nhắc, 2 chưa đọc' })).toBeInTheDocument())
    expect(within(menu).getByRole('menuitem', { name: 'Đã gắn sao' })).toHaveAttribute('href', '/channels/starred')
    expect(router.state.location.pathname).toBe('/drive')
  })

  it('opens the chip a Đi nhanh row names, on Chat', async () => {
    const router = await renderShell(<ModuleBar scope={null} />, '/drive')
    hold(within(await bar()).getByRole('link', { name: 'Chat' }))
    fireEvent.click(await screen.findByRole('menuitem', { name: /^Trực tiếp/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/channels'))
    expect(router.state.location.search).toMatchObject({ tab: 'direct' })
  })

  it('opens the Thread screen on Đang theo dõi from Đi nhanh', async () => {
    const router = await renderShell(<ModuleBar scope={null} />, '/channels')
    hold(within(await bar()).getByRole('link', { name: 'Chat' }))
    fireEvent.click(await screen.findByRole('menuitem', { name: /^Thread theo dõi/ }))
    await waitFor(() => expect(router.state.location.search).toMatchObject({ threads: 'followed' }))
  })

  it('on any other slot opens Sửa thanh đáy, and the tap that ends it goes nowhere', async () => {
    const router = await renderShell(<ModuleBar scope={null} />, '/drive')
    hold(within(await bar()).getByRole('link', { name: 'Lịch' }))
    await dialogOf('Sửa thanh đáy')
    expect(router.state.location.pathname).toBe('/drive')
  })

  it('on Thêm opens Sửa thanh đáy and not the Thêm sheet', async () => {
    await renderShell(<ModuleBar scope={null} />, '/drive')
    hold(within(await bar()).getByRole('button', { name: /^Thêm/ }))
    await dialogOf('Sửa thanh đáy')
    expect(screen.queryByRole('dialog', { name: 'Thêm' })).toBeNull()
  })
})

describe('Thêm', () => {
  it('lists the workspace and every module the organization has, Thông báo among them, and no Quản trị', async () => {
    const user = userEvent.setup()
    await renderShell(<ModuleBar scope={null} />, '/drive')
    await user.click(await within(await bar()).findByRole('button', { name: /^Thêm/ }))
    const sheet = await dialogOf('Thêm')
    expect(within(sheet).getByRole('button', { name: /^Đổi workspace, đang ở Khối Vận hành/ })).toBeInTheDocument()
    const grid = within(sheet).getByRole('navigation', { name: 'Tất cả phân hệ' })
    await within(grid).findByRole('link', { name: /Phê duyệt/ })
    expect(namesIn(grid)).toEqual([
      'Chat', 'Lịch', 'Tài liệu', 'Phê duyệt', 'Tài sản', 'Danh bạ', 'Thông báo',
    ])
    // Quản trị lives in Cài đặt workspace on a phone, behind the ⚙ of the workspace block.
    expect(within(sheet).queryByRole('link', { name: 'Quản trị' })).toBeNull()
  })

  it('opens Cài đặt workspace from the ⚙ on the workspace block, and closes', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<ModuleBar scope={null} />, `/drive?ws=${WS_ID}`)
    await user.click(await within(await bar()).findByRole('button', { name: /^Thêm/ }))
    await user.click(within(await dialogOf('Thêm')).getByRole('button', { name: 'Cài đặt workspace' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/workspace'))
    expect(router.state.location.search).toMatchObject({ ws: WS_ID })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thêm' })).toBeNull())
    expect(screen.queryByRole('dialog', { name: 'Đổi workspace' })).toBeNull()
  })

  it('offers the ⚙ in a personal workspace too', async () => {
    server.personal = true
    const user = userEvent.setup()
    await renderShell(<ModuleBar scope={null} />, '/drive')
    await waitFor(async () => expect(await names()).toContain('Danh bạ'))
    await user.click(screen.getByRole('button', { name: /^Thêm/ }))
    expect(within(await dialogOf('Thêm')).getByRole('button', { name: 'Cài đặt workspace' })).toBeInTheDocument()
  })

  it('leaves Phê duyệt, Tài sản and Quản trị out of a personal workspace', async () => {
    server.personal = true
    const user = userEvent.setup()
    await renderShell(<ModuleBar scope={null} />, '/drive')
    await waitFor(async () => expect(await names()).toContain('Danh bạ'))
    await user.click(screen.getByRole('button', { name: /^Thêm/ }))
    const sheet = await dialogOf('Thêm')
    const grid = within(sheet).getByRole('navigation', { name: 'Tất cả phân hệ' })
    expect(namesIn(grid)).toEqual(['Chat', 'Lịch', 'Tài liệu', 'Danh bạ', 'Thông báo'])
    expect(within(sheet).queryByRole('link', { name: 'Quản trị' })).toBeNull()
    expect(within(sheet).getByText(/chỉ bạn/)).toBeInTheDocument()
  })

  it('names the counts, rings the modules that are on the bar, and goes to a module with one press', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<ModuleBar scope={null} />, '/drive')
    await user.click(await within(await bar()).findByRole('button', { name: /^Thêm/ }))
    const sheet = await dialogOf('Thêm')
    const approval = await within(sheet).findByRole('link', { name: 'Phê duyệt, 2 chờ bạn' })
    expect(approval.firstElementChild!.className).toContain('ring-accent')
    expect(within(sheet).getByRole('link', { name: 'Tài sản' }).firstElementChild!.className).not.toContain('ring-accent')
    await user.click(within(sheet).getByRole('link', { name: 'Tài sản' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/assets'))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('opens the notifications screen from its tile', async () => {
    server.notificationsUnread = 4
    const user = userEvent.setup()
    await renderShell(<ModuleBar scope={null} />, '/drive')
    await user.click(await within(await bar()).findByRole('button', { name: /^Thêm/ }))
    const sheet = await dialogOf('Thêm')
    await user.click(await within(sheet).findByRole('button', { name: 'Thông báo, 4 chưa đọc' }))
    await dialogOf('Thông báo')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thêm' })).toBeNull())
  })

  it('opens Sửa thanh đáy from its heading', async () => {
    const user = userEvent.setup()
    await renderShell(<ModuleBar scope={null} />, '/drive')
    await user.click(await within(await bar()).findByRole('button', { name: /^Thêm/ }))
    await user.click(within(await dialogOf('Thêm')).getByRole('button', { name: 'Sửa thanh đáy' }))
    await dialogOf('Sửa thanh đáy')
  })

  it('opens the workspace list from the workspace row', async () => {
    const user = userEvent.setup()
    await renderShell(<ModuleBar scope={null} />, '/drive')
    await user.click(await within(await bar()).findByRole('button', { name: /^Thêm/ }))
    await user.click(within(await dialogOf('Thêm')).getByRole('button', { name: /^Đổi workspace/ }))
    const list = await dialogOf('Đổi workspace')
    expect(await within(list).findByRole('option', { name: /NovaPay Kế toán/ })).toBeInTheDocument()
  })
})

describe('Sửa thanh đáy, end to end', () => {
  const openEdit = async () => {
    const user = userEvent.setup()
    await renderShell(<ModuleBar scope={null} />, '/drive')
    await user.click(await within(await bar()).findByRole('button', { name: /^Thêm/ }))
    await user.click(within(await dialogOf('Thêm')).getByRole('button', { name: 'Sửa thanh đáy' }))
    return { user, edit: await dialogOf('Sửa thanh đáy') }
  }

  it('saves the new order for this workspace and the bar follows at once', async () => {
    const { user, edit } = await openEdit()
    await user.click(within(edit).getByRole('button', { name: 'Đưa Lịch lên trước' }))
    await user.click(within(edit).getByRole('button', { name: 'Xong' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(server.settingsPuts).toEqual([{ workspaceId: WS_ID, body: { 'mobile.bar': ['calendar', 'chat', 'drive', 'approval'] } }])
    expect(await names()).toEqual(['Lịch', 'Chat', 'Tài liệu', 'Phê duyệt', 'Thêm'])
  })

  it('keeps the bar and the server as they were on Quay lại', async () => {
    const { user, edit } = await openEdit()
    await user.click(within(edit).getByRole('button', { name: 'Đưa Lịch lên trước' }))
    await user.click(within(edit).getByRole('button', { name: 'Quay lại' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(server.settingsPuts).toEqual([])
    expect(await names()).toEqual(['Chat', 'Lịch', 'Tài liệu', 'Phê duyệt', 'Thêm'])
  })

  it('stays open and keeps the old bar when the server refuses', async () => {
    const { user, edit } = await openEdit()
    server.settingsDown = true
    await user.click(within(edit).getByRole('button', { name: 'Đưa Lịch lên trước' }))
    await user.click(within(edit).getByRole('button', { name: 'Xong' }))
    await waitFor(() => expect(within(edit).getByRole('button', { name: 'Xong' })).toBeEnabled())
    expect(screen.getByRole('dialog', { name: 'Sửa thanh đáy' })).toBeInTheDocument()
    expect(await names()).toEqual(['Chat', 'Lịch', 'Tài liệu', 'Phê duyệt', 'Thêm'])
  })
})
