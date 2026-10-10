import { CircleAlert, Pencil } from 'lucide-react'
import { useApprovalTemplate } from '../../hooks/useApproval'
import { entityLabel, fieldTypeLabel, personFor } from '../../lib/approval-model'
import { formatDateTime } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { Avatar, Button } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { SidePanel } from '../spaces/SidePanel'
import { StatusPill } from './StatusPill'

interface TemplateDetailPanelProps {
  templateId: string
  people: PeopleDirectory
  onEdit: (id: string) => void
  onClose: () => void
}

/** Right panel for a template: what it is for, the form it asks, and the chain it runs. */
export function TemplateDetailPanel({ templateId, people, onEdit, onClose }: TemplateDetailPanelProps) {
  const q = useApprovalTemplate(templateId)
  const t = q.data

  if (q.isError && !t) {
    return (
      <SidePanel label="Chi tiết mẫu" title="Mẫu phê duyệt" closeLabel="chi tiết" onClose={onClose}>
        <EmptyState
          compact
          icon={<CircleAlert size={24} strokeWidth={1.75} />}
          text="Không tải được mẫu này. Kiểm tra kết nối rồi thử lại."
          action={<Button variant="soft" size="sm" onClick={() => void q.refetch()}>Thử lại</Button>}
        />
      </SidePanel>
    )
  }

  const creator = t ? personFor(people, t.created_by, t.created_by_name) : undefined
  return (
    <SidePanel
      label="Chi tiết mẫu"
      title={t?.name ?? 'Mẫu phê duyệt'}
      sub={t ? entityLabel(t.entity_type) : undefined}
      closeLabel="chi tiết"
      onClose={onClose}
      footer={
        t ? (
          <div className="px-4 pt-2 pb-4">
            <Button variant="soft" className="w-full" onClick={() => onEdit(t.id)}>
              <Pencil size={16} strokeWidth={1.75} aria-hidden="true" />
              Sửa mẫu
            </Button>
          </div>
        ) : undefined
      }
    >
      {!t ? (
        <div className="grid gap-4 px-3 pt-1 pb-3" aria-busy="true" aria-label="Đang tải mẫu">
          <div className="skeleton h-4 w-2/3 rounded-sm" />
          <div className="skeleton h-4 w-1/2 rounded-sm" />
          <div className="skeleton h-24 rounded-sm" />
        </div>
      ) : (
        <div className="grid gap-5 px-3 pt-1 pb-3">
          <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm m-0">
            <dt className="text-ink-muted">Trạng thái</dt>
            <dd className="m-0">
              <StatusPill pill={t.is_active ? { tone: 'ok', label: 'Đang dùng' } : { tone: 'idle', label: 'Đã tắt' }} />
            </dd>
            {creator && t.created_by_name && (
              <>
                <dt className="text-ink-muted">Người tạo</dt>
                <dd className="m-0 truncate">{creator.name}</dd>
              </>
            )}
            <dt className="text-ink-muted">Tạo lúc</dt>
            <dd className="m-0 tnum">{formatDateTime(t.created_at)}</dd>
          </dl>

          <section className="grid gap-2.5">
            <h3 className="m-0 text-label text-ink-muted">Biểu mẫu</h3>
            {(t.form_fields ?? []).length === 0 ? (
              <p className="m-0 text-sm text-ink-muted">Mẫu này không hỏi thêm thông tin.</p>
            ) : (
              <ul aria-label="Trường biểu mẫu" className="grid gap-2 m-0 p-0 list-none">
                {(t.form_fields ?? []).map((f) => (
                  <li key={f.label} className="flex items-center justify-between gap-2 text-sm">
                    <span className="truncate">{f.label}</span>
                    <span className="shrink-0 text-xs text-ink-muted">
                      {fieldTypeLabel(f.field_type)}{f.required ? ' · Bắt buộc' : ''}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="grid gap-2.5">
            <h3 className="m-0 text-label text-ink-muted">Chuỗi phê duyệt</h3>
            {(t.steps ?? []).length === 0 ? (
              <p className="m-0 text-sm text-ink-muted">Mẫu này chưa có bước duyệt.</p>
            ) : (
              <ol aria-label="Các bước duyệt" className="grid gap-3 m-0 p-0 list-none">
                {(t.steps ?? []).map((s) => {
                  const who = s.approver_name
                    ? s.approver_name
                    : 'Chưa chọn người duyệt'
                  const known = people.byNodeId.get(s.approver_value)
                  return (
                    <li key={s.step_order} className="grid grid-cols-[24px_minmax(0,1fr)] items-center gap-x-2.5">
                      <Avatar name={who} hueKey={known?.userId || s.approver_value} src={known?.avatarUrl} size={24} />
                      <span className="min-w-0 text-sm truncate">
                        <b className="font-semibold">{who}</b>
                      </span>
                      <small className="col-start-2 text-xs text-ink-muted">
                        {s.name}
                        {s.required_count > 1 ? ` · cần ${s.required_count} người duyệt` : ''}
                      </small>
                    </li>
                  )
                })}
              </ol>
            )}
          </section>
        </div>
      )}
    </SidePanel>
  )
}
