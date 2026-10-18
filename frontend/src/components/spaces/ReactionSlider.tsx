import { useEffect, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Pressable } from '../primitives'

/** The reaction row of the long-press slide (mockup `mobile-shell.html` §9). */
export const SLIDE_REACTIONS = ['👍', '❤️', '😂', '😮', '🙏', '✅'] as const
/** A finger that travels further than this from where the press rested has slid. */
export const SLIDE_SLOP = 10
/** The row reaches this far above and below itself: a finger a little off the row still picks. */
const REACH_Y = 28
/** Height of the row (44px buttons + 6px padding) and its gap to the message. */
const ROW_H = 56
const GAP = 12

export interface SlideRect {
  top: number
  bottom: number
  left: number
  width: number
}

interface ReactionSliderProps {
  /** The pointer that pressed, and where it rested when the long press fired. */
  start: { pointerId: number; x: number; y: number }
  /** Where the pressed message sits on screen: the row floats above it (below it when there is no room). */
  anchor: SlideRect
  /** The message lifted above the scrim, drawn at `anchor`. */
  preview?: ReactNode
  emojis?: readonly string[]
  /** Released over an emoji. */
  onSelect: (emoji: string) => void
  /** Lifted without sliding: the long-press sheet takes over, as before. */
  onLift: () => void
  /** Released after sliding off the row, or the system took the touch. */
  onCancel: () => void
}

/**
 * Long press, then slide onto a reaction (DESIGN.md §6 "Vuốt", mockup §9): while the finger that pressed stays
 * down, a row of reactions floats over the message; the one under the finger rises and grows 1.5×, and lifting
 * there picks it. Lifting off the row cancels; lifting before sliding at all hands over to the sheet. The page does
 * not scroll while the row is up. The row is a toolbar named "Thả cảm xúc" so a screen reader names what it is;
 * its buttons also take a tap.
 */
export function ReactionSlider({ start, anchor, preview, emojis = SLIDE_REACTIONS, onSelect, onLift, onCancel }: ReactionSliderProps) {
  const [hot, setHot] = useState<string | null>(null)
  const buttons = useRef(new Map<string, HTMLElement>())
  const latest = useRef({ onSelect, onLift, onCancel })
  useEffect(() => {
    latest.current = { onSelect, onLift, onCancel }
  })

  useEffect(() => {
    let moved = false
    let current: string | null = null
    const under = (x: number, y: number): string | null => {
      for (const [emoji, el] of buttons.current) {
        const r = el.getBoundingClientRect()
        if (x >= r.left && x <= r.right && y >= r.top - REACH_Y && y <= r.bottom + REACH_Y) return emoji
      }
      return null
    }
    const onMove = (e: PointerEvent) => {
      if (e.pointerId !== start.pointerId) return
      if (Math.hypot(e.clientX - start.x, e.clientY - start.y) > SLIDE_SLOP) moved = true
      current = under(e.clientX, e.clientY)
      setHot(current)
    }
    const onUp = (e: PointerEvent) => {
      if (e.pointerId !== start.pointerId) return
      const picked = under(e.clientX, e.clientY) ?? current
      if (picked) latest.current.onSelect(picked)
      else if (!moved) latest.current.onLift()
      else latest.current.onCancel()
    }
    const onSystemCancel = (e: PointerEvent) => {
      if (e.pointerId === start.pointerId) latest.current.onCancel()
    }
    // The finger is on the row now, not on the page: a vertical slide must not scroll the stream under it.
    const holdScroll = (e: TouchEvent) => {
      if (e.cancelable) e.preventDefault()
    }
    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
    window.addEventListener('pointercancel', onSystemCancel)
    document.addEventListener('touchmove', holdScroll, { passive: false })
    return () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
      window.removeEventListener('pointercancel', onSystemCancel)
      document.removeEventListener('touchmove', holdScroll)
    }
  }, [start.pointerId, start.x, start.y])

  const roomAbove = anchor.top - ROW_H - GAP >= 8
  const rowTop = roomAbove ? anchor.top - ROW_H - GAP : anchor.bottom + GAP

  return createPortal(
    <div className="fixed inset-0 z-modal touch-none select-none" data-testid="reaction-slide">
      <div aria-hidden="true" className="absolute inset-0 bg-scrim animate-fade-in" />
      {preview && (
        <div
          aria-hidden="true"
          className="absolute"
          style={{ top: anchor.top, left: anchor.left, width: anchor.width }}
        >
          {preview}
        </div>
      )}
      <div
        role="toolbar"
        aria-label="Thả cảm xúc"
        className="absolute inset-x-4 flex justify-between px-2.5 py-1.5 rounded-full bg-overlay shadow-overlay"
        style={{ top: rowTop }}
      >
        {emojis.map((emoji) => (
          <Pressable
            key={emoji}
            tabIndex={-1}
            aria-label={emoji}
            aria-pressed={hot === emoji}
            ref={(el) => {
              if (el) buttons.current.set(emoji, el)
              else buttons.current.delete(emoji)
            }}
            onClick={() => latest.current.onSelect(emoji)}
            className={`grid place-items-center w-11 h-11 rounded-full text-[1.6rem] leading-none
              transition-transform duration-[120ms] motion-reduce:transition-none
              ${hot === emoji ? '-translate-y-3.5 scale-150 bg-accent-wash' : ''}`}
          >
            {emoji}
          </Pressable>
        ))}
      </div>
    </div>,
    document.body,
  )
}
