import { useState } from 'react'
import { Check } from 'lucide-react'
import type { CalendarSettings as CalendarOptions } from '../../../api/settings'
import { minutesOfClock } from '../../../lib/calendar-findtime'
import { Select } from '../../primitives'
import { SettingsGroup, SettingsPage, SettingsRow } from '../SettingsGroup'

/** Every half hour of the day: the steps a working day starts and ends on. */
const HALF_HOURS = Array.from({ length: 48 }, (_, i) => `${String(Math.floor(i / 2)).padStart(2, '0')}:${i % 2 ? '30' : '00'}`)

/** A saved time off the half-hour steps still reads as itself. */
const timesWith = (t: string) => (!t || HALF_HOURS.includes(t) ? HALF_HOURS : [...HALF_HOURS, t].sort())

const HOURS_WRONG = 'Giờ kết thúc phải sau giờ bắt đầu.'
const HOURS_USE = 'Tìm giờ chỉ gợi ý trong giờ làm việc của mọi người.'

type Part = 'workStart' | 'workEnd'
type Hours = Record<Part, string>

interface WorkHoursProps {
  settings: CalendarOptions
  /** Saves the patch; the caller's hook puts the old value back if the server refuses. */
  save: (patch: CalendarOptions) => unknown
}

const endsAfterStart = (h: Hours) => minutesOfClock(h.workEnd, 0) > minutesOfClock(h.workStart, 0)

/** "08:00–17:30", the value on the phone's row. */
export const workHoursLabel = (s: CalendarOptions) => (s.workStart && s.workEnd ? `${s.workStart}–${s.workEnd}` : undefined)

/**
 * The working day being picked. A pair whose end is not after its start is held here, shown with the reason and
 * not sent (the server would refuse it, spec user-settings "A bad merged value"); a good pair saves only the fields
 * that differ from the stored ones.
 */
function useWorkHours({ settings, save }: WorkHoursProps) {
  const [held, setHeld] = useState<Hours | null>(null)
  const saved: Hours = { workStart: settings.workStart ?? '', workEnd: settings.workEnd ?? '' }
  const hours = held ?? saved

  const set = (part: Part, value: string) => {
    const next = { ...hours, [part]: value }
    if (!endsAfterStart(next)) {
      setHeld(next)
      return
    }
    setHeld(null)
    const patch: CalendarOptions = {}
    if (next.workStart !== saved.workStart) patch.workStart = next.workStart
    if (next.workEnd !== saved.workEnd) patch.workEnd = next.workEnd
    if (Object.keys(patch).length > 0) void save(patch)
  }

  return { hours, wrong: held != null, set }
}

/** Wide: the row with the start and the end side by side (mockup settings-desktop.html §4, `.timepair`). */
export function WorkHoursRow(props: WorkHoursProps) {
  const { hours, wrong, set } = useWorkHours(props)
  const time = (part: Part, label: string) => (
    <div className="w-28">
      <Select aria-label={label} aria-invalid={wrong || undefined} className="tnum" value={hours[part]} onChange={(e) => set(part, e.target.value)}>
        {timesWith(hours[part]).map((t) => <option key={t} value={t}>{t}</option>)}
      </Select>
    </div>
  )
  return (
    <SettingsRow
      label="Giờ làm việc"
      hint={
        <>
          {HOURS_USE}
          {wrong && <span role="alert" className="block font-medium text-danger">{HOURS_WRONG}</span>}
        </>
      }
      trailing={
        <span className="flex items-center gap-2 shrink-0">
          {time('workStart', 'Bắt đầu làm việc')}
          <span aria-hidden="true" className="text-ink-muted">–</span>
          {time('workEnd', 'Kết thúc làm việc')}
        </span>
      }
    />
  )
}

/** Phone: the page under the Giờ làm việc row, a column of half hours for the start and one for the end. */
export function WorkHoursPage({ onBack, ...props }: WorkHoursProps & { onBack: () => void }) {
  const { hours, wrong, set } = useWorkHours(props)
  const column = (part: Part, title: string, note?: string) => (
    <SettingsGroup title={title} role="radiogroup" note={note}>
      {timesWith(hours[part]).map((t) => (
        <SettingsRow
          key={t}
          role="radio"
          checked={t === hours[part]}
          label={t}
          onPress={() => set(part, t)}
          trailing={
            <Check size={20} strokeWidth={2} aria-hidden="true" className={`shrink-0 text-accent ${t === hours[part] ? '' : 'invisible'}`} />
          }
        />
      ))}
    </SettingsGroup>
  )
  return (
    <SettingsPage title="Giờ làm việc" onBack={onBack}>
      {wrong && <p role="alert" className="m-0 px-1 text-[0.8125rem] font-medium text-danger">{HOURS_WRONG}</p>}
      {column('workStart', 'Bắt đầu')}
      {column('workEnd', 'Kết thúc', HOURS_USE)}
    </SettingsPage>
  )
}
