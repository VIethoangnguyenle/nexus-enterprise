import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, screen } from '@testing-library/react'
import { ApiError } from '../../api/client'
import { Room as FakeRoom } from '../../test/fake-livekit'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { useCallStore } from '../../stores/call.store'
import type { Call, CallJoin } from '../../api/calls'

const api = vi.hoisted(() => ({ get: vi.fn(), token: vi.fn() }))
const route = vi.hoisted(() => ({ callId: 'c1', search: {} as { answer?: 'video' | 'audio' } }))
const event = vi.hoisted(() => ({ q: { data: undefined, isSuccess: false, isError: false } as { data?: unknown; isSuccess: boolean; isError: boolean } }))

vi.mock('livekit-client', async () => (await import('../../test/fake-livekit')).livekitMock)
vi.mock('../../lib/call-sound', () => ({ playChime: vi.fn() }))
vi.mock('@tanstack/react-router', async () => {
  const { routerMockFactory } = await import('../../test/router-mock')
  return { ...routerMockFactory(), getRouteApi: () => ({ useParams: () => ({ callId: route.callId }), useSearch: () => route.search }) }
})
vi.mock('../../api/calls', async (orig) => ({ ...(await orig<object>()), callsApi: { get: api.get, token: api.token, invite: vi.fn() } }))
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'w1' }) }))
vi.mock('../../hooks/useConversations', () => ({ useConversations: () => ({ isLoading: false, conversations: [] }) }))
vi.mock('../../hooks/useCalendar', async (orig) => ({ ...(await orig<object>()), useEventDetail: () => event.q }))
vi.mock('../../hooks/usePeople', async () => {
  const { EMPTY_DIRECTORY } = await import('../../lib/people')
  return { usePeople: () => ({ ...EMPTY_DIRECTORY, isLoading: false }) }
})
const since = vi.hoisted(() => vi.fn(() => 0))
vi.mock('../../hooks/useCalls', async (orig) => ({ ...(await orig<object>()), useMessagesSince: since }))
vi.mock('./CallSidePanel', () => ({ CallSidePanel: () => <aside /> }))

import { CallProvider } from './CallProvider'
import { CallRoute } from './CallScreen'

/** The call of an event: no conversation, and the event it belongs to. */
const eventCall = (over: Partial<Call> = {}): Call => ({
  id: 'c1', channel_id: '', calendar_event_id: 'ev1', kind: 'video', status: 'active', started_by: 'other', started_by_name: 'Trần Minh Đức',
  participants: [{ user_id: 'a', display_name: 'Trần Minh Đức' }], taker_count: 1, ...over,
})
const ticket = (c: Call = eventCall()): CallJoin => ({ call: c, token: 'tok', url: 'ws://lk' })

const HOP = {
  title: 'Họp giao ban', start: '2026-10-15T12:00:00+07:00', end: '2026-10-15T13:00:00+07:00', allDay: false, eventId: 'ev1',
}

function mount() {
  return renderWithClient(<CallProvider><CallRoute /></CallProvider>)
}

describe('the call of a calendar event', () => {
  beforeEach(() => {
    FakeRoom.instances.length = 0
    api.get.mockReset().mockResolvedValue(eventCall())
    api.token.mockReset().mockImplementation(async () => ticket())
    event.q = { data: HOP, isSuccess: true, isError: false }
    route.search = {}
    route.callId = 'c1'
    routerState.navigate.mockReset()
    routerState.pathname = '/calls/c1'
    useCallStore.getState().clear()
    localStorage.clear()
  })
  afterEach(() => { cleanup(); resetClient() })

  async function joined() {
    mount()
    fireEvent.click(await screen.findByRole('button', { name: 'Tham gia ngay' }))
    await screen.findByTestId('call-stage')
  }

  it('offers the check although the call is in no conversation', async () => {
    mount()
    expect(await screen.findByText('Sẵn sàng tham gia?')).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/\bev1\b|\bc1\b/)
  })

  it('is named after the event, has no chat, and is for the event\'s guests', async () => {
    await joined()
    expect(screen.getByTestId('call-stage')).toHaveTextContent('Họp giao ban')
    expect(useCallStore.getState().session).toMatchObject({ title: 'Họp giao ban', channelId: '', eventId: 'ev1', isDm: false })
    expect(screen.queryByRole('button', { name: 'Chat' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Người tham gia' })).toBeInTheDocument()
    expect(screen.queryByText('Chỉ thành viên nhóm')).toBeNull()
    expect(screen.getByText('Chỉ khách của sự kiện')).toBeInTheDocument()
    expect(since).toHaveBeenCalledWith('', expect.anything(), expect.anything())
  })

  it('goes back to the event, not to a conversation, and counts no messages', async () => {
    await joined()
    fireEvent.click(screen.getByRole('button', { name: 'Rời cuộc gọi' }))
    expect(await screen.findByText('Cuộc gọi đã kết thúc')).toBeInTheDocument()
    expect(screen.queryByText('Tin trong lúc gọi')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Về cuộc trò chuyện' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Về sự kiện' }))
    expect(routerState.navigate).toHaveBeenCalledWith(expect.objectContaining({ to: '/calendar' }))
    const sent = vi.mocked(routerState.navigate).mock.calls
    const search = sent[sent.length - 1]![0].search as (p: { ws?: string }) => unknown
    expect(search({ ws: 'w1' })).toEqual({ ws: 'w1', date: '2026-10-15' })
  })

  it('says "Về sự kiện" when the call is over before it is opened', async () => {
    api.get.mockResolvedValue(eventCall({ status: 'ended' }))
    mount()
    expect(await screen.findByText('Cuộc gọi đã kết thúc')).toBeInTheDocument()
    expect(screen.getByText(/bắt đầu một cuộc gọi mới từ sự kiện/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Về sự kiện' })).toBeInTheDocument()
  })

  it('still lets the guest in when the event itself cannot be read, under a plain name', async () => {
    event.q = { data: undefined, isSuccess: false, isError: true }
    await joined()
    expect(screen.getByTestId('call-stage')).toHaveTextContent('Sự kiện')
  })

  it('refuses someone who is not a guest in words about the event', async () => {
    api.token.mockRejectedValue(new ApiError('forbidden', 403))
    mount()
    fireEvent.click(await screen.findByRole('button', { name: 'Tham gia ngay' }))
    expect(await screen.findByText('Bạn không còn là khách của sự kiện này')).toBeInTheDocument()
    expect(screen.queryByText('Bạn không ở trong cuộc trò chuyện này')).toBeNull()
  })

  it('leaves the live room when you leave, with the stage gone', async () => {
    await joined()
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Rời cuộc gọi' })) })
    expect(screen.queryByTestId('call-stage')).toBeNull()
  })
})
