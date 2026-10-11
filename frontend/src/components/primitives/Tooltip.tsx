import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { motion } from 'motion/react'
import { DURATION, EASE, useMotionPresets } from '../../lib/motion'

/** Props the trigger must receive: spread them onto the one element the tooltip describes. */
export interface TooltipTriggerProps {
  ref: (el: HTMLElement | null) => void
  onMouseEnter: () => void
  onMouseLeave: () => void
  onMouseDown: () => void
  onFocus: () => void
  onBlur: () => void
  'aria-describedby'?: string
}

interface TooltipProps {
  content: ReactNode
  /** Switch it off when the text is already visible; no wrapper, no description. */
  disabled?: boolean
  children: (props: TooltipTriggerProps) => ReactNode
}

const HOVER_DELAY_MS = 400
const GAP = 8

// Whether the last input was a pointer. Focus that follows a press is not "visible" focus, so it
// does not raise a tooltip; focus that follows a key (Tab) does. `:focus-visible` says the same in
// browsers, but not in jsdom, which the tests run in.
let pointerLast = false
if (typeof document !== 'undefined') {
  document.addEventListener('pointerdown', () => { pointerLast = true }, true)
  document.addEventListener('keydown', () => { pointerLast = false }, true)
}

/**
 * Tooltip for a control whose text is hidden (DESIGN.md §5 / §6). It never
 * carries the only name: the trigger keeps its own `aria-label`. Hover shows
 * it after 400ms, keyboard focus at once, Esc / blur / press hide it
 * (WCAG 1.4.13). It sits 8px to the right of the trigger.
 */
export function Tooltip({ content, disabled = false, children }: TooltipProps) {
  const id = useId()
  const m = useMotionPresets()
  const anchor = useRef<HTMLElement | null>(null)
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null)

  const hide = useCallback(() => {
    clearTimeout(timer.current)
    setPos(null)
  }, [])
  const show = useCallback(() => {
    const r = anchor.current?.getBoundingClientRect()
    if (r) setPos({ left: r.right + GAP, top: r.top + r.height / 2 })
  }, [])

  useEffect(() => () => clearTimeout(timer.current), [])
  useEffect(() => {
    if (!pos) return
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') hide() }
    document.addEventListener('keydown', onKey)
    window.addEventListener('scroll', hide, true)
    window.addEventListener('resize', hide)
    return () => {
      document.removeEventListener('keydown', onKey)
      window.removeEventListener('scroll', hide, true)
      window.removeEventListener('resize', hide)
    }
  }, [pos, hide])

  if (disabled) {
    return <>{children({
      ref: (el) => { anchor.current = el },
      onMouseEnter: () => {}, onMouseLeave: () => {}, onMouseDown: () => {}, onFocus: () => {}, onBlur: () => {},
    })}</>
  }

  const trigger: TooltipTriggerProps = {
    ref: (el) => { anchor.current = el },
    onMouseEnter: () => {
      clearTimeout(timer.current)
      timer.current = setTimeout(show, HOVER_DELAY_MS)
    },
    onMouseLeave: hide,
    onMouseDown: hide,
    onFocus: () => { if (!pointerLast) show() },
    onBlur: hide,
    'aria-describedby': pos ? id : undefined,
  }

  return (
    <>
      {children(trigger)}
      {pos && createPortal(
        <motion.div
          id={id}
          role="tooltip"
          initial={m.reduced ? false : { opacity: 0, x: -4 }}
          animate={{ opacity: 1, x: 0, transition: { duration: m.reduced ? 0 : DURATION.quick, ease: EASE.out } }}
          className="fixed z-tooltip max-w-65 -translate-y-1/2 px-2.5 py-1.5 rounded-md bg-overlay shadow-overlay
            text-xs font-medium text-ink pointer-events-none"
          style={{ left: pos.left, top: pos.top }}
        >
          {content}
        </motion.div>,
        document.body,
      )}
    </>
  )
}
