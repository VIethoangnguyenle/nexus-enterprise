import { useEffect, useMemo, useRef, useState, type RefObject } from 'react'
import { addDays, startOfDay } from 'date-fns'
import type { CalendarSummary, EventInput, EventKind } from '../../api/calendar'
import { formatWhen, wireOf } from '../../lib/calendar-format'
import { Button, FilterChip, Popover, Select, Text, TextField } from '../primitives'

export interface QuickDraft {
  start: Date
  end: Date
  allDay: boolean
}

const KINDS: { kind: EventKind; label: string; hint: string }[] = [
  { kind: 'event', label: 'Sự kiện', hint: '' },
  { kind: 'focus', label: 'Giờ làm việc riêng', hint: 'Người khác thấy bạn bận.' },
  { kind: 'ooo', label: 'Vắng mặt', hint: 'Cả ngày hoặc nhiều ngày.' },
]

interface QuickCreateProps {
  draft: QuickDraft | null
  anchorRef: RefObject<HTMLElement | null>
  /** Calendars the person may write to; the first personal one of theirs is offered first. */
  calendars: CalendarSummary[]
  /** Resolves once the event is saved (or refused); the popover closes on success only. */
  onSave: (input: EventInput) => Promise<boolean>
  onCancel: () => void
  /** Tùy chọn khác: carry the draft, the title typed and the calendar to the full editor. */
  onMore?: (draft: QuickDraft & { title: string; calendarId: string }) => void
}

/** Writable calendars, the person's own personal one first. */
export function defaultCalendar(calendars: CalendarSummary[]): CalendarSummary | undefined {
  return calendars.find((c) => c.kind === 'personal' && c.isMine) ?? calendars[0]
}

/**
 * The small card that opens on a drag or a click on empty time (DESIGN.md §6, Tạo nhanh): a title, the kind,
 * the calendar when there is a choice, and Lưu. Esc cancels; a click outside cancels while the title is empty
 * and otherwise keeps the card open so nothing typed is lost.
 */
export function QuickCreate({ draft, anchorRef, calendars, onSave, onCancel, onMore }: QuickCreateProps) {
  // Whether the title has text; a click outside must not throw that away.
  const dirty = useRef(false)
  return (
    <Popover
      open={!!draft}
      onClose={() => {
        if (!dirty.current) onCancel()
      }}
      anchorRef={anchorRef}
      label="Tạo nhanh"
      className="w-90 max-w-screen"
    >
      {draft && (
        <QuickForm
          key={draft.start.getTime() + ':' + draft.end.getTime()}
          draft={draft}
          calendars={calendars}
          dirty={dirty}
          onSave={onSave}
          onCancel={onCancel}
          onMore={onMore}
        />
      )}
    </Popover>
  )
}

function QuickForm({ draft, calendars, dirty, onSave, onCancel, onMore }: Omit<QuickCreateProps, 'anchorRef' | 'draft'> & { draft: QuickDraft; dirty: RefObject<boolean> }) {
  const [title, setTitle] = useState('')
  const [kind, setKind] = useState<EventKind>('event')
  const [calendarId, setCalendarId] = useState(() => defaultCalendar(calendars)?.id ?? '')
  const [saving, setSaving] = useState(false)
  const ref = useRef<HTMLFormElement>(null)
  useEffect(() => {
    dirty.current = false
    return () => {
      dirty.current = false
    }
  }, [dirty])

  // An absence covers whole days: the days the drag touched, the end exclusive.
  const span = useMemo(() => {
    if (kind === 'ooo' || draft.allDay) {
      const first = startOfDay(draft.start)
      const last = startOfDay(new Date(Math.max(draft.start.getTime(), draft.end.getTime() - 1)))
      return { start: first, end: addDays(last, 1), allDay: true }
    }
    return { start: draft.start, end: draft.end, allDay: false }
  }, [draft, kind])

  const submit = async () => {
    if (saving || !calendarId) return
    setSaving(true)
    const ok = await onSave({
      calendarId,
      title: title.trim() || (kind === 'focus' ? 'Giờ làm việc riêng' : kind === 'ooo' ? 'Vắng mặt' : '(Không có tiêu đề)'),
      start: wireOf(span.start, span.allDay),
      end: wireOf(span.end, span.allDay),
      allDay: span.allDay,
      tzid: Intl.DateTimeFormat().resolvedOptions().timeZone,
      kind,
      transparency: 'busy',
      visibility: 'default',
      attendees: [],
      guestsCanInvite: false,
      guestsCanSeeList: true,
      hasCall: false,
    })
    if (ok) dirty.current = false
    else setSaving(false)
  }

  return (
    <form
      ref={ref}
      className="grid gap-3 p-2.5"
      onSubmit={(e) => {
        e.preventDefault()
        void submit()
      }}
      onKeyDown={(e) => {
        if (e.key === 'Escape') onCancel()
      }}
    >
      <TextField
        label="Tiêu đề"
        value={title}
        onChange={(e) => {
          setTitle(e.target.value)
          dirty.current = e.target.value.trim().length > 0
        }}
        placeholder="Thêm tiêu đề"
        autoFocus
        autoComplete="off"
      />
      <div role="group" aria-label="Loại" className="flex flex-wrap gap-1.5">
        {KINDS.map((k) => (
          <FilterChip key={k.kind} pressed={kind === k.kind} onClick={() => setKind(k.kind)}>{k.label}</FilterChip>
        ))}
      </div>
      <Text variant="small" muted>{formatWhen(span.start, span.end, span.allDay)}</Text>
      {KINDS.find((k) => k.kind === kind)?.hint && (
        <Text variant="caption" muted>{KINDS.find((k) => k.kind === kind)!.hint}</Text>
      )}
      {calendars.length > 1 && (
        <label className="grid gap-1 text-xs font-semibold text-ink-muted">
          Lịch
          <CalendarSelect calendars={calendars} value={calendarId} onChange={setCalendarId} />
        </label>
      )}
      <div className="flex justify-end gap-2">
        {onMore && (
          <Button
            type="button" variant="ghost" size="sm" className="rounded-full mr-auto"
            onClick={() => onMore({ start: span.start, end: span.end, allDay: span.allDay, title: title.trim(), calendarId })}
          >
            Tùy chọn khác
          </Button>
        )}
        <Button type="button" variant="soft" size="sm" className="rounded-full" onClick={onCancel}>Huỷ</Button>
        <Button type="submit" size="sm" className="rounded-full" loading={saving} disabled={!calendarId}>Lưu</Button>
      </div>
    </form>
  )
}

function CalendarSelect({ calendars, value, onChange }: { calendars: CalendarSummary[]; value: string; onChange: (id: string) => void }) {
  return (
    <Select value={value} onChange={(e) => onChange(e.target.value)}>
      {calendars.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
    </Select>
  )
}
