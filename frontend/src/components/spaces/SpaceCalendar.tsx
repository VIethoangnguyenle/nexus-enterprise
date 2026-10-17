import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { addDays, addMonths, format, isSameDay, isSameMonth, startOfDay } from 'date-fns'
import { CalendarDays, ChevronLeft, ChevronRight, CircleAlert, Plus } from 'lucide-react'
import type { Occurrence } from '../../api/calendar'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useOccurrences } from '../../hooks/useCalendar'
import { useGroupCalendar, useNewGroupEvent } from '../../hooks/useGroupCalendar'
import { useNow } from '../../hooks/useNow'
import { instanceKey } from '../../lib/calendar-cache'
import { colorStyle } from '../../lib/calendar-color'
import {
  AGENDA_DAYS, formatClock, formatDayMonth, monthWeeks, rangeOf, spanOf, toDateKey, weekdayShort,
} from '../../lib/calendar-format'
import { statusOf } from '../../lib/errors'
import { AgendaList } from '../calendar/AgendaList'
import { Button, IconButton, Pressable } from '../primitives'
import { EmptyState } from './EmptyState'

type View = 'upcoming' | 'month'

const VIEWS: { id: View; label: string }[] = [
  { id: 'upcoming', label: 'Sắp tới' },
  { id: 'month', label: 'Tháng' },
]

const MAX_DOTS = 3

/** Occurrences that touch `day`. */
const onDay = (list: Occurrence[], day: Date) =>
  list.filter((o) => {
    const { start, end } = spanOf(o)
    return start < addDays(day, 1) && (end > day || start >= day)
  })

/**
 * The Lịch tab of a group (DESIGN.md "Lịch của nhóm chat", mockup §10): what is coming up on the group's own
 * calendar, or the month at a glance, with Tạo sự kiện (which makes the calendar the first time) and Mở trong
 * Lịch. The calendar belongs to the group's members: someone who is not one is refused by the server and sees
 * a sentence, not events.
 */
export function SpaceCalendar({ channelId }: { channelId: string }) {
  const { workspaceId: wsId } = useActiveWorkspace()
  const navigate = useNavigate()
  const now = useNow()
  const { calendar, query: calendars } = useGroupCalendar(wsId, channelId)
  const create = useNewGroupEvent(wsId, channelId)
  const [view, setView] = useState<View>('upcoming')
  const [shown, setShown] = useState(() => new Date())
  const [picked, setPicked] = useState<Date | null>(null)

  const range = useMemo(() => {
    if (view === 'month') return rangeOf('month', shown)
    const from = startOfDay(now)
    return { from, to: addDays(from, AGENDA_DAYS) }
  }, [view, shown, now])
  const ids = useMemo(() => (calendar ? [calendar.id] : []), [calendar])
  const events = useOccurrences(wsId, range.from, range.to, ids)
  const list = useMemo(() => events.data ?? [], [events.data])

  const openInCalendar = (date?: Date) =>
    void navigate({ to: '/calendar', search: (prev: { ws?: string }) => ({ ws: prev.ws, ...(date ? { date: toDateKey(date) } : {}) }) })

  const controls = (
    <div className="flex flex-wrap items-center gap-2 px-5 py-3">
      <span role="radiogroup" aria-label="Cách xem" className="inline-flex p-0.5 rounded-full bg-sunk">
        {VIEWS.map((v) => (
          <Pressable
            key={v.id}
            role="radio"
            aria-checked={view === v.id}
            onClick={() => setView(v.id)}
            className={`h-8 px-3.5 rounded-full text-sm font-semibold ${view === v.id ? 'bg-raised text-ink shadow-sm' : 'text-ink-muted hover:text-ink'}`}
          >
            {v.label}
          </Pressable>
        ))}
      </span>
      <span className="grow" />
      <Button variant="ghost" size="sm" onClick={() => openInCalendar()}>Mở trong Lịch</Button>
      <Button size="sm" className="rounded-full" loading={create.pending} onClick={() => void create.open()}>
        <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
        Tạo sự kiện
      </Button>
    </div>
  )

  let body
  if (calendars.isError) {
    body = (
      <EmptyState
        icon={<CircleAlert size={24} strokeWidth={1.75} />}
        text="Chưa tải được lịch của nhóm."
        action={<Button variant="soft" size="sm" onClick={() => void calendars.refetch()}>Thử lại</Button>}
      />
    )
  } else if (events.isError) {
    body = statusOf(events.error) === 403
      ? <EmptyState icon={<CalendarDays size={24} strokeWidth={1.75} />} text="Bạn không xem được lịch của nhóm này." />
      : (
        <EmptyState
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text="Chưa tải được lịch của nhóm."
          action={<Button variant="soft" size="sm" onClick={() => void events.refetch()}>Thử lại</Button>}
        />
      )
  } else if (calendars.isSuccess && !calendar) {
    body = <EmptyState icon={<CalendarDays size={24} strokeWidth={1.75} />} text="Nhóm chưa có sự kiện nào." />
  } else if (view === 'upcoming') {
    body = !events.isPending && list.length === 0
      ? <EmptyState compact icon={<CalendarDays size={24} strokeWidth={1.75} />} text="Chưa có sự kiện nào sắp tới." />
      : (
        <AgendaList
          from={range.from}
          days={AGENDA_DAYS}
          occurrences={list}
          now={now}
          keyOf={instanceKey}
          colorOf={() => calendar?.color ?? -1}
          onOpen={(o) => openInCalendar(spanOf(o).start)}
        />
      )
  } else {
    const weeks = monthWeeks(shown)
    const day = picked && isSameMonth(picked, shown) ? picked : null
    body = (
      <div className="px-5 pb-5 grid gap-3 content-start overflow-y-auto" style={colorStyle(calendar?.color ?? -1)}>
        <div className="flex items-center">
          <span className="flex-1 text-sm font-semibold text-ink">{`Tháng ${format(shown, 'M')} năm ${format(shown, 'yyyy')}`}</span>
          <IconButton size="md" aria-label="Tháng trước" onClick={() => setShown(addMonths(shown, -1))}><ChevronLeft size={16} strokeWidth={1.75} /></IconButton>
          <IconButton size="md" aria-label="Tháng sau" onClick={() => setShown(addMonths(shown, 1))}><ChevronRight size={16} strokeWidth={1.75} /></IconButton>
        </div>
        <div role="grid" aria-label={`Tháng ${format(shown, 'M')} năm ${format(shown, 'yyyy')}`} aria-busy={events.isPending || undefined} className="grid grid-cols-7 text-center gap-y-1">
          {weeks[0]!.map((d) => <span key={d.getDay()} className="h-6 leading-6 text-2xs font-semibold text-ink-muted" aria-hidden="true">{weekdayShort(d)}</span>)}
          {weeks.flat().map((d) => {
            const n = onDay(list, d).length
            const today = isSameDay(d, now)
            const chosen = !!day && isSameDay(d, day)
            return (
              <Pressable
                key={d.toISOString()}
                aria-label={n > 0 ? `${formatDayMonth(d)}, ${n} sự kiện` : formatDayMonth(d)}
                aria-current={today ? 'date' : undefined}
                aria-pressed={chosen}
                onClick={() => setPicked(d)}
                className={`grid justify-items-center content-center gap-0.5 h-11 rounded-lg text-sm tnum
                  ${chosen ? 'bg-accent-wash text-ink' : 'hover:bg-hover'} ${isSameMonth(d, shown) ? 'text-ink' : 'text-ink-subtle'}`}
              >
                <span className={`grid place-items-center size-6 rounded-full ${today ? 'bg-accent text-on-accent font-semibold' : ''}`}>{format(d, 'd')}</span>
                <span className="flex gap-0.5 h-1.5" aria-hidden="true">
                  {Array.from({ length: Math.min(n, MAX_DOTS) }, (_, i) => <span key={i} className="size-1.5 rounded-full bg-(--c)" />)}
                </span>
              </Pressable>
            )
          })}
        </div>
        {day && (
          <ul aria-label={`Sự kiện ngày ${formatDayMonth(day)}`} className="m-0 p-0 list-none grid gap-0.5 border-t border-line pt-2">
            {onDay(list, day).length === 0 && <li className="text-sm text-ink-muted py-1">Không có sự kiện nào.</li>}
            {onDay(list, day).map((o) => {
              const { start, end } = spanOf(o)
              return (
                <li key={instanceKey(o)}>
                  <Pressable onClick={() => openInCalendar(start)} className="w-full grid grid-cols-[8rem_1fr] gap-2.5 items-center min-h-9 px-2 rounded-lg hover:bg-hover">
                    <span className="text-sm text-ink-muted tnum">{o.allDay ? 'Cả ngày' : `${formatClock(start)} – ${formatClock(end)}`}</span>
                    <span className="truncate text-sm font-semibold text-ink">{o.title}</span>
                  </Pressable>
                </li>
              )
            })}
          </ul>
        )}
      </div>
    )
  }

  return (
    <div className="flex-1 min-h-0 flex flex-col">
      {controls}
      {body}
    </div>
  )
}
