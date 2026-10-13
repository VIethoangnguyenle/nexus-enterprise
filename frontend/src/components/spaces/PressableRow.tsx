import { useRef, type KeyboardEvent, type MouseEvent, type PointerEvent, type ReactNode } from 'react'
import { motion } from 'motion/react'
import { useLongPress } from '../../hooks/useLongPress'
import { useMotionPresets } from '../../lib/motion'

/** Controls that keep their own press; unlike the shared default, a link is not one: the row is a link. */
const OWN_PRESS = 'button, input, textarea, select, [role="button"], [contenteditable="true"]'

/**
 * Opens a sheet on a long press, or on the Menu key / Shift+F10, for an element
 * that is itself (or holds) a link. Spread `props` on the element and give it
 * `ref`. The tap that ends a long press is swallowed so what the element opens
 * does not open behind the sheet. `ignore` is the selector of controls inside
 * that keep their own press.
 */
export function useMenuPress<T extends HTMLElement>(onMenu: (el: HTMLElement) => void, ignore = OWN_PRESS) {
  const ref = useRef<T>(null)
  const swallowTap = useRef(false)
  const press = useLongPress(() => {
    swallowTap.current = true
    if (ref.current) onMenu(ref.current)
  }, true, ignore)

  const props = {
    ...press,
    onPointerDown: (e: PointerEvent<HTMLElement>) => {
      swallowTap.current = false
      press.onPointerDown(e)
    },
    onClickCapture: (e: MouseEvent) => {
      if (!swallowTap.current) return
      swallowTap.current = false
      e.preventDefault()
      e.stopPropagation()
    },
    onKeyDown: (e: KeyboardEvent) => {
      if (e.key === 'ContextMenu' || (e.key === 'F10' && e.shiftKey)) {
        e.preventDefault()
        if (ref.current) onMenu(ref.current)
      }
    },
  }
  return { ref, props }
}

/** A list row that opens its sheet on a long press (`useMenuPress`). */
export function PressableRow({ onMenu, slide = true, children }: {
  onMenu: (el: HTMLElement) => void
  /** Rows re-order with a slide; a muted one just jumps. */
  slide?: boolean
  children: ReactNode
}) {
  const m = useMotionPresets()
  const { ref, props } = useMenuPress<HTMLLIElement>(onMenu)

  return (
    <motion.li
      ref={ref}
      layout={slide ? m.layoutProp : false}
      transition={m.layout}
      {...m.row}
      {...props}
      className="select-none [-webkit-touch-callout:none]"
    >
      {children}
    </motion.li>
  )
}
