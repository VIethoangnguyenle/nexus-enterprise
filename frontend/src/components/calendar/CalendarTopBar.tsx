import { useRef, useState } from 'react'
import { Calendar, ChevronDown, ChevronLeft, ChevronRight } from 'lucide-react'
import { CALENDAR_VIEWS, VIEW_LABEL, titleOf, weekLabel, type CalendarView } from '../../lib/calendar-format'
import { Button, Heading, IconButton, MenuItem, Popover } from '../primitives'

/** The key that switches to each view, beside its name in the menu (DESIGN.md §6, Phím tắt). */
export const VIEW_KEY: Record<CalendarView, string> = { day: 'D', week: 'W', month: 'M', agenda: 'A', '3day': 'X' }

/** Menu order: the four of the design, then the 3-day view the phone also offers. */
const MENU_ORDER: CalendarView[] = ['day', 'week', 'month', 'agenda', '3day']

interface CalendarTopBarProps {
  view: CalendarView
  date: Date
  onView: (v: CalendarView) => void
  onToday: () => void
  /** One step back (-1) or forward (+1) in the unit of the view. */
  onStep: (steps: -1 | 1) => void
}

/**
 * The wide bar over the calendar (DESIGN.md §6 Lịch): Hôm nay, the two arrows, the heading of the range and
 * the view menu. No primary button here: Tạo lives in the side column.
 */
export function CalendarTopBar({ view, date, onView, onToday, onStep }: CalendarTopBarProps) {
  const [open, setOpen] = useState(false)
  const menuRef = useRef<HTMLButtonElement>(null)
  const unit = view === 'month' ? 'tháng' : view === 'day' ? 'ngày' : view === 'agenda' ? '30 ngày' : view === '3day' ? '3 ngày' : 'tuần'
  return (
    <header className="flex items-center gap-1.5 px-4 pt-3 pb-2 min-w-0">
      <span className="hidden lg:flex items-center gap-2.5 w-56 shrink-0 mr-2">
        <Calendar size={26} strokeWidth={1.75} className="text-accent" aria-hidden="true" />
        <Heading as="h1" className="text-xl! font-normal! text-ink-muted! font-body!">Lịch</Heading>
      </span>
      <Button variant="secondary" size="sm" className="rounded-full h-9 px-4" onClick={onToday}>Hôm nay</Button>
      <IconButton size="lg" shape="round" aria-label={`Trước, ${unit}`} onClick={() => onStep(-1)}>
        <ChevronLeft size={20} strokeWidth={1.75} />
      </IconButton>
      <IconButton size="lg" shape="round" aria-label={`Sau, ${unit}`} onClick={() => onStep(1)}>
        <ChevronRight size={20} strokeWidth={1.75} />
      </IconButton>
      <Heading as="h2" className="ml-1 min-w-0 truncate text-xl! font-normal! font-body!">
        {titleOf(view, date)}
        {(view === 'week' || view === '3day') && <span className="ml-2 text-sm text-ink-muted">{weekLabel(date)}</span>}
      </Heading>
      <span className="flex-1" />
      <Button
        ref={menuRef}
        variant="secondary"
        size="sm"
        className="rounded-full h-9 pl-4 pr-3"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
      >
        {VIEW_LABEL[view]}
        <ChevronDown size={16} strokeWidth={1.75} aria-hidden="true" />
      </Button>
      <Popover open={open} onClose={() => setOpen(false)} anchorRef={menuRef} role="menu" label="Kiểu xem" placement="bottom-end" className="w-60">
        {MENU_ORDER.filter((v) => (CALENDAR_VIEWS as readonly string[]).includes(v)).map((v) => (
          <MenuItem
            key={v}
            role="menuitemradio"
            aria-checked={v === view}
            onClick={() => onView(v)}
            className={v === view ? 'bg-accent-wash' : ''}
          >
            <span className="flex-1">{VIEW_LABEL[v]}</span>
            <kbd className="font-body text-xs text-ink-muted">{VIEW_KEY[v]}</kbd>
          </MenuItem>
        ))}
      </Popover>
    </header>
  )
}
