import { forwardRef, type CSSProperties, type PointerEvent as ReactPointerEvent } from 'react'
import { Video } from 'lucide-react'
import type { Occurrence } from '../../api/calendar'
import { colorStyle } from '../../lib/calendar-color'
import { eventAriaLabel, formatClock, toneOf, type BlockTone } from '../../lib/calendar-format'
import { personStyle } from '../../lib/person-hue'
import { Pressable } from '../primitives'

/** Surfaces of each tone (DESIGN.md §6, Khối sự kiện). The calendar's colour arrives as `--c` and `--w`. */
const TONE: Record<BlockTone, string> = {
  solid: 'bg-(--c) text-on-person ring-1 ring-base',
  past: 'bg-(--w) text-ink ring-1 ring-base',
  pending: 'bg-raised text-(--c) ring-1 ring-inset ring-(--c)',
  declined: 'bg-raised text-ink-muted ring-1 ring-inset ring-line',
  busy: 'busy-stripes text-ink-muted ring-1 ring-inset ring-(--w)',
}
/** The time line is a shade softer than the title on every tone. */
const TIME: Record<BlockTone, string> = {
  solid: 'text-on-person/90',
  past: 'text-ink-muted',
  pending: 'text-(--c)',
  declined: 'text-ink-muted',
  busy: 'text-ink-muted',
}

export interface BlockRealtime {
  /** Author key of the change (user id); only its hue ever reaches the screen. */
  author?: string
}

interface EventBlockProps {
  occurrence: Occurrence
  start: Date
  end: Date
  now: Date
  color: number
  /** Pixels from the top and height in the day column, and percent of the width. */
  box: { top: number; height: number; left: number; width: number }
  selected?: boolean
  /** Being dragged: drawn over its neighbours. */
  dragging?: boolean
  realtime?: BlockRealtime
  /** Pointer press on the body (starts a move) and on the lower edge (starts a resize). */
  onBodyPointerDown?: (e: ReactPointerEvent<HTMLElement>) => void
  onEdgePointerDown?: (e: ReactPointerEvent<HTMLElement>) => void
  onOpen: (el: HTMLElement) => void
}

/** A timed event in a day column: a real button, solid in the calendar's colour. */
export const EventBlock = forwardRef<HTMLButtonElement, EventBlockProps>(function EventBlock(
  { occurrence: o, start, end, now, color, box, selected, dragging, realtime, onBodyPointerDown, onEdgePointerDown, onOpen },
  ref,
) {
  const tone = toneOf(o, end, now)
  const oneLine = box.height < 36
  const style: CSSProperties = {
    ...colorStyle(color),
    ...(realtime ? personStyle(realtime.author ?? '') : {}),
    top: box.top,
    height: box.height,
    left: `${box.left}%`,
    width: `${box.width}%`,
  }
  const draggable = o.canEdit && !o.busyOnly
  return (
    <Pressable
      ref={ref}
      data-event={o.eventId}
      data-tone={tone}
      aria-label={eventAriaLabel(o, start, end)}
      aria-haspopup={o.busyOnly ? undefined : 'dialog'}
      onClick={(e) => {
        if (!o.busyOnly) onOpen(e.currentTarget)
      }}
      onPointerDown={draggable ? onBodyPointerDown : undefined}
      style={style}
      className={`absolute rounded-md px-1.75 py-0.75 overflow-hidden text-left text-xs leading-snug select-none
        ${TONE[tone]} ${draggable ? 'cursor-grab active:cursor-grabbing touch-manipulation' : o.busyOnly ? 'cursor-default' : ''}
        ${selected || dragging ? 'shadow-overlay z-10' : 'z-1'} ${realtime ? 'rt-halo' : ''}`}
    >
      <span className={oneLine ? 'flex items-center whitespace-nowrap' : 'grid gap-px content-start'}>
        <span className={`truncate font-medium ${tone === 'declined' ? 'line-through' : ''}`}>{o.title}</span>
        <span className={`flex items-center gap-1 tnum whitespace-nowrap ${TIME[tone]} ${oneLine ? 'shrink-0' : ''}`}>
          {oneLine ? `, ${formatClock(start)}` : `${formatClock(start)} – ${formatClock(end)}`}
          {o.hasCall && !oneLine && <Video size={11} strokeWidth={1.75} aria-hidden="true" />}
        </span>
        {realtime && !oneLine && <span className="rt-tag text-2xs font-semibold">vừa cập nhật</span>}
      </span>
      {draggable && onEdgePointerDown && (
        <span
          aria-hidden="true"
          data-resize
          onPointerDown={onEdgePointerDown}
          className="absolute inset-x-0 bottom-0 h-2 cursor-ns-resize"
        />
      )}
    </Pressable>
  )
})
