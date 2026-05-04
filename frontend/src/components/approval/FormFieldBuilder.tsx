import { useCallback } from 'react'
import { Plus, GripVertical, Trash2 } from 'lucide-react'
import { Input, Select, Button, Text } from '../primitives'
import { IconButton } from '../primitives'

export interface FormFieldItem {
  label: string
  field_type: string
  required: boolean
  options: string
  placeholder: string
}

interface FormFieldBuilderProps {
  fields: FormFieldItem[]
  onChange: (fields: FormFieldItem[]) => void
}

const FIELD_TYPES = [
  { value: 'text', label: 'Text' },
  { value: 'number', label: 'Number' },
  { value: 'currency', label: 'Currency' },
  { value: 'date', label: 'Date' },
  { value: 'select', label: 'Select' },
  { value: 'textarea', label: 'Textarea' },
]

/** Builder for template form field definitions — drag handle + label + type + required + delete. */
export function FormFieldBuilder({ fields, onChange }: FormFieldBuilderProps) {
  const addField = useCallback(() => {
    onChange([...fields, { label: '', field_type: 'text', required: false, options: '', placeholder: '' }])
  }, [fields, onChange])

  const updateField = useCallback(
    (index: number, patch: Partial<FormFieldItem>) => {
      const next = fields.map((f, i) => (i === index ? { ...f, ...patch } : f))
      onChange(next)
    },
    [fields, onChange],
  )

  const removeField = useCallback(
    (index: number) => {
      onChange(fields.filter((_, i) => i !== index))
    },
    [fields, onChange],
  )

  return (
    <div className="space-y-3">
      <Text variant="caption" muted className="uppercase tracking-wider">
        Form Fields — Define submitter inputs
      </Text>

      {fields.map((field, i) => (
        <div
          key={i}
          className="flex items-center gap-2 p-3 bg-surface-container-lowest border border-outline-variant rounded-lg"
        >
          <GripVertical size={14} className="text-outline shrink-0 cursor-grab" />

          <Input
            type="text"
            value={field.label}
            onChange={(e) => updateField(i, { label: e.target.value })}
            placeholder="Field label"
            className="flex-1 min-w-0"
          />

          <Select
            value={field.field_type}
            onChange={(e) => updateField(i, { field_type: e.target.value })}
            className="w-28 shrink-0"
          >
            {FIELD_TYPES.map((t) => (
              <option key={t.value} value={t.value}>{t.label}</option>
            ))}
          </Select>

          {field.field_type === 'select' && (
            <Input
              type="text"
              value={field.options}
              onChange={(e) => updateField(i, { options: e.target.value })}
              placeholder="opt1, opt2"
              className="w-32 shrink-0"
            />
          )}

          <label className="flex items-center gap-1 shrink-0 cursor-pointer text-caption-ui text-on-surface-variant">
            <input
              type="checkbox"
              checked={field.required}
              onChange={(e) => updateField(i, { required: e.target.checked })}
              className="w-4 h-4 accent-primary"
            />
            Req
          </label>

          <IconButton
            aria-label="Remove field"
            onClick={() => removeField(i)}
            size="sm"
          >
            <Trash2 size={14} />
          </IconButton>
        </div>
      ))}

      <Button variant="ghost" onClick={addField} className="w-full">
        <Plus size={14} />
        Add Field
      </Button>
    </div>
  )
}
