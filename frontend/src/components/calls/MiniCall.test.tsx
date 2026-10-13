import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { RoomEvent } from '../../test/fake-livekit'
import { MiniCall } from './MiniCall'
import { makeCallRoom, WithSnapshot } from './call-test-utils'

vi.mock('livekit-client', async () => (await import('../../test/fake-livekit')).livekitMock)
afterEach(cleanup)

function setup() {
  const h = makeCallRoom()
  const a = h.room.join('u1', 'Trần Minh Đức')
  h.room.join('u2', 'Lê Quang Vinh')
  const startedAt = Date.now() - 65_500
  const onOpen = vi.fn()
  const onLeave = vi.fn()
  render(
    <WithSnapshot callRoom={h.callRoom}>
      {(snap) => <MiniCall snapshot={snap} startedAt={startedAt} onOpen={onOpen} onLeave={onLeave} />}
    </WithSnapshot>,
  )
  return { ...h, a, onOpen, onLeave }
}

describe('MiniCall', () => {
  it('shows the speaker by name and the clock', () => {
    const { room, a } = setup()
    act(() => { room.emit(RoomEvent.ActiveSpeakersChanged, [a]) })
    expect(screen.getByRole('region', { name: 'Cuộc gọi đang thu nhỏ' })).toBeInTheDocument()
    expect(screen.getByText('Trần Minh Đức')).toBeInTheDocument()
    expect(screen.getByText('01:05')).toBeInTheDocument()
  })

  it('a tap opens the call again; the red button leaves', () => {
    const { onOpen, onLeave } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Mở lại cuộc gọi' }))
    expect(onOpen).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('button', { name: 'Rời cuộc gọi' }))
    expect(onLeave).toHaveBeenCalledTimes(1)
  })

  it('is dragged to the left edge', () => {
    setup()
    const box = screen.getByRole('region')
    fireEvent.pointerDown(box, { clientX: 700, pointerId: 1 })
    fireEvent.pointerMove(box, { clientX: 100, pointerId: 1 })
    fireEvent.pointerUp(box, { clientX: 100, pointerId: 1 })
    expect(box).toHaveClass('cs-mini-left')
  })
})
