import { useMemo, useState } from 'react'
import { ArrowRight, Plus, Trash2 } from 'lucide-react'
import type { AssetType } from '../../api/assets'
import { useUpdateTypeSchema } from '../../hooks/useAssets'
import {
  FIELD_KINDS, MAX_FIELDS, TITLE_MAX, fieldKindLabel, fieldsToSchema, newFieldKey, parseFields,
  type FieldDef, type FieldKind,
} from '../../lib/asset-fields'
import { ASSET_STATES, categoryLabel, lifecycleNotes, statePill } from '../../lib/asset-model'
import { statusOf } from '../../lib/errors'
import { AlertBanner } from '../composites/AlertBanner'
import { Button, Checkbox, IconButton, Select, TextField, toast } from '../primitives'
import { SidePanel } from '../spaces/SidePanel'
import { AssetPill } from './AssetPill'

interface TypeDetailPanelProps {
  type: AssetType
  workspaceId: string
  /** The caller may edit the type's fields. */
  canManage: boolean
  onClose: () => void
}

/**
 * Right panel of Loại tài sản (mockup §5): the type's name and category, its own
 * fields (editable by whoever may manage types), and its lifecycle, read-only,
 * with the right each step needs said in words. Keyed by type and saved state by
 * the screen, so another type — or a save — starts from what the server holds.
 */
export function TypeDetailPanel({ type, workspaceId, canManage, onClose }: TypeDetailPanelProps) {
  const save = useUpdateTypeSchema(workspaceId)
  const original = useMemo(() => parseFields(type.fields_schema), [type.fields_schema])
  const [fields, setFields] = useState<FieldDef[]>(original)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [failure, setFailure] = useState<string | undefined>()

  const dirty = JSON.stringify(fields) !== JSON.stringify(original)
  const states = (type.lifecycle?.states?.length ? type.lifecycle.states : [...ASSET_STATES])
  const notes = lifecycleNotes(type.lifecycle?.transitions ?? [])

  const patch = (key: string, next: Partial<FieldDef>) => {
    setFields((all) => all.map((f) => (f.key === key ? { ...f, ...next } : f)))
    setErrors(({ [key]: _gone, ...rest }) => rest)
    setFailure(undefined)
  }
  const add = () =>
    setFields((all) => [...all, { key: newFieldKey('truong', all.map((f) => f.key)), title: '', kind: 'text', required: false }])

  const submit = async () => {
    const found: Record<string, string> = {}
    const seen = new Set<string>()
    for (const f of fields) {
      const title = f.title.trim()
      if (!title) found[f.key] = 'Nhập tên trường.'
      else if (seen.has(title.toLocaleLowerCase('vi'))) found[f.key] = 'Đã có trường tên này.'
      else if (f.kind === 'choice' && (f.options ?? []).length === 0) found[f.key] = 'Nhập ít nhất một lựa chọn.'
      seen.add(title.toLocaleLowerCase('vi'))
    }
    setErrors(found)
    if (Object.keys(found).length > 0) return
    setFailure(undefined)
    try {
      await save.mutateAsync({ typeId: type.id, schema: fieldsToSchema(fields.map((f) => ({ ...f, title: f.title.trim() }))) })
    } catch (err) {
      const s = statusOf(err)
      setFailure(
        s === 403 ? 'Bạn chưa có quyền sửa loại tài sản này. Cần quyền Quản lý trên vùng tài sản; hỏi quản trị viên.'
        : s === 400 ? 'Các trường chưa hợp lệ. Kiểm tra lại rồi lưu.'
        : 'Chưa lưu được. Kiểm tra kết nối mạng rồi thử lại.',
      )
      return
    }
    toast('Đã lưu trường thông tin')
  }

  return (
    <SidePanel
      label="Chi tiết loại tài sản"
      title={type.name}
      sub={categoryLabel(type.category)}
      closeLabel="chi tiết"
      onClose={onClose}
      footer={canManage ? (
        <div className="flex justify-end px-4 pt-2 pb-4">
          <Button loading={save.isPending} disabled={!dirty} onClick={() => void submit()}>Lưu</Button>
        </div>
      ) : undefined}
    >
      <div className="grid gap-5 px-3 pt-1 pb-3">
        <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm m-0">
          <dt className="text-ink-muted">Danh mục</dt>
          <dd className="m-0">{categoryLabel(type.category)}</dd>
          <dt className="text-ink-muted">Tài sản</dt>
          <dd className="m-0 tnum">{type.asset_count ?? 0}</dd>
        </dl>

        <section className="grid gap-2.5">
          <h3 className="m-0 text-label text-ink-muted">Trường thông tin riêng</h3>
          {fields.length === 0 && !canManage && <p className="m-0 text-sm text-ink-muted">Loại này chưa có trường riêng.</p>}
          {canManage ? (
            <>
              {fields.map((f, i) => (
                <FieldEditor key={f.key} index={i} field={f} error={errors[f.key]} onChange={(p) => patch(f.key, p)}
                  onRemove={() => setFields((all) => all.filter((x) => x.key !== f.key))} />
              ))}
              <Button variant="ghost" size="sm" className="justify-self-start" disabled={fields.length >= MAX_FIELDS} onClick={add}>
                <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
                Thêm trường
              </Button>
            </>
          ) : (
            <ul className="grid gap-1.5 m-0 p-0 list-none text-sm">
              {fields.map((f) => (
                <li key={f.key} className="flex justify-between gap-3">
                  <span>{f.title}{f.required && <span className="text-ink-muted"> (bắt buộc)</span>}</span>
                  <span className="text-ink-muted">{fieldKindLabel(f.kind)}</span>
                </li>
              ))}
            </ul>
          )}
          {failure && <AlertBanner variant="error">{failure}</AlertBanner>}
        </section>

        <section className="grid gap-2.5">
          <h3 className="m-0 text-label text-ink-muted">Vòng đời</h3>
          <ol aria-label="Vòng đời tài sản" className="flex flex-wrap items-center gap-1.5 m-0 p-0 list-none">
            {states.map((s, i) => (
              <li key={s} className="flex items-center gap-1.5">
                {i > 0 && <ArrowRight size={14} strokeWidth={1.75} className="text-ink-muted" aria-hidden="true" />}
                <AssetPill pill={statePill(s)} />
              </li>
            ))}
          </ol>
          {notes.length > 0 && (
            <p className="m-0 text-xs text-ink-muted leading-relaxed">
              {notes.map((n) => `Cần quyền ${n.right}: ${n.steps.join(', ')}.`).join(' ')}
            </p>
          )}
        </section>
      </div>
    </SidePanel>
  )
}

function FieldEditor({ index, field, error, onChange, onRemove }: {
  index: number
  field: FieldDef
  error?: string
  onChange: (patch: Partial<FieldDef>) => void
  onRemove: () => void
}) {
  const n = index + 1
  return (
    <div className="grid gap-2.5 p-3 rounded-md bg-base">
      <div className="flex items-start gap-2">
        <TextField
          className="flex-1 min-w-0"
          label={`Tên trường ${n}`}
          value={field.title}
          maxLength={TITLE_MAX}
          error={error}
          onChange={(e) => onChange({ title: e.target.value })}
        />
        <IconButton className="mt-6" aria-label={`Xoá trường ${field.title || n}`} onClick={onRemove}>
          <Trash2 size={16} strokeWidth={1.75} />
        </IconButton>
      </div>
      <div className="grid gap-1.5">
        <label htmlFor={`kind-${field.key}`} className="text-sm font-semibold text-ink">Kiểu {n}</label>
        <Select
          id={`kind-${field.key}`}
          value={field.kind}
          onChange={(e) => {
            const kind = e.target.value as FieldKind
            onChange({ kind, ...(kind === 'choice' ? { options: field.options ?? [] } : { options: undefined }) })
          }}
        >
          {FIELD_KINDS.map((k) => (
            <option key={k} value={k}>{fieldKindLabel(k)}</option>
          ))}
        </Select>
      </div>
      {field.kind === 'choice' && <OptionsField label={`Các lựa chọn ${n}`} options={field.options ?? []} onChange={(options) => onChange({ options })} />}
      <label className="inline-flex items-center gap-2 text-sm text-ink cursor-pointer">
        <Checkbox label={`Bắt buộc nhập ${field.title || `trường ${n}`}`} checked={field.required} onChange={(e) => onChange({ required: e.target.checked })} />
        Bắt buộc nhập
      </label>
    </div>
  )
}

/** The choices as one line of text, kept as typed so a comma can be typed; the parsed list goes up. */
function OptionsField({ label, options, onChange }: { label: string; options: string[]; onChange: (o: string[]) => void }) {
  const [text, setText] = useState(options.join(', '))
  return (
    <TextField
      label={label}
      labelHint="(cách nhau bằng dấu phẩy)"
      value={text}
      onChange={(e) => {
        setText(e.target.value)
        onChange([...new Set(e.target.value.split(',').map((o) => o.trim()).filter(Boolean))])
      }}
    />
  )
}
