import { Input, Select, Textarea, Text } from '../primitives'
import type { FormFieldDefinition } from '../../api/approval'

interface DynamicFormRendererProps {
  fields: FormFieldDefinition[]
  values: Record<string, string>
  onChange: (fieldLabel: string, value: string) => void
  disabled?: boolean
}

/** Renders dynamic form fields from a template definition.
 * Each field type maps to an existing primitive component. */
export function DynamicFormRenderer({
  fields,
  values,
  onChange,
  disabled = false,
}: DynamicFormRendererProps) {
  const sorted = [...fields].sort((a, b) => a.field_order - b.field_order)

  return (
    <div className="space-y-4">
      {sorted.map((field) => (
        <div key={field.label} className="flex flex-col gap-1">
          <label className="text-caption-ui text-on-surface-variant flex items-center gap-1">
            {field.label}
            {field.required && <span className="text-danger">*</span>}
          </label>
          {renderField(field, values[field.label] || '', onChange, disabled)}
        </div>
      ))}
    </div>
  )
}

/** Maps a FormFieldDefinition to the correct primitive input. */
function renderField(
  field: FormFieldDefinition,
  value: string,
  onChange: (label: string, val: string) => void,
  disabled: boolean,
) {
  const common = {
    disabled,
    placeholder: field.placeholder || '',
  }

  switch (field.field_type) {
    case 'text':
      return (
        <Input
          type="text"
          value={value}
          onChange={(e) => onChange(field.label, e.target.value)}
          {...common}
        />
      )

    case 'number':
      return (
        <Input
          type="number"
          value={value}
          onChange={(e) => onChange(field.label, e.target.value)}
          {...common}
        />
      )

    case 'currency':
      return (
        <div className="relative">
          <Input
            type="number"
            value={value}
            onChange={(e) => onChange(field.label, e.target.value)}
            className="pl-8"
            {...common}
          />
          <Text
            variant="caption"
            muted
            className="absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none"
          >
            ₫
          </Text>
        </div>
      )

    case 'date':
      return (
        <Input
          type="date"
          value={value}
          onChange={(e) => onChange(field.label, e.target.value)}
          {...common}
        />
      )

    case 'select':
      return (
        <Select
          value={value}
          onChange={(e) => onChange(field.label, e.target.value)}
          disabled={disabled}
        >
          <option value="">Select...</option>
          {(field.options || '').split(',').filter(Boolean).map((opt) => (
            <option key={opt.trim()} value={opt.trim()}>
              {opt.trim()}
            </option>
          ))}
        </Select>
      )

    case 'textarea':
      return (
        <Textarea
          value={value}
          onChange={(e) => onChange(field.label, e.target.value)}
          rows={3}
          {...common}
        />
      )

    default:
      return (
        <Input
          type="text"
          value={value}
          onChange={(e) => onChange(field.label, e.target.value)}
          {...common}
        />
      )
  }
}

// --- Read-only display ---

interface FormDataDisplayProps {
  fields: FormFieldDefinition[]
  data: Record<string, string>
}

/** Displays submitted form data in a read-only label/value layout. */
export function FormDataDisplay({ fields, data }: FormDataDisplayProps) {
  const sorted = [...fields].sort((a, b) => a.field_order - b.field_order)

  return (
    <div className="space-y-2">
      {sorted.map((field) => {
        const value = data[field.label]
        if (!value) return null

        return (
          <div key={field.label} className="flex items-start gap-2">
            <Text variant="caption" muted className="min-w-[100px] shrink-0">
              {field.label}
            </Text>
            <Text variant="body" className="break-words">
              {field.field_type === 'currency'
                ? `${Number(value).toLocaleString('vi-VN')} ₫`
                : value}
            </Text>
          </div>
        )
      })}
    </div>
  )
}
