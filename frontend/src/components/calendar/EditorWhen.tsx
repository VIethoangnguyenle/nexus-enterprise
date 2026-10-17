import { useState } from 'react'
import { Globe } from 'lucide-react'
import type { EventKind } from '../../api/calendar'
import {
  durationLabel, formDuration, withStart, type EditorForm, type FormErrors,
} from '../../lib/calendar-editor'
import { parseDateKey } from '../../lib/calendar-format'
import { Button, Checkbox, FilterChip, FieldInput, FieldSelect } from '../primitives'
import { zoneLabel, zoneNames } from './editor-ui'
import { RecurrenceSelect } from './RecurrenceSelect'

const KINDS: { kind: EventKind; label: string }[] = [
  { kind: 'event', label: 'Sự kiện' },
  { kind: 'focus', label: 'Giờ làm việc riêng' },
  { kind: 'ooo', label: 'Vắng mặt' },
]

interface Props {
  form: EditorForm
  onChange: (next: EditorForm) => void
  errors: FormErrors
  /** Which errors may show: the ones for fields the person has left, or all after Lưu. */
  show: { start: boolean; end: boolean }
  onBlurField: (field: 'start' | 'end') => void
}

/**
 * The time of an event (design/mockups/calendar.html §3): the kind, the dates and times, the length beside
 * the end ("1 giờ"), all day, the repeat select worked out from the day, and the zone, hidden until asked for.
 * Changing the start keeps the length. Errors show once the field has been left.
 */
export function EditorWhen({ form, onChange, errors, show, onBlurField }: Props) {
  const [zone, setZone] = useState(false)
  const date = parseDateKey(form.startDate) ?? new Date()
  const minutes = formDuration(form)
  const endError = show.end ? errors.end : undefined
  const startError = show.start ? errors.start : undefined

  return (
    <div className="grid gap-2.5">
      <div role="group" aria-label="Loại" className="flex flex-wrap gap-1.5">
        {KINDS.map((k) => (
          <FilterChip
            key={k.kind}
            pressed={form.kind === k.kind}
            onClick={() => onChange({ ...form, kind: k.kind, ...(k.kind === 'ooo' ? { allDay: true } : {}) })}
          >
            {k.label}
          </FilterChip>
        ))}
      </div>

      <div className="flex flex-wrap items-center gap-2 text-sm text-ink">
        <FieldInput
          type="date" aria-label="Ngày bắt đầu" value={form.startDate} invalid={!!startError}
          onChange={(e) => onChange(withStart(form, { startDate: e.target.value }))} onBlur={() => onBlurField('start')}
        />
        {!form.allDay && (
          <>
            <FieldInput
              type="time" step={900} aria-label="Giờ bắt đầu" value={form.startTime} invalid={!!startError}
              onChange={(e) => onChange(withStart(form, { startTime: e.target.value }))} onBlur={() => onBlurField('start')}
            />
            <span className="text-ink-muted">đến</span>
            <FieldInput
              type="time" step={900} aria-label="Giờ kết thúc" value={form.endTime} invalid={!!endError}
              onChange={(e) => onChange({ ...form, endTime: e.target.value })} onBlur={() => onBlurField('end')}
            />
          </>
        )}
        {form.allDay && <span className="text-ink-muted">đến</span>}
        <FieldInput
          type="date" aria-label="Ngày kết thúc" value={form.endDate} invalid={!!endError}
          onChange={(e) => onChange({ ...form, endDate: e.target.value })} onBlur={() => onBlurField('end')}
        />
        {!form.allDay && minutes > 0 && <span className="text-ink-muted tnum">{durationLabel(minutes)}</span>}
        <Button type="button" variant="ghost" size="sm" className="rounded-full" aria-expanded={zone} onClick={() => setZone((v) => !v)}>
          <Globe size={16} strokeWidth={1.75} aria-hidden="true" />Múi giờ
        </Button>
      </div>
      {(startError || endError) && (
        <p role="alert" className="m-0 text-xs font-medium text-danger">{endError ?? startError}</p>
      )}

      {zone && (
        <label className="flex items-center gap-2 text-sm text-ink">
          Múi giờ của sự kiện
          <FieldSelect value={form.tzid} onChange={(e) => onChange({ ...form, tzid: e.target.value })}>
            {zoneNames(form.tzid).map((z) => <option key={z} value={z}>{zoneLabel(z)}</option>)}
          </FieldSelect>
        </label>
      )}

      <div className="flex flex-wrap items-center gap-3 text-sm text-ink">
        <label className="flex items-center gap-2 cursor-pointer">
          <Checkbox label="Cả ngày" checked={form.allDay} onChange={(e) => onChange({ ...form, allDay: e.target.checked })} />
          Cả ngày
        </label>
        <RecurrenceSelect date={date} rrule={form.rrule} onChange={(rrule) => onChange({ ...form, rrule })} />
      </div>
    </div>
  )
}
