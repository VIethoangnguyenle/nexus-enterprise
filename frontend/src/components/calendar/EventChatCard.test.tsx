import { cleanup, configure, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { CAL, E, calendarFixtureApi, calls, enableGroupCalendar, mode, resetFixtures } from '../../test/calendar-fixtures'
import { CH } from '../../test/chat-fixtures'
import { validateCalendarSearch } from '../../lib/calendar-search'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { EventChatCard } from './EventChatCard'

vi.mock('../../api/client', async (orig) => {
  const { calendarFixtureApi } = await import('../../test/calendar-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(calendarFixtureApi) }
})

configure({ asyncUtilTimeout: 4000 })
vi.setConfig({ testTimeout: 15_000 })

async function renderCard(eventId = E.teamSync, text = '📅 Gặp đối tác VNPay') {
  const root = createRootRoute()
  const chat = createRoute({
    getParentRoute: () => root, path: '/',
    component: () => <EventChatCard eventId={eventId} channelId={CH.doisoat} text={text} />,
  })
  const grid = createRoute({ getParentRoute: () => root, path: '/calendar', validateSearch: validateCalendarSearch, component: () => <p>Lưới lịch</p> })
  const call = createRoute({ getParentRoute: () => root, path: '/calls/$callId', component: () => <p>Cuộc gọi</p> })
  const router = createRouter({ routeTree: root.addChildren([chat, grid, call]), history: createMemoryHistory({ initialEntries: ['/'] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

const setup = () => userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
const writes = (method: string) => calls.filter((c) => c.method === method)

beforeEach(() => {
  localStorage.clear()
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(2026, 9, 14, 10, 40))
  resetFixtures()
  enableGroupCalendar()
  vi.mocked(apiFetch).mockImplementation(calendarFixtureApi)
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
})

describe('the event card in the conversation', () => {
  it('shows the event as the calendar has it: when, where, who and how many have answered', async () => {
    await renderCard()
    const card = await screen.findByRole('group', { name: 'Sự kiện: Gặp đối tác VNPay' })
    expect(within(card).getByText('Gặp đối tác VNPay')).toBeInTheDocument()
    expect(within(card).getByText('T5')).toBeInTheDocument()
    expect(within(card).getByText('15')).toBeInTheDocument()
    expect(within(card).getByText(/Thứ Năm, 15 tháng 10 · 13:30 – 14:30/)).toBeInTheDocument()
    expect(within(card).getByText(/Phòng khách tầng 1/)).toBeInTheDocument()
    expect(within(card).getByText('1 có · 1 chưa trả lời')).toBeInTheDocument()
    expect(within(card).getByRole('button', { name: 'Mở trong Lịch' })).toBeInTheDocument()
    expectNoIds()
  })

  it('answers from the card and shows the answer', async () => {
    const user = setup()
    await renderCard()
    const seg = await screen.findByRole('radiogroup', { name: 'Trả lời' })
    expect(within(seg).getAllByRole('radio').map((r) => r.textContent)).toEqual(['Có', 'Không', 'Có thể'])
    await user.click(within(seg).getByRole('radio', { name: 'Không' }))
    await waitFor(() => expect(writes('POST').some((c) => c.path.endsWith(`/events/${E.teamSync}/rsvp`))).toBe(true))
    expect(writes('POST').find((c) => c.path.endsWith('/rsvp'))!.body).toMatchObject({ response: 'declined', scope: 'all' })
    await waitFor(() => expect(within(screen.getByRole('radiogroup', { name: 'Trả lời' })).getByRole('radio', { name: 'Không' })).toBeChecked())
  })

  it('says so and keeps the card as it was when the answer is refused', async () => {
    const user = setup()
    await renderCard()
    const seg = await screen.findByRole('radiogroup', { name: 'Trả lời' })
    mode.writeStatus = 403
    await user.click(within(seg).getByRole('radio', { name: 'Có' }))
    await waitFor(() => expect(useToastStore.getState().toasts.length).toBeGreaterThan(0))
    expect(within(screen.getByRole('radiogroup', { name: 'Trả lời' })).getByRole('radio', { name: 'Có' })).not.toBeChecked()
  })

  it('offers no answer to someone who is not a guest', async () => {
    await renderCard(E.teamOpen, '📅 Họp nhóm mở')
    const card = await screen.findByRole('group', { name: 'Sự kiện: Họp nhóm mở' })
    expect(within(card).queryByRole('radiogroup')).toBeNull()
    expect(within(card).getByText('Chưa mời ai')).toBeInTheDocument()
  })

  it('shows the answer a guest already gave', async () => {
    await renderCard(E.teamLater, '📅 Chốt phí giao dịch')
    const seg = await screen.findByRole('radiogroup', { name: 'Trả lời' })
    expect(within(seg).getByRole('radio', { name: 'Có' })).toBeChecked()
  })

  it('is dimmed with the label "Đã huỷ" once the event is gone, still naming it', async () => {
    await renderCard('e0000000-aaaa-4bbb-8ccc-0000000000ee', '📅 Họp đã bỏ')
    const card = await screen.findByRole('group', { name: 'Sự kiện đã huỷ: Họp đã bỏ' })
    expect(within(card).getByText('Đã huỷ')).toBeInTheDocument()
    expect(within(card).getByText('Họp đã bỏ')).toBeInTheDocument()
    expect(within(card).queryByRole('radiogroup')).toBeNull()
    expect(within(card).queryByRole('button', { name: 'Mở trong Lịch' })).toBeNull()
  })

  it('shows nothing of an event the viewer may no longer read', async () => {
    mode.groupForbidden = true
    await renderCard()
    expect(await screen.findByText('Không xem được sự kiện này')).toBeInTheDocument()
    expect(screen.queryByText('Gặp đối tác VNPay')).toBeNull()
    expect(screen.queryByRole('radiogroup')).toBeNull()
  })

  it('opens the calendar on the day of the event', async () => {
    const user = setup()
    const router = await renderCard()
    await user.click(await screen.findByRole('button', { name: 'Mở trong Lịch' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calendar'))
    expect(router.state.location.search).toMatchObject({ date: '2026-10-15' })
  })

  it('joins the call of the group while the event runs', async () => {
    vi.setSystemTime(new Date(2026, 9, 15, 13, 40))
    const user = setup()
    const router = await renderCard()
    const card = await screen.findByRole('group', { name: 'Sự kiện: Gặp đối tác VNPay' })
    expect(within(card).getByText('Đang diễn ra')).toBeInTheDocument()
    await user.click(within(card).getByRole('button', { name: 'Tham gia' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/calls/ca110000-aaaa-4bbb-8ccc-000000000002'))
    expect(writes('POST').some((c) => c.path === `/channels/${CH.doisoat}/calls`)).toBe(true)
  })

  it('offers no join before the call opens', async () => {
    await renderCard()
    const card = await screen.findByRole('group', { name: 'Sự kiện: Gặp đối tác VNPay' })
    expect(within(card).queryByRole('button', { name: 'Tham gia' })).toBeNull()
  })

  it('does not show an event of another group as this conversation\'s', async () => {
    // The event sits on the calendar of the "Đối soát" group, but this card is in another conversation.
    const root = createRootRoute()
    const chat = createRoute({
      getParentRoute: () => root, path: '/',
      component: () => <EventChatCard eventId={E.teamSync} channelId={CH.vanhanh} text="📅 Gặp đối tác VNPay" />,
    })
    const router = createRouter({ routeTree: root.addChildren([chat]), history: createMemoryHistory({ initialEntries: ['/'] }) })
    await router.load()
    renderWithClient(<RouterProvider router={router} />)
    expect(await screen.findByText('Không xem được sự kiện này')).toBeInTheDocument()
    expect(CAL.group).toBeTruthy()
  })
})
