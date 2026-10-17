import { act, cleanup, configure, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { formatISO } from 'date-fns'
import { renderWithClient, resetClient } from '../../test/render'
import {
  CAL, E, NOW, UUID_RE, calendarFixtureApi, calls, mode, resetFixtures,
} from '../../test/calendar-fixtures'
import { validateCalendarSearch } from '../../lib/calendar-search'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { CalendarScreen } from './CalendarScreen'

vi.mock('../../api/client', async (orig) => {
  const { calendarFixtureApi } = await import('../../test/calendar-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(calendarFixtureApi) }
})

const api = vi.mocked(apiFetch)

// The whole suite runs in parallel with the rest; the first render of a heavy screen can be slow.
configure({ asyncUtilTimeout: 4000 })
vi.setConfig({ testTimeout: 15_000 })
const local = (day: number, h: number, m = 0) => formatISO(new Date(2026, 9, day, h, m))

async function renderCalendar(url = '/calendar') {
  const root = createRootRoute()
  const route = createRoute({
    getParentRoute: () => root, path: '/calendar', validateSearch: validateCalendarSearch, component: CalendarScreen,
  })
  const router = createRouter({
    routeTree: root.addChildren([route]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

const writes = (method: string) => calls.filter((c) => c.method === method)
const block = (name: RegExp | string) => screen.findByRole('button', { name })

beforeEach(() => {
  // The view a person last chose is remembered on the device; every test starts without one.
  localStorage.clear()
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(NOW)
  resetFixtures()
  api.mockImplementation(calendarFixtureApi)
})
afterEach(() => {
  // Unmount before the cache is emptied: a screen still mounted would rebuild its queries into the cleared cache.
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
})

/** Columns get a real width, so a drop can tell which day the pointer is over. */
function layOutColumns() {
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
    const day = this.dataset?.day
    if (day) return new DOMRect((Number(day.slice(-2)) - 12) * 100, 0, 100, 1152)
    return new DOMRect()
  })
}

const press = (el: Element, x: number, y: number) =>
  fireEvent.pointerDown(el, { clientX: x, clientY: y, pointerId: 1, pointerType: 'mouse', button: 0 })
const dragTo = (x: number, y: number) => fireEvent.pointerMove(document, { clientX: x, clientY: y, pointerId: 1, pointerType: 'mouse' })
const release = (x: number, y: number) => fireEvent.pointerUp(document, { clientX: x, clientY: y, pointerId: 1, pointerType: 'mouse' })

describe('the week', () => {
  it('starts on Monday, numbers the days and marks today', async () => {
    await renderCalendar()
    await block(/Họp giao ban/)
    const grid = screen.getByTestId('time-grid')
    const numbers = within(grid).getAllByRole('button', { name: /^Mở ngày/ }).map((b) => b.textContent)
    expect(numbers).toEqual(['12', '13', '14', '15', '16', '17', '18'])
    expect(within(grid).getAllByText(/^(T[2-7]|CN)$/).map((e) => e.textContent)).toEqual(['T2', 'T3', 'T4', 'T5', 'T6', 'T7', 'CN'])
    expect(within(grid).getByRole('button', { name: 'Mở ngày 14 tháng 10' })).toHaveAttribute('aria-current', 'date')
    expect(screen.getByRole('heading', { name: /Tháng 10 năm 2026/ })).toHaveTextContent('Tuần 42')
  })

  it('draws the red line of the present moment', async () => {
    await renderCalendar()
    await block(/Họp giao ban/)
    expect(screen.getByRole('img', { name: 'Bây giờ, 10:40' })).toBeInTheDocument()
  })

  it('names each block with its time and answer, and draws each state its own way', async () => {
    await renderCalendar()
    const interview = await block('Phỏng vấn kế toán viên, 15:00 đến 16:00, chưa trả lời, Phòng họp 3')
    expect(interview).toHaveAttribute('data-tone', 'pending')
    expect(await block('Họp giao ban, 09:00 đến 10:00')).toHaveAttribute('data-tone', 'past')
    expect(await block('Báo cáo tuần, 14:00 đến 15:00')).toHaveAttribute('data-tone', 'solid')
    expect(await block('Bận, 13:00 đến 14:00')).toHaveAttribute('data-tone', 'busy')
    expect(await block('Nghỉ phép, cả ngày')).toBeInTheDocument()
  })

  it('splits two events that overlap into two columns, the later one over the right half', async () => {
    await renderCalendar()
    const first = await block(/^Họp giao ban/)
    const second = await block(/^Review A/)
    expect(first.style.left).toBe('0%')
    expect(second.style.left).toBe('50%')
    expect(Number.parseFloat(first.style.width)).toBeGreaterThan(50)
  })

  it('puts the calendars in two groups with a ticked, named checkbox each', async () => {
    await renderCalendar()
    const side = await screen.findByRole('complementary', { name: 'Lịch' })
    expect(await within(side).findByRole('checkbox', { name: 'Lê Thị Hoa' })).toBeChecked()
    expect(within(side).getByRole('region', { name: 'Lịch của tôi' })).toContainElement(within(side).getByRole('checkbox', { name: 'Khối Vận hành' }))
    expect(within(side).getByRole('region', { name: 'Lịch khác' })).toContainElement(within(side).getByRole('checkbox', { name: 'Lê Quang Vinh' }))
  })

  it('hides a calendar from the grid when its tick is taken off, and tells the server', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await block(/^Bận/)
    await user.click(await screen.findByRole('checkbox', { name: 'Lê Quang Vinh' }))
    await waitFor(() => expect(writes('PUT')).toHaveLength(1))
    expect(writes('PUT')[0]).toMatchObject({ path: `/workspaces/0a8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a6b/calendar/calendars/${CAL.vinh}/subscription`, body: { visible: false } })
    await waitFor(() => {
      const listed = calls.filter((c) => c.path.endsWith('/calendar/events') && c.method === 'GET').slice(-1)[0]!
      expect(listed.query.get('calendars')).toBe(`${CAL.mine},${CAL.ws}`)
    })
  })

  it('shows no identifier anywhere', async () => {
    await renderCalendar()
    await block(/Họp giao ban/)
    expect(document.body.textContent).not.toMatch(UUID_RE)
    for (const el of document.body.querySelectorAll('[aria-label],[title]')) {
      expect(el.getAttribute('aria-label') ?? '').not.toMatch(UUID_RE)
      expect(el.getAttribute('title') ?? '').not.toMatch(UUID_RE)
    }
  })
})

describe('the popover', () => {
  it('opens on the block with the title, the time and the place, and Esc closes it and gives focus back', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    const b = await block(/^Phỏng vấn kế toán viên/)
    await user.click(b)
    const dialog = await screen.findByRole('dialog', { name: 'Phỏng vấn kế toán viên' })
    expect(within(dialog).getByText('Thứ Tư, 14 tháng 10 · 15:00 – 16:00')).toBeInTheDocument()
    expect(within(dialog).getByText('Phòng họp 3')).toBeInTheDocument()
    expect(await within(dialog).findByText('Phạm Hải Yến')).toBeInTheDocument()
    expect(within(dialog).getByText('1 đã nhận lời, 1 đã từ chối, 1 chưa trả lời')).toBeInTheDocument()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Phỏng vấn kế toán viên' })).toBeNull())
    expect(b).toHaveFocus()
  })

  it('lets a guest answer, which calls the API with the answer and the scope and shows at once', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await user.click(await block(/^Phỏng vấn kế toán viên/))
    const dialog = await screen.findByRole('dialog', { name: 'Phỏng vấn kế toán viên' })
    await user.click(within(dialog).getByRole('button', { name: 'Có' }))
    await waitFor(() => expect(writes('POST')).toHaveLength(1))
    expect(writes('POST')[0]!.path).toMatch(new RegExp(`/events/${E.interview}/rsvp$`))
    expect(writes('POST')[0]!.body).toMatchObject({ response: 'accepted', scope: 'this', occurrence: local(14, 15) })
    await waitFor(() => expect(within(dialog).getByRole('button', { name: 'Có' })).toHaveAttribute('aria-pressed', 'true'))
    expect(await block('Phỏng vấn kế toán viên, 15:00 đến 16:00, Phòng họp 3')).toHaveAttribute('data-tone', 'solid')
  })

  it('offers the organizer no answer strip, only that they organize', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await user.click(await block(/^Họp giao ban/))
    const dialog = await screen.findByRole('dialog', { name: 'Họp giao ban' })
    expect(within(dialog).getByText('Bạn là người tổ chức')).toBeInTheDocument()
    expect(within(dialog).queryByRole('group', { name: 'Bạn sẽ tham gia?' })).toBeNull()
    expect(within(dialog).getByText('Cuộc gọi video Nexus')).toBeInTheDocument()
  })

  it('hides Xoá where the person may not edit, and shows it where they may', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await user.click(await block(/^Phỏng vấn kế toán viên/))
    const guest = await screen.findByRole('dialog', { name: 'Phỏng vấn kế toán viên' })
    expect(within(guest).queryByRole('button', { name: 'Xoá sự kiện' })).toBeNull()
    expect(within(guest).queryByRole('button', { name: 'Sửa sự kiện' })).toBeNull()
    await user.keyboard('{Escape}')
    await user.click(await block(/^Báo cáo tuần/))
    const own = await screen.findByRole('dialog', { name: 'Báo cáo tuần' })
    expect(within(own).getByRole('button', { name: 'Xoá sự kiện' })).toBeInTheDocument()
  })

  it('does not open the details of busy time', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await user.click(await block(/^Bận/))
    expect(screen.queryByRole('dialog')).toBeNull()
  })

  it('removes an event at once and does not send the delete while Hoàn tác is on offer; undo brings it back', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await user.click(await block(/^Báo cáo tuần/))
    await user.click(await screen.findByRole('button', { name: 'Xoá sự kiện' }))
    await waitFor(() => expect(screen.queryByRole('button', { name: /^Báo cáo tuần/ })).toBeNull())
    expect(useToastStore.getState().toasts[0]).toMatchObject({ message: 'Đã xoá sự kiện', action: { label: 'Hoàn tác' } })
    expect(writes('DELETE')).toHaveLength(0)
    await act(async () => useToastStore.getState().toasts[0]!.action!.onClick())
    expect(await block(/^Báo cáo tuần/)).toBeInTheDocument()
    expect(writes('DELETE')).toHaveLength(0)
  })

  it('sends the delete once the undo window has passed, without a scope for a single event', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await user.click(await block(/^Báo cáo tuần/))
    // Take hold of the timer the delete sets, so the five seconds need not be waited out.
    const real = globalThis.setTimeout
    const undoWindow: (() => void)[] = []
    vi.spyOn(globalThis, 'setTimeout').mockImplementation(((fn: () => void, ms?: number, ...rest: unknown[]) => {
      if (ms === 5000) {
        undoWindow.push(fn)
        return 0 as unknown as ReturnType<typeof setTimeout>
      }
      return real(fn, ms, ...rest)
    }) as typeof setTimeout)
    await user.click(await screen.findByRole('button', { name: 'Xoá sự kiện' }))
    expect(undoWindow).toHaveLength(1)
    expect(writes('DELETE')).toHaveLength(0)
    await act(async () => undoWindow[0]!())
    await waitFor(() => expect(writes('DELETE')).toHaveLength(1))
    expect(writes('DELETE')[0]!.path).toMatch(new RegExp(`/events/${E.report}$`))
    expect(writes('DELETE')[0]!.query.get('scope')).toBeNull()
  })

  it('asks which occurrences before deleting a recurring event, and passes the scope and the instance', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await user.click(await block(/^Standup/))
    await user.click(await screen.findByRole('button', { name: 'Xoá sự kiện' }))
    const dialog = await screen.findByRole('dialog', { name: 'Xoá sự kiện lặp' })
    const real = globalThis.setTimeout
    const undoWindow: (() => void)[] = []
    vi.spyOn(globalThis, 'setTimeout').mockImplementation(((fn: () => void, ms?: number, ...rest: unknown[]) => {
      if (ms === 5000) {
        undoWindow.push(fn)
        return 0 as unknown as ReturnType<typeof setTimeout>
      }
      return real(fn, ms, ...rest)
    }) as typeof setTimeout)
    await user.click(within(dialog).getByRole('radio', { name: 'Tất cả sự kiện' }))
    await user.click(within(dialog).getByRole('button', { name: 'OK' }))
    await waitFor(() => expect(undoWindow).toHaveLength(1))
    await act(async () => undoWindow[0]!())
    await waitFor(() => expect(writes('DELETE')).toHaveLength(1))
    expect(writes('DELETE')[0]!.query.get('scope')).toBe('all')
    expect(writes('DELETE')[0]!.query.get('occurrence')).toBe(local(16, 9))
  })
})

describe('dragging', () => {
  it('moves an event by the distance dragged, in quarter hours, and saves it with the version it began from', async () => {
    await renderCalendar()
    const b = await block(/^Báo cáo tuần/)
    press(b, 350, 700)
    dragTo(350, 748)
    // Live: the block is already at 15:00 while the pointer is down.
    expect(await screen.findByRole('button', { name: 'Báo cáo tuần, 15:00 đến 16:00' })).toBeInTheDocument()
    release(350, 748)
    await waitFor(() => expect(writes('PATCH')).toHaveLength(1))
    const patch = writes('PATCH')[0]!
    expect(patch.path).toMatch(new RegExp(`/events/${E.report}$`))
    expect(patch.query.get('scope')).toBeNull()
    expect(patch.body).toMatchObject({ start: local(15, 15), end: local(15, 16), version: 3 })
    await waitFor(() => expect(useToastStore.getState().toasts[0]?.message).toBe('Đã dời sang 15:00'))
  })

  it('snaps to five minutes with Alt', async () => {
    await renderCalendar()
    const b = await block(/^Báo cáo tuần/)
    press(b, 350, 700)
    fireEvent.pointerMove(document, { clientX: 350, clientY: 704, pointerId: 1, pointerType: 'mouse', altKey: true })
    fireEvent.pointerUp(document, { clientX: 350, clientY: 704, pointerId: 1, pointerType: 'mouse', altKey: true })
    await waitFor(() => expect(writes('PATCH')).toHaveLength(1))
    expect(writes('PATCH')[0]!.body).toMatchObject({ start: local(15, 14, 5) })
  })

  it('carries an event to another day when dropped on its column', async () => {
    layOutColumns()
    await renderCalendar()
    const b = await block(/^Báo cáo tuần/)
    press(b, 350, 700)
    dragTo(450, 700)
    release(450, 700)
    await waitFor(() => expect(writes('PATCH')).toHaveLength(1))
    expect(writes('PATCH')[0]!.body).toMatchObject({ start: local(16, 14), end: local(16, 15) })
  })

  it('resizes from the lower edge and moves only the end', async () => {
    await renderCalendar()
    const b = await block(/^Báo cáo tuần/)
    const edge = b.querySelector('[data-resize]')!
    press(edge, 350, 760)
    dragTo(350, 784)
    release(350, 784)
    await waitFor(() => expect(writes('PATCH')).toHaveLength(1))
    expect(writes('PATCH')[0]!.body).toMatchObject({ start: local(15, 14), end: local(15, 15, 30) })
  })

  it('does not drag what the person may not edit, and a small wobble is still a click', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    const guest = await block(/^Phỏng vấn kế toán viên/)
    press(guest, 350, 700)
    dragTo(350, 800)
    release(350, 800)
    expect(writes('PATCH')).toHaveLength(0)

    const own = await block(/^Báo cáo tuần/)
    press(own, 350, 700)
    dragTo(351, 701)
    release(351, 701)
    await user.click(own)
    expect(writes('PATCH')).toHaveLength(0)
    expect(await screen.findByRole('dialog', { name: 'Báo cáo tuần' })).toBeInTheDocument()
  })

  it('puts the block back and says so when the server refuses', async () => {
    mode.writeStatus = 403
    await renderCalendar()
    const b = await block(/^Báo cáo tuần/)
    press(b, 350, 700)
    dragTo(350, 748)
    release(350, 748)
    await waitFor(() => expect(useToastStore.getState().toasts[0]?.tone).toBe('error'))
    expect(useToastStore.getState().toasts[0]!.message).toMatch(/quyền/)
    expect(await block('Báo cáo tuần, 14:00 đến 15:00')).toBeInTheDocument()
  })

  it('sends the old times back on Hoàn tác', async () => {
    await renderCalendar()
    const b = await block(/^Báo cáo tuần/)
    press(b, 350, 700)
    dragTo(350, 748)
    release(350, 748)
    await waitFor(() => expect(useToastStore.getState().toasts[0]?.action?.label).toBe('Hoàn tác'))
    await act(async () => useToastStore.getState().toasts[0]!.action!.onClick())
    await waitFor(() => expect(writes('PATCH')).toHaveLength(2))
    expect(writes('PATCH')[1]!.body).toMatchObject({ start: local(15, 14), end: local(15, 15), version: 4 })
  })

  it('asks the scope of a recurring event first, then saves with it and the instance', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    const b = await block(/^Standup/)
    press(b, 450, 460)
    dragTo(450, 484)
    release(450, 484)
    expect(writes('PATCH')).toHaveLength(0)
    const dialog = await screen.findByRole('dialog', { name: 'Dời sự kiện lặp' })
    expect(within(dialog).getByRole('radio', { name: 'Chỉ sự kiện này' })).toBeChecked()
    await user.click(within(dialog).getByRole('radio', { name: 'Sự kiện này và các sự kiện sau' }))
    await user.click(within(dialog).getByRole('button', { name: 'OK' }))
    await waitFor(() => expect(writes('PATCH')).toHaveLength(1))
    expect(writes('PATCH')[0]!.query.get('scope')).toBe('following')
    expect(writes('PATCH')[0]!.query.get('occurrence')).toBe(local(16, 9))
    expect(writes('PATCH')[0]!.body).toMatchObject({ start: local(16, 9, 30) })
  })

  it('does nothing when the scope question is cancelled', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    press(await block(/^Standup/), 450, 460)
    dragTo(450, 484)
    release(450, 484)
    await user.click(within(await screen.findByRole('dialog', { name: 'Dời sự kiện lặp' })).getByRole('button', { name: 'Huỷ' }))
    expect(writes('PATCH')).toHaveLength(0)
  })
})

describe('quick create', () => {
  it('draws a draft across empty time, opens a card with the title focused, and saves into the personal calendar', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await block(/^Báo cáo tuần/)
    const column = document.querySelector('[data-day="2026-10-13"]')!
    press(column, 120, 12 * 48)
    dragTo(120, 13 * 48 + 20)
    release(120, 13 * 48 + 20)
    const card = await screen.findByRole('dialog', { name: 'Tạo nhanh' })
    expect(document.querySelector('[data-draft]')).toHaveTextContent('12:00 – 13:30')
    const title = within(card).getByRole('textbox', { name: /Tiêu đề/ })
    await waitFor(() => expect(title).toHaveFocus())
    await user.type(title, 'Họp với kế toán')
    await user.click(within(card).getByRole('button', { name: 'Lưu' }))
    await waitFor(() => expect(writes('POST')).toHaveLength(1))
    expect(writes('POST')[0]!.body).toMatchObject({
      calendarId: CAL.mine, title: 'Họp với kế toán', start: local(13, 12), end: local(13, 13, 30), allDay: false, kind: 'event',
    })
    await waitFor(() => expect(useToastStore.getState().toasts[0]?.message).toBe('Đã tạo sự kiện'))
    expect(useToastStore.getState().toasts[0]!.action?.label).toBe('Hoàn tác')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Tạo nhanh' })).toBeNull())
  })

  it('makes an hour from a click on empty time and cancels with Esc without asking', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await block(/^Báo cáo tuần/)
    const column = document.querySelector('[data-day="2026-10-13"]')!
    press(column, 120, 5 * 48)
    release(120, 5 * 48)
    await screen.findByRole('dialog', { name: 'Tạo nhanh' })
    expect(document.querySelector('[data-draft]')).toHaveTextContent('05:00 – 06:00')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Tạo nhanh' })).toBeNull())
    expect(document.querySelector('[data-draft]')).toBeNull()
    expect(writes('POST')).toHaveLength(0)
  })

  it('keeps the card when a click outside lands after the title has text, and drops it when the title is empty', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await block(/^Báo cáo tuần/)
    const column = document.querySelector('[data-day="2026-10-13"]')!
    press(column, 120, 5 * 48)
    release(120, 5 * 48)
    const card = await screen.findByRole('dialog', { name: 'Tạo nhanh' })
    await user.type(within(card).getByRole('textbox', { name: /Tiêu đề/ }), 'Nháp')
    fireEvent.pointerDown(document.body)
    expect(screen.getByRole('dialog', { name: 'Tạo nhanh' })).toBeInTheDocument()
    await user.clear(within(card).getByRole('textbox', { name: /Tiêu đề/ }))
    fireEvent.pointerDown(document.body)
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Tạo nhanh' })).toBeNull())
  })

  it('opens from Tạo and from the C key', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await block(/^Báo cáo tuần/)
    await user.click(screen.getByRole('button', { name: 'Tạo' }))
    await screen.findByRole('dialog', { name: 'Tạo nhanh' })
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Tạo nhanh' })).toBeNull())
    await user.keyboard('c')
    expect(await screen.findByRole('dialog', { name: 'Tạo nhanh' })).toBeInTheDocument()
  })
})

describe('views and keys', () => {
  it('changes view with the letter keys and keeps it in the address', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const router = await renderCalendar()
    await block(/^Báo cáo tuần/)
    await user.keyboard('m')
    expect(await screen.findByTestId('month-grid')).toBeInTheDocument()
    expect(router.state.location.search).toMatchObject({ mode: 'month' })
    await user.keyboard('a')
    expect(await screen.findByTestId('agenda')).toBeInTheDocument()
    await user.keyboard('d')
    expect(await screen.findByRole('heading', { name: /14 tháng 10 năm 2026/ })).toBeInTheDocument()
    await user.keyboard('x')
    await waitFor(() => expect(router.state.location.search).toMatchObject({ mode: '3day' }))
  })

  it('steps with J and K, and T returns to today', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const router = await renderCalendar()
    await block(/^Báo cáo tuần/)
    await user.keyboard('j')
    await waitFor(() => expect(router.state.location.search).toMatchObject({ date: '2026-10-21' }))
    await user.keyboard('t')
    await waitFor(() => expect(router.state.location.search.date).toBeUndefined())
  })

  it('does not take the letters while a field has them', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const router = await renderCalendar()
    await block(/^Báo cáo tuần/)
    await user.keyboard('c')
    const title = await screen.findByRole('textbox', { name: /Tiêu đề/ })
    await user.type(title, 'mad')
    expect(title).toHaveValue('mad')
    expect(router.state.location.search.mode).toBeUndefined()
  })

  it('switches from the view menu, which names the shortcut of each', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await block(/^Báo cáo tuần/)
    await user.click(screen.getByRole('button', { name: /^Tuần/ }))
    const menu = await screen.findByRole('menu', { name: 'Kiểu xem' })
    expect(within(menu).getAllByRole('menuitemradio').map((i) => i.textContent)).toEqual(['NgàyD', 'TuầnW', 'ThángM', 'Lịch biểuA', '3 ngàyX'])
    await user.click(within(menu).getByRole('menuitemradio', { name: /^Tháng/ }))
    expect(await screen.findByTestId('month-grid')).toBeInTheDocument()
  })

  it('deletes the open event with the Delete key, and leaves an event the person may not edit alone', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar()
    await user.click(await block(/^Phỏng vấn kế toán viên/))
    await screen.findByRole('dialog', { name: 'Phỏng vấn kế toán viên' })
    await user.keyboard('{Delete}')
    expect(useToastStore.getState().toasts).toHaveLength(0)
    await user.keyboard('{Escape}')
    await user.click(await block(/^Báo cáo tuần/))
    await screen.findByRole('dialog', { name: 'Báo cáo tuần' })
    await user.keyboard('{Delete}')
    await waitFor(() => expect(useToastStore.getState().toasts[0]?.message).toBe('Đã xoá sự kiện'))
    await act(async () => useToastStore.getState().toasts[0]!.action!.onClick())
  })

  it('opens the day from a day number', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const router = await renderCalendar()
    await block(/^Báo cáo tuần/)
    await user.click(within(screen.getByTestId('time-grid')).getByRole('button', { name: 'Mở ngày 15 tháng 10' }))
    await waitFor(() => expect(router.state.location.search).toMatchObject({ mode: 'day', date: '2026-10-15' }))
  })
})

describe('the month', () => {
  it('lays the days out from Monday and folds what does not fit into a named button', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    await renderCalendar('/calendar?mode=month')
    const grid = await screen.findByTestId('month-grid')
    await within(grid).findByRole('button', { name: /^Việc A/ })
    const first = grid.querySelector('[data-day]')!
    expect(first.getAttribute('data-day')).toBe('2026-09-28')
    // 15 October holds four events; only room for one chip and the button.
    const more = within(grid).getByRole('button', { name: 'Thêm 3 sự kiện ngày 15 tháng 10' })
    expect(more).toHaveTextContent('3 nữa')
    await user.click(more)
    const list = await screen.findByRole('dialog', { name: 'Sự kiện ngày 15 tháng 10' })
    expect(within(list).getAllByRole('menuitem')).toHaveLength(4)
  })

  it('shows an all-day event as a bar over its days', async () => {
    await renderCalendar('/calendar?mode=month')
    const bar = await screen.findByRole('button', { name: 'Nghỉ phép, cả ngày' })
    expect(bar.style.width).toMatch(/28\.57/)
  })

  it('drags an event to another day, keeping its time', async () => {
    await renderCalendar('/calendar?mode=month')
    const grid = await screen.findByTestId('month-grid')
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      const day = this.dataset?.day
      if (!day) return new DOMRect()
      const [y, m, d] = day.split('-').map(Number)
      const index = Math.round((new Date(y!, m! - 1, d!).getTime() - new Date(2026, 8, 28).getTime()) / 86_400_000)
      return new DOMRect((index % 7) * 100, Math.floor(index / 7) * 100, 100, 100)
    })
    const chip = await within(grid).findByRole('button', { name: /^Việc A/ })
    // 15 Oct is column 3 of week 3 (index 17); 20 Oct is column 1 of week 4.
    press(chip, 350, 250)
    dragTo(150, 350)
    release(150, 350)
    await waitFor(() => expect(writes('PATCH')).toHaveLength(1))
    expect(writes('PATCH')[0]!.body).toMatchObject({ start: local(20, 8), end: local(20, 9) })
  })
})

describe('the agenda', () => {
  it('lists only days that have events, with a Bây giờ line between today\'s past and coming events', async () => {
    await renderCalendar('/calendar?mode=agenda')
    const list = await screen.findByTestId('agenda')
    expect(within(list).getAllByRole('region').map((r) => r.getAttribute('aria-label'))).toEqual(['Ngày 14 tháng 10', 'Ngày 15 tháng 10', 'Ngày 16 tháng 10'])
    expect(within(list).getByRole('separator', { name: 'Bây giờ' })).toBeInTheDocument()
    expect(within(list).getByRole('heading', { name: 'Tháng 10 năm 2026' })).toBeInTheDocument()
    expect(within(list).getByRole('button', { name: 'Tải thêm 30 ngày' })).toBeInTheDocument()
  })
})

describe('states', () => {
  it('shows the shape of the grid while loading', async () => {
    api.mockImplementation(() => new Promise(() => {}))
    await renderCalendar()
    expect(await screen.findByTestId('grid-skeleton')).toHaveAttribute('aria-busy', 'true')
  })

  it('says the week is empty and offers to create', async () => {
    mode.noEvents = true
    await renderCalendar()
    expect(await screen.findByText('Tuần này bạn chưa có sự kiện nào.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Tạo sự kiện' })).toBeInTheDocument()
    expect(screen.getByTestId('time-grid')).toBeInTheDocument()
  })

  it('says the events could not load and tries again', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    mode.eventsError = true
    await renderCalendar()
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Sự kiện đã lưu không bị mất')
    mode.eventsError = false
    await user.click(within(alert).getByRole('button', { name: 'Thử lại' }))
    expect(await block(/^Báo cáo tuần/)).toBeInTheDocument()
  })

  it('says the calendars could not load', async () => {
    mode.calendarsError = true
    await renderCalendar()
    expect(await screen.findByRole('alert')).toHaveTextContent('Không tải được lịch')
  })
})
