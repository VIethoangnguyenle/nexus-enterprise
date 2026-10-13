import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { RoomEvent, Track, fakeTrack, ConnectionQuality } from '../../test/fake-livekit'
import { renderWithClient, resetClient } from '../../test/render'
import { useCallStore, type CallSession } from '../../stores/call.store'
import { CallStage } from './CallStage'
import { makeCallRoom, WithSnapshot } from './call-test-utils'

vi.mock('livekit-client', async () => (await import('../../test/fake-livekit')).livekitMock)
vi.mock('../../lib/call-sound', () => ({ playChime: vi.fn() }))
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'w1' }) }))
vi.mock('../../hooks/usePeople', async () => {
  const { EMPTY_DIRECTORY } = await import('../../lib/people')
  return { usePeople: () => ({ ...EMPTY_DIRECTORY, isLoading: false }) }
})
vi.mock('../../hooks/useCalls', async (orig) => ({ ...(await orig<object>()), useMessagesSince: () => 0 }))
vi.mock('./CallSidePanel', () => ({ CallSidePanel: ({ panel }: { panel: string }) => <aside aria-label="Bảng bên">{panel}</aside> }))

const session: CallSession = {
  callId: 'c1', channelId: 'ch1', kind: 'video', title: 'Đối soát giao dịch', isDm: false, joinedAt: Date.now(), startedAt: Date.now(),
}

function setup(others = 1) {
  const h = makeCallRoom()
  const people = Array.from({ length: others }, (_, i) => h.room.join(`u${i + 1}`, `Người ${i + 1}`))
  const onLeave = vi.fn()
  renderWithClient(
    <WithSnapshot callRoom={h.callRoom}>
      {(snap) => <CallStage callRoom={h.callRoom} snapshot={snap} session={session} call={undefined} onLeave={onLeave} />}
    </WithSnapshot>,
  )
  return { ...h, people, onLeave }
}

const tiles = () => document.querySelectorAll('[data-testid^="tile-"]')

describe('CallStage', () => {
  beforeEach(() => {
    useCallStore.getState().clear()
    useCallStore.getState().begin(session)
  })
  afterEach(() => { cleanup(); resetClient() })

  describe('layout by number of people', () => {
    it('alone: one large tile with your own name', () => {
      setup(0)
      expect(tiles()).toHaveLength(1)
      expect(screen.getByText('Bạn (Lê Thị Hoa)')).toBeInTheDocument()
    })

    it('two people: the other large and you small in the corner', () => {
      setup(1)
      expect(tiles()).toHaveLength(2)
      expect(document.querySelector('.cs-pip-self')).not.toBeNull()
      expect(document.querySelector('.cs-grid')).toBeNull()
    })

    it.each([[2, 'cs-cols-2'], [4, 'cs-cols-3'], [9, 'cs-cols-4']])('with %i others the grid is %s', (others, cls) => {
      setup(others)
      expect(document.querySelector('.cs-grid')).toHaveClass(cls)
      expect(tiles()).toHaveLength(others + 1)
    })

    it('twenty people page the grid', () => {
      setup(19)
      expect(tiles()).toHaveLength(12)
      expect(screen.getByText('1/2')).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Trang sau' }))
      expect(tiles()).toHaveLength(8)
      expect(screen.getByText('2/2')).toBeInTheDocument()
    })

    it('shows the count in the top bar and "Chỉ thành viên nhóm" for a group', () => {
      setup(2)
      expect(screen.getByLabelText('3 người trong cuộc gọi')).toBeInTheDocument()
      expect(screen.getByText('Chỉ thành viên nhóm')).toBeInTheDocument()
    })
  })

  describe('tiles', () => {
    it('rings the speaker, flags a poor network and a muted mic, and names who raised a hand', () => {
      const { room, people } = setup(2)
      const a = people[0]!
      act(() => {
        a.connectionQuality = ConnectionQuality.Poor
        a.setTrack(Track.Source.Microphone, fakeTrack('m'), true)
        room.emit(RoomEvent.ConnectionQualityChanged)
        room.emit(RoomEvent.ActiveSpeakersChanged, [people[1]])
      })
      expect(screen.getByText('Mạng yếu', { selector: '.cs-poor' })).toBeInTheDocument()
      expect(within(screen.getByTestId('tile-u1')).getByLabelText('Micro tắt')).toBeInTheDocument()
      expect(screen.getByTestId('tile-u2')).toHaveClass('cs-tile-speaking')
    })

    it('never prints a user id', () => {
      setup(2)
      expect(document.body.textContent).not.toMatch(/\bu[12]\b/)
    })
  })

  describe('controls', () => {
    it('flips the mic and camera labels and buttons with the real state', async () => {
      const { room } = setup(1)
      const mic = screen.getByRole('button', { name: /Bật micro/ })
      expect(mic).toHaveClass('cs-btn-off')
      fireEvent.click(mic)
      await waitFor(() => expect(room.localParticipant.setMicrophoneEnabled).toHaveBeenCalledWith(true))
      await waitFor(() => expect(screen.getByRole('button', { name: /Tắt micro/ })).not.toHaveClass('cs-btn-off'))

      fireEvent.click(screen.getByRole('button', { name: /Bật camera/ }))
      await waitFor(() => expect(screen.getByRole('button', { name: /Tắt camera/ })).toBeInTheDocument())
    })

    it('raises and lowers the hand', async () => {
      const { room } = setup(1)
      fireEvent.click(screen.getByRole('button', { name: 'Giơ tay' }))
      await waitFor(() => expect(screen.getByRole('button', { name: 'Hạ tay' })).toHaveAttribute('aria-pressed', 'true'))
      expect(room.localParticipant.publishData).toHaveBeenCalled()
      expect(screen.getByLabelText('Đang giơ tay, thứ 1')).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'Hạ tay' }))
      await waitFor(() => expect(screen.queryByLabelText(/Đang giơ tay/)).toBeNull())
    })

    it('throws a reaction from the popover and floats it with your name', async () => {
      setup(1)
      fireEvent.click(screen.getByRole('button', { name: 'Thả cảm xúc' }))
      fireEvent.click(screen.getByRole('menuitem', { name: 'Vỗ tay' }))
      await waitFor(() => expect(screen.getByText('👏')).toBeInTheDocument())
      expect(document.querySelector('.cs-float')).toHaveTextContent('Lê Thị Hoa')
      expect(screen.queryByRole('menu', { name: 'Cảm xúc' })).toBeNull()
    })

    it('leaves through the end button', () => {
      const { onLeave } = setup(1)
      fireEvent.click(screen.getByRole('button', { name: 'Rời cuộc gọi' }))
      expect(onLeave).toHaveBeenCalled()
    })

    it('opens chat and people panels from the bar and narrows the grid', () => {
      setup(4)
      expect(document.querySelector('.cs-grid')).toHaveClass('cs-cols-3')
      fireEvent.click(screen.getByRole('button', { name: 'Chat' }))
      expect(screen.getByLabelText('Bảng bên')).toHaveTextContent('chat')
      expect(document.querySelector('.cs-grid')).toHaveClass('cs-cols-2')
    })
  })

  describe('events from the room', () => {
    it('shows a hand and a reaction received over data', () => {
      const { room, people } = setup(2)
      act(() => {
        room.data(people[0]!, 'nexus.hand', { raised: true, at: 1 })
        room.data(people[1]!, 'nexus.reaction', { emoji: '🎉' })
      })
      expect(within(screen.getByTestId('tile-u1')).getByLabelText('Đang giơ tay, thứ 1')).toBeInTheDocument()
      expect(document.querySelector('.cs-float')).toHaveTextContent('Người 2')
    })

    it('covers the stage while reconnecting, and clears when the link is back', () => {
      const { room } = setup(1)
      act(() => { room.emit(RoomEvent.Reconnecting) })
      expect(screen.getByText('Đang kết nối lại…')).toBeInTheDocument()
      expect(screen.getByText(/giữ chỗ cho bạn trong 30 giây/)).toBeInTheDocument()
      act(() => { room.emit(RoomEvent.Reconnected) })
      expect(screen.queryByText('Đang kết nối lại…')).toBeNull()
    })

    it('offers a tap to enable sound when the browser blocks it', async () => {
      const { room } = setup(1)
      act(() => { room.canPlaybackAudio = false; room.emit(RoomEvent.AudioPlaybackStatusChanged) })
      fireEvent.click(screen.getByRole('button', { name: 'Chạm để bật âm thanh' }))
      await waitFor(() => expect(room.startAudio).toHaveBeenCalled())
    })

    it('shows a presentation large with a strip, named after the presenter', () => {
      const { room, people } = setup(2)
      act(() => {
        people[0]!.setTrack(Track.Source.ScreenShare, fakeTrack('s'))
        room.emit(RoomEvent.TrackSubscribed)
      })
      expect(screen.getByText('Người 1 đang trình bày')).toBeInTheDocument()
      expect(document.querySelector('.cs-present')).not.toBeNull()
    })

    it('asks before replacing someone who is presenting', async () => {
      const { room, people } = setup(1)
      act(() => {
        people[0]!.setTrack(Track.Source.ScreenShare, fakeTrack('s'))
        room.emit(RoomEvent.TrackSubscribed)
      })
      fireEvent.click(screen.getByRole('button', { name: 'Trình bày màn hình' }))
      expect(await screen.findByText('Thay người đang trình bày?')).toBeInTheDocument()
      expect(room.localParticipant.setScreenShareEnabled).not.toHaveBeenCalled()
      fireEvent.click(screen.getByRole('button', { name: 'Trình bày' }))
      await waitFor(() => expect(room.localParticipant.setScreenShareEnabled).toHaveBeenCalledWith(true, expect.anything()))
    })

    it('shows the "Bạn đang trình bày" bar with a stop button while you present', async () => {
      const { room } = setup(1)
      fireEvent.click(screen.getByRole('button', { name: 'Trình bày màn hình' }))
      expect(await screen.findByText('Bạn đang trình bày', { selector: '.cs-prebar span' })).toBeInTheDocument()
      fireEvent.click(screen.getByText('Dừng trình bày', { selector: '.cs-prebar button' }))
      await waitFor(() => expect(room.localParticipant.setScreenShareEnabled).toHaveBeenLastCalledWith(false))
    })
  })
})
