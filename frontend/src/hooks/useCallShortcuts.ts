import { useEffect } from 'react'
import type { CallRoom } from '../lib/call-room'

const isTyping = (t: EventTarget | null): boolean =>
  t instanceof HTMLElement && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName))

/**
 * Keys on the call screen: Ctrl+D microphone, Ctrl+E camera, Ctrl+Alt+H raise
 * or lower the hand, and Space held to talk while the microphone is off. None
 * of them fire while the person is typing (the chat panel). Cmd stands in for
 * Ctrl on a Mac.
 */
export function useCallShortcuts(callRoom: CallRoom | null, enabled: boolean): void {
  useEffect(() => {
    if (!callRoom || !enabled) return
    const onDown = (e: KeyboardEvent) => {
      const mod = e.ctrlKey || e.metaKey
      if (mod && e.altKey && e.code === 'KeyH') { e.preventDefault(); void callRoom.toggleHand(); return }
      if (mod && !e.altKey && !e.shiftKey && e.code === 'KeyD') { e.preventDefault(); void callRoom.toggleMic(); return }
      if (mod && !e.altKey && !e.shiftKey && e.code === 'KeyE') { e.preventDefault(); void callRoom.toggleCamera(); return }
      if (e.code === 'Space' && !mod && !e.altKey && !isTyping(e.target)) {
        e.preventDefault()
        if (!e.repeat) void callRoom.holdToTalk(true)
      }
    }
    const onUp = (e: KeyboardEvent) => {
      if (e.code !== 'Space' || isTyping(e.target)) return
      e.preventDefault()
      void callRoom.holdToTalk(false)
    }
    const release = () => { void callRoom.holdToTalk(false) }
    window.addEventListener('keydown', onDown)
    window.addEventListener('keyup', onUp)
    window.addEventListener('blur', release)
    return () => {
      window.removeEventListener('keydown', onDown)
      window.removeEventListener('keyup', onUp)
      window.removeEventListener('blur', release)
    }
  }, [callRoom, enabled])
}
