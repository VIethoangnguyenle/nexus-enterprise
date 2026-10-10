import { Plus, Trash2 } from 'lucide-react'
import { Button, Checkbox, IconButton, Select, TextField } from '../primitives'
import { fieldTypeLabel, newUid } from '../../lib/approval-model'

/** A form field as the builder holds it. `uid` is only a React key. */
export interface FieldDraft {
  uid: number
  label: string
  fieldType: string
  required: boolean
  options: string
}

export interface FieldErrors { label?: string; options?: string }

export const FIELD_TYPES = ['text', 'textarea', 'number', 'currency', 'date', 'select']

export const blankField = (): FieldDraft => ({ uid: newUid(), label: '', fieldType: 'text', required: false, options: '' })

interface FormFieldBuilderProps {
  fields: FieldDraft[]
  onChange: (fields: FieldDraft[]) => void
  errors: Record<number, FieldErrors>
}

/** What the submitter is asked: a label, a type, whether it is required, and options for a list. */
export function FormFieldBuilder({ fields, onChange, errors }: FormFieldBuilderProps) {
  const update = (uid: number, patch: Partial<FieldDraft>) =>
    onChange(fields.map((f) => (f.uid === uid ? { ...f, ...patch } : f)))

  return (
    <div className="grid gap-3">
      {fields.length === 0 && (
        <p className="m-0 text-sm text-ink-muted">Chưa hỏi thông tin nào. Thêm trường để người gửi điền số tiền, lý do và các chi tiết khác.</p>
      )}
      <ul aria-label="Trường biểu mẫu" className="grid gap-3 m-0 p-0 list-none">
        {fields.map((f, i) => {
          const err = errors[f.uid]
          const n = i + 1
          return (
            <li key={f.uid} className="grid gap-3 p-3 rounded-surface bg-sunk">
              <div className="flex items-start gap-2.5">
                <TextField
                  className="flex-1 min-w-0"
                  label={`Tên trường ${n}`}
                  value={f.label}
                  placeholder="Ví dụ: Số tiền"
                  error={err?.label}
                  onChange={(e) => update(f.uid, { label: e.target.value })}
                />
                <div className="grid gap-1.5 w-40 shrink-0">
                  <label htmlFor={`field-type-${f.uid}`} className="text-sm font-semibold text-ink">{`Kiểu trường ${n}`}</label>
                  <Select id={`field-type-${f.uid}`} value={f.fieldType} onChange={(e) => update(f.uid, { fieldType: e.target.value })}>
                    {FIELD_TYPES.map((t) => <option key={t} value={t}>{fieldTypeLabel(t)}</option>)}
                  </Select>
                </div>
                <IconButton
                  size="md"
                  tone="danger"
                  className="mt-7 shrink-0"
                  aria-label={`Xoá trường ${n}`}
                  onClick={() => onChange(fields.filter((x) => x.uid !== f.uid))}
                >
                  <Trash2 size={16} strokeWidth={1.75} />
                </IconButton>
              </div>
              {f.fieldType === 'select' && (
                <TextField
                  label={`Các lựa chọn của trường ${n}`}
                  labelHint="(cách nhau bằng dấu phẩy)"
                  value={f.options}
                  placeholder="Máy bay, Tàu hoả, Ô tô"
                  error={err?.options}
                  onChange={(e) => update(f.uid, { options: e.target.value })}
                />
              )}
              <label className="inline-flex items-center gap-2 text-sm cursor-pointer justify-self-start">
                <Checkbox
                  label={`Trường ${n} bắt buộc`}
                  checked={f.required}
                  onChange={(e) => update(f.uid, { required: e.target.checked })}
                />
                Bắt buộc điền
              </label>
            </li>
          )
        })}
      </ul>
      <Button variant="soft" size="sm" className="justify-self-start" onClick={() => onChange([...fields, blankField()])}>
        <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
        Thêm trường
      </Button>
    </div>
  )
}
