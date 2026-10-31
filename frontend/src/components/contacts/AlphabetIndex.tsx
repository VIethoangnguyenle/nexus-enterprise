import { useRef, useState, type PointerEvent } from 'react'
import { Pressable } from '../primitives'

interface AlphabetIndexProps {
  /** The letters of the list, in its order (`indexLettersOf`). */
  letters: string[]
  /** Scroll the list to the first person filed under `letter`. */
  onJump: (letter: string) => void
}

/**
 * The A–Z column of a long list on a phone (DESIGN.md "Danh bạ" › Điện thoại, mockup `mobile-shell.html` §12): an
 * 18px column at the right edge, a tap or a drag along it jumps to that letter, and while dragging the letter shows
 * large in the middle of the screen. Each letter is also a button, so a keyboard or a screen reader jumps the same way.
 */
export function AlphabetIndex({ letters, onJump }: AlphabetIndexProps) {
  const [dragging, setDragging] = useState<string | null>(null)
  const last = useRef<string | null>(null)

  if (letters.length === 0) return null

  const letterAt = (e: PointerEvent<HTMLElement>): string | undefined => {
    const rect = e.currentTarget.getBoundingClientRect()
    if (rect.height > 0) {
      const i = Math.floor(((e.clientY - rect.top) / rect.height) * letters.length)
      return letters[Math.min(letters.length - 1, Math.max(0, i))]
    }
    // No layout (a test, a hidden column): the letter under the finger is the one pressed.
    return (e.target as HTMLElement).closest<HTMLElement>('[data-letter]')?.dataset.letter
  }
  const jump = (letter: string | undefined) => {
    if (!letter) return
    setDragging(letter)
    if (letter === last.current) return
    last.current = letter
    onJump(letter)
  }
  const end = () => {
    last.current = null
    setDragging(null)
  }

  return (
    <>
      <nav
        aria-label="Nhảy theo chữ cái"
        className="absolute right-0.5 top-1/2 -translate-y-1/2 z-sticky grid w-4.5 touch-none select-none"
        onPointerDown={(e) => {
          e.currentTarget.setPointerCapture?.(e.pointerId)
          jump(letterAt(e))
        }}
        onPointerMove={(e) => { if (last.current !== null) jump(letterAt(e)) }}
        onPointerUp={end}
        onPointerCancel={end}
      >
        {letters.map((letter) => (
          <Pressable
            key={letter}
            data-letter={letter}
            aria-label={`Nhảy tới ${letter}`}
            onClick={() => onJump(letter)}
            className="grid place-items-center h-4.5 p-0 border-none bg-transparent text-[0.625rem] leading-none font-semibold
              text-accent cursor-pointer rounded-sm focus-ring"
          >
            {letter}
          </Pressable>
        ))}
      </nav>
      {dragging && (
        <div
          data-testid="az-bubble"
          aria-hidden="true"
          className="pointer-events-none fixed left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 z-modal grid place-items-center
            w-18 h-18 rounded-surface bg-overlay shadow-overlay font-display text-2xl font-bold text-ink"
        >
          {dragging}
        </div>
      )}
    </>
  )
}
