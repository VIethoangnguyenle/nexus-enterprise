import { useRef, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { motion, type Variants } from 'motion/react'
import { ArrowLeft } from 'lucide-react'
import { useModalFocus } from '../../hooks/useModalFocus'
import { DURATION, EASE, useMotionPresets } from '../../lib/motion'
import { IconButton } from '../primitives'

/**
 * The push of a level into view: from the right over 280ms (expo), a fade under reduced motion.
 *
 * The leave is a variant that takes `custom`, the `AnimatePresence`'s: a layer being removed can no longer
 * be given new props, so the screen tells it through `custom` whether the person left by the device's own
 * back (`true`: the device has drawn that step already, a second fade over it is the flash) or by a step of
 * the app's.
 */
export function phonePush(m: ReturnType<typeof useMotionPresets>) {
  const push = m.reduced ? m.route : {
    initial: { opacity: 0, x: 24 },
    animate: { opacity: 1, x: 0, transition: { duration: DURATION.layout, ease: EASE.outExpo } },
    exit: { opacity: 0, transition: { duration: DURATION.quick, ease: EASE.in } },
  }
  const variants: Variants = {
    from: push.initial,
    to: push.animate,
    leave: (byDevice: boolean) => (byDevice ? { opacity: 0, transition: { duration: 0 } } : push.exit),
  }
  return { variants, initial: 'from', animate: 'to', exit: 'leave' } as const
}

/**
 * One screen of Danh bạ's phone stack (DESIGN.md "Danh bạ", tiers 2 and 3): a full-screen layer in the body, so
 * the app's own top bar steps aside, and the page behind is inert. It comes in from the right over 280ms (a
 * fade under reduced motion). Esc, like the system back, is one step back.
 */
export function PhoneLayer({ label, onBack, children, initialFocusRef }: {
  label: string
  onBack: () => void
  children: ReactNode
  /** Where focus lands on opening, when not on the first control (the search screen's box). */
  initialFocusRef?: RefObject<HTMLElement | null>
}) {
  const m = useMotionPresets()
  const surfaceRef = useRef<HTMLDivElement>(null)
  useModalFocus({ active: true, surfaceRef, onClose: onBack, initialFocusRef })
  const push = phonePush(m)
  return createPortal(
    <motion.div
      ref={surfaceRef}
      role="dialog"
      aria-modal="true"
      aria-label={label}
      {...push}
      className="fixed inset-x-0 top-0 z-modal h-app flex flex-col min-h-0 bg-base pt-safe overflow-hidden"
    >
      {children}
    </motion.div>,
    document.body,
  )
}

/** The bar of a list screen: back (44), the name of this level and, under it, where it is; `end` is one action at the far side. */
export function PhoneBackBar({ title, sub, onBack, end, focusBack }: {
  title: string; sub: ReactNode; onBack: () => void; end?: ReactNode
  /** The level was just changed under the reader's finger: focus goes to the new bar's back button. */
  focusBack?: boolean
}) {
  return (
    <div className="flex items-center gap-1 px-2 pt-3 pb-1 min-w-0">
      <IconButton aria-label="Quay lại" title="Quay lại" shape="round" size="lg" onClick={onBack} autoFocus={focusBack} className="w-11 h-11 shrink-0">
        <ArrowLeft size={20} strokeWidth={1.75} aria-hidden="true" />
      </IconButton>
      <div className="grid min-w-0">
        <h2 className="m-0 truncate font-display text-[1.0625rem] leading-snug font-bold text-ink">{title}</h2>
        <span className="flex min-w-0 text-xs text-ink-muted">{sub}</span>
      </div>
      {end && <div className="ml-auto shrink-0 pr-1">{end}</div>}
    </div>
  )
}
