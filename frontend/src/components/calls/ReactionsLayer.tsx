import type { FloatingReaction } from '../../lib/call-room'
import './call-stage.css'

/** Reactions rising from the bottom left for three seconds, with the name of who sent them. Nothing is kept. */
export function ReactionsLayer({ reactions }: { reactions: FloatingReaction[] }) {
  return (
    <div className="cs-floats" aria-live="polite" aria-atomic="false">
      {reactions.map((r) => (
        <div key={r.id} className="cs-float" style={{ left: 8 + ((r.id * 37) % 140) }}>
          <span aria-hidden="true">{r.emoji}</span>
          <span>{r.name}</span>
        </div>
      ))}
    </div>
  )
}
