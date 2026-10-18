import { useState } from 'react'
import { motion } from 'motion/react'
import { addDays, addMonths, format, isSameDay, startOfWeek } from 'date-fns'
import { CalendarCheck, ChevronDown, MoreHorizontal, PanelRight, Settings } from 'lucide-react'
import { VIEW_LABEL, formatDayMonth, weekOptions, weekdayShort, type CalendarView, type WeekStart } from '../../lib/calendar-format'
import { Button, Heading, IconButton, Pressable } from '../primitives'
import { useSwipeStep } from '../../hooks/useSwipeStep'
import { NavSheet, SheetItem } from '../spaces/NavSheet'
import { ModuleMenu } from '../patterns/ModuleMenu'
import { useSettingsNav } from '../settings/settings-nav'
import { MiniMonth } from './MiniMonth'

/** The views a phone offers, in the order of its sheet (DESIGN.md §6, Điện thoại). */
const PHONE_VIEWS: CalendarView[] = ['agenda', 'day', '3day', 'month']

interface StripProps {
  date: Date
  now: Date
  hasEvents: (day: Date) => boolean
  onPick: (day: Date) => void
  onStep: (steps: -1 | 1) => void
  /** The first cell: Monday unless the person chose Sunday. */
  weekStart?: WeekStart
}

/** Seven cells of the week with a dot under the days that have events; a swipe changes the week (`useSwipeStep`). */
export function WeekStrip({ date, now, hasEvents, onPick, onStep, weekStart = 1 }: StripProps) {
  const first = startOfWeek(date, weekOptions(weekStart))
  const { bind, x } = useSwipeStep(onStep)
  return (
    <motion.div
      role="group"
      aria-label="Tuần đang xem"
      className="grid grid-cols-7 gap-1 px-2 pb-2 touch-pan-y"
      data-testid="week-strip"
      style={{ x }}
      {...bind}
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
    </motion.div>
  )
}

/** The month folded open under "Tháng 10 ▾"; a swipe changes the month, as on the month grid. */
function MonthFold({ date, now, onDate, onPicked, weekStart }: {
  date: Date; now: Date; onDate: (d: Date) => void; onPicked: () => void; weekStart?: WeekStart
}) {
  const { bind, x } = useSwipeStep((step) => onDate(addMonths(date, step)))
  return (
    <motion.div className="px-4 pb-2 touch-pan-y" data-testid="month-fold" style={{ x }} {...bind}>
      <MiniMonth date={date} now={now} weekStart={weekStart} onPick={(d) => { onDate(d); onPicked() }} />
    </motion.div>
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
  /** The first day of the week strip and the folded month: Monday unless the person chose Sunday. */
  weekStart?: WeekStart
}

/**
 * The header of the calendar below 1024 (design/mockups/calendar.html §11, §12): "Tháng 10 ▾" that folds the
 * month open, Hôm nay, the view sheet (⋯ on a phone, a named button on a tablet) and, in the agenda and the
 * day, the week strip.
 */
export function PhoneCalendarHeader({ view, date, now, tablet, hasEvents, onView, onDate, onStep, onToday, onCalendars, weekStart }: Props) {
  const [month, setMonth] = useState(false)
  const [sheet, setSheet] = useState(false)
  const settings = useSettingsNav()
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
            <ModuleMenu section="calendar" label="Lịch" />
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
      {month && <MonthFold date={date} now={now} weekStart={weekStart} onDate={onDate} onPicked={() => setMonth(false)} />}
      {(view === 'agenda' || view === 'day') && (
        <WeekStrip date={date} now={now} weekStart={weekStart} hasEvents={hasEvents} onPick={onDate} onStep={onStep} />
      )}
      {sheet && (
        <NavSheet label="Kiểu xem" onClose={() => setSheet(false)}>
          {/* The ⋯ of a module starts with its settings (DESIGN.md §5, Hàng tiêu đề của phân hệ). */}
          <SheetItem icon={<Settings size={20} strokeWidth={1.75} />} onClick={() => { settings.open('calendar'); setSheet(false) }}>
            Cài đặt Lịch
          </SheetItem>
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
