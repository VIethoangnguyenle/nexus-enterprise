import { useId } from 'react'
import type { FormFieldDefinition } from '../../api/approval'
import { Select, Textarea, TextField } from '../primitives'

interface DynamicFormRendererProps {
  fields: FormFieldDefinition[]
  /** Values keyed by field label. */
  values: Record<string, string>
  /** Messages keyed by field label, shown under the field. */
  errors?: Record<string, string>
  onChange: (fieldLabel: string, value: string) => void
  disabled?: boolean
}

/**
 * The submitter's form, built from a template's fields (DESIGN.md §6: label
 * above, error below, never a placeholder as the label). Each field type maps
 * to the matching control.
 */
export function DynamicFormRenderer({ fields, values, errors = {}, onChange, disabled = false }: DynamicFormRendererProps) {
  const sorted = [...fields].sort((a, b) => a.field_order - b.field_order)
  return (
    <div className="grid gap-4">
      {sorted.map((f) => (
        <FieldControl
          key={f.label}
          field={f}
          value={values[f.label] ?? ''}
          error={errors[f.label]}
          disabled={disabled}
          onChange={(v) => onChange(f.label, v)}
        />
      ))}
    </div>
  )
}

function FieldControl({ field, value, error, disabled, onChange }: {
  field: FormFieldDefinition
  value: string
  error?: string
  disabled: boolean
  onChange: (value: string) => void
}) {
  const id = useId()
  const label = field.label
  const hint = field.required ? undefined : '(không bắt buộc)'
  const common = { disabled, placeholder: field.placeholder || undefined }

  switch (field.field_type) {
    case 'textarea':
      return (
        <div className="grid gap-1.5">
          <label htmlFor={id} className="text-sm font-semibold text-ink">
            {label}{hint && <span className="font-normal text-ink-muted"> {hint}</span>}
          </label>
          <Textarea id={id} rows={3} value={value} error={error} onChange={(e) => onChange(e.target.value)} {...common} />
        </div>
      )
    case 'select':
      return (
        <div className="grid gap-1.5">
          <label htmlFor={id} className="text-sm font-semibold text-ink">
            {label}{hint && <span className="font-normal text-ink-muted"> {hint}</span>}
          </label>
          <Select id={id} value={value} error={error} disabled={disabled} onChange={(e) => onChange(e.target.value)}>
            <option value="">Chọn một mục</option>
            {(field.options || '').split(',').map((o) => o.trim()).filter(Boolean).map((o) => (
              <option key={o} value={o}>{o}</option>
            ))}
          </Select>
        </div>
      )
    case 'currency':
      return (
        <TextField
          label={label} labelHint={hint ? `${hint} · ₫` : '₫'} inputMode="numeric" value={value} error={error}
          onChange={(e) => onChange(e.target.value.replace(/[^\d]/g, ''))} className="tnum" {...common}
        />
      )
    case 'number':
      return <TextField label={label} labelHint={hint} type="number" value={value} error={error} onChange={(e) => onChange(e.target.value)} {...common} />
    case 'date':
      return <TextField label={label} labelHint={hint} type="date" value={value} error={error} onChange={(e) => onChange(e.target.value)} {...common} />
    default:
      return <TextField label={label} labelHint={hint} value={value} error={error} onChange={(e) => onChange(e.target.value)} {...common} />
  }
}
