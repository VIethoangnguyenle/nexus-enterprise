import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, screen, waitFor } from '@testing-library/react'
import { RoomEvent } from '../../test/fake-livekit'
import { renderWithClient, resetClient } from '../../test/render'
import { useCallStore, type CallSession } from '../../stores/call.store'
import { PhoneCallView, HIDE_AFTER_MS } from './PhoneCallView'
import { makeCallRoom, WithSnapshot } from './call-test-utils'

vi.mock('livekit-client', async () => (await import('../../test/fake-livekit')).livekitMock)
vi.mock('../../lib/call-sound', () => ({ playChime: vi.fn() }))
vi.mock('../../hooks/useActiveWorkspace', () => ({ useActiveWorkspace: () => ({ workspaceId: 'w1' }) }))
vi.mock('../../hooks/usePeople', async () => {
  const { EMPTY_DIRECTORY } = await import('../../lib/people')
  return { usePeople: () => ({ ...EMPTY_DIRECTORY, isLoading: false }) }
})
vi.mock('../../hooks/useCalls', async (orig) => ({ ...(await orig<object>()), useMessagesSince: () => 2 }))
vi.mock('./CallSidePanel', () => ({ CallSidePanel: ({ panel }: { panel: string }) => <aside aria-label="Bảng bên">{panel}</aside> }))

const session = (isDm: boolean): CallSession => ({
  callId: 'c1', channelId: 'ch1', kind: 'video', title: 'Đối soát giao dịch', isDm, joinedAt: Date.now(), startedAt: Date.now(),
})

function setup(others: number, isDm = others === 1) {
  const h = makeCallRoom()
  Array.from({ length: others }, (_, i) => h.room.join(`u${i + 1}`, `Người ${i + 1}`))
  const onLeave = vi.fn()
  const onMinimize = vi.fn()
  useCallStore.getState().clear()
  useCallStore.getState().begin(session(isDm))
  renderWithClient(
    <WithSnapshot callRoom={h.callRoom}>
      {(snap) => <PhoneCallView callRoom={h.callRoom} snapshot={snap} session={session(isDm)} call={undefined} onLeave={onLeave} onMinimize={onMinimize} />}
    </WithSnapshot>,
  )
  return { ...h, onLeave, onMinimize }
}

describe('PhoneCallView', () => {
  beforeEach(() => vi.useFakeTimers({ shouldAdvanceTime: true }))
  afterEach(() => { vi.useRealTimers(); cleanup(); resetClient() })

  it('1-1: the other person full screen and your own small tile', () => {
    setup(1)
    expect(screen.getByTestId('tile-u1')).toBeInTheDocument()
    expect(document.querySelector('.cs-pself')).not.toBeNull()
    expect(screen.getByText('Người 1', { selector: 'b' })).toBeInTheDocument()
  })

  it('group: a two-column grid in pages with dots', () => {
    setup(8)
    expect(document.querySelectorAll('.cs-pgrid')).toHaveLength(2)
    expect(document.querySelector('.cs-dots')).not.toBeNull()
    expect(screen.getByText(/9 người/)).toBeInTheDocument()
  })

  it('has no Trình bày anywhere, and the controls are the five round buttons', () => {
    setup(3)
    expect(screen.queryByRole('button', { name: /Trình bày/ })).toBeNull()
    for (const name of [/micro/, /camera$/, 'Đổi camera trước/sau', 'Thêm', 'Rời cuộc gọi']) {
      expect(screen.getByRole('button', { name })).toBeInTheDocument()
    }
  })

  it('minimises and leaves', () => {
    const { onMinimize, onLeave } = setup(1)
    fireEvent.click(screen.getByRole('button', { name: 'Thu nhỏ cuộc gọi' }))
    fireEvent.click(screen.getByRole('button', { name: 'Rời cuộc gọi' }))
    expect(onMinimize).toHaveBeenCalled()
    expect(onLeave).toHaveBeenCalled()
  })

  it('flips between the front and back camera', async () => {
    const { room } = setup(1)
    fireEvent.click(screen.getByRole('button', { name: 'Bật camera' }))
    await waitFor(() => expect(room.localParticipant.setCameraEnabled).toHaveBeenCalled())
    fireEvent.click(screen.getByRole('button', { name: 'Đổi camera trước/sau' }))
    expect(screen.getByRole('button', { name: 'Đổi camera trước/sau' })).toBeInTheDocument()
  })

  it('the sheet holds six reactions, hand, chat, people and devices', async () => {
    const { room } = setup(2)
    fireEvent.click(screen.getByRole('button', { name: 'Thêm' }))
    const sheet = screen.getByRole('dialog', { name: 'Thêm' })
    expect(sheet).toBeInTheDocument()
    for (const r of ['Thích', 'Thương', 'Cười', 'Ngạc nhiên', 'Vỗ tay', 'Chúc mừng']) expect(screen.getByRole('button', { name: r })).toBeInTheDocument()
    expect(screen.getByText('2 mới')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Vỗ tay' }))
    await waitFor(() => expect(room.localParticipant.publishData).toHaveBeenCalled())
    expect(screen.queryByRole('dialog', { name: 'Thêm' })).toBeNull()
  })

  it('opens the chat full screen from the sheet', () => {
    setup(2)
    fireEvent.click(screen.getByRole('button', { name: 'Thêm' }))
    fireEvent.click(screen.getByRole('button', { name: /Chat trong cuộc gọi/ }))
    expect(screen.getByLabelText('Bảng bên')).toHaveTextContent('chat')
  })

  it('hides the controls after 4 seconds and a tap brings them back', () => {
    setup(1)
    const root = screen.getByTestId('phone-call')
    expect(root).not.toHaveClass('cs-phone-hidden')
    act(() => { vi.advanceTimersByTime(HIDE_AFTER_MS + 50) })
    expect(root).toHaveClass('cs-phone-hidden')
    fireEvent.click(document.querySelector('.cs-tap')!)
    expect(root).not.toHaveClass('cs-phone-hidden')
  })

  it('keeps the controls while the sheet is open', () => {
    setup(1)
    fireEvent.click(screen.getByRole('button', { name: 'Thêm' }))
    act(() => { vi.advanceTimersByTime(HIDE_AFTER_MS * 3) })
    expect(screen.getByTestId('phone-call')).not.toHaveClass('cs-phone-hidden')
  })

  it('shows "Chạm để bật âm thanh" and reconnecting overlays', () => {
    const { room } = setup(1)
    act(() => { room.canPlaybackAudio = false; room.emit(RoomEvent.AudioPlaybackStatusChanged); room.emit(RoomEvent.Reconnecting) })
    expect(screen.getByRole('button', { name: 'Chạm để bật âm thanh' })).toBeInTheDocument()
    expect(screen.getByText('Đang kết nối lại…')).toBeInTheDocument()
  })
})
