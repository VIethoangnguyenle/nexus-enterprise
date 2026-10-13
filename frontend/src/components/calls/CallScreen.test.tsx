import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, screen, waitFor } from '@testing-library/react'
import { ApiError } from '../../api/client'
import { Room as FakeRoom, RoomEvent, DisconnectReason, createLocalVideoTrack } from '../../test/fake-livekit'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { useCallStore } from '../../stores/call.store'
import type { Call, CallJoin } from '../../api/calls'

const api = vi.hoisted(() => ({ get: vi.fn(), token: vi.fn() }))
const convo = vi.hoisted(() => ({ kind: 'space' as 'space' | 'dm' }))
const route = vi.hoisted(() => ({ search: {} as { answer?: 'video' | 'audio' } }))

vi.mock('livekit-client', async () => (await import('../../test/fake-livekit')).livekitMock)
vi.mock('../../lib/call-sound', () => ({ playChime: vi.fn() }))
vi.mock('@tanstack/react-router', async () => {
  const { routerMockFactory } = await import('../../test/router-mock')
  return { ...routerMockFactory(), getRouteApi: () => ({ useParams: () => ({ callId: 'c1' }), useSearch: () => route.search }) }
})
vi.mock('../../api/calls', async (orig) => ({ ...(await orig<object>()), callsApi: { get: api.get, token: api.token, invite: vi.fn() } }))
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'w1' }) }))
vi.mock('../../hooks/useConversations', () => ({
  useConversations: () => ({
    isLoading: false,
    conversations: [{ id: 'ch1', kind: convo.kind, title: convo.kind === 'dm' ? 'Phạm Hải Yến' : 'Đối soát giao dịch' }],
  }),
}))
vi.mock('../../hooks/usePeople', async () => {
  const { EMPTY_DIRECTORY } = await import('../../lib/people')
  return { usePeople: () => ({ ...EMPTY_DIRECTORY, isLoading: false }) }
})
vi.mock('../../hooks/useCalls', async (orig) => ({ ...(await orig<object>()), useMessagesSince: () => 12 }))
vi.mock('./CallSidePanel', () => ({ CallSidePanel: () => <aside /> }))

import { CallProvider } from './CallProvider'
import { CallRoute } from './CallScreen'

const call = (over: Partial<Call> = {}): Call => ({
  id: 'c1', channel_id: 'ch1', kind: 'video', status: 'active', started_by: 'other', started_by_name: 'Trần Minh Đức',
  participants: [
    { user_id: 'a', display_name: 'Trần Minh Đức' }, { user_id: 'b', display_name: 'Lê Quang Vinh' }, { user_id: 'c', display_name: 'Nguyễn Thu Lan' },
  ],
  taker_count: 3, ...over,
})
const ticket = (c: Call = call()): CallJoin => ({ call: c, token: 'tok', url: 'ws://lk' })
const latestRoom = () => FakeRoom.instances[FakeRoom.instances.length - 1]!

function mount() {
  return renderWithClient(<CallProvider><CallRoute /></CallProvider>)
}

describe('call screen', () => {
  beforeEach(() => {
    FakeRoom.instances.length = 0
    api.get.mockReset().mockResolvedValue(call())
    api.token.mockReset().mockImplementation(async () => ticket())
    convo.kind = 'space'
    route.search = {}
    routerState.navigate.mockReset()
    routerState.pathname = '/calls/c1'
    useCallStore.getState().clear()
    localStorage.clear()
  })
  afterEach(() => { cleanup(); resetClient() })

  describe('check before joining a group call', () => {
    it('names who is in the call and joins with the ticket', async () => {
      mount()
      expect(await screen.findByText('Sẵn sàng tham gia?')).toBeInTheDocument()
      expect(screen.getByText('Trần Minh Đức, Lê Quang Vinh và 1 người đang trong cuộc gọi')).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Tham gia ngay' }))
      expect(await screen.findByTestId('call-stage')).toBeInTheDocument()
      expect(api.token).toHaveBeenCalledWith('c1')
      expect(latestRoom().connect).toHaveBeenCalledWith('ws://lk', 'tok')
      expect(useCallStore.getState().phase).toBe('connected')
    })

    it('remembers the choices per user on this device', async () => {
      mount()
      fireEvent.click(await screen.findByRole('button', { name: 'Tắt camera' }))
      fireEvent.click(screen.getByRole('button', { name: 'Tham gia ngay' }))
      await screen.findByTestId('call-stage')
      expect(JSON.parse(localStorage.getItem('ngac-call-prefs:11111111-aaaa-4bbb-8ccc-000000000001')!)).toMatchObject({ camOn: false })
    })

    it('releases the preview camera before the room takes it', async () => {
      mount()
      await screen.findByText('Sẵn sàng tham gia?')
      await waitFor(() => expect(createLocalVideoTrack).toHaveBeenCalled())
      const track = await vi.mocked(createLocalVideoTrack).mock.results[0]!.value
      fireEvent.click(screen.getByRole('button', { name: 'Tham gia ngay' }))
      await screen.findByTestId('call-stage')
      expect(track.stop).toHaveBeenCalled()
    })

    it('shows the browser-blocked banner when the camera is refused, and still lets you in', async () => {
      vi.mocked(createLocalVideoTrack).mockRejectedValueOnce(new DOMException('no', 'NotAllowedError'))
      mount()
      expect(await screen.findByText('Trình duyệt đang chặn camera và micro')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Tham gia ngay' })).toBeEnabled()
    })

    it('a full call: "Cuộc gọi đã đầy (20/20)" disabled with a line why', async () => {
      api.get.mockResolvedValue(call({ taker_count: 20 }))
      mount()
      const full = await screen.findByRole('button', { name: 'Cuộc gọi đã đầy (20/20)' })
      expect(full).toBeDisabled()
      expect(screen.getByText(/đủ 20 người/)).toBeInTheDocument()
    })

    it('a call that fills while you read: the ticket is refused with call_full', async () => {
      api.token.mockRejectedValue(new ApiError('conflict', 409, { code: 'call_full' }))
      mount()
      fireEvent.click(await screen.findByRole('button', { name: 'Tham gia ngay' }))
      expect(await screen.findByRole('button', { name: 'Cuộc gọi đã đầy (20/20)' })).toBeDisabled()
      expect(FakeRoom.instances).toHaveLength(0)
    })

    it('a call that ended while you read: call_ended', async () => {
      api.token.mockRejectedValue(new ApiError('conflict', 409, { code: 'call_ended' }))
      mount()
      fireEvent.click(await screen.findByRole('button', { name: 'Tham gia ngay' }))
      expect(await screen.findByText('Cuộc gọi đã kết thúc')).toBeInTheDocument()
    })

    it('a call that is already over shows no check', async () => {
      api.get.mockResolvedValue(call({ status: 'ended' }))
      mount()
      expect(await screen.findByText('Cuộc gọi đã kết thúc')).toBeInTheDocument()
      expect(screen.queryByText('Sẵn sàng tham gia?')).toBeNull()
    })

    it('calls unavailable (503) says so and keeps the check', async () => {
      api.token.mockRejectedValue(new ApiError('down', 503, { code: 'calls_unavailable' }))
      mount()
      fireEvent.click(await screen.findByRole('button', { name: 'Tham gia ngay' }))
      expect(await screen.findByText('Cuộc gọi chưa dùng được lúc này')).toBeInTheDocument()
    })

    it('prints no id anywhere', async () => {
      mount()
      await screen.findByText('Sẵn sàng tham gia?')
      expect(document.body.textContent).not.toMatch(/11111111-aaaa|\bch1\b|\bc1\b/)
    })
  })

  describe('joining straight in', () => {
    it('?answer=video skips the check', async () => {
      route.search = { answer: 'video' }
      mount()
      expect(await screen.findByTestId('call-stage')).toBeInTheDocument()
      expect(screen.queryByText('Sẵn sàng tham gia?')).toBeNull()
      expect(latestRoom().localParticipant.setCameraEnabled).toHaveBeenCalledWith(true)
    })

    it('?answer=audio joins without the camera', async () => {
      route.search = { answer: 'audio' }
      mount()
      await screen.findByTestId('call-stage')
      expect(latestRoom().localParticipant.setCameraEnabled).not.toHaveBeenCalled()
      expect(latestRoom().localParticipant.setMicrophoneEnabled).toHaveBeenCalledWith(true)
    })

    it('a direct conversation joins at once too', async () => {
      convo.kind = 'dm'
      mount()
      expect(await screen.findByTestId('call-stage')).toBeInTheDocument()
      expect(screen.queryByText('Chỉ thành viên nhóm')).toBeNull()
    })
  })

  describe('in the call', () => {
    async function joined() {
      route.search = { answer: 'video' }
      mount()
      await screen.findByTestId('call-stage')
      return latestRoom()
    }

    it('removed from the group: cut off at once with the danger banner', async () => {
      const room = await joined()
      act(() => { room.emit(RoomEvent.Disconnected, DisconnectReason.PARTICIPANT_REMOVED) })
      expect(await screen.findByText('Bạn không còn ở trong nhóm này')).toBeInTheDocument()
      expect(screen.getByText(/Người quản trị nhóm có thể mời lại bạn/)).toBeInTheDocument()
      expect(screen.queryByTestId('call-stage')).toBeNull()
      expect(useCallStore.getState().phase).toBe('removed')
    })

    it('leaving shows the summary with duration, people and messages, and "Vào lại"', async () => {
      const room = await joined()
      act(() => { room.join('u1', 'Người 1') })
      fireEvent.click(screen.getByRole('button', { name: 'Rời cuộc gọi' }))
      expect(await screen.findByText('Cuộc gọi đã kết thúc')).toBeInTheDocument()
      expect(screen.getByText('Thời lượng')).toBeInTheDocument()
      expect(screen.getByText('Người tham gia').previousSibling).toHaveTextContent('2')
      expect(screen.getByText('Tin trong lúc gọi').previousSibling).toHaveTextContent('12')
      expect(room.disconnect).toHaveBeenCalled()
      fireEvent.click(screen.getByRole('button', { name: 'Về cuộc trò chuyện' }))
      expect(routerState.navigate).toHaveBeenCalledWith(expect.objectContaining({ to: '/channels/$channelId', params: { channelId: 'ch1' } }))
    })

    it('the call ended for everyone: summary too', async () => {
      const room = await joined()
      act(() => { room.emit(RoomEvent.Disconnected, DisconnectReason.ROOM_DELETED) })
      expect(await screen.findByText('Cuộc gọi đã kết thúc')).toBeInTheDocument()
    })

    it('the link breaks for good: "Không kết nối được" and Thử lại asks for a new ticket', async () => {
      const room = await joined()
      act(() => { room.emit(RoomEvent.Disconnected, DisconnectReason.CONNECTION_TIMEOUT) })
      expect(await screen.findByText('Không kết nối được tới máy chủ cuộc gọi')).toBeInTheDocument()
      expect(screen.getByText(/mạng công ty/)).toBeInTheDocument()
      api.token.mockClear()
      fireEvent.click(screen.getByRole('button', { name: 'Thử lại' }))
      await waitFor(() => expect(api.token).toHaveBeenCalledTimes(1))
      expect(await screen.findByTestId('call-stage')).toBeInTheDocument()
      expect(FakeRoom.instances).toHaveLength(2)
    })

    it('cannot reach the server at all: the same failure screen', async () => {
      route.search = { answer: 'video' }
      api.token.mockRejectedValueOnce(new TypeError('Failed to fetch'))
      mount()
      expect(await screen.findByText('Không kết nối được tới máy chủ cuộc gọi')).toBeInTheDocument()
    })

    it('shows the reconnecting overlay and takes it away when back', async () => {
      const room = await joined()
      act(() => { room.emit(RoomEvent.Reconnecting) })
      expect(screen.getByText('Đang kết nối lại…')).toBeInTheDocument()
      expect(useCallStore.getState().phase).toBe('reconnecting')
      act(() => { room.emit(RoomEvent.Reconnected) })
      expect(screen.queryByText('Đang kết nối lại…')).toBeNull()
    })
  })

  it('asks "Rời và vào" when another call is running', async () => {
    useCallStore.getState().begin({ callId: 'other', channelId: 'ch9', kind: 'video', title: 'Kế hoạch quý 4', isDm: false, joinedAt: 1, startedAt: 1 })
    useCallStore.getState().setPhase('connected')
    mount()
    expect(await screen.findByText('Bạn đang ở trong một cuộc gọi khác')).toBeInTheDocument()
    expect(screen.getByText('Rời "Kế hoạch quý 4" để vào cuộc gọi này?')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Ở lại' }))
    expect(routerState.navigate).toHaveBeenCalledWith(expect.objectContaining({ to: '/calls/$callId', params: { callId: 'other' } }))
  })

  it('"Rời và vào" joins this call', async () => {
    useCallStore.getState().begin({ callId: 'other', channelId: 'ch9', kind: 'video', title: 'Kế hoạch quý 4', isDm: false, joinedAt: 1, startedAt: 1 })
    useCallStore.getState().setPhase('connected')
    mount()
    fireEvent.click(await screen.findByRole('button', { name: 'Rời và vào' }))
    expect(await screen.findByTestId('call-stage')).toBeInTheDocument()
    expect(useCallStore.getState().session?.callId).toBe('c1')
  })
})
