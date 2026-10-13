import { afterEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render } from '@testing-library/react'
import type { CallRoom } from '../lib/call-room'
import { Textarea } from '../components/primitives'
import { useCallShortcuts } from './useCallShortcuts'

function room() {
  return {
    toggleMic: vi.fn(), toggleCamera: vi.fn(), toggleHand: vi.fn(), holdToTalk: vi.fn(),
  } as unknown as CallRoom & Record<'toggleMic' | 'toggleCamera' | 'toggleHand' | 'holdToTalk', ReturnType<typeof vi.fn>>
}

function Probe({ cr, enabled = true }: { cr: CallRoom; enabled?: boolean }) {
  useCallShortcuts(cr, enabled)
  return <Textarea aria-label="Soạn" />
}

afterEach(() => vi.restoreAllMocks())

describe('useCallShortcuts', () => {
  it('Ctrl+D toggles the mic and stops the browser bookmarking the page', () => {
    const cr = room()
    render(<Probe cr={cr} />)
    const notCancelled = fireEvent.keyDown(window, { key: 'd', code: 'KeyD', ctrlKey: true })
    expect(cr.toggleMic).toHaveBeenCalledTimes(1)
    expect(notCancelled).toBe(false)
  })

  it('Ctrl+E toggles the camera', () => {
    const cr = room()
    render(<Probe cr={cr} />)
    fireEvent.keyDown(window, { key: 'e', code: 'KeyE', ctrlKey: true })
    expect(cr.toggleCamera).toHaveBeenCalledTimes(1)
  })

  it('Ctrl+Alt+H raises the hand, whatever character Alt makes', () => {
    const cr = room()
    render(<Probe cr={cr} />)
    fireEvent.keyDown(window, { key: '˙', code: 'KeyH', ctrlKey: true, altKey: true })
    expect(cr.toggleHand).toHaveBeenCalledTimes(1)
  })

  it('holds Space to talk once, and lets go on release', () => {
    const cr = room()
    render(<Probe cr={cr} />)
    fireEvent.keyDown(window, { code: 'Space' })
    fireEvent.keyDown(window, { code: 'Space', repeat: true })
    expect(cr.holdToTalk).toHaveBeenCalledTimes(1)
    expect(cr.holdToTalk).toHaveBeenLastCalledWith(true)
    fireEvent.keyUp(window, { code: 'Space' })
    expect(cr.holdToTalk).toHaveBeenLastCalledWith(false)
  })

  it('leaves Space and letters alone while typing in the chat', () => {
    const cr = room()
    const { getByLabelText } = render(<Probe cr={cr} />)
    fireEvent.keyDown(getByLabelText('Soạn'), { code: 'Space' })
    fireEvent.keyUp(getByLabelText('Soạn'), { code: 'Space' })
    expect(cr.holdToTalk).not.toHaveBeenCalled()
  })

  it('does nothing when disabled or without a room', () => {
    const cr = room()
    render(<Probe cr={cr} enabled={false} />)
    fireEvent.keyDown(window, { key: 'd', code: 'KeyD', ctrlKey: true })
    expect(cr.toggleMic).not.toHaveBeenCalled()
  })

  it('stops listening when the screen goes', () => {
    const cr = room()
    const { unmount } = render(<Probe cr={cr} />)
    unmount()
    fireEvent.keyDown(window, { key: 'd', code: 'KeyD', ctrlKey: true })
    expect(cr.toggleMic).not.toHaveBeenCalled()
  })

  it('releases hold-to-talk when the window loses focus', () => {
    const cr = room()
    render(<Probe cr={cr} />)
    fireEvent.blur(window)
    expect(cr.holdToTalk).toHaveBeenCalledWith(false)
  })
})
