import { useRef, useState, type TouchEvent } from 'react'
import { addDays, format, isSameDay, startOfWeek } from 'date-fns'
import { CalendarCheck, ChevronDown, MoreHorizontal, PanelRight } from 'lucide-react'
import { VIEW_LABEL, WEEK_OPTIONS, formatDayMonth, weekdayShort, type CalendarView } from '../../lib/calendar-format'
import { Button, Heading, IconButton, Pressable } from '../primitives'
import { NavSheet, SheetItem } from '../spaces/NavSheet'
import { MiniMonth } from './MiniMonth'

/** The views a phone offers, in the order of its sheet (DESIGN.md §6, Điện thoại). */
const PHONE_VIEWS: CalendarView[] = ['agenda', 'day', '3day', 'month']
const SWIPE_PX = 48

interface StripProps {
  date: Date
  now: Date
  hasEvents: (day: Date) => boolean
  onPick: (day: Date) => void
  onStep: (steps: -1 | 1) => void
}

/** Seven cells of the week with a dot under the days that have events; a swipe changes the week. */
export function WeekStrip({ date, now, hasEvents, onPick, onStep }: StripProps) {
  const first = startOfWeek(date, WEEK_OPTIONS)
  const x0 = useRef<number | null>(null)
  const onEnd = (e: TouchEvent) => {
    const start = x0.current
    x0.current = null
    const dx = start === null ? 0 : (e.changedTouches[0]?.clientX ?? start) - start
    if (Math.abs(dx) >= SWIPE_PX) onStep(dx < 0 ? 1 : -1)
  }
  return (
    <div
      role="group"
      aria-label="Tuần đang xem"
      className="grid grid-cols-7 gap-1 px-2 pb-2"
      data-testid="week-strip"
      onTouchStart={(e) => { x0.current = e.touches[0]?.clientX ?? null }}
      onTouchEnd={onEnd}
    >
      {Array.from({ length: 7 }, (_, i) => addDays(first, i)).map((d) => {
        const selected = isSameDay(d, date)
        const today = isSameDay(d, now)
        return (
          <Pressable
            key={d.toISOString()}
            aria-label={`Chọn ngày ${formatDayMonth(d)}`}
            aria-pressed={selected}
            aria-current={today ? 'date' : undefined}
            onClick={() => onPick(d)}
            className="grid justify-items-center gap-0.5 min-h-14 pt-1.5 rounded-xl"
          >
            <span className="text-xs text-ink-muted">{weekdayShort(d)}</span>
            <span className={`grid place-items-center size-9 rounded-full text-16 tnum
              ${selected ? 'bg-accent text-on-accent font-semibold' : today ? 'ring-2 ring-inset ring-accent text-ink font-semibold' : 'text-ink'}`}>
              {format(d, 'd')}
            </span>
            <span aria-hidden="true" className={`size-1 rounded-full ${hasEvents(d) ? 'bg-ink-muted' : 'bg-transparent'}`} />
          </Pressable>
        )
      })}
    </div>
  )
}

interface Props {
  view: CalendarView
  date: Date
  now: Date
  /** A tablet has its own words ("Hôm nay", the view name) and a button for the calendar list. */
  tablet: boolean
  hasEvents: (day: Date) => boolean
  onView: (v: CalendarView) => void
  onDate: (d: Date) => void
  onStep: (steps: -1 | 1) => void
  onToday: () => void
  /** Tablet: opens the sheet with the small month and the calendars. */
  onCalendars: () => void
}

/**
 * The header of the calendar below 1024 (design/mockups/calendar.html §11, §12): "Tháng 10 ▾" that folds the
 * month open, Hôm nay, the view sheet (⋯ on a phone, a named button on a tablet) and, in the agenda and the
 * day, the week strip.
 */
export function PhoneCalendarHeader({ view, date, now, tablet, hasEvents, onView, onDate, onStep, onToday, onCalendars }: Props) {
  const [month, setMonth] = useState(false)
  const [sheet, setSheet] = useState(false)
  const label = `Tháng ${format(date, 'M')}`
  return (
    <div>
      <div className="flex items-center gap-1 px-4 pt-1 pb-1">
        <Button
          type="button"
          variant="ghost"
          className="rounded-full gap-1 h-11 px-2 -ml-2 text-[1.625rem]! font-display font-bold! text-ink!"
          aria-expanded={month}
          aria-label={`${label} năm ${format(date, 'yyyy')}, mở lịch tháng`}
          onClick={() => setMonth((v) => !v)}
        >
          <Heading as="h1" className="text-[1.625rem]! m-0">{label}</Heading>
          <ChevronDown size={20} strokeWidth={1.75} aria-hidden="true" />
        </Button>
        <span className="flex-1" />
        {tablet ? (
          <>
            <Button type="button" variant="soft" className="rounded-full h-11 px-4" onClick={onToday}>Hôm nay</Button>
            <Button type="button" variant="soft" className="rounded-full h-11 pl-4 pr-3 gap-1" aria-haspopup="dialog" onClick={() => setSheet(true)}>
              {VIEW_LABEL[view]}<ChevronDown size={16} strokeWidth={1.75} aria-hidden="true" />
            </Button>
            <IconButton className="size-11!" aria-label="Lịch đang hiện" onClick={onCalendars}><PanelRight size={20} strokeWidth={1.75} /></IconButton>
          </>
        ) : (
          <>
            <IconButton className="size-11!" aria-label="Hôm nay" onClick={onToday}><CalendarCheck size={20} strokeWidth={1.75} /></IconButton>
            <IconButton className="size-11!" aria-label="Kiểu xem" aria-haspopup="dialog" onClick={() => setSheet(true)}>
              <MoreHorizontal size={20} strokeWidth={1.75} />
            </IconButton>
          </>
        )}
      </div>
      {month && (
        <div className="px-4 pb-2" data-testid="month-fold">
          <MiniMonth date={date} now={now} onPick={(d) => { onDate(d); setMonth(false) }} />
        </div>
      )}
      {(view === 'agenda' || view === 'day') && <WeekStrip date={date} now={now} hasEvents={hasEvents} onPick={onDate} onStep={onStep} />}
      {sheet && (
        <NavSheet label="Kiểu xem" onClose={() => setSheet(false)}>
          <div role="radiogroup" aria-label="Kiểu xem" className="grid gap-1">
            {PHONE_VIEWS.map((v) => (
              <SheetItem key={v} checked={v === view} onClick={() => { onView(v); setSheet(false) }}>{VIEW_LABEL[v]}</SheetItem>
            ))}
          </div>
        </NavSheet>
      )}
    </div>
  )
}
