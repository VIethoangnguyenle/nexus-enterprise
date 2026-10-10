import { useMemo, useState } from 'react'
import { ArrowRight } from 'lucide-react'
import type { ApprovalTemplate } from '../../api/approval'
import { useApprovalTemplate, useApprovalTemplates, useCreateRequest } from '../../hooks/useApproval'
import { entityLabel } from '../../lib/approval-model'
import { explain } from '../../lib/errors'
import { Dialog } from '../composites/Dialog'
import { AlertBanner } from '../composites/AlertBanner'
import { Button, Select, toast } from '../primitives'
import { DynamicFormRenderer } from './DynamicFormRenderer'

interface CreateRequestDialogProps {
  open: boolean
  onClose: () => void
  /** The request was sent; the screen shows it on "Tôi đã gửi". */
  onCreated: () => void
}

/**
 * "Tạo đề nghị": pick a template by its name, fill its form, send. The server
 * creates the request from the chosen template; nothing here makes up an id.
 */
export function CreateRequestDialog({ open, onClose, onCreated }: CreateRequestDialogProps) {
  const createMut = useCreateRequest()
  return (
    <Dialog open={open} onClose={createMut.isPending ? () => {} : onClose} title="Tạo đề nghị">
      {/* Mounted only while open, so every opening starts from a blank form. */}
      <RequestForm onClose={onClose} onCreated={onCreated} createMut={createMut} />
    </Dialog>
  )
}

function RequestForm({ onClose, onCreated, createMut }: {
  onClose: () => void
  onCreated: () => void
  createMut: ReturnType<typeof useCreateRequest>
}) {
  const templates = useApprovalTemplates()
  const [templateId, setTemplateId] = useState('')
  const [values, setValues] = useState<Record<string, string>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [failure, setFailure] = useState<string | undefined>()
  const template = useApprovalTemplate(templateId)
  const chosen: ApprovalTemplate | undefined = templateId ? template.data : undefined
  const fields = useMemo(() => chosen?.form_fields ?? [], [chosen])
  const list = templates.data?.templates ?? []

  const choose = (id: string) => {
    setTemplateId(id)
    setValues({})
    setErrors({})
    setFailure(undefined)
  }

  const submit = async () => {
    if (!chosen) return
    const missing: Record<string, string> = {}
    for (const f of fields) {
      if (f.required && !(values[f.label] ?? '').trim()) missing[f.label] = `Nhập ${f.label.toLocaleLowerCase('vi')}.`
    }
    setErrors(missing)
    if (Object.keys(missing).length > 0) return
    setFailure(undefined)
    try {
      await createMut.mutateAsync({ template_id: chosen.id, form_data_json: JSON.stringify(values) })
    } catch (err) {
      setFailure(explain(err, 'gửi đề nghị'))
      return
    }
    toast('Đã gửi đề nghị')
    onCreated()
    onClose()
  }

  return (
    <form
      className="grid gap-4.5"
      onSubmit={(e) => {
        e.preventDefault()
        void submit()
      }}
    >
      <div className="grid gap-1.5">
        <label htmlFor="request-template" className="text-sm font-semibold text-ink">Loại đề nghị</label>
        {templates.isLoading ? (
          <div className="skeleton h-10 rounded-md" aria-busy="true" />
        ) : templates.isError ? (
          <AlertBanner variant="error">Không tải được danh sách mẫu. Đóng hộp này rồi mở lại.</AlertBanner>
        ) : list.length === 0 ? (
          <p className="m-0 text-sm text-ink-muted">Chưa có mẫu phê duyệt nào đang dùng. Nhờ quản trị viên tạo mẫu trước.</p>
        ) : (
          <Select id="request-template" value={templateId} onChange={(e) => choose(e.target.value)}>
            <option value="">Chọn loại đề nghị</option>
            {list.map((t) => (
              <option key={t.id} value={t.id}>{t.name}</option>
            ))}
          </Select>
        )}
        {chosen && <span className="text-xs text-ink-muted">{entityLabel(chosen.entity_type)}</span>}
      </div>

      {templateId && template.isLoading && <div className="skeleton h-24 rounded-md" aria-busy="true" />}
      {chosen && fields.length > 0 && (
        <DynamicFormRenderer
          fields={fields}
          values={values}
          errors={errors}
          disabled={createMut.isPending}
          onChange={(label, v) => {
            setValues((prev) => ({ ...prev, [label]: v }))
            if (errors[label]) setErrors(({ [label]: _gone, ...rest }) => rest)
          }}
        />
      )}

      {chosen?.steps && chosen.steps.length > 0 && (
        <section className="grid gap-2">
          <h3 className="m-0 text-label text-ink-muted">Chuỗi phê duyệt</h3>
          <ol aria-label="Chuỗi phê duyệt" className="flex flex-wrap items-center gap-1.5 m-0 p-0 list-none text-sm">
            {chosen.steps.map((s, i) => (
              <li key={s.step_order} className="flex items-center gap-1.5">
                {i > 0 && <ArrowRight size={14} strokeWidth={1.75} className="text-ink-muted" aria-hidden="true" />}
                <span className="px-2.5 py-1 rounded-full bg-sunk">
                  {s.name}
                  {s.approver_name && (
                    <span className="text-ink-muted"> · {s.approver_name}</span>
                  )}
                </span>
              </li>
            ))}
          </ol>
        </section>
      )}

      {failure && <AlertBanner variant="error">{failure}</AlertBanner>}

      <div className="flex justify-end gap-2">
        <Button type="button" variant="soft" onClick={onClose} disabled={createMut.isPending}>Huỷ</Button>
        <Button type="submit" loading={createMut.isPending} disabled={!chosen}>Gửi đề nghị</Button>
      </div>
    </form>
  )
}
