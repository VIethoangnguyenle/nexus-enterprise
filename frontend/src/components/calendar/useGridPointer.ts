import { useCallback, useEffect, useRef, useState, type PointerEvent as ReactPointerEvent, type RefObject } from 'react'
import { addMinutes } from 'date-fns'
import type { Occurrence } from '../../api/calendar'
import { SNAP_MINUTES, SNAP_MINUTES_FINE, dragResult, timeAt } from '../../lib/calendar-layout'

/** Pointer travel under which a press is still a click. */
const THRESHOLD = 4
/** A click on an empty slot makes an hour-long draft. */
const CLICK_MINUTES = 60

export interface MovePreview {
  kind: 'move' | 'resize'
  occurrence: Occurrence
  start: Date
  end: Date
}

export interface Selection {
  start: Date
  end: Date
}

interface Press {
  kind: 'move' | 'resize' | 'create'
  pointerId: number
  x: number
  y: number
  moved: boolean
  /** Column the press began in, and the rectangles of all columns at that moment. */
  col: number
  rects: DOMRect[]
  occurrence?: Occurrence
  start: Date
  end: Date
  /** Create only: where in the column the press began. */
  anchor?: Date
}

interface Options {
  days: Date[]
  /** The day columns, in order, for hit-testing. */
  columns: RefObject<(HTMLElement | null)[]>
  /** A drag of a block ended on new times. */
  onMove: (occurrence: Occurrence, start: Date, end: Date) => void
  /** A drag across empty time (or a click on it) chose a span. */
  onSelect: (start: Date, end: Date) => void
}

/**
 * Pointer handling of the time grid, on `pointermove`/`pointerup` of the document so a drag survives leaving
 * the block. A press only becomes a drag after 4px of travel; before that the click that follows still opens
 * the event. Touch is left to scrolling and taps (the tablet's press-and-hold arrives with its layout).
 */
export function useGridPointer({ days, columns, onMove, onSelect }: Options) {
  const [preview, setPreview] = useState<MovePreview | null>(null)
  const [selection, setSelection] = useState<Selection | null>(null)
  const press = useRef<Press | null>(null)
  const cleanup = useRef<(() => void) | null>(null)
  const swallowClick = useRef(false)
  // The newest callbacks and days, read from listeners that outlive a render.
  const latest = useRef({ days, onMove, onSelect })
  useEffect(() => {
    latest.current = { days, onMove, onSelect }
  })
  useEffect(() => () => cleanup.current?.(), [])

  const stop = useCallback(() => {
    cleanup.current?.()
    cleanup.current = null
    press.current = null
    setPreview(null)
    setSelection(null)
  }, [])

  const start = useCallback(
    (e: ReactPointerEvent<HTMLElement>, init: Omit<Press, 'pointerId' | 'x' | 'y' | 'moved' | 'rects'>) => {
      if (e.pointerType === 'touch' || e.button !== 0) return
      cleanup.current?.()
      const rects = (columns.current ?? []).map((el) => el?.getBoundingClientRect() ?? new DOMRect())
      const p: Press = { ...init, pointerId: e.pointerId, x: e.clientX, y: e.clientY, moved: false, rects }
      press.current = p

      const colAt = (x: number) => {
        const i = p.rects.findIndex((r) => x >= r.left && x < r.right)
        return i === -1 ? p.col : i
      }

      const onMovePointer = (ev: PointerEvent) => {
        if (ev.pointerId !== p.pointerId) return
        const dx = ev.clientX - p.x
        const dy = ev.clientY - p.y
        if (!p.moved && Math.hypot(dx, dy) < THRESHOLD) return
        p.moved = true
        const step = ev.altKey ? SNAP_MINUTES_FINE : SNAP_MINUTES
        if (p.kind === 'create') {
          const top = p.rects[p.col]?.top ?? 0
          const day = latest.current.days[p.col]!
          const here = timeAt(day, ev.clientY - top, step)
          const a = p.anchor!
          const [s, en] = here < a ? [here, a] : [a, here]
          setSelection({ start: s, end: en.getTime() === s.getTime() ? addMinutes(s, step) : en })
          return
        }
        const days = p.kind === 'move' ? colAt(ev.clientX) - p.col : 0
        const r = dragResult({ mode: p.kind, start: p.start, end: p.end, dy, days, step })
        setPreview({ kind: p.kind, occurrence: p.occurrence!, start: r.start, end: r.end })
      }

      const finish = (ev: PointerEvent) => {
        if (ev.pointerId !== p.pointerId) return
        const { onMove: move, onSelect: select, days: dayList } = latest.current
        if (p.moved) {
          swallowClick.current = true
          setTimeout(() => (swallowClick.current = false), 0)
        }
        if (p.kind === 'create') {
          const day = dayList[p.col]!
          if (p.moved) {
            const step = ev.altKey ? SNAP_MINUTES_FINE : SNAP_MINUTES
            const top = p.rects[p.col]?.top ?? 0
            const here = timeAt(day, ev.clientY - top, step)
            const a = p.anchor!
            const [s, en] = here < a ? [here, a] : [a, here]
            select(s, en.getTime() === s.getTime() ? addMinutes(s, step) : en)
          } else {
            select(p.anchor!, addMinutes(p.anchor!, CLICK_MINUTES))
          }
        } else if (p.moved) {
          const step = ev.altKey ? SNAP_MINUTES_FINE : SNAP_MINUTES
          const days = p.kind === 'move' ? colAt(ev.clientX) - p.col : 0
          const r = dragResult({ mode: p.kind, start: p.start, end: p.end, dy: ev.clientY - p.y, days, step })
          if (r.start.getTime() !== p.start.getTime() || r.end.getTime() !== p.end.getTime()) move(p.occurrence!, r.start, r.end)
        }
        stop()
      }

      const onKey = (ev: KeyboardEvent) => {
        if (ev.key === 'Escape') {
          swallowClick.current = p.moved
          stop()
        }
      }

      document.addEventListener('pointermove', onMovePointer)
      document.addEventListener('pointerup', finish)
      document.addEventListener('pointercancel', stop)
      document.addEventListener('keydown', onKey)
      cleanup.current = () => {
        document.removeEventListener('pointermove', onMovePointer)
        document.removeEventListener('pointerup', finish)
        document.removeEventListener('pointercancel', stop)
        document.removeEventListener('keydown', onKey)
      }
    },
    [columns, stop],
  )

  return {
    preview,
    selection,
    /** Press on a block's body. */
    beginMove: (e: ReactPointerEvent<HTMLElement>, occurrence: Occurrence, s: Date, en: Date, col: number) =>
      start(e, { kind: 'move', occurrence, start: s, end: en, col }),
    /** Press on a block's lower edge. */
    beginResize: (e: ReactPointerEvent<HTMLElement>, occurrence: Occurrence, s: Date, en: Date, col: number) => {
      e.stopPropagation()
      start(e, { kind: 'resize', occurrence, start: s, end: en, col })
    },
    /** Press on empty time of column `col`; `y` is the pointer's offset in the column. */
    beginCreate: (e: ReactPointerEvent<HTMLElement>, col: number, y: number) => {
      const day = days[col]!
      const anchor = timeAt(day, y, e.altKey ? SNAP_MINUTES_FINE : SNAP_MINUTES)
      start(e, { kind: 'create', start: anchor, end: anchor, col, anchor })
    },
    /** True once for the click that ends a drag, so a drop does not also open the event. */
    consumeClick: () => {
      const v = swallowClick.current
      swallowClick.current = false
      return v
    },
  }
}
