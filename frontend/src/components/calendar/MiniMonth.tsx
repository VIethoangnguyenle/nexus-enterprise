import { useState } from 'react'
import { addMonths, format, isSameDay, isSameMonth, startOfWeek } from 'date-fns'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { WEEK_OPTIONS, formatDayMonth, monthWeeks, weekdayShort } from '../../lib/calendar-format'
import { IconButton, Pressable } from '../primitives'

interface MiniMonthProps {
  /** The day the main view is on; its week is washed. */
  date: Date
  now: Date
  onPick: (day: Date) => void
}

/** The small month of the side column: today in accent, the viewed week in wash. */
export function MiniMonth({ date, now, onPick }: MiniMonthProps) {
  // The month shown follows the main view, and can be paged without moving it.
  const [shown, setShown] = useState(date)
  const [followed, setFollowed] = useState(date)
  if (!isSameMonth(followed, date) || !isSameDay(followed, date)) {
    setFollowed(date)
    if (!isSameMonth(shown, date)) setShown(date)
  }
  const weeks = monthWeeks(shown)
  const week0 = startOfWeek(date, WEEK_OPTIONS)

  return (
    <section aria-label="Lịch tháng nhỏ" className="grid gap-1 px-1">
      <div className="flex items-center pl-1.5">
        <span className="flex-1 text-sm font-semibold text-ink">{`Tháng ${format(shown, 'M')} năm ${format(shown, 'yyyy')}`}</span>
        <IconButton size="md" aria-label="Tháng trước" onClick={() => setShown(addMonths(shown, -1))}>
          <ChevronLeft size={16} strokeWidth={1.75} />
        </IconButton>
        <IconButton size="md" aria-label="Tháng sau" onClick={() => setShown(addMonths(shown, 1))}>
          <ChevronRight size={16} strokeWidth={1.75} />
        </IconButton>
      </div>
      <div className="grid grid-cols-7 text-center text-2xs font-semibold text-ink-muted gap-y-0.5" role="grid" aria-label={`Tháng ${format(shown, 'M')}`}>
        {weeks[0]!.map((d) => <span key={d.getDay()} className="h-6 leading-6" aria-hidden="true">{weekdayShort(d)}</span>)}
        {weeks.flat().map((d) => {
          const today = isSameDay(d, now)
          const inWeek = d.getTime() >= week0.getTime() && d.getTime() < week0.getTime() + 7 * 86_400_000
          return (
            <Pressable
              key={d.toISOString()}
              onClick={() => onPick(d)}
              aria-label={formatDayMonth(d)}
              aria-current={today ? 'date' : undefined}
              className={`mx-auto size-7 rounded-full text-xs tnum
                ${today ? 'bg-accent text-on-accent font-semibold' : inWeek ? 'bg-hover text-ink' : isSameMonth(d, shown) ? 'text-ink hover:bg-hover' : 'text-ink-subtle hover:bg-hover'}`}
            >
              {format(d, 'd')}
            </Pressable>
          )
        })}
      </div>
    </section>
  )
}
