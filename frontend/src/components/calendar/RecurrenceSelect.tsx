import { useState } from 'react'
import { Repeat } from 'lucide-react'
import { presetIdOf, recurrencePresets, recurrenceLabel, type PresetId } from '../../lib/calendar-recurrence'
import { Button, FieldSelect } from '../primitives'
import { RecurrenceDialog } from './RecurrenceDialog'

interface Props {
  /** The day the event is on; the choices are worked out from it. */
  date: Date
  rrule: string | null
  onChange: (rrule: string | null) => void
}

/** The Lặp lại select (DESIGN.md §6): ready-made choices from the day, and "Tùy chọn…" for the dialog. */
export function RecurrenceSelect({ date, rrule, onChange }: Props) {
  const [custom, setCustom] = useState(false)
  const presets = recurrencePresets(date)
  const current = presetIdOf(rrule, date)
  // A rule the choices do not name (set elsewhere, or made in the dialog) is listed as itself.
  const ownLabel = current === 'custom' ? recurrenceLabel(rrule, date) : null

  const pick = (id: PresetId) => {
    if (id === 'custom') setCustom(true)
    else onChange(presets.find((p) => p.id === id)!.rrule)
  }

  return (
    <>
      <span className="inline-flex items-center gap-1.5 min-w-0 max-w-full">
        <Repeat size={16} strokeWidth={1.75} className="text-ink-muted shrink-0" aria-hidden="true" />
        <FieldSelect aria-label="Lặp lại" className="min-w-0 truncate" value={current} onChange={(e) => pick(e.target.value as PresetId)}>
          {presets.map((p) => (
            <option key={p.id} value={p.id}>{p.id === 'custom' && ownLabel ? ownLabel : p.label}</option>
          ))}
        </FieldSelect>
        {current === 'custom' && (
          <Button type="button" variant="ghost" size="sm" className="rounded-full" onClick={() => setCustom(true)}>Sửa quy tắc</Button>
        )}
      </span>
      <RecurrenceDialog
        open={custom}
        date={date}
        rrule={rrule}
        onClose={() => setCustom(false)}
        onDone={(r) => {
          setCustom(false)
          onChange(r)
        }}
      />
    </>
  )
}
