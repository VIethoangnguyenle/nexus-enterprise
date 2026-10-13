import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { callsApi, type Call, type CallParticipant } from '../../api/calls'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { U, CH } from '../../test/chat-fixtures'
import { CallCard } from './CallCard'

vi.mock('../../api/calls', () => ({
  callsApi: { get: vi.fn(), start: vi.fn(), decline: vi.fn(), cancel: vi.fn(), token: vi.fn(), invite: vi.fn() },
  MAX_CALL_PEOPLE: 20,
}))

const navigateMock = vi.fn()
vi.mock('@tanstack/react-router', async (orig) => {
  const real = await orig<typeof import('@tanstack/react-router')>()
  return { ...real, useNavigate: () => navigateMock }
})

const CALL_ID = '99999999-aaaa-4bbb-8ccc-000000000001'

const participant = (key: keyof typeof U, name: string): CallParticipant => ({
  user_id: U[key].id,
  display_name: name,
  avatar_url: '',
  joined_at: { seconds: 1696833600, nanos: 0 },
})

const ts = (h: number, m: number) => ({ seconds: Math.floor(new Date(2026, 9, 13, h, m).getTime() / 1000), nanos: 0 })

function makeCall(overrides: Partial<Call> = {}): Call {
  return {
    id: CALL_ID,
    channel_id: CH.doisoat,
    kind: 'video',
    status: 'active',
    started_by: U.duc.id,
    started_by_name: 'Trần Minh Đức',
    participants: [participant('duc', 'Trần Minh Đức'), participant('vinh', 'Lê Quang Vinh'), participant('lan', 'Nguyễn Thu Lan')],
    taker_count: 3,
    created_at: ts(14, 2),
    started_at: ts(14, 2),
    ...overrides,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers({ shouldAdvanceTime: true })
  vi.setSystemTime(new Date(2026, 9, 13, 14, 14, 30))
})
afterEach(() => {
  vi.useRealTimers()
  resetClient()
})

describe('CallCard', () => {
  it('shows an active call with participant avatars, live clock, and "Tham gia" button', async () => {
    vi.mocked(callsApi.get).mockResolvedValue(makeCall())
    renderWithClient(<CallCard callId={CALL_ID} channelId={CH.doisoat} />)
    expect(await screen.findByText('Cuộc gọi đang diễn ra')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Tham gia/ })).toBeInTheDocument()
    // Participant names visible (not IDs)
    expect(screen.getByText(/Trần Minh Đức/)).toBeInTheDocument()
    expectNoIds()
  })

  it('shows "Mở lại cuộc gọi" when current user is among participants', async () => {
    vi.mocked(callsApi.get).mockResolvedValue(
      makeCall({
        participants: [
          participant('hoa', 'Lê Thị Hoa'),
          participant('duc', 'Trần Minh Đức'),
        ],
        taker_count: 2,
      }),
    )
    renderWithClient(<CallCard callId={CALL_ID} channelId={CH.doisoat} />)
    expect(await screen.findByRole('button', { name: /Mở lại cuộc gọi/ })).toBeInTheDocument()
    expectNoIds()
  })

  it('navigates to /calls/$callId on "Tham gia" click', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    vi.mocked(callsApi.get).mockResolvedValue(makeCall())
    renderWithClient(<CallCard callId={CALL_ID} channelId={CH.doisoat} />)
    await user.click(await screen.findByRole('button', { name: /Tham gia/ }))
    expect(navigateMock).toHaveBeenCalledWith({ to: '/calls/$callId', params: { callId: CALL_ID }, search: {} })
  })

  it('shows an ended call with participant count, duration and time range', async () => {
    vi.mocked(callsApi.get).mockResolvedValue(
      makeCall({
        status: 'ended',
        participants: [participant('duc', 'Trần Minh Đức'), participant('vinh', 'Lê Quang Vinh'), participant('lan', 'Nguyễn Thu Lan'), participant('hoa', 'Lê Thị Hoa')],
        taker_count: 4,
        started_at: ts(13, 10),
        ended_at: ts(13, 42),
      }),
    )
    renderWithClient(<CallCard callId={CALL_ID} channelId={CH.doisoat} />)
    expect(await screen.findByText(/Cuộc gọi/)).toBeInTheDocument()
    // Shows participant count and time
    await waitFor(() => {
      expect(screen.getByText(/4 người/)).toBeInTheDocument()
      expect(screen.getByText(/13:10/)).toBeInTheDocument()
    })
    // No join button for ended calls
    expect(screen.queryByRole('button', { name: /Tham gia/ })).toBeNull()
    expectNoIds()
  })

  it('shows a missed call with "Gọi lại" button', async () => {
    vi.mocked(callsApi.get).mockResolvedValue(
      makeCall({
        status: 'missed',
        started_by: U.yen.id,
        started_by_name: 'Phạm Hải Yến',
        participants: [],
        taker_count: 0,
      }),
    )
    renderWithClient(<CallCard callId={CALL_ID} channelId={CH.doisoat} />)
    expect(await screen.findByText(/gọi lúc/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Gọi lại/ })).toBeInTheDocument()
    expectNoIds()
  })

  it('starts a new call on "Gọi lại" and navigates', async () => {
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    const newCallId = '99999999-aaaa-4bbb-8ccc-000000000099'
    vi.mocked(callsApi.get).mockResolvedValue(
      makeCall({
        status: 'missed',
        started_by: U.yen.id,
        started_by_name: 'Phạm Hải Yến',
        participants: [],
        taker_count: 0,
      }),
    )
    vi.mocked(callsApi.start).mockResolvedValue({
      call: { ...makeCall({ id: newCallId }), status: 'ringing' },
      token: 'tok',
      url: 'wss://lk',
    })
    renderWithClient(<CallCard callId={CALL_ID} channelId={CH.doisoat} />)
    await user.click(await screen.findByRole('button', { name: /Gọi lại/ }))
    expect(callsApi.start).toHaveBeenCalledWith(CH.doisoat, 'video')
    await waitFor(() =>
      expect(navigateMock).toHaveBeenCalledWith(expect.objectContaining({ to: '/calls/$callId', params: { callId: newCallId } })),
    )
  })

  it('shows a declined call as danger wash', async () => {
    vi.mocked(callsApi.get).mockResolvedValue(
      makeCall({
        status: 'declined',
        started_by: U.yen.id,
        started_by_name: 'Phạm Hải Yến',
        participants: [],
        taker_count: 0,
      }),
    )
    renderWithClient(<CallCard callId={CALL_ID} channelId={CH.doisoat} />)
    expect(await screen.findByText(/gọi lúc/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Gọi lại/ })).toBeInTheDocument()
    expectNoIds()
  })

  it('shows a ringing call with "Đang gọi…" for the caller', async () => {
    vi.mocked(callsApi.get).mockResolvedValue(
      makeCall({
        status: 'ringing',
        started_by: U.hoa.id,
        started_by_name: 'Lê Thị Hoa',
        participants: [participant('hoa', 'Lê Thị Hoa')],
        taker_count: 1,
      }),
    )
    renderWithClient(<CallCard callId={CALL_ID} channelId={CH.doisoat} />)
    expect(await screen.findByText(/Đang gọi/)).toBeInTheDocument()
    expectNoIds()
  })

  it('shows skeleton while loading', () => {
    vi.mocked(callsApi.get).mockReturnValue(new Promise(() => {}))
    renderWithClient(<CallCard callId={CALL_ID} channelId={CH.doisoat} />)
    expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument()
  })

  it('renders "và N người" when more than 3 participants', async () => {
    vi.mocked(callsApi.get).mockResolvedValue(
      makeCall({
        participants: [
          participant('duc', 'Trần Minh Đức'),
          participant('vinh', 'Lê Quang Vinh'),
          participant('lan', 'Nguyễn Thu Lan'),
          participant('yen', 'Phạm Hải Yến'),
          participant('hoa', 'Lê Thị Hoa'),
        ],
        taker_count: 5,
      }),
    )
    renderWithClient(<CallCard callId={CALL_ID} channelId={CH.doisoat} />)
    expect(await screen.findByText(/và 2 người/)).toBeInTheDocument()
  })
})
