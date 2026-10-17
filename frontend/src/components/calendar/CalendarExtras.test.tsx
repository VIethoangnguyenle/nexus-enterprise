import { cleanup, configure, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import {
  CAL, E, NOW, OCCURRENCES, PEOPLE, UUID_RE, calendarFixtureApi, calls, resetFixtures,
} from '../../test/calendar-fixtures'
import { validateCalendarSearch, validateEditEventSearch, validateNewEventSearch } from '../../lib/calendar-search'
import { apiFetch } from '../../api/client'
import type { CalendarSummary } from '../../api/calendar'
import { useToastStore } from '../primitives'
import { CalendarScreen } from './CalendarScreen'
import { EventDetailBody } from './EventPopover'

vi.mock('../../api/client', async (orig) => {
  const { calendarFixtureApi } = await import('../../test/calendar-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(calendarFixtureApi) }
})

const api = vi.mocked(apiFetch)
configure({ asyncUtilTimeout: 4000 })
vi.setConfig({ testTimeout: 15_000 })

async function renderCalendar(url = '/calendar', ui = <CalendarScreen />) {
  const root = createRootRoute()
  const page = (path: string, component: () => React.ReactNode, validateSearch?: (s: Record<string, unknown>) => object) =>
    createRoute({ getParentRoute: () => root, path, component, ...(validateSearch ? { validateSearch } : {}) })
  const router = createRouter({
    routeTree: root.addChildren([
      page('/calendar', () => ui, validateCalendarSearch),
      page('/calendar/new', () => <p>Trình soạn</p>, validateNewEventSearch),
      page('/calendar/event/$eventId', () => <p>Trình soạn</p>, validateEditEventSearch),
      page('/calls/$callId', () => <p>Cuộc gọi</p>),
      page('/channels/$channelId', () => <p>Cuộc trò chuyện</p>),
    ]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

const writes = (method: string) => calls.filter((c) => c.method === method)
const block = (name: RegExp | string) => screen.findByRole('button', { name })
const setup = () => userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

/** The viewport: a phone (< 768), a tablet (768-1023) or the desktop. */
function viewport(kind: 'phone' | 'tablet' | 'desktop') {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: kind === 'phone' ? query.includes('max-width') : kind === 'tablet' ? query.includes('1023') : false,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

beforeEach(() => {
  localStorage.clear()
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(NOW)
  resetFixtures()
  api.mockImplementation(calendarFixtureApi)
  viewport('desktop')
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
})

describe('the ways to the editor', () => {
  it('opens the editor on the instance from Sửa and from the E key', async () => {
    const user = setup()
    const router = await renderCalendar()
    await user.click(await block(/^Báo cáo tuần/))
    await user.click(await screen.findByRole('button', { name: 'Sửa sự kiện' }))
    await waitFor(() => expect(router.state.location.pathname).toBe(`/calendar/event/${E.report}`))
    expect(router.state.location.search).toMatchObject({ occurrence: expect.stringContaining('2026-10-15T14:00') })
    await router.navigate({ to: '/calendar' })
    await user.click(await block(/^Báo cáo tuần/))
    await screen.findByRole('dialog', { name: 'Báo cáo tuần' })
    await user.keyboard('e')
    await waitFor(() => expect(router.state.location.pathname).toBe(`/calendar/event/${E.report}`))
  })

  it('offers no Sửa on an event the person cannot change', async () => {
    const user = setup()
    await renderCalendar()
    await user.click(await block(/^Phỏng vấn kế toán viên/))
    await screen.findByRole('dialog', { name: 'Phỏng vấn kế toán viên' })
    expect(screen.queryByRole('button', { name: 'Sửa sự kiện' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Thêm' })).toBeNull()
  })

  it('copies an event from Nhân bản in the ⋯ menu', async () => {
    const user = setup()
    const router = await renderCalendar()
    await user.click(await block(/^Báo cáo tuần/))
    await user.click(await screen.findByRole('button', { name: 'Thêm' }))
    await user.click(screen.getByRole('menuitem', { name: 'Nhân bản' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar/new'))
    expect(router.state.location.search).toMatchObject({ copy: E.report, date: '2026-10-15', start: '14:00', end: '15:00' })
  })

  it('deletes from the ⋯ menu the way the trash button does', async () => {
    const user = setup()
    await renderCalendar()
    await user.click(await block(/^Báo cáo tuần/))
    await user.click(await screen.findByRole('button', { name: 'Thêm' }))
    await user.click(screen.getByRole('menuitem', { name: 'Xoá' }))
    await waitFor(() => expect(screen.queryByRole('button', { name: /^Báo cáo tuần/ })).toBeNull())
    const toast = useToastStore.getState().toasts.find((t) => t.message === 'Đã xoá sự kiện')
    expect(toast).toBeDefined()
    // Hoàn tác, so the pending delete does not outlive the test.
    toast!.action!.onClick()
  })

  it('links a group calendar event to the conversation of the group', async () => {
    const user = setup()
    const calendar: CalendarSummary = {
      id: CAL.ws, kind: 'channel', name: 'Nhóm Vận hành', color: 3, visible: true, access: 'write', isMine: false,
      channel: { id: 'cccccccc-aaaa-4bbb-8ccc-000000000001', name: 'Nhóm Vận hành' },
    }
    // The body is drawn straight on the route: it is the same content as the popover and the sheet.
    const root = createRootRoute({
      component: () => (
        <EventDetailBody
          wsId="w" occurrence={OCCURRENCES.find((o) => o.eventId === E.report)!} calendar={calendar} onClose={() => {}}
          onDelete={() => {}} onRsvp={() => {}}
        />
      ),
    })
    const chat = createRoute({ getParentRoute: () => root, path: '/channels/$channelId', component: () => <p>Cuộc trò chuyện</p> })
    const r2 = createRouter({ routeTree: root.addChildren([chat]), history: createMemoryHistory({ initialEntries: ['/'] }) })
    await r2.load()
    renderWithClient(<RouterProvider router={r2} />)
    await user.click(await screen.findByRole('button', { name: 'Thêm' }))
    await user.click(screen.getByRole('menuitem', { name: 'Mở cuộc trò chuyện' }))
    await waitFor(() => expect(r2.state.location.pathname).toBe('/channels/cccccccc-aaaa-4bbb-8ccc-000000000001'))
  })
})

describe('the video call of an event', () => {
  const at = (h: number, m: number) => new Date(2026, 9, 14, h, m)
  const open = async (now: Date) => {
    vi.setSystemTime(now)
    const user = setup()
    const router = await renderCalendar()
    await user.click(await block(/^Họp giao ban/))
    await screen.findByRole('dialog', { name: 'Họp giao ban' })
    return { user, router }
  }

  it('is hidden before the window opens and says when it does', async () => {
    await open(at(8, 30))
    expect(screen.queryByRole('button', { name: 'Tham gia cuộc gọi' })).toBeNull()
    expect(screen.getByText('Mở từ 15 phút trước giờ bắt đầu.')).toBeInTheDocument()
  })

  it('shows from 15 minutes before the start and joins the call screen', async () => {
    const { user, router } = await open(at(8, 45))
    await user.click(screen.getByRole('button', { name: 'Tham gia cuộc gọi' }))
    await waitFor(() => expect(writes('POST').some((c) => c.path === `/calendar-events/${E.giaoBan}/call`)).toBe(true))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calls/ca110000-aaaa-4bbb-8ccc-000000000001'))
  })

  it('stays while the event runs and goes at its end', async () => {
    await open(at(9, 59))
    expect(screen.getByRole('button', { name: 'Tham gia cuộc gọi' })).toBeInTheDocument()
    cleanup()
    await open(at(10, 0))
    expect(screen.queryByRole('button', { name: 'Tham gia cuộc gọi' })).toBeNull()
    expect(screen.queryByText('Mở từ 15 phút trước giờ bắt đầu.')).toBeNull()
  })
})

describe('search', () => {
  it('opens from the icon and from the / key, and lists matches as an agenda', async () => {
    const user = setup()
    await renderCalendar()
    await block(/^Báo cáo tuần/)
    await user.click(screen.getByRole('button', { name: 'Tìm sự kiện' }))
    const box = await screen.findByRole('searchbox', { name: 'Tìm sự kiện' })
    await user.type(box, 'Báo cáo')
    const result = await screen.findByRole('button', { name: /^Báo cáo tuần/ })
    expect(screen.getByTestId('agenda')).toContainElement(result)
    expect(calls.some((c) => c.path.endsWith('/events/search') && c.query.get('q') === 'Báo cáo')).toBe(true)
    expect(screen.queryByTestId('time-grid')).toBeNull()
  })

  it('says when nothing matches, and Esc brings the grid back', async () => {
    const user = setup()
    await renderCalendar()
    await block(/^Báo cáo tuần/)
    await user.keyboard('/')
    await user.type(await screen.findByRole('searchbox', { name: 'Tìm sự kiện' }), 'zzz')
    expect(await screen.findByText('Không có sự kiện nào khớp “zzz”.')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(await screen.findByTestId('time-grid')).toBeInTheDocument()
  })

  it('opens the details of a result', async () => {
    const user = setup()
    await renderCalendar()
    await user.keyboard('/')
    await user.type(await screen.findByRole('searchbox', { name: 'Tìm sự kiện' }), 'Standup')
    await user.click(await block(/^Standup/))
    expect(await screen.findByRole('dialog', { name: 'Standup' })).toBeInTheDocument()
  })
})

describe('settings', () => {
  it('saves the zone, the working hours and the default reminder', async () => {
    const user = setup()
    await renderCalendar()
    await block(/^Báo cáo tuần/)
    await user.click(screen.getByRole('button', { name: 'Cài đặt lịch' }))
    const dialog = await screen.findByRole('dialog', { name: 'Cài đặt lịch' })
    await waitFor(() => expect(within(dialog).getByLabelText('Kết thúc làm việc')).toHaveValue('17:00'))
    const end = within(dialog).getByLabelText('Kết thúc làm việc')
    await user.clear(end)
    await user.type(end, '18:30')
    await user.selectOptions(within(dialog).getByLabelText('Lời nhắc mặc định'), '30')
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }))
    await waitFor(() => expect(writes('PUT').some((c) => c.path.endsWith('/calendar/settings'))).toBe(true))
    expect(writes('PUT').find((c) => c.path.endsWith('/calendar/settings'))!.body).toEqual({
      tzid: 'Asia/Ho_Chi_Minh', workStart: '08:00', workEnd: '18:30', defaultReminder: 30,
    })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Cài đặt lịch' })).toBeNull())
  })

  it('refuses working hours that end before they start', async () => {
    const user = setup()
    await renderCalendar()
    await user.click(await screen.findByRole('button', { name: 'Cài đặt lịch' }))
    const dialog = await screen.findByRole('dialog', { name: 'Cài đặt lịch' })
    await waitFor(() => expect(within(dialog).getByLabelText('Kết thúc làm việc')).toHaveValue('17:00'))
    const end = within(dialog).getByLabelText('Kết thúc làm việc')
    await user.clear(end)
    await user.type(end, '07:00')
    expect(within(dialog).getByRole('alert')).toHaveTextContent('Giờ kết thúc phải sau giờ bắt đầu')
    expect(within(dialog).getByRole('button', { name: 'Lưu' })).toBeDisabled()
  })
})

describe('sharing a calendar', () => {
  async function openShare() {
    const user = setup()
    await renderCalendar()
    await user.click(await screen.findByRole('button', { name: 'Tuỳ chọn của lịch Lê Thị Hoa' }))
    await user.click(screen.getByRole('menuitem', { name: 'Chia sẻ' }))
    const dialog = await screen.findByRole('dialog', { name: 'Chia sẻ lịch “Lê Thị Hoa”' })
    return { user, dialog }
  }

  it('is offered only on a calendar the person manages', async () => {
    const user = setup()
    await renderCalendar()
    await user.click(await screen.findByRole('button', { name: 'Tuỳ chọn của lịch Khối Vận hành' }))
    expect(screen.queryByRole('menuitem', { name: 'Chia sẻ' })).toBeNull()
  })

  it('pins the whole workspace as busy/free only', async () => {
    const { dialog } = await openShare()
    expect(within(dialog).getByText(/^Mọi người trong /)).toBeInTheDocument()
    expect(within(dialog).getByText('Thấy bận/rảnh')).toBeInTheDocument()
  })

  it('shares with a person at the first level, changes the level and takes it away', async () => {
    const { user, dialog } = await openShare()
    await user.type(within(dialog).getByRole('combobox', { name: 'Thêm người' }), 'Yến')
    await user.click(await within(dialog).findByRole('option', { name: /Phạm Hải Yến/ }))
    await waitFor(() => expect(writes('PUT').some((c) => c.path.endsWith(`/shares/${PEOPLE.yen.nodeId}`))).toBe(true))
    expect(writes('PUT').find((c) => c.path.endsWith(`/shares/${PEOPLE.yen.nodeId}`))!.body).toEqual({ level: 'view' })
    const level = await within(dialog).findByLabelText('Mức chia sẻ của Phạm Hải Yến')
    expect(within(level).getAllByRole('option').map((o) => o.textContent)).toEqual(['Xem mọi chi tiết', 'Sửa sự kiện', 'Sửa và quản lý chia sẻ'])

    await user.selectOptions(level, 'edit')
    await waitFor(() => expect(writes('PUT').filter((c) => c.path.includes('/shares/'))).toHaveLength(2))
    expect(writes('PUT').filter((c) => c.path.includes('/shares/'))[1]!.body).toEqual({ level: 'edit' })
    expect(await within(dialog).findByLabelText('Mức chia sẻ của Phạm Hải Yến')).toHaveValue('edit')

    await user.click(within(dialog).getByRole('button', { name: 'Bỏ chia sẻ với Phạm Hải Yến' }))
    await waitFor(() => expect(writes('DELETE')).toHaveLength(1))
    await waitFor(() => expect(within(dialog).queryByLabelText('Mức chia sẻ của Phạm Hải Yến')).toBeNull())
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })
})

describe('Lịch khác', () => {
  it('adds the calendar of a person picked by name', async () => {
    const user = setup()
    await renderCalendar()
    const side = await screen.findByRole('complementary', { name: 'Lịch' })
    await user.type(await within(side).findByRole('combobox', { name: 'Tìm người' }), 'Yến')
    await user.click(await screen.findByRole('option', { name: /Phạm Hải Yến/ }))
    await waitFor(() => expect(writes('POST').some((c) => c.path.endsWith('/calendar/subscriptions'))).toBe(true))
    expect(writes('POST').find((c) => c.path.endsWith('/calendar/subscriptions'))!.body).toEqual({ userNodeId: PEOPLE.yen.nodeId })
    expect(await within(side).findByRole('checkbox', { name: 'Phạm Hải Yến' })).toBeChecked()
  })

  it('takes a calendar of somebody else out of the list', async () => {
    const user = setup()
    await renderCalendar()
    await user.click(await screen.findByRole('button', { name: 'Tuỳ chọn của lịch Lê Quang Vinh' }))
    await user.click(screen.getByRole('menuitem', { name: 'Bỏ khỏi danh sách' }))
    await waitFor(() => expect(writes('DELETE')).toHaveLength(1))
    expect(writes('DELETE')[0]!.path).toContain(`/subscriptions/${CAL.vinh}`)
  })
})

describe('on a phone', () => {
  it('opens on the agenda with a week strip, no side column and a floating +', async () => {
    viewport('phone')
    await renderCalendar()
    expect(await screen.findByTestId('agenda')).toBeInTheDocument()
    expect(screen.getByTestId('week-strip')).toBeInTheDocument()
    expect(screen.queryByRole('complementary', { name: 'Lịch' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Tạo sự kiện' })).toBeInTheDocument()
    expect(within(screen.getByTestId('week-strip')).getByRole('button', { name: 'Chọn ngày 14 tháng 10' })).toHaveAttribute('aria-current', 'date')
  })

  it('draws a dot under the days that have events', async () => {
    viewport('phone')
    await renderCalendar()
    await screen.findByTestId('agenda')
    const strip = screen.getByTestId('week-strip')
    await waitFor(() => {
      const dot = (label: string) => within(strip).getByRole('button', { name: label }).querySelector('span:last-child')!
      expect(dot('Chọn ngày 14 tháng 10')).toHaveClass('bg-ink-muted')
      expect(dot('Chọn ngày 17 tháng 10')).not.toHaveClass('bg-ink-muted')
    })
  })

  it('moves to a day from the strip', async () => {
    viewport('phone')
    const user = setup()
    const router = await renderCalendar()
    await screen.findByTestId('agenda')
    await user.click(within(screen.getByTestId('week-strip')).getByRole('button', { name: 'Chọn ngày 16 tháng 10' }))
    await waitFor(() => expect((router.state.location.search as { date?: string }).date).toBe('2026-10-16'))
  })

  it('opens the details of an event as a bottom sheet', async () => {
    viewport('phone')
    const user = setup()
    await renderCalendar()
    await user.click(await screen.findByRole('button', { name: /^Họp giao ban/ }))
    const sheet = await screen.findByRole('dialog', { name: 'Họp giao ban' })
    expect(within(sheet).getByRole('button', { name: 'Sửa sự kiện' })).toBeInTheDocument()
    expect(within(sheet).getByText('Cuộc gọi video Nexus')).toBeInTheDocument()
  })

  it('changes the view from the ⋯ sheet, to the four a phone offers', async () => {
    viewport('phone')
    const user = setup()
    const router = await renderCalendar()
    await screen.findByTestId('agenda')
    await user.click(screen.getByRole('button', { name: 'Kiểu xem' }))
    const group = await screen.findByRole('radiogroup', { name: 'Kiểu xem' })
    expect(within(group).getAllByRole('radio').map((r) => r.textContent)).toEqual(['Lịch biểu', 'Ngày', '3 ngày', 'Tháng'])
    await user.click(within(group).getByRole('radio', { name: '3 ngày' }))
    await waitFor(() => expect((router.state.location.search as { mode?: string }).mode).toBe('3day'))
    expect(await screen.findByTestId('time-grid')).toBeInTheDocument()
  })

  it('folds the month open from the month name', async () => {
    viewport('phone')
    const user = setup()
    await renderCalendar()
    await screen.findByTestId('agenda')
    await user.click(screen.getByRole('button', { name: /^Tháng 10 năm 2026/ }))
    expect(await screen.findByTestId('month-fold')).toBeInTheDocument()
  })

  it('opens the editor from the floating + and the phone search from the bar pill', async () => {
    viewport('phone')
    const user = setup()
    const router = await renderCalendar()
    await screen.findByTestId('agenda')
    const pill = document.querySelector<HTMLElement>('[data-module-search]')!
    pill.click()
    expect(await screen.findByRole('searchbox', { name: 'Tìm sự kiện' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Đóng tìm kiếm' }))
    await user.click(screen.getByRole('button', { name: 'Tạo sự kiện' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar/new'))
  })
})

describe('on a tablet', () => {
  it('opens on three days and shows the details in a sheet on the right', async () => {
    viewport('tablet')
    const user = setup()
    await renderCalendar()
    expect(await screen.findByTestId('time-grid')).toBeInTheDocument()
    expect(screen.queryByTestId('week-strip')).toBeNull()
    await user.click(await screen.findByRole('button', { name: /^Họp giao ban/ }))
    expect(await screen.findByRole('dialog', { name: 'Họp giao ban' })).toBeInTheDocument()
  })

  it('opens the calendars from the panel button', async () => {
    viewport('tablet')
    const user = setup()
    await renderCalendar()
    await user.click(await screen.findByRole('button', { name: 'Lịch đang hiện' }))
    const sheet = await screen.findByRole('dialog', { name: 'Lịch đang hiện' })
    expect(await within(sheet).findByRole('checkbox', { name: 'Lê Thị Hoa' })).toBeChecked()
  })
})

describe('touch', () => {
  const touch = (type: 'pointerDown' | 'pointerMove' | 'pointerUp', el: Element | Document, y: number) =>
    fireEvent[type](el, { clientX: 100, clientY: y, pointerId: 7, pointerType: 'touch', button: 0 })
  const hold = () => new Promise((r) => setTimeout(r, 450))

  it('picks a block up after holding it 400ms and moves it by the finger', async () => {
    viewport('tablet')
    await renderCalendar()
    const b = await block(/^Báo cáo tuần/)
    touch('pointerDown', b, 700)
    await hold()
    touch('pointerMove', document, 700 + 48)
    touch('pointerUp', document, 700 + 48)
    await waitFor(() => expect(writes('PATCH')).toHaveLength(1))
    expect(String(writes('PATCH')[0]!.body!.start)).toMatch(/T15:00:00/)
  })

  it('leaves a swipe alone: moving before the hold is over is a scroll, not a drag', async () => {
    viewport('tablet')
    await renderCalendar()
    const b = await block(/^Báo cáo tuần/)
    touch('pointerDown', b, 700)
    touch('pointerMove', document, 760)
    await hold()
    touch('pointerMove', document, 820)
    touch('pointerUp', document, 820)
    expect(writes('PATCH')).toHaveLength(0)
  })

  it('does not move a block that is only tapped', async () => {
    viewport('tablet')
    await renderCalendar()
    const b = await block(/^Báo cáo tuần/)
    touch('pointerDown', b, 700)
    touch('pointerUp', document, 700)
    expect(writes('PATCH')).toHaveLength(0)
  })
})
