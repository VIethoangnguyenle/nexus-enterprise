import {
  createContext, forwardRef, useCallback, useContext, useEffect, useLayoutEffect, useRef, useState,
  type ButtonHTMLAttributes, type KeyboardEvent, type ReactNode, type RefObject,
} from 'react'
import { createPortal } from 'react-dom'
import { AnimatePresence, motion } from 'motion/react'
import { useMotionPresets } from '../../lib/motion'

type Placement = 'bottom-start' | 'bottom-end' | 'top-start'

interface PopoverProps {
  open: boolean
  onClose: () => void
  /** Element the popover hangs from; focus returns here on close. */
  anchorRef: RefObject<HTMLElement | null>
  placement?: Placement
  /** `menu` adds role + roving arrow keys; `dialog` is a free-form surface. */
  role?: 'menu' | 'dialog'
  label?: string
  children: ReactNode
  className?: string
}

const GAP = 6

/**
 * Overlay anchored to a trigger, rendered in a portal so no `overflow: hidden`
 * ancestor can clip it (DESIGN.md §6 Popover / Menu). Closes on Esc and on a
 * pointer press outside; Esc returns focus to the trigger.
 */
export function Popover({
  open, onClose, anchorRef, placement = 'bottom-start', role = 'dialog', label, children, className = '',
}: PopoverProps) {
  const m = useMotionPresets()
  const panelRef = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState<{ top?: number; bottom?: number; left?: number; right?: number }>({})

  const place = useCallback(() => {
    const a = anchorRef.current?.getBoundingClientRect()
    if (!a) return
    if (placement === 'bottom-end') setPos({ top: a.bottom + GAP, right: window.innerWidth - a.right })
    else if (placement === 'top-start') setPos({ bottom: window.innerHeight - a.top + GAP, left: a.left })
    else setPos({ top: a.bottom + GAP, left: a.left })
  }, [anchorRef, placement])

  useLayoutEffect(() => {
    if (!open) return
    place()
    window.addEventListener('resize', place)
    window.addEventListener('scroll', place, true)
    return () => {
      window.removeEventListener('resize', place)
      window.removeEventListener('scroll', place, true)
    }
  }, [open, place])

  // Move focus into the surface: first menu item, or the first focusable.
  useEffect(() => {
    if (!open) return
    const id = requestAnimationFrame(() => {
      const first = panelRef.current?.querySelector<HTMLElement>(
        role === 'menu' ? '[role="menuitem"]' : 'input, button, [href], [tabindex]:not([tabindex="-1"])',
      )
      first?.focus()
    })
    return () => cancelAnimationFrame(id)
  }, [open, role])

  useEffect(() => {
    if (!open) return
    const onPointer = (e: PointerEvent) => {
      const t = e.target as Node
      if (panelRef.current?.contains(t) || anchorRef.current?.contains(t)) return
      onClose()
    }
    const onKey = (e: globalThis.KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.stopPropagation()
      onClose()
      anchorRef.current?.focus()
    }
    document.addEventListener('pointerdown', onPointer)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('pointerdown', onPointer)
      document.removeEventListener('keydown', onKey)
    }
  }, [open, onClose, anchorRef])

  const onMenuKeys = (e: KeyboardEvent<HTMLDivElement>) => {
    if (role !== 'menu') return
    const items = Array.from(panelRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])
    const i = items.indexOf(document.activeElement as HTMLElement)
    let next = -1
    if (e.key === 'ArrowDown') next = (i + 1) % items.length
    else if (e.key === 'ArrowUp') next = (i - 1 + items.length) % items.length
    else if (e.key === 'Home') next = 0
    else if (e.key === 'End') next = items.length - 1
    else if (e.key === 'Tab') onClose()
    if (next < 0) return
    e.preventDefault()
    items[next]?.focus()
  }

  return createPortal(
    <AnimatePresence>
      {open && (
        <motion.div
          ref={panelRef}
          role={role}
          aria-label={label}
          onKeyDown={onMenuKeys}
          {...m.popover}
          className={`fixed z-dropdown min-w-56 rounded-overlay bg-overlay shadow-overlay p-1.5 ${className}`}
          style={pos}
        >
          <MenuCloseContext.Provider value={onClose}>{children}</MenuCloseContext.Provider>
        </motion.div>
      )}
    </AnimatePresence>,
    document.body,
  )
}

const MenuCloseContext = createContext<() => void>(() => {})

interface MenuItemProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon?: ReactNode
  tone?: 'default' | 'danger'
  /** Keep the menu open after selecting (rare). */
  keepOpen?: boolean
}

/** A row in a `Popover role="menu"`. Selecting it closes the menu. */
export const MenuItem = forwardRef<HTMLButtonElement, MenuItemProps>(
  ({ icon, tone = 'default', keepOpen, children, onClick, className = '', ...props }, ref) => {
    const close = useContext(MenuCloseContext)
    return (
      <button
        ref={ref}
        type="button"
        role="menuitem"
        tabIndex={-1}
        onClick={(e) => {
          onClick?.(e)
          if (!keepOpen) close()
        }}
        className={`w-full flex items-center gap-2.5 h-9 px-2.5 rounded-md border-none bg-transparent
          text-sm font-medium text-left cursor-pointer focus-ring hover:bg-hover focus-visible:bg-hover
          transition-colors duration-quick
          ${tone === 'danger' ? 'text-danger' : 'text-ink'} ${className}`}
        {...props}
      >
        {icon}
        {children}
      </button>
    )
  },
)
MenuItem.displayName = 'MenuItem'

/** Hairline between menu groups. */
export function MenuSeparator() {
  return <div role="separator" className="h-px bg-line my-1 mx-1.5" />
}
