import { useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'

export interface TabItem {
  id: string
  label: string
  /** Optional trailing element, e.g. a count pill. */
  badge?: ReactNode
}

interface TabBarProps {
  tabs: TabItem[]
  value: string
  onChange: (id: string) => void
  /** Accessible name of the tablist. */
  label: string
  /** Prefix for tab/panel ids so `aria-controls` can point at the panel. */
  idPrefix: string
  className?: string
}

/** Width the indicator is drawn at before scaling; see the note below. */
const INK_BASE = 100

/**
 * Underlined tabs with a sliding active indicator (DESIGN.md §7: 220ms, expo).
 *
 * The indicator only ever animates `transform`: it is a fixed 100px bar moved
 * with translateX and sized with scaleX, never with `left`/`width`. Arrow keys
 * move between tabs (roving tabindex), as the WAI-ARIA tabs pattern expects.
 */
export function TabBar({ tabs, value, onChange, label, idPrefix, className = '' }: TabBarProps) {
  const listRef = useRef<HTMLDivElement>(null)
  const tabRefs = useRef<Record<string, HTMLButtonElement | null>>({})
  const [ink, setInk] = useState<{ x: number; w: number } | null>(null)

  useLayoutEffect(() => {
    const place = () => {
      const el = tabRefs.current[value]
      if (!el) return
      // Inset by 8px each side, like the mockup's ink.
      setInk({ x: el.offsetLeft + 8, w: Math.max(el.offsetWidth - 16, 0) })
    }
    place()
    const ro = typeof ResizeObserver !== 'undefined' ? new ResizeObserver(place) : null
    if (ro && listRef.current) ro.observe(listRef.current)
    document.fonts?.ready.then(place).catch(() => {})
    return () => ro?.disconnect()
  }, [value, tabs])

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const i = tabs.findIndex((t) => t.id === value)
    let next = -1
    if (e.key === 'ArrowRight') next = (i + 1) % tabs.length
    else if (e.key === 'ArrowLeft') next = (i - 1 + tabs.length) % tabs.length
    else if (e.key === 'Home') next = 0
    else if (e.key === 'End') next = tabs.length - 1
    const target = tabs[next]
    if (!target) return
    e.preventDefault()
    onChange(target.id)
    tabRefs.current[target.id]?.focus()
  }

  return (
    <div
      ref={listRef}
      role="tablist"
      aria-label={label}
      onKeyDown={onKeyDown}
      className={`relative flex gap-1 ${className}`}
    >
      {tabs.map((t) => {
        const selected = t.id === value
        return (
          <button
            key={t.id}
            ref={(el) => { tabRefs.current[t.id] = el }}
            type="button"
            role="tab"
            id={`${idPrefix}-tab-${t.id}`}
            aria-selected={selected}
            aria-controls={`${idPrefix}-panel-${t.id}`}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(t.id)}
            className={`relative inline-flex items-center gap-1.5 h-10 px-3 rounded-t-md border-none
              bg-transparent cursor-pointer text-sm font-semibold focus-ring
              transition-colors duration-quick ease-out
              ${selected ? 'text-ink' : 'text-ink-muted hover:text-ink'}`}
          >
            {t.label}
            {t.badge}
          </button>
        )
      })}
      {ink && (
        <span
          aria-hidden="true"
          className="absolute bottom-0 left-0 h-0.75 rounded-t-sm bg-accent origin-left
            transition-transform duration-base ease-out-expo motion-reduce:transition-none"
          style={{ width: INK_BASE, transform: `translateX(${ink.x}px) scaleX(${ink.w / INK_BASE})` }}
        />
      )}
    </div>
  )
}
