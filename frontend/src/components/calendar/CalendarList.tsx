import { useRef, useState } from 'react'
import { Check, MoreHorizontal, Share2, X } from 'lucide-react'
import type { CalendarSummary } from '../../api/calendar'
import { ACCENT_COLOR, CALENDAR_HUES, colorName, colorStyle } from '../../lib/calendar-color'
import { Heading, IconButton, MenuItem, Popover, Pressable, Text } from '../primitives'

interface CalendarListProps {
  calendars: CalendarSummary[]
  onToggle: (calendar: CalendarSummary, visible: boolean) => void
  onColor: (calendar: CalendarSummary, color: number) => void
  /** Opens Chia sẻ lịch; offered only for a calendar the person manages. */
  onShare?: (calendar: CalendarSummary) => void
  /** Takes a calendar that somebody else owns out of the list. */
  onRemove?: (calendar: CalendarSummary) => void
}

/** Whether the person may change who sees this calendar: their own, or one they were given management of. */
const canShare = (c: CalendarSummary) => c.kind === 'personal' && c.access === 'manage'

/** Lịch của tôi holds the person's own, the workspace's and the groups'; Lịch khác what others share or the person added. */
export const isMineGroup = (c: CalendarSummary) => c.isMine || c.kind !== 'personal'

function Row({ calendar, onToggle, onColor, onShare, onRemove }: { calendar: CalendarSummary } & Omit<CalendarListProps, 'calendars'>) {
  const [open, setOpen] = useState(false)
  const moreRef = useRef<HTMLButtonElement>(null)
  return (
    <li className="flex items-center gap-2.5 h-8.5 pl-2 pr-1.5 rounded-lg hover:bg-hover" style={colorStyle(calendar.color)}>
      <Pressable
        role="checkbox"
        aria-checked={calendar.visible}
        aria-label={calendar.name}
        onClick={() => onToggle(calendar, !calendar.visible)}
        className="flex-1 min-w-0 flex items-center gap-2.5 h-8.5 text-sm font-medium text-ink"
      >
        <span
          aria-hidden="true"
          className={`size-4.5 rounded-sm shrink-0 grid place-items-center ring-2 ring-inset ring-(--c)
            ${calendar.visible ? 'bg-(--c) text-on-person' : 'bg-transparent'}`}
        >
          {calendar.visible && <Check size={12} strokeWidth={3} />}
        </span>
        <span className="truncate">{calendar.name}</span>
      </Pressable>
      <IconButton
        ref={moreRef}
        size="md"
        aria-label={`Tuỳ chọn của lịch ${calendar.name}`}
        aria-haspopup="dialog"
        onClick={() => setOpen((v) => !v)}
      >
        <MoreHorizontal size={16} strokeWidth={1.75} />
      </IconButton>
      <Popover open={open} onClose={() => setOpen(false)} anchorRef={moreRef} label={`Màu của lịch ${calendar.name}`} placement="bottom-end" className="w-60">
        <Text variant="caption" muted className="block px-2 pt-1 pb-2 font-semibold">Màu của lịch</Text>
        <div className="flex flex-wrap gap-2 px-2 pb-2">
          {[...CALENDAR_HUES, ACCENT_COLOR].map((hue) => (
            <Pressable
              key={hue}
              aria-label={colorName(hue)}
              aria-pressed={calendar.color === hue}
              onClick={() => { onColor(calendar, hue); setOpen(false) }}
              style={colorStyle(hue)}
              className="size-7 rounded-full bg-(--c) text-on-person grid place-items-center"
            >
              {calendar.color === hue && <Check size={14} strokeWidth={3} />}
            </Pressable>
          ))}
        </div>
        {onShare && canShare(calendar) && (
          <MenuItem icon={<Share2 size={16} strokeWidth={1.75} />} onClick={() => onShare(calendar)}>Chia sẻ</MenuItem>
        )}
        {onRemove && !calendar.isMine && calendar.kind === 'personal' && (
          <MenuItem icon={<X size={16} strokeWidth={1.75} />} onClick={() => onRemove(calendar)}>Bỏ khỏi danh sách</MenuItem>
        )}
      </Popover>
    </li>
  )
}

function Group({ title, calendars, ...rest }: { title: string } & CalendarListProps) {
  if (calendars.length === 0) return null
  return (
    <section aria-label={title} className="grid gap-0.5">
      <Heading as="h3" className="px-2 pb-1 text-sm! font-semibold! font-body!">{title}</Heading>
      <ul className="m-0 p-0 list-none grid gap-px">
        {calendars.map((c) => <Row key={c.id} calendar={c} {...rest} />)}
      </ul>
    </section>
  )
}

/** The side list of calendars (DESIGN.md §6 Lịch): a coloured tick shows or hides each on the grid. */
export function CalendarList({ calendars, ...rest }: CalendarListProps) {
  return (
    <div className="grid gap-4.5">
      <Group title="Lịch của tôi" calendars={calendars.filter(isMineGroup)} {...rest} />
      <Group title="Lịch khác" calendars={calendars.filter((c) => !isMineGroup(c))} {...rest} />
    </div>
  )
}
