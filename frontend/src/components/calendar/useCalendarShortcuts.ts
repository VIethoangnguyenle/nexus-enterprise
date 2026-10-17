import { useEffect, useRef } from 'react'
import type { CalendarView } from '../../lib/calendar-format'

interface Handlers {
  onView: (v: CalendarView) => void
  onToday: () => void
  /** J is forward, K is back. */
  onStep: (steps: -1 | 1) => void
  onCreate: () => void
  /** / opens the search. */
  onSearch?: () => void
  /** E and Delete act on the event that is open; absent when none is. */
  onEdit?: () => void
  onDelete?: () => void
}

const VIEW_KEYS: Record<string, CalendarView> = { d: 'day', w: 'week', m: 'month', a: 'agenda', x: '3day' }

/** Typing somewhere: the keys belong to it. */
function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)
}

/**
 * The calendar's keys, the same as Google's (DESIGN.md §6, Phím tắt): D W M A X change the view, T goes to
 * today, J and K step forward and back, C creates, E edits and Delete removes the open event. Esc is handled
 * by whatever is open. Nothing fires while typing, with a modifier held, or when `enabled` is false.
 */
export function useCalendarShortcuts(handlers: Handlers, enabled: boolean) {
  const latest = useRef(handlers)
  useEffect(() => {
    latest.current = handlers
  })

  useEffect(() => {
    if (!enabled) return
    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey || isTyping(e.target)) return
      const h = latest.current
      const key = e.key.length === 1 ? e.key.toLowerCase() : e.key
      const view = VIEW_KEYS[key]
      if (view) h.onView(view)
      else if (key === 't') h.onToday()
      else if (key === 'j') h.onStep(1)
      else if (key === 'k') h.onStep(-1)
      else if (key === 'c') h.onCreate()
      else if (key === '/' && h.onSearch) h.onSearch()
      else if (key === 'e' && h.onEdit) h.onEdit()
      else if ((key === 'Delete' || key === 'Backspace') && h.onDelete) h.onDelete()
      else return
      e.preventDefault()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [enabled])
}
