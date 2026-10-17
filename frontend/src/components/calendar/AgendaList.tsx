import { Fragment, useMemo } from 'react'
import { addDays, format, isSameDay, isSameMonth, startOfDay } from 'date-fns'
import { Video } from 'lucide-react'
import type { Occurrence } from '../../api/calendar'
import { colorStyle } from '../../lib/calendar-color'
import { eventAriaLabel, formatClock, formatDayShort, responseWord, spanOf, toneOf } from '../../lib/calendar-format'
import { Button, Heading, Pressable } from '../primitives'
import type { BlockRealtime } from './EventBlock'
import { personStyle } from '../../lib/person-hue'

interface AgendaListProps {
  /** First day listed. */
  from: Date
  /** Days listed, from `from`. */
  days: number
  occurrences: Occurrence[]
  now: Date
  selectedKey?: string
  realtime?: Map<string, BlockRealtime>
  keyOf: (o: Occurrence) => string
  colorOf: (o: Occurrence) => number
  onOpen: (o: Occurrence, el: HTMLElement) => void
  /** Absent for a fixed list, such as search results. */
  onLoadMore?: () => void
}

/** The red line between the events that are over and those to come, in today's list. */
function NowRow() {
  return (
    <div role="separator" aria-label="Bây giờ" className="flex items-center gap-1.5 px-2 text-2xs font-semibold text-danger">
      <span aria-hidden="true" className="size-2 rounded-full bg-danger" />
      <span aria-hidden="true" className="flex-1 h-0.5 bg-danger" />
    </div>
  )
}

/**
 * Lịch biểu (DESIGN.md §6): day by day, only days that have something. Each row is a line: dot, time,
 * title, and on the right the video and the unanswered state. A new month gets its heading.
 */
export function AgendaList({
  from, days, occurrences, now, selectedKey, realtime, keyOf, colorOf, onOpen, onLoadMore,
}: AgendaListProps) {
  const groups = useMemo(() => {
    const rows = occurrences.map((o) => ({ o, key: keyOf(o), ...spanOf(o) })).sort((a, b) => a.start.getTime() - b.start.getTime())
    const out: { day: Date; rows: typeof rows }[] = []
    for (let n = 0; n < days; n++) {
      const day = addDays(startOfDay(from), n)
      const end = addDays(day, 1)
      const here = rows.filter((r) => r.start < end && (r.end > day || r.start >= day))
      if (here.length > 0) out.push({ day, rows: here })
    }
    return out
  }, [occurrences, from, days, keyOf])

  return (
    <div className="flex-1 min-h-0 overflow-y-auto px-5 pb-5" data-testid="agenda">
      {groups.map(({ day, rows }, gi) => {
        const today = isSameDay(day, now)
        const newMonth = gi === 0 || !isSameMonth(groups[gi - 1]!.day, day)
        const nowAt = today ? rows.findIndex((r) => r.start > now) : -2
        return (
          <Fragment key={day.toISOString()}>
            {newMonth && (
              <Heading as="h3" look="panel" className="pt-4 pb-1">
                {`Tháng ${format(day, 'M')} năm ${format(day, 'yyyy')}`}
              </Heading>
            )}
            <section aria-label={`Ngày ${format(day, 'd')} tháng ${format(day, 'M')}`} className="grid grid-cols-1 sm:grid-cols-[7.5rem_1fr] gap-1.5 sm:gap-3 py-3 border-t border-line">
              <div className="flex items-baseline gap-2.5">
                <span className={`w-8 font-display text-xl font-semibold tnum ${today ? 'text-accent' : 'text-ink'}`}>{format(day, 'd')}</span>
                <span className="text-xs font-semibold tracking-wide text-ink-muted">{formatDayShort(day)}</span>
              </div>
              <ul className="m-0 p-0 list-none grid gap-0.5">
                {rows.map((r, i) => {
                  const tone = toneOf(r.o, r.end, now)
                  const rt = realtime?.get(r.key)
                  return (
                    <Fragment key={r.key}>
                      {nowAt === i && <li><NowRow /></li>}
                      <li>
                        <Pressable
                          data-event={r.o.eventId}
                          aria-label={eventAriaLabel(r.o, r.start, r.end)}
                          aria-haspopup={r.o.busyOnly ? undefined : 'dialog'}
                          onClick={(e) => !r.o.busyOnly && onOpen(r.o, e.currentTarget)}
                          style={{ ...colorStyle(colorOf(r.o)), ...(rt ? personStyle(rt.author ?? '') : {}) }}
                          className={`w-full grid grid-cols-[0.875rem_minmax(0,1fr)_auto] sm:grid-cols-[0.875rem_8rem_1fr_auto] items-center gap-2.5
                            min-h-9 max-sm:min-h-14 px-2 max-sm:px-3 py-1 max-sm:py-2 rounded-lg max-sm:border max-sm:border-line
                            hover:bg-hover ${selectedKey === r.key ? 'bg-hover' : ''} ${rt ? 'rt-halo' : ''}`}
                        >
                          <span aria-hidden="true" className="size-2.5 rounded-full bg-(--c)" />
                          {/* Phone: title on its own line, time and place under it. From `sm` up the two
                              are separate columns of the row. */}
                          <span className="min-w-0 grid sm:contents">
                            <span className="text-sm text-ink-muted tnum max-sm:order-2" data-agenda-when>
                              {r.o.allDay ? 'Cả ngày' : `${formatClock(r.start)} – ${formatClock(r.end)}`}
                              {r.o.location && !r.o.busyOnly && <span className="sm:hidden">{` · ${r.o.location}`}</span>}
                            </span>
                            <span
                              data-agenda-title
                              className={`truncate text-sm max-sm:text-16 font-semibold max-sm:order-1 ${tone === 'declined' ? 'line-through text-ink-muted font-medium' : 'text-ink'}`}
                            >
                              {r.o.title}
                            </span>
                          </span>
                          <span className="flex items-center gap-2 text-xs text-ink-muted">
                            {r.o.hasCall && <Video size={14} strokeWidth={1.75} aria-hidden="true" />}
                            {tone === 'pending' && r.o.myResponse && <span>{responseWord(r.o.myResponse)}</span>}
                            {rt && <span className="rt-tag font-semibold">vừa cập nhật</span>}
                          </span>
                        </Pressable>
                      </li>
                    </Fragment>
                  )
                })}
                {nowAt === -1 && <li><NowRow /></li>}
              </ul>
            </section>
          </Fragment>
        )
      })}
      {onLoadMore && (
        <div className="flex justify-center pt-4">
          <Button variant="soft" size="sm" className="rounded-full" onClick={onLoadMore}>Tải thêm 30 ngày</Button>
        </div>
      )}
    </div>
  )
}
