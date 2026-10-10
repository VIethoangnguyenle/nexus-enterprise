import { useMemo, useState, type ReactNode } from 'react'
import { ArrowLeft, CircleAlert } from 'lucide-react'
import type { ApprovalTemplate, CreateTemplateInput, FormFieldDefinition } from '../../api/approval'
import { useApprovalTemplate, useCreateTemplate, useUpdateTemplate } from '../../hooks/useApproval'
import { APPROVER_KINDS, ENTITY_TYPES, entityLabel, newUid, type ApproverType } from '../../lib/approval-model'
import { statusOf } from '../../lib/errors'
import type { PeopleDirectory } from '../../lib/people'
import { Button, Checkbox, Heading, IconButton, Select, TextField, toast } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { FormFieldBuilder, type FieldDraft, type FieldErrors } from './FormFieldBuilder'
import { blankStep, StepBuilder, type StepDraft, type StepErrors } from './StepBuilder'

interface TemplateEditorProps {
  /** A template id, or `new`. */
  templateId: string
  workspaceId: string
  people: PeopleDirectory
  /** Leave the builder (saved or abandoned). */
  onDone: () => void
}

/**
 * The template builder: name and kind, the form the submitter fills, and the
 * chain of steps with an approver chosen by picker on each. It replaces the
 * list in place (a modal is too small for it) and is addressed by the URL.
 */
export function TemplateEditor({ templateId, workspaceId, people, onDone }: TemplateEditorProps) {
  const creating = templateId === 'new'
  const q = useApprovalTemplate(creating ? '' : templateId)

  if (!creating && q.isError) {
    const gone = statusOf(q.error) === 404
    return (
      <Frame title="Sửa mẫu" onBack={onDone}>
        <EmptyState
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text={gone ? 'Mẫu này không còn nữa.' : 'Không tải được mẫu. Kiểm tra kết nối rồi thử lại.'}
          action={
            gone
              ? <Button variant="soft" size="sm" onClick={onDone}>Về danh sách mẫu</Button>
              : <Button variant="soft" size="sm" onClick={() => void q.refetch()}>Thử lại</Button>
          }
        />
      </Frame>
    )
  }
  if (!creating && !q.data) {
    return (
      <Frame title="Sửa mẫu" onBack={onDone}>
        <div className="grid gap-4 max-w-2xl" aria-busy="true" aria-label="Đang tải mẫu">
          <div className="skeleton h-10 rounded-md" />
          <div className="skeleton h-24 rounded-md" />
          <div className="skeleton h-40 rounded-md" />
        </div>
      </Frame>
    )
  }
  // Keyed by template: another template starts from its own values, never the last one's.
  return <TemplateForm key={templateId} template={q.data} workspaceId={workspaceId} people={people} onDone={onDone} />
}

function Frame({ title, onBack, children, footer }: {
  title: string
  onBack: () => void
  children: ReactNode
  footer?: ReactNode
}) {
  return (
    <section className="flex-1 flex flex-col min-w-0 min-h-0 bg-base" aria-label="Trình tạo mẫu">
      <header className="flex items-center gap-2 px-5 pt-4 pb-3">
        <IconButton aria-label="Quay lại danh sách mẫu" title="Quay lại" onClick={onBack}>
          <ArrowLeft size={18} strokeWidth={1.75} />
        </IconButton>
        <div className="grid min-w-0">
          <span className="text-xs text-ink-muted">Mẫu phê duyệt</span>
          <Heading as="h1" look="panel" className="truncate">{title}</Heading>
        </div>
      </header>
      <div className="flex-1 min-h-0 overflow-y-auto px-5 pt-2 pb-4">{children}</div>
      {footer}
    </section>
  )
}

const KNOWN: ApproverType[] = APPROVER_KINDS.map((k) => k.type)

function initialSteps(t: ApprovalTemplate | undefined): StepDraft[] {
  const steps = [...(t?.steps ?? [])].sort((a, b) => a.step_order - b.step_order)
  if (steps.length === 0) return [blankStep()]
  return steps.map((s) => {
    // A kind the server cannot resolve (an old "creator_manager") comes back as
    // an empty pick, so saving forces a real choice instead of keeping a dead one.
    const known = KNOWN.includes(s.approver_type as ApproverType)
    return {
      uid: newUid(),
      name: s.name,
      approver: known
        ? { type: s.approver_type as ApproverType, value: s.approver_value, name: s.approver_name ?? '' }
        : { type: 'specific_user', value: '', name: '' },
      requiredCount: s.required_count || 1,
      timeoutHours: s.timeout_hours || 0,
    }
  })
}

function TemplateForm({ template, workspaceId, people, onDone }: {
  template: ApprovalTemplate | undefined
  workspaceId: string
  people: PeopleDirectory
  onDone: () => void
}) {
  const editing = !!template
  const createMut = useCreateTemplate()
  const updateMut = useUpdateTemplate()
  const saving = createMut.isPending || updateMut.isPending

  const [name, setName] = useState(template?.name ?? '')
  const [entityType, setEntityType] = useState(template?.entity_type ?? ENTITY_TYPES[0]!)
  const [active, setActive] = useState(template?.is_active ?? true)
  const [fields, setFields] = useState<FieldDraft[]>(() =>
    (template?.form_fields ?? []).map((f) => ({
      uid: newUid(), label: f.label, fieldType: f.field_type, required: f.required, options: f.options ?? '',
    })),
  )
  const [steps, setSteps] = useState<StepDraft[]>(() => initialSteps(template))
  const [errors, setErrors] = useState<{
    name?: string; steps?: string
    step: Record<number, StepErrors>; field: Record<number, FieldErrors>
  }>({ step: {}, field: {} })

  const kinds = useMemo(() => {
    // A saved template may be of a kind this list does not offer; keep it selectable.
    const all = new Set(ENTITY_TYPES)
    if (template?.entity_type) all.add(template.entity_type)
    return [...all]
  }, [template])

  const validate = () => {
    const next: typeof errors = { step: {}, field: {} }
    if (!name.trim()) next.name = 'Đặt tên cho mẫu để người gửi nhận ra.'
    if (steps.length === 0) next.steps = 'Thêm ít nhất một bước duyệt.'
    for (const s of steps) {
      const e: StepErrors = {}
      if (!s.name.trim()) e.name = 'Đặt tên cho bước này.'
      if (!s.approver.value) e.approver = 'Chọn người duyệt.'
      if (e.name || e.approver) next.step[s.uid] = e
    }
    for (const f of fields) {
      const e: FieldErrors = {}
      if (!f.label.trim()) e.label = 'Đặt tên cho trường này.'
      if (f.fieldType === 'select' && !f.options.split(',').some((o) => o.trim())) e.options = 'Nhập ít nhất một lựa chọn.'
      if (e.label || e.options) next.field[f.uid] = e
    }
    setErrors(next)
    return !next.name && !next.steps && Object.keys(next.step).length === 0 && Object.keys(next.field).length === 0
  }

  const save = async () => {
    if (!validate()) return
    const body: CreateTemplateInput = {
      name: name.trim(),
      entity_type: entityType,
      priority: template?.priority ?? 0,
      form_fields: fields.map((f) => ({
        label: f.label.trim(),
        field_type: f.fieldType as FormFieldDefinition['field_type'],
        required: f.required,
        options: f.fieldType === 'select' ? f.options.split(',').map((o) => o.trim()).filter(Boolean).join(', ') : '',
        placeholder: '',
      })),
      steps: steps.map((s, i) => ({
        step_order: i + 1,
        name: s.name.trim(),
        approver_type: s.approver.type,
        approver_value: s.approver.value,
        required_count: s.requiredCount,
        timeout_hours: s.timeoutHours,
      })),
    }
    try {
      if (template) {
        // The server writes these two as given, so an edit must say what it keeps.
        // The kind of a template does not change, so it is not sent. The stamp it
        // was read at is: if someone else saved since, the server answers 409.
        await updateMut.mutateAsync({
          id: template.id,
          input: {
            name: body.name, form_fields: body.form_fields, steps: body.steps,
            is_active: active, priority: template.priority, expected_updated_at: template.updated_at,
          },
        })
      } else {
        await createMut.mutateAsync(body)
      }
    } catch {
      // The shared handler has told the user what failed; the form stays for another try.
      return
    }
    toast(editing ? 'Đã lưu mẫu' : 'Đã tạo mẫu')
    onDone()
  }

  return (
    <Frame
      title={editing ? template.name : 'Mẫu mới'}
      onBack={onDone}
      footer={
        <div className="flex justify-end gap-2 px-5 py-3 bg-raised">
          <Button variant="soft" onClick={onDone} disabled={saving}>Huỷ</Button>
          <Button loading={saving} onClick={() => void save()}>{editing ? 'Lưu mẫu' : 'Tạo mẫu'}</Button>
        </div>
      }
    >
      {/* Not a <form>: the builder is full of buttons (add, move, remove) and none of
          them may submit it. Saving is the one explicit button below. */}
      <div className="grid gap-7 max-w-2xl">
        <section className="grid gap-4" aria-label="Thông tin mẫu">
          <h2 className="m-0 text-label text-ink-muted">Thông tin</h2>
          <TextField
            label="Tên mẫu"
            value={name}
            error={errors.name}
            placeholder="Ví dụ: Tạm ứng công tác phí"
            onChange={(e) => {
              setName(e.target.value)
              if (errors.name) setErrors((p) => ({ ...p, name: undefined }))
            }}
          />
          <div className="grid gap-1.5">
            <label htmlFor="template-kind" className="text-sm font-semibold text-ink">Loại đề nghị</label>
            {/* The kind of an existing template cannot be changed (requests and
                routing already depend on it), so it is shown, not offered. */}
            <Select id="template-kind" value={entityType} disabled={editing} onChange={(e) => setEntityType(e.target.value)}>
              {kinds.map((k) => <option key={k} value={k}>{entityLabel(k)}</option>)}
            </Select>
          </div>
          {editing && (
            <label className="inline-flex items-center gap-2 text-sm cursor-pointer justify-self-start">
              <Checkbox label="Đang dùng" checked={active} onChange={(e) => setActive(e.target.checked)} />
              Đang dùng (người dùng chọn được mẫu này khi tạo đề nghị)
            </label>
          )}
        </section>

        <section className="grid gap-3" aria-label="Biểu mẫu">
          <h2 className="m-0 text-label text-ink-muted">Biểu mẫu người gửi điền</h2>
          <FormFieldBuilder fields={fields} onChange={setFields} errors={errors.field} />
        </section>

        <section className="grid gap-3" aria-label="Chuỗi phê duyệt">
          <h2 className="m-0 text-label text-ink-muted">Chuỗi phê duyệt</h2>
          {errors.steps && <span role="alert" className="text-xs font-medium text-danger">{errors.steps}</span>}
          <StepBuilder steps={steps} onChange={setSteps} errors={errors.step} workspaceId={workspaceId} people={people} />
        </section>
      </div>
    </Frame>
  )
}
