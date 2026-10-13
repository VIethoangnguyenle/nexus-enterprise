import { useRef, useState } from 'react'
import { PhoneOff } from 'lucide-react'
import type { CallSnapshot } from '../../lib/call-room'
import { useElapsed } from '../../hooks/useElapsed'
import { Avatar, Pressable } from '../primitives'
import { TrackVideo } from './media'
import './call-stage.css'

/**
 * The call shrunk to a tile (mockup §6): the speaker, the clock and a leave
 * button. It floats over every screen while a call is open and its own screen
 * is not, can be dragged to the left or right edge, and a tap opens the call
 * again. `pip` is the same tile filling a Picture-in-Picture window.
 */
export function MiniCall({ snapshot, startedAt, onOpen, onLeave, pip = false }: {
  snapshot: CallSnapshot
  startedAt: number
  onOpen: () => void
  onLeave: () => void
  pip?: boolean
}) {
  const clock = useElapsed(startedAt)
  const [left, setLeft] = useState(false)
  const drag = useRef<{ x: number; moved: boolean } | null>(null)

  const person = snapshot.participants.find((p) => p.id === snapshot.speakerId)
    ?? snapshot.participants.find((p) => !p.isLocal)
    ?? snapshot.participants[0]

  const onPointerDown = (e: React.PointerEvent) => {
    drag.current = { x: e.clientX, moved: false }
  }
  const onPointerMove = (e: React.PointerEvent) => {
    const d = drag.current
    if (!d || d.moved || Math.abs(e.clientX - d.x) <= 8) return
    d.moved = true
    // Captured only once it is a drag: capturing at the press would steal the click from the buttons.
    ;(e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId)
  }
  const onPointerUp = (e: React.PointerEvent) => {
    const d = drag.current
    drag.current = null
    if (!d?.moved) return
    setLeft(e.clientX < window.innerWidth / 2)
  }

  return (
    <div
      className={pip ? 'cs-mini cs-mini-pip' : `cs-mini ${left ? 'cs-mini-left' : ''}`}
      role="region"
      aria-label="Cuộc gọi đang thu nhỏ"
      onPointerDown={pip ? undefined : onPointerDown}
      onPointerMove={pip ? undefined : onPointerMove}
      onPointerUp={pip ? undefined : onPointerUp}
    >
      <Pressable
        className={`cs-tile ${person?.speaking ? 'cs-tile-speaking' : ''}`}
        aria-label="Mở lại cuộc gọi"
        onClick={() => { if (!drag.current?.moved) onOpen() }}
      >
        {person?.camera
          ? <TrackVideo track={person.camera} speakerMark />
          : <span className="cs-avatar"><Avatar name={person?.name ?? 'Cuộc gọi'} hueKey={person?.id} size={52} /></span>}
        {person && <span className="cs-name">{person.isLocal ? 'Bạn' : person.name}</span>}
      </Pressable>
      <div className="cs-mini-bar">
        <span>{clock}</span>
        <Pressable className="cs-mini-end" aria-label="Rời cuộc gọi" onClick={onLeave}>
          <PhoneOff className="cs-icon-s" />
        </Pressable>
      </div>
    </div>
  )
}
