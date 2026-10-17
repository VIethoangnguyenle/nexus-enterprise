import { useEffect, useMemo, useRef, type Ref } from 'react'
import { addDays, format, isSameDay, startOfDay } from 'date-fns'
import { ChevronDown } from 'lucide-react'
import type { Occurrence } from '../../api/calendar'
import { colorStyle } from '../../lib/calendar-color'
import { formatClock, gmtLabel, spanOf, weekdayShort } from '../../lib/calendar-format'
import { DAY_PX, HOUR_PX, layoutDay, layoutSpans, yOf, type TimedItem } from '../../lib/calendar-layout'
import { Pressable } from '../primitives'
import { EventBlock, type BlockRealtime } from './EventBlock'
import { useGridPointer } from './useGridPointer'

/** All-day bars shown before the rest fold into "+ N". */
const ALL_DAY_LANES = 3
const HOURS = Array.from({ length: 24 }, (_, h) => h)

export interface GridDraft {
  start: Date
  end: Date
}

interface TimeGridProps {
  days: Date[]
  occurrences: Occurrence[]
  now: Date
  /** The one open or chosen (instance key). */
  selectedKey?: string
  /** The draft of a quick create, drawn as an accent block. */
  draft?: GridDraft | null
  /** Receives the draft block, so the quick-create popover can hang from it. */
  draftRef?: Ref<HTMLElement>
  /** Changes made by someone else, by instance key. */
  realtime?: Map<string, BlockRealtime>
  keyOf: (o: Occurrence) => string
  colorOf: (o: Occurrence) => number
  onOpen: (o: Occurrence, el: HTMLElement) => void
  onMove: (o: Occurrence, start: Date, end: Date) => void
  /** A span of time was chosen by dragging or clicking empty time. */
  onSelect: (start: Date, end: Date) => void
  /** The day number was pressed. */
  onOpenDay: (day: Date) => void
}

/** The red line of the present moment (DESIGN.md §6); named so a screen reader and a test can find it. */
function NowLine({ now }: { now: Date }) {
  return (
    <div
      role="img"
      aria-label={`Bây giờ, ${formatClock(now)}`}
      className="absolute inset-x-0 h-0.5 bg-danger z-2 pointer-events-none"
      style={{ top: yOf(now) }}
    >
      <span className="absolute -left-0.25 -top-1.25 size-3 rounded-full bg-danger" />
    </div>
  )
}

/**
 * Day, 3 days and week (DESIGN.md §6 Lịch): a row of day headings, a row for all-day items, and 24 hours of
 * 48px with a column per day. Blocks are real buttons; dragging one moves it, dragging its lower edge
 * resizes it, and dragging across empty time draws a draft.
 */
export function TimeGrid({
  days, occurrences, now, selectedKey, draft, draftRef, realtime, keyOf, colorOf, onOpen, onMove, onSelect, onOpenDay,
}: TimeGridProps) {
  const columns = useRef<(HTMLElement | null)[]>([])
  const scroller = useRef<HTMLDivElement>(null)
  const pointer = useGridPointer({ days, columns, onMove, onSelect })

  // Open on the hour before now, once per view; moving between ranges keeps the place.
  useEffect(() => {
    const el = scroller.current
    if (!el) return
    el.scrollTop = Math.max(0, (now.getHours() - 1) * HOUR_PX)
    // Only when the grid first appears (or the number of columns changes with the view).
  }, [days.length])

  const spans = useMemo(() => occurrences.map((o) => ({ o, ...spanOf(o) })), [occurrences])
  const allDay = useMemo(
    () => spans.filter((x) => x.o.allDay),
    [spans],
  )
  const timed = useMemo(() => spans.filter((x) => !x.o.allDay), [spans])

  const placedAllDay = useMemo(
    () => layoutSpans(allDay.map((x) => ({ id: keyOf(x.o), start: x.start, end: x.end })), days),
    [allDay, days, keyOf],
  )
  const lanes = placedAllDay.reduce((n, s) => Math.max(n, s.lane + 1), 0)
  const shownLanes = Math.min(lanes, ALL_DAY_LANES)
  const hidden = days.map((_, c) =>
    placedAllDay.filter((s) => s.lane >= ALL_DAY_LANES && s.startCol <= c && s.endCol >= c).length)

  const byKey = useMemo(() => new Map(spans.map((x) => [keyOf(x.o), x])), [spans, keyOf])
  const previewKey = pointer.preview ? keyOf(pointer.preview.occurrence) : undefined

  return (
    <div className="flex flex-col flex-1 min-h-0" data-testid="time-grid">
      <div
        className="grid shrink-0 pr-2"
        style={{ gridTemplateColumns: `3.5rem repeat(${days.length}, minmax(0, 1fr))` }}
      >
        <span className="self-end pb-1.5 pl-1.5 text-2xs text-ink-muted">{gmtLabel(now)}</span>
        {days.map((day) => {
          const today = isSameDay(day, now)
          return (
            <div key={day.toISOString()} className="grid justify-items-center gap-0.5 pt-1 pb-2">
              <span className={`text-2xs font-semibold tracking-wide ${today ? 'text-accent' : 'text-ink-muted'}`}>
                {weekdayShort(day)}
              </span>
              <Pressable
                onClick={() => onOpenDay(day)}
                aria-label={`Mở ngày ${format(day, 'd')} tháng ${format(day, 'M')}`}
                aria-current={today ? 'date' : undefined}
                className={`size-11.5 rounded-full grid place-items-center text-2xl font-normal tnum
                  ${today ? 'bg-accent text-on-accent' : 'text-ink hover:bg-hover'}`}
              >
                {format(day, 'd')}
              </Pressable>
            </div>
          )
        })}
      </div>

      {allDay.length > 0 && (
        <div
          className="grid shrink-0 pr-2 pb-1 border-b border-line"
          style={{ gridTemplateColumns: `3.5rem repeat(${days.length}, minmax(0, 1fr))` }}
        >
          <span className="pt-1.5 pr-1.5 text-right text-2xs text-ink-muted">Cả ngày</span>
          <div
            className="relative"
            style={{ gridColumn: `2 / span ${days.length}`, height: shownLanes * 24 + (lanes > shownLanes ? 22 : 0) }}
          >
            {placedAllDay.filter((s) => s.lane < ALL_DAY_LANES).map((s) => {
              const x = byKey.get(s.id)!
              const o = x.o
              return (
                <Pressable
                  key={s.id}
                  aria-label={`${o.title}, cả ngày`}
                  aria-haspopup={o.busyOnly ? undefined : 'dialog'}
                  onClick={(e) => !o.busyOnly && onOpen(o, e.currentTarget)}
                  style={{
                    ...colorStyle(colorOf(o)),
                    left: `calc(${(s.startCol / days.length) * 100}% + 2px)`,
                    width: `calc(${((s.endCol - s.startCol + 1) / days.length) * 100}% - 4px)`,
                    top: s.lane * 24 + 2,
                    height: 22,
                  }}
                  className={`absolute px-2 text-xs font-medium text-left truncate flex items-center
                    ${s.continuesBefore ? 'rounded-l-none' : 'rounded-l-md'} ${s.continuesAfter ? 'rounded-r-none' : 'rounded-r-md'}
                    ${o.busyOnly ? 'busy-stripes text-ink-muted' : o.myResponse === 'declined'
                      ? 'bg-raised text-ink-muted ring-1 ring-inset ring-line line-through'
                      : 'bg-(--c) text-on-person'}
                    ${selectedKey === s.id ? 'shadow-overlay z-10' : ''}`}
                >
                  {o.title}
                </Pressable>
              )
            })}
            {lanes > shownLanes && (
              <div className="absolute inset-x-0 bottom-0 flex">
                {hidden.map((n, c) => (
                  <span key={c} className="flex-1 text-center text-2xs font-medium text-ink-muted">
                    {n > 0 && (
                      <span className="inline-flex items-center">
                        + {n}<ChevronDown size={12} strokeWidth={1.75} aria-hidden="true" />
                      </span>
                    )}
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      <div ref={scroller} className="flex-1 min-h-0 overflow-y-auto">
        <div
          className="relative grid pr-2"
          style={{ gridTemplateColumns: `3.5rem repeat(${days.length}, minmax(0, 1fr))`, height: DAY_PX }}
        >
          <div className="relative" aria-hidden="true">
            {HOURS.slice(1).map((h) => (
              <span
                key={h}
                className="absolute right-2 -translate-y-2 text-2xs text-ink-muted tnum"
                style={{ top: h * HOUR_PX }}
              >
                {String(h).padStart(2, '0')}:00
              </span>
            ))}
          </div>

          {/* Hour lines run under every column. */}
          <div
            className="absolute inset-y-0 right-2 left-14 pointer-events-none"
            aria-hidden="true"
          >
            {HOURS.map((h) => (
              <div key={h} className="h-12 border-t border-line first:border-t-0" />
            ))}
          </div>

          {days.map((day, c) => {
            const items: TimedItem[] = timed
              .filter((x) => !(pointer.preview && keyOf(x.o) === previewKey))
              .map((x) => ({ id: keyOf(x.o), start: x.start, end: x.end }))
            if (pointer.preview) items.push({ id: previewKey!, start: pointer.preview.start, end: pointer.preview.end })
            const placed = layoutDay(items, day)
            const showDraft = draft && isSameDay(draft.start, day)
            const sel = pointer.selection && isSameDay(pointer.selection.start, day) ? pointer.selection : null
            return (
              <div
                key={day.toISOString()}
                ref={(el) => {
                  columns.current[c] = el
                }}
                data-day={format(day, 'yyyy-MM-dd')}
                className="relative border-l border-line"
                onPointerDown={(e) => {
                  if ((e.target as HTMLElement).closest('[data-event]')) return
                  const rect = e.currentTarget.getBoundingClientRect()
                  pointer.beginCreate(e, c, e.clientY - rect.top)
                }}
              >
                {placed.map((b) => {
                  const x = byKey.get(b.id)!
                  const live = pointer.preview && b.id === previewKey ? pointer.preview : null
                  const start = live?.start ?? x.start
                  const end = live?.end ?? x.end
                  return (
                    <EventBlock
                      key={b.id}
                      occurrence={x.o}
                      start={start}
                      end={end}
                      now={now}
                      color={colorOf(x.o)}
                      box={b}
                      selected={selectedKey === b.id}
                      dragging={!!live}
                      realtime={realtime?.get(b.id)}
                      onBodyPointerDown={(e) => pointer.beginMove(e, x.o, x.start, x.end, c)}
                      onEdgePointerDown={(e) => pointer.beginResize(e, x.o, x.start, x.end, c)}
                      onOpen={(el) => {
                        if (!pointer.consumeClick()) onOpen(x.o, el)
                      }}
                    />
                  )
                })}
                {(sel || showDraft) && (
                  <GhostBlock
                    span={(sel ?? draft)!}
                    refProp={sel ? undefined : draftRef}
                  />
                )}
                {isSameDay(day, now) && <NowLine now={now} />}
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

/** The temporary block of a draft or of a drag across empty time (accent, with a grip at the foot). */
function GhostBlock({ span, refProp }: { span: GridDraft; refProp?: Ref<HTMLElement> }) {
  const top = yOf(span.start)
  const bottom = span.end >= addDays(startOfDay(span.start), 1) ? DAY_PX : yOf(span.end)
  return (
    <div
      ref={refProp as Ref<HTMLDivElement> | undefined}
      data-draft
      className="absolute inset-x-0.5 rounded-md bg-accent text-on-accent shadow-overlay z-10 px-1.75 py-0.75 text-xs pointer-events-none"
      style={{ top, height: Math.max(20, bottom - top) }}
    >
      <span className="font-medium tnum">{formatClock(span.start)} – {formatClock(span.end)}</span>
      <span aria-hidden="true" className="absolute bottom-0.5 left-1/2 -ml-2.25 h-0.75 w-4.5 rounded-sm bg-on-accent" />
    </div>
  )
}
