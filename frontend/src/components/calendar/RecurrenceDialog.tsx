import { useEffect, useId, useState } from 'react'
import { format } from 'date-fns'
import {
  MAX_COUNT, MAX_INTERVAL, UNIT_LABEL, WEEKDAY_CHIP, WEEKDAY_CODES, buildRrule, defaultRule, describeRule, parseRrule,
  type CustomRule, type RepeatUnit, type RuleEnd, type WeekdayCode,
} from '../../lib/calendar-recurrence'
import { Button, Pressable, FieldInput, FieldSelect } from '../primitives'
import { Dialog } from '../composites/Dialog'

interface Props {
  open: boolean
  /** The day the event is on: the weekly rule starts from its weekday. */
  date: Date
  /** The rule the event has now, when it has one. */
  rrule: string | null
  onClose: () => void
  /** The rule to send. */
  onDone: (rrule: string) => void
}

function RadioRow({ checked, onSelect, label, children }: { checked: boolean; onSelect: () => void; label: string; children?: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 min-h-10">
      <Pressable role="radio" aria-checked={checked} onClick={onSelect} className="flex items-center gap-3 w-28 text-left text-sm text-ink">
        <span aria-hidden="true" className={`size-4.5 shrink-0 rounded-full grid place-items-center ring-2 ring-inset ${checked ? 'ring-accent' : 'ring-ink-muted'}`}>
          {checked && <span className="size-2.5 rounded-full bg-accent" />}
        </span>
        {label}
      </Pressable>
      {children}
    </div>
  )
}

/**
 * Lặp lại tùy chỉnh (DESIGN.md §6, Lặp lại): every N days/weeks/months/years, on which weekdays, and how it
 * ends. It speaks the rule in words under the fields and hands back an RRULE; the rule itself is never shown.
 */
export function RecurrenceDialog({ open, date, rrule, onClose, onDone }: Props) {
  const [rule, setRule] = useState<CustomRule>(() => defaultRule(date))
  const [untilDate, setUntilDate] = useState('')
  const [count, setCount] = useState('10')
  const daysId = useId()

  // Each time it opens it starts from the rule the event has (or every week on its day).
  useEffect(() => {
    if (!open) return
    const parsed = rrule ? parseRrule(rrule) : null
    const start = parsed ?? defaultRule(date)
    setRule(start)
    setUntilDate(start.end.kind === 'until' ? start.end.date : format(new Date(date.getFullYear(), date.getMonth() + 3, date.getDate()), 'yyyy-MM-dd'))
    setCount(start.end.kind === 'count' ? String(start.end.count) : '10')
  }, [open, rrule, date])

  const patch = (p: Partial<CustomRule>) => setRule((r) => ({ ...r, ...p }))
  const setEnd = (end: RuleEnd) => patch({ end })
  const toggleDay = (d: WeekdayCode) =>
    patch({ days: rule.days.includes(d) ? (rule.days.length > 1 ? rule.days.filter((x) => x !== d) : rule.days) : [...rule.days, d] })

  const endKind = rule.end.kind
  const effective: CustomRule = {
    ...rule,
    end: endKind === 'until' ? { kind: 'until', date: untilDate } : endKind === 'count' ? { kind: 'count', count: Number(count) || 1 } : rule.end,
  }
  const invalidUntil = endKind === 'until' && !untilDate

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Lặp lại tùy chỉnh"
      onSubmit={() => !invalidUntil && onDone(buildRrule(effective, date))}
      footer={
        <>
          <Button type="button" variant="soft" className="rounded-full" onClick={onClose}>Huỷ</Button>
          <Button type="submit" className="rounded-full" disabled={invalidUntil}>Xong</Button>
        </>
      }
    >
      <div className="flex items-center gap-2 text-sm text-ink">
        <span>Lặp lại mỗi</span>
        <FieldInput
          type="number" min={1} max={MAX_INTERVAL} aria-label="Số lần" className="w-16" value={rule.interval}
          onChange={(e) => patch({ interval: Math.max(1, Math.min(MAX_INTERVAL, Number(e.target.value) || 1)) })}
        />
        <FieldSelect aria-label="Đơn vị" value={rule.unit} onChange={(e) => patch({ unit: e.target.value as RepeatUnit })}>
          {(Object.keys(UNIT_LABEL) as RepeatUnit[]).map((u) => <option key={u} value={u}>{UNIT_LABEL[u]}</option>)}
        </FieldSelect>
      </div>

      {rule.unit === 'week' && (
        <div className="grid gap-2">
          <span id={daysId} className="text-xs font-semibold text-ink-muted">Vào các ngày</span>
          <div role="group" aria-labelledby={daysId} className="flex flex-wrap gap-1.5">
            {WEEKDAY_CODES.map((d) => (
              <Pressable
                key={d}
                aria-pressed={rule.days.includes(d)}
                onClick={() => toggleDay(d)}
                className={`size-9 rounded-full text-sm font-semibold ${rule.days.includes(d) ? 'bg-accent text-on-accent' : 'bg-sunk text-ink hover:bg-hover'}`}
              >
                {WEEKDAY_CHIP[d]}
              </Pressable>
            ))}
          </div>
        </div>
      )}

      <div role="radiogroup" aria-label="Kết thúc" className="grid gap-0.5">
        <span className="text-xs font-semibold text-ink-muted pb-1">Kết thúc</span>
        <RadioRow checked={endKind === 'never'} onSelect={() => setEnd({ kind: 'never' })} label="Không bao giờ" />
        <RadioRow checked={endKind === 'until'} onSelect={() => setEnd({ kind: 'until', date: untilDate })} label="Vào ngày">
          <FieldInput type="date" aria-label="Ngày kết thúc" value={untilDate} disabled={endKind !== 'until'} onChange={(e) => setUntilDate(e.target.value)} />
        </RadioRow>
        <RadioRow checked={endKind === 'count'} onSelect={() => setEnd({ kind: 'count', count: Number(count) || 1 })} label="Sau">
          <FieldInput
            type="number" min={1} max={MAX_COUNT} aria-label="Số lần lặp" className="w-18" value={count} disabled={endKind !== 'count'}
            onChange={(e) => setCount(e.target.value)}
          />
          <span className="text-sm text-ink-muted">lần</span>
        </RadioRow>
      </div>

      <p className="m-0 text-sm text-ink-muted" aria-live="polite">{describeRule(effective, date)}</p>
    </Dialog>
  )
}
