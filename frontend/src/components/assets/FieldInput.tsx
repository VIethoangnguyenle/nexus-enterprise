import { useId } from 'react'
import type { FieldDef } from '../../lib/asset-fields'
import type { PeopleDirectory, Person } from '../../lib/people'
import { PeoplePicker, Select, TextField } from '../primitives'

interface FieldInputProps {
  field: FieldDef
  /** What is typed: text for every kind; a person's value is their user id. */
  value: string
  error?: string
  people: PeopleDirectory
  disabled?: boolean
  onChange: (value: string) => void
}

/** One custom field of an asset type, as the control its kind calls for. */
export function FieldInput({ field, value, error, people, disabled, onChange }: FieldInputProps) {
  const id = useId()
  const hint = field.required ? undefined : '(không bắt buộc)'

  switch (field.kind) {
    case 'date':
      return <TextField label={field.title} labelHint={hint} type="date" value={value} error={error} disabled={disabled} onChange={(e) => onChange(e.target.value)} />
    case 'number':
      return <TextField label={field.title} labelHint={hint} inputMode="decimal" value={value} error={error} disabled={disabled} onChange={(e) => onChange(e.target.value)} />
    case 'choice':
      return (
        <div className="grid gap-1.5">
          <label htmlFor={id} className="text-sm font-semibold text-ink">
            {field.title}{hint && <span className="font-normal text-ink-muted"> {hint}</span>}
          </label>
          <Select id={id} value={value} error={error} disabled={disabled} onChange={(e) => onChange(e.target.value)}>
            <option value="">Chọn</option>
            {(field.options ?? []).map((o) => (
              <option key={o} value={o}>{o}</option>
            ))}
          </Select>
        </div>
      )
    case 'person': {
      const picked: Person[] = value ? [people.byUserId.get(value)].filter((p): p is Person => !!p) : []
      return (
        <div className="grid gap-1.5">
          <span id={`${id}-label`} className="text-sm font-semibold text-ink">
            {field.title}{hint && <span className="font-normal text-ink-muted"> {hint}</span>}
          </span>
          <PeoplePicker
            label={field.title}
            labelledBy={`${id}-label`}
            people={people.list}
            value={picked}
            max={1}
            onChange={(next) => onChange(next[0]?.userId ?? '')}
          />
          {error && <span role="alert" className="text-xs font-medium text-danger">{error}</span>}
        </div>
      )
    }
    default:
      return <TextField label={field.title} labelHint={hint} value={value} maxLength={500} error={error} disabled={disabled} onChange={(e) => onChange(e.target.value)} />
  }
}
