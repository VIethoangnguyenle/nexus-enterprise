import { useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent, type Ref } from 'react'
import { addDays, differenceInCalendarDays, format, isSameDay, isSameMonth, startOfDay } from 'date-fns'
import type { Occurrence } from '../../api/calendar'
import { colorStyle, colorTokens } from '../../lib/calendar-color'
import { personStyle } from '../../lib/person-hue'
import {
  eventAriaLabel, formatClock, formatDayMonth, monthWeeks, spanOf, toneOf, weekdayShort,
} from '../../lib/calendar-format'
import { layoutSpans, moveByDays, spansDays } from '../../lib/calendar-layout'
import { MenuItem, Popover, Pressable, Text } from '../primitives'
import type { BlockRealtime } from './EventBlock'

/** Height of the day-number row and of one event row, in pixels (rows are 22px with 2px between). */
const HEAD_PX = 30
const ROW_PX = 24
/** Event rows a cell always has room for, besides its "+ N nữa" button. */
const MIN_ROWS = 3

interface Item {
  o: Occurrence
  key: string
  start: Date
  end: Date
}

interface MonthGridProps {
  /** Any day of the month shown. */
  date: Date
  occurrences: Occurrence[]
  now: Date
  selectedKey?: string
  /** A quick create from a click on an empty cell: its day. */
  draftDay?: Date | null
  draftRef?: Ref<HTMLElement>
  realtime?: Map<string, BlockRealtime>
  keyOf: (o: Occurrence) => string
  colorOf: (o: Occurrence) => number
  onOpen: (o: Occurrence, el: HTMLElement) => void
  onMove: (o: Occurrence, start: Date, end: Date) => void
  onOpenDay: (day: Date) => void
  onSelectDay: (day: Date) => void
}

/** One chip: a dot, the time and the title; a bar when the event is all-day or crosses days. */
function chipTone(o: Occurrence, end: Date, now: Date) {
  const tone = toneOf(o, end, now)
  return tone === 'declined' ? 'text-ink-muted line-through' : tone === 'past' ? 'text-ink-muted' : 'text-ink'
}

/**
 * The month view (DESIGN.md §6 Lịch, Tháng): as many rows as the month needs, each tall enough for the
 * events that fit; the rest sit behind "Thêm N sự kiện". All-day and multi-day events are solid bars across
 * their days. Press a day number for that day; press empty space for a quick create; drag an event to
 * another day to move it there at the same time.
 */
export function MonthGrid({
  date, occurrences, now, selectedKey, draftDay, draftRef, realtime, keyOf, colorOf, onOpen, onMove, onOpenDay, onSelectDay,
}: MonthGridProps) {
  const weeks = useMemo(() => monthWeeks(date), [date])
  const body = useRef<HTMLDivElement>(null)
  const [rowHeight, setRowHeight] = useState(0)
  const [more, setMore] = useState<{ day: Date; el: HTMLElement } | null>(null)
  const moreRef = useRef<HTMLElement | null>(null)
  const [dropDay, setDropDay] = useState<Date | null>(null)
  const swallow = useRef(false)

  useEffect(() => {
    const el = body.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const measure = () => setRowHeight(el.clientHeight / weeks.length)
    measure()
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [weeks.length])

  const capacity = Math.max(MIN_ROWS, Math.floor((rowHeight - HEAD_PX) / ROW_PX))

  const items: Item[] = useMemo(
    () => occurrences.map((o) => ({ o, key: keyOf(o), ...spanOf(o) })).sort((a, b) => a.start.getTime() - b.start.getTime()),
    [occurrences, keyOf],
  )

  /** Drag by pointer: the chip follows nothing on screen; the cell under the pointer lights up, and the drop moves the event. */
  const beginDrag = (e: ReactPointerEvent<HTMLElement>, item: Item) => {
    if (e.pointerType === 'touch' || e.button !== 0 || !item.o.canEdit || item.o.busyOnly) return
    const cells = Array.from(body.current?.querySelectorAll<HTMLElement>('[data-day]') ?? [])
    const rects = cells.map((c) => ({ day: c.dataset.day!, rect: c.getBoundingClientRect() }))
    const x0 = e.clientX
    const y0 = e.clientY
    let moved = false
    const hit = (x: number, y: number) =>
      rects.find((r) => x >= r.rect.left && x < r.rect.right && y >= r.rect.top && y < r.rect.bottom)?.day
    const toDay = (key: string) => {
      const [y, m, d] = key.split('-').map(Number)
      return new Date(y!, m! - 1, d!)
    }
    const move = (ev: PointerEvent) => {
      if (!moved && Math.hypot(ev.clientX - x0, ev.clientY - y0) < 4) return
      moved = true
      const day = hit(ev.clientX, ev.clientY)
      setDropDay(day ? toDay(day) : null)
    }
    const end = (ev: PointerEvent) => {
      cleanup()
      setDropDay(null)
      if (!moved) return
      swallow.current = true
      setTimeout(() => (swallow.current = false), 0)
      const day = hit(ev.clientX, ev.clientY)
      if (!day) return
      const days = differenceInCalendarDays(toDay(day), startOfDay(item.start))
      if (days === 0) return
      const r = moveByDays(item.start, item.end, days)
      onMove(item.o, r.start, r.end)
    }
    const key = (ev: KeyboardEvent) => {
      if (ev.key === 'Escape') {
        cleanup()
        setDropDay(null)
      }
    }
    const cleanup = () => {
      document.removeEventListener('pointermove', move)
      document.removeEventListener('pointerup', end)
      document.removeEventListener('keydown', key)
    }
    document.addEventListener('pointermove', move)
    document.addEventListener('pointerup', end)
    document.addEventListener('keydown', key)
  }

  const open = (o: Occurrence, el: HTMLElement) => {
    if (swallow.current) return
    onOpen(o, el)
  }

  const moreItems = more ? items.filter((i) => i.start < addDays(startOfDay(more.day), 1) && i.end > startOfDay(more.day)) : []

  return (
    <div className="flex flex-col flex-1 min-h-0 px-2 pb-2" data-testid="month-grid">
      <div className="grid grid-cols-7 text-center text-2xs font-semibold tracking-wide text-ink-muted pb-1.5" aria-hidden="true">
        {weeks[0]!.map((d) => <span key={d.getDay()}>{weekdayShort(d)}</span>)}
      </div>
      <div ref={body} className="flex-1 min-h-0 grid" style={{ gridTemplateRows: `repeat(${weeks.length}, minmax(0, 1fr))` }}>
        {weeks.map((week) => {
          const bars = items.filter((i) => i.o.allDay || spansDays(i.start, i.end))
          const barKeys = new Set(bars.map((b) => b.key))
          const placed = layoutSpans(bars.map((b) => ({ id: b.key, start: b.start, end: b.end })), week)
          const barRows = Math.min(placed.reduce((n, s) => Math.max(n, s.lane + 1), 0), capacity - 2)
          const visibleBars = placed.filter((s) => s.lane < barRows)
          return (
            <div key={week[0]!.toISOString()} className="relative grid grid-cols-7 border-t border-line">
              {week.map((day) => {
                const key = format(day, 'yyyy-MM-dd')
                const chips = items.filter((i) => !barKeys.has(i.key) && isSameDay(i.start, day))
                const hiddenBars = placed.filter((s) => s.lane >= barRows
                  && week[s.startCol]!.getTime() <= day.getTime() && week[s.endCol]!.getTime() >= day.getTime()).length
                const room = capacity - barRows
                const overflow = chips.length + hiddenBars > room
                const shown = overflow ? chips.slice(0, Math.max(0, room - 1)) : chips
                const extra = chips.length - shown.length + hiddenBars
                const today = isSameDay(day, now)
                const draft = draftDay && isSameDay(draftDay, day)
                return (
                  <div
                    key={key}
                    data-day={key}
                    onClick={(e) => {
                      if (e.target === e.currentTarget) onSelectDay(day)
                    }}
                    className={`border-l border-line first:border-l-0 min-w-0 overflow-hidden px-1 pt-1
                      ${dropDay && isSameDay(dropDay, day) ? 'bg-accent-wash' : ''}`}
                  >
                    <div className="flex justify-center" style={{ height: HEAD_PX - 4 }}>
                      <Pressable
                        onClick={() => onOpenDay(day)}
                        aria-label={`Mở ngày ${formatDayMonth(day)}`}
                        aria-current={today ? 'date' : undefined}
                        className={`size-6.5 rounded-full grid place-items-center text-xs font-medium tnum
                          ${today ? 'bg-accent text-on-accent' : isSameMonth(day, date) ? 'text-ink hover:bg-hover' : 'text-ink-subtle hover:bg-hover'}`}
                      >
                        {format(day, 'd')}
                      </Pressable>
                    </div>
                    <div className="grid gap-0.5" style={{ paddingTop: barRows * ROW_PX }}>
                      {shown.map((i) => (
                        <Pressable
                          key={i.key}
                          data-event={i.o.eventId}
                          aria-label={eventAriaLabel(i.o, i.start, i.end)}
                          aria-haspopup={i.o.busyOnly ? undefined : 'dialog'}
                          onClick={(e) => !i.o.busyOnly && open(i.o, e.currentTarget)}
                          onPointerDown={(e) => beginDrag(e, i)}
                          style={{ ...colorStyle(colorOf(i.o)), ...(realtime?.has(i.key) ? personStyle(realtime.get(i.key)!.author ?? '') : {}) }}
                          className={`flex items-center gap-1.5 h-5.5 px-1.5 rounded-md text-xs min-w-0 whitespace-nowrap
                            hover:bg-hover ${chipTone(i.o, i.end, now)} ${selectedKey === i.key ? 'bg-hover' : ''}
                            ${realtime?.has(i.key) ? 'rt-halo' : ''}`}
                        >
                          <span aria-hidden="true" className="size-2 rounded-full bg-(--c) shrink-0" />
                          {!i.o.busyOnly && <span className="text-ink-muted tnum shrink-0 max-sm:hidden">{formatClock(i.start)}</span>}
                          <span className="truncate font-semibold">{i.o.title}</span>
                        </Pressable>
                      ))}
                      {extra > 0 && (
                        <Pressable
                          aria-label={`Thêm ${extra} sự kiện ngày ${formatDayMonth(day)}`}
                          aria-haspopup="dialog"
                          onClick={(e) => {
                            moreRef.current = e.currentTarget
                            setMore({ day, el: e.currentTarget })
                          }}
                          className="h-5.5 px-1.5 rounded-md text-xs font-semibold text-ink-muted hover:bg-hover"
                        >
                          {extra} nữa
                        </Pressable>
                      )}
                      {draft && (
                        <div
                          ref={draftRef as Ref<HTMLDivElement> | undefined}
                          data-draft
                          className="h-5.5 px-1.5 rounded-md bg-accent text-on-accent text-xs font-medium flex items-center pointer-events-none"
                        >
                          (Không có tiêu đề)
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
              {visibleBars.map((s) => {
                const b = bars.find((x) => x.key === s.id)!
                return (
                  <Pressable
                    key={s.id}
                    data-event={b.o.eventId}
                    aria-label={eventAriaLabel(b.o, b.start, b.end)}
                    aria-haspopup={b.o.busyOnly ? undefined : 'dialog'}
                    onClick={(e) => !b.o.busyOnly && open(b.o, e.currentTarget)}
                    onPointerDown={(e) => beginDrag(e, b)}
                    style={{
                      ...colorStyle(colorOf(b.o)),
                      left: `calc(${(s.startCol / 7) * 100}% + 4px)`,
                      width: `calc(${((s.endCol - s.startCol + 1) / 7) * 100}% - 8px)`,
                      top: HEAD_PX + s.lane * ROW_PX,
                      height: 22,
                    }}
                    className={`absolute px-1.5 flex items-center text-xs font-medium text-left whitespace-nowrap truncate
                      ${b.o.busyOnly ? 'busy-stripes text-ink-muted' : b.o.myResponse === 'declined'
                        ? 'bg-raised text-ink-muted ring-1 ring-inset ring-line line-through'
                        : 'bg-(--c) text-on-person'}
                      ${s.continuesBefore ? 'rounded-l-none' : 'rounded-l-md'} ${s.continuesAfter ? 'rounded-r-none' : 'rounded-r-md'}
                      ${selectedKey === s.id ? 'shadow-overlay z-10' : ''}`}
                  >
                    {b.o.title}
                  </Pressable>
                )
              })}
            </div>
          )
        })}
      </div>

      <Popover
        open={!!more}
        onClose={() => setMore(null)}
        anchorRef={moreRef}
        label={more ? `Sự kiện ngày ${formatDayMonth(more.day)}` : undefined}
        className="w-72"
      >
        {more && (
          <div className="grid gap-0.5">
            <Text variant="caption" muted className="block px-2.5 py-1 font-semibold">{`Ngày ${formatDayMonth(more.day)}`}</Text>
            {moreItems.map((i) => (
              <MenuItem
                key={i.key}
                disabled={i.o.busyOnly}
                onClick={() => {
                  const el = more.el
                  window.setTimeout(() => open(i.o, el), 0)
                }}
                className="text-left"
              >
                <span aria-hidden="true" className="size-2 rounded-full shrink-0" style={{ background: colorTokens(colorOf(i.o)).fg }} />
                <span className="truncate">{i.o.allDay ? '' : `${formatClock(i.start)} `}{i.o.title}</span>
              </MenuItem>
            ))}
          </div>
        )}
      </Popover>
    </div>
  )
}
