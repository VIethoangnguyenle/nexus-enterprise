import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { callsApi, type CallJoin } from '../../api/calls'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { expectCallRoute } from '../../test/call-nav'
import { takeTicket } from '../../stores/call.store'
import { CallHeaderActions } from './CallHeaderActions'

vi.mock('../../api/calls', () => ({
  callsApi: { get: vi.fn(), open: vi.fn(), start: vi.fn(), decline: vi.fn(), cancel: vi.fn(), token: vi.fn(), invite: vi.fn() },
  MAX_CALL_PEOPLE: 20,
}))

const navigateMock = vi.fn()
vi.mock('@tanstack/react-router', async (orig) => {
  const real = await orig<typeof import('@tanstack/react-router')>()
  return { ...real, useNavigate: () => navigateMock }
})

const CH_DM = '33333333-aaaa-4bbb-8ccc-000000000004'
const CH_GROUP = '33333333-aaaa-4bbb-8ccc-000000000001'
const CALL_ID = '99999999-aaaa-4bbb-8ccc-000000000050'

const joinResult: CallJoin = {
  call: { id: CALL_ID, channel_id: CH_GROUP, kind: 'video', status: 'active' },
  token: 'tok',
  url: 'wss://lk',
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(callsApi.open).mockResolvedValue(null)
  vi.useFakeTimers({ shouldAdvanceTime: true })
  vi.setSystemTime(new Date(2026, 9, 13, 14, 14, 30))
})
afterEach(() => {
  vi.useRealTimers()
  resetClient()
})

describe('CallHeaderActions', () => {
  describe('DM mode', () => {
    it('renders audio and video call buttons', () => {
      renderWithClient(
        <CallHeaderActions channelId={CH_DM} kind="dm" />,
      )
      expect(screen.getByRole('button', { name: 'Gọi thoại' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Gọi video' })).toBeInTheDocument()
    })

    it('hands the ticket from starting to the call screen, so no second one is minted', async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      vi.mocked(callsApi.start).mockResolvedValue(joinResult)
      renderWithClient(<CallHeaderActions channelId={CH_DM} kind="dm" />)
      await user.click(screen.getByRole('button', { name: 'Gọi video' }))
      await waitFor(() => expectCallRoute(navigateMock, CALL_ID, 'video'))
      expect(takeTicket(CALL_ID)).toMatchObject({ token: 'tok', url: 'wss://lk' })
    })

    it('starts an audio call and navigates with answer=audio', async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      vi.mocked(callsApi.start).mockResolvedValue({
        ...joinResult,
        call: { ...joinResult.call, channel_id: CH_DM, kind: 'audio' },
      })
      renderWithClient(
        <CallHeaderActions channelId={CH_DM} kind="dm" />,
      )
      await user.click(screen.getByRole('button', { name: 'Gọi thoại' }))
      expect(callsApi.start).toHaveBeenCalledWith(CH_DM, 'audio')
      await waitFor(() =>
        expectCallRoute(navigateMock, CALL_ID, 'audio'),
      )
    })

    it('starts a video call and navigates with answer=video', async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      vi.mocked(callsApi.start).mockResolvedValue(joinResult)
      renderWithClient(
        <CallHeaderActions channelId={CH_DM} kind="dm" />,
      )
      await user.click(screen.getByRole('button', { name: 'Gọi video' }))
      expect(callsApi.start).toHaveBeenCalledWith(CH_DM, 'video')
      await waitFor(() =>
        expectCallRoute(navigateMock, CALL_ID, 'video'),
      )
    })
  })

  describe('Group mode', () => {
    it('renders "Bắt đầu cuộc gọi" button', () => {
      renderWithClient(
        <CallHeaderActions channelId={CH_GROUP} kind="space" />,
      )
      expect(screen.getByRole('button', { name: /Bắt đầu cuộc gọi/ })).toBeInTheDocument()
    })

    it('starts a call and navigates without answer param for group', async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      vi.mocked(callsApi.start).mockResolvedValue(joinResult)
      renderWithClient(
        <CallHeaderActions channelId={CH_GROUP} kind="space" />,
      )
      await user.click(screen.getByRole('button', { name: /Bắt đầu cuộc gọi/ }))
      expect(callsApi.start).toHaveBeenCalledWith(CH_GROUP, 'video')
      await waitFor(() =>
        expectCallRoute(navigateMock, CALL_ID),
      )
    })

    it('shows "Tham gia" with the call clock when the group has a call open', async () => {
      vi.mocked(callsApi.open).mockResolvedValue({
        ...joinResult.call, started_at: { seconds: Math.floor(new Date(2026, 9, 13, 14, 2).getTime() / 1000) },
      })
      renderWithClient(<CallHeaderActions channelId={CH_GROUP} kind="space" />)
      expect(await screen.findByRole('button', { name: /Tham gia/ })).toBeInTheDocument()
      expect(screen.getByText('12:30')).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: /Bắt đầu cuộc gọi/ })).toBeNull()
    })

    it('navigates to the active call on "Tham gia" click', async () => {
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      vi.mocked(callsApi.open).mockResolvedValue(joinResult.call)
      renderWithClient(<CallHeaderActions channelId={CH_GROUP} kind="space" />)
      await user.click(await screen.findByRole('button', { name: /Tham gia/ }))
      expectCallRoute(navigateMock, CALL_ID)
    })
  })

  it('never exposes IDs', () => {
    renderWithClient(
      <CallHeaderActions channelId={CH_DM} kind="dm" />,
    )
    expectNoIds()
  })
})
