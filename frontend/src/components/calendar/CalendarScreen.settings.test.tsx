import { cleanup, configure, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { CAL, NOW, WS_ID, calendarFixtureApi, calls, resetFixtures } from '../../test/calendar-fixtures'
import { validateCalendarSearch, validateEditEventSearch, validateNewEventSearch } from '../../lib/calendar-search'
import { apiFetch } from '../../api/client'
import type { CalendarSettings } from '../../api/settings'
import { queryClient } from '../../lib/query-client'
import { settingsQueryKey } from '../../hooks/useServerSettings'
import { CALENDAR_DEFAULTS } from '../../lib/settings-defaults'
import { useToastStore } from '../primitives'
import { CreateActionFab } from '../patterns/ModuleBar'
import { clearCreateActions } from '../../hooks/useModuleCreateAction'
import { CalendarScreen } from './CalendarScreen'

vi.mock('../../api/client', async (orig) => {
  const { calendarFixtureApi } = await import('../../test/calendar-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(calendarFixtureApi) }
})
const api = vi.mocked(apiFetch)
configure({ asyncUtilTimeout: 4000 })
vi.setConfig({ testTimeout: 15_000 })

async function renderCalendar(url = '/calendar') {
  const root = createRootRoute()
  const page = (path: string, component: () => React.ReactNode, validateSearch?: (s: Record<string, unknown>) => object) =>
    createRoute({ getParentRoute: () => root, path, component, ...(validateSearch ? { validateSearch } : {}) })
  const router = createRouter({
    routeTree: root.addChildren([
      page('/calendar', () => <><CalendarScreen /><CreateActionFab /></>, validateCalendarSearch),
      page('/calendar/new', () => <p>Trình soạn</p>, validateNewEventSearch),
      page('/calendar/event/$eventId', () => <p>Trình soạn</p>, validateEditEventSearch),
    ]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

/** A phone (< 768), a tablet (768-1023) or the desktop. */
function viewport(kind: 'phone' | 'tablet' | 'desktop') {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: kind === 'phone' ? query.includes('max-width') : kind === 'tablet' ? query.includes('1023') : false,
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

const seed = (s: CalendarSettings) => queryClient.setQueryData(settingsQueryKey('calendar', WS_ID), { ...CALENDAR_DEFAULTS, ...s })
const setup = () => userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
const block = (name: RegExp | string) => screen.findByRole('button', { name })

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
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  resetClient()
  useToastStore.getState().clear()
  clearCreateActions()
})

describe('Cài đặt Lịch → Tuần bắt đầu vào', () => {
  it('Chủ Nhật: the week view starts on Sunday', async () => {
    seed({ weekStart: 0 })
    await renderCalendar()
    await block(/Họp giao ban/)
    const grid = screen.getByTestId('time-grid')
    await waitFor(() => expect(within(grid).getAllByText(/^(T[2-7]|CN)$/).map((e) => e.textContent)).toEqual(['CN', 'T2', 'T3', 'T4', 'T5', 'T6', 'T7']))
    expect(within(grid).getAllByRole('button', { name: /^Mở ngày/ }).map((b) => b.textContent)).toEqual(['11', '12', '13', '14', '15', '16', '17'])
  })

  it('Thứ Hai (the default): the week view starts on Monday as before', async () => {
    seed({ weekStart: 1 })
    await renderCalendar()
    await block(/Họp giao ban/)
    expect(within(screen.getByTestId('time-grid')).getAllByRole('button', { name: /^Mở ngày/ }).map((b) => b.textContent))
      .toEqual(['12', '13', '14', '15', '16', '17', '18'])
  })

  it('Chủ Nhật: the month grid and the small month start on Sunday', async () => {
    seed({ weekStart: 0 })
    await renderCalendar('/calendar?mode=month')
    const small = await screen.findByRole('grid', { name: 'Tháng 10' })
    await waitFor(() => expect(within(small).getAllByText(/^(T[2-7]|CN)$/)[0]).toHaveTextContent('CN'))
    // 1 October 2026 is a Thursday: a Sunday-first grid starts on 27 September.
    const grid = await screen.findByTestId('month-grid')
    expect(within(grid).getAllByRole('button', { name: /^Mở ngày/ })[0]).toHaveAccessibleName(/27 tháng 9/)
  })

  it('Chủ Nhật: the week strip of a phone starts on Sunday', async () => {
    viewport('phone')
    seed({ weekStart: 0 })
    await renderCalendar()
    const strip = await screen.findByTestId('week-strip')
    await waitFor(() => expect(within(strip).getAllByRole('button')[0]).toHaveAccessibleName('Chọn ngày 11 tháng 10'))
  })
})

describe('Cài đặt Lịch → Lịch đang hiện', () => {
  it('a list of visible calendars draws only those; an empty list draws every one', async () => {
    seed({ visibleCalendars: [CAL.mine] })
    await renderCalendar()
    await block(/Họp giao ban/)
    expect(screen.queryByRole('button', { name: /^Phỏng vấn kế toán viên/ })).toBeNull()
    const side = screen.getByRole('complementary', { name: 'Lịch' })
    expect(within(side).getByRole('checkbox', { name: 'Khối Vận hành' })).not.toBeChecked()
    expect(within(side).getByRole('checkbox', { name: 'Lê Thị Hoa' })).toBeChecked()
    cleanup()
    resetClient()

    seed({ visibleCalendars: [] })
    await renderCalendar()
    expect(await block(/^Phỏng vấn kế toán viên/)).toBeInTheDocument()
  })

  it('ticking a calendar the list leaves out adds it to the list, and it shows', async () => {
    const user = setup()
    seed({ visibleCalendars: [CAL.mine] })
    await renderCalendar()
    await block(/Họp giao ban/)
    const side = screen.getByRole('complementary', { name: 'Lịch' })
    await user.click(within(side).getByRole('checkbox', { name: 'Khối Vận hành' }))
    await waitFor(() => expect(calls.some((c) => c.method === 'PUT' && c.path.endsWith('/calendar/settings'))).toBe(true))
    const put = calls.find((c) => c.method === 'PUT' && c.path.endsWith('/calendar/settings'))!
    expect(put.body).toEqual({ visibleCalendars: [CAL.mine, CAL.ws] })
    expect(await block(/^Phỏng vấn kế toán viên/)).toBeInTheDocument()
  })
})

describe('Cài đặt Lịch → Chế độ xem mặc định', () => {
  it('a phone opens on the chosen view', async () => {
    viewport('phone')
    seed({ defaultView: 'month' })
    await renderCalendar()
    expect(await screen.findByTestId('month-grid')).toBeInTheDocument()
    expect(screen.queryByTestId('agenda')).toBeNull()
  })

  it('a phone opens on the agenda by default', async () => {
    viewport('phone')
    seed({ defaultView: 'agenda' })
    await renderCalendar()
    expect(await screen.findByTestId('agenda')).toBeInTheDocument()
  })

  it('a wide screen keeps opening on the week, whatever the setting', async () => {
    seed({ defaultView: 'month' })
    await renderCalendar()
    expect(await screen.findByTestId('time-grid')).toBeInTheDocument()
    expect(screen.queryByTestId('month-grid')).toBeNull()
  })
})

describe('Cài đặt Lịch → Thời lượng mặc định', () => {
  it('below 1024px a new event lasts the chosen duration', async () => {
    viewport('phone')
    const user = setup()
    seed({ defaultDuration: 45 })
    const router = await renderCalendar()
    await screen.findByTestId('agenda')
    await user.click(screen.getByRole('button', { name: 'Sự kiện mới' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar/new'))
    expect(router.state.location.search).toMatchObject({ date: '2026-10-14', start: '11:00', end: '11:45' })
  })

  it('a wide screen keeps the hour, whatever the setting', async () => {
    const user = setup()
    seed({ defaultDuration: 45 })
    const router = await renderCalendar()
    await block(/Họp giao ban/)
    await user.click(screen.getByRole('button', { name: 'Tạo' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar/new'))
    expect(router.state.location.search).toMatchObject({ start: '11:00', end: '12:00' })
  })
})
