import { cleanup, configure, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { CAL, calendarFixtureApi, calls, enableGroupCalendar, mode, resetFixtures } from '../../test/calendar-fixtures'
import { CH } from '../../test/chat-fixtures'
import { validateCalendarSearch, validateNewEventSearch } from '../../lib/calendar-search'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { SpaceCalendar } from './SpaceCalendar'

vi.mock('../../api/client', async (orig) => {
  const { calendarFixtureApi } = await import('../../test/calendar-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(calendarFixtureApi) }
})

configure({ asyncUtilTimeout: 4000 })
vi.setConfig({ testTimeout: 15_000 })

async function renderTab(channelId = CH.doisoat) {
  const root = createRootRoute()
  const tab = createRoute({ getParentRoute: () => root, path: '/', component: () => <SpaceCalendar channelId={channelId} /> })
  const grid = createRoute({ getParentRoute: () => root, path: '/calendar', validateSearch: validateCalendarSearch, component: () => <p>Lưới lịch</p> })
  const added = createRoute({ getParentRoute: () => root, path: '/calendar/new', validateSearch: validateNewEventSearch, component: () => <p>Trình soạn</p> })
  const router = createRouter({ routeTree: root.addChildren([tab, grid, added]), history: createMemoryHistory({ initialEntries: ['/'] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

const setup = () => userEvent.setup({ advanceTimers: vi.advanceTimersByTime })

beforeEach(() => {
  localStorage.clear()
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(2026, 9, 14, 10, 40))
  resetFixtures()
  vi.mocked(apiFetch).mockImplementation(calendarFixtureApi)
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
})

describe('the Lịch tab of a group', () => {
  it('lists what is coming up on the calendar of this group and nothing else', async () => {
    enableGroupCalendar()
    await renderTab()
    const agenda = await screen.findByTestId('agenda')
    expect(await within(agenda).findByRole('button', { name: /^Gặp đối tác VNPay/ })).toBeInTheDocument()
    expect(within(agenda).getByRole('button', { name: /^Chốt phí giao dịch/ })).toBeInTheDocument()
    expect(within(agenda).queryByText('Họp giao ban')).toBeNull()
    const asked = calls.find((c) => c.method === 'GET' && c.path.endsWith('/calendar/events'))!
    expect(asked.query.get('calendars')).toBe(CAL.group)
    expectNoIds()
  })

  it('has a small month with the days that have events marked, and lists the day that is chosen', async () => {
    enableGroupCalendar()
    const user = setup()
    await renderTab()
    await screen.findByTestId('agenda')
    await user.click(screen.getByRole('radio', { name: 'Tháng' }))
    const month = await screen.findByRole('grid', { name: 'Tháng 10 năm 2026' })
    const day = await within(month).findByRole('button', { name: '15 tháng 10, 1 sự kiện' })
    expect(within(month).getByRole('button', { name: '20 tháng 10, 1 sự kiện' })).toBeInTheDocument()
    expect(within(month).getByRole('button', { name: '16 tháng 10' })).toBeInTheDocument()
    await user.click(day)
    const list = screen.getByRole('list', { name: 'Sự kiện ngày 15 tháng 10' })
    expect(within(list).getByText('Gặp đối tác VNPay')).toBeInTheDocument()
    expect(within(list).queryByText('Chốt phí giao dịch')).toBeNull()
  })

  it('opens the editor on the group calendar without making it again when it exists', async () => {
    enableGroupCalendar()
    const user = setup()
    const router = await renderTab()
    await user.click(await screen.findByRole('button', { name: 'Tạo sự kiện' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar/new'))
    expect(router.state.location.search).toMatchObject({ calendar: CAL.group })
    expect(calls.some((c) => c.method === 'POST')).toBe(false)
  })

  it('makes the calendar of the group the first time an event is created', async () => {
    const user = setup()
    const router = await renderTab()
    expect(await screen.findByText('Nhóm chưa có sự kiện nào.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Tạo sự kiện' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar/new'))
    expect(router.state.location.search).toMatchObject({ calendar: CAL.group })
    expect(calls.filter((c) => c.method === 'POST').map((c) => c.path)).toEqual([expect.stringMatching(new RegExp(`/channels/${CH.doisoat}/calendar$`))])
  })

  it('says why and stays here when the group calendar cannot be made', async () => {
    mode.groupForbidden = true
    const user = setup()
    const router = await renderTab()
    await user.click(await screen.findByRole('button', { name: 'Tạo sự kiện' }))
    await waitFor(() => expect(useToastStore.getState().toasts.length).toBeGreaterThan(0))
    expect(router.state.location.pathname).toBe('/')
  })

  it('opens the calendar app', async () => {
    enableGroupCalendar()
    const user = setup()
    const router = await renderTab()
    await user.click(await screen.findByRole('button', { name: 'Mở trong Lịch' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar'))
  })

  it('shows none of the calendar to someone who is not in the group', async () => {
    enableGroupCalendar()
    mode.groupForbidden = true
    await renderTab()
    expect(await screen.findByText('Bạn không xem được lịch của nhóm này.')).toBeInTheDocument()
    expect(screen.queryByText('Gặp đối tác VNPay')).toBeNull()
    expect(screen.queryByTestId('agenda')).toBeNull()
  })

  it('says so when nothing is coming up', async () => {
    enableGroupCalendar(false)
    await renderTab()
    expect(await screen.findByText('Chưa có sự kiện nào sắp tới.')).toBeInTheDocument()
  })

  it('offers a retry when the calendar cannot be loaded', async () => {
    mode.calendarsError = true
    const user = setup()
    await renderTab()
    expect(await screen.findByText('Chưa tải được lịch của nhóm.')).toBeInTheDocument()
    mode.calendarsError = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByText('Nhóm chưa có sự kiện nào.')).toBeInTheDocument()
  })
})
