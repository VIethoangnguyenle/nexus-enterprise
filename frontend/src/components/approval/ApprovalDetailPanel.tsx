import { Check, CircleAlert, Lock } from 'lucide-react'
import type { ApprovalRequest, ApprovalRequestDetail, FormFieldDefinition } from '../../api/approval'
import { useApprovalRequest } from '../../hooks/useApproval'
import { amountOf, chainOf, parseFormData, personFor, requestPill, requestTitle, type Pill } from '../../lib/approval-model'
import { statusOf } from '../../lib/errors'
import { formatDate, formatDateTime, formatMoney } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { Button, PersonChip } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'
import { SidePanel } from '../spaces/SidePanel'
import { ApprovalChain } from './ApprovalChain'
import { AuditList } from './AuditList'
import { StatusPill } from './StatusPill'

interface ApprovalDetailPanelProps {
  requestId: string
  /** The list row, shown while the detail loads. */
  summary?: ApprovalRequest
  people: PeopleDirectory
  /** The signed-in user's NGAC node, to find their own place in the chain. */
  meNodeId: string | undefined
  approving: boolean
  onApprove: (request: ApprovalRequest) => void
  onReturn: (request: ApprovalRequest) => void
  onClose: () => void
}

/**
 * Right panel of Phê duyệt (mockup §3): the amount, who sent it and when, the
 * reason, the approval chain, the trail, and Duyệt / Trả lại for whoever's
 * turn it is. Everything is a name: no request, user or role code is shown.
 */
export function ApprovalDetailPanel({
  requestId, summary, people, meNodeId, approving, onApprove, onReturn, onClose,
}: ApprovalDetailPanelProps) {
  const detail = useApprovalRequest(requestId)
  const d = detail.data
  const request = d?.request ?? summary
  const title = request ? requestTitle(request) : 'Đề nghị'

  if (detail.isError && !d) {
    const forbidden = statusOf(detail.error) === 403
    return (
      <SidePanel label="Chi tiết đề nghị" title="Đề nghị" closeLabel="chi tiết" onClose={onClose}>
        <EmptyState
          compact
          icon={forbidden ? <Lock size={24} strokeWidth={1.75} /> : <CircleAlert size={24} strokeWidth={1.75} />}
          text={
            forbidden
              ? 'Không mở được đề nghị này. Có thể nó đã bị xoá hoặc bạn chưa có quyền xem.'
              : 'Không tải được đề nghị. Kiểm tra kết nối rồi thử lại.'
          }
          action={!forbidden ? <Button variant="soft" size="sm" onClick={() => void detail.refetch()}>Thử lại</Button> : undefined}
        />
      </SidePanel>
    )
  }

  // The server says whose turn it is: a named approver, or a member of the role
  // or department the step was given to, who has not yet acted.
  const myTurn = !!d && d.can_act

  return (
    <SidePanel
      label="Chi tiết đề nghị"
      title={title}
      sub={request ? <StatusPill pill={headerPill(request, d, myTurn)} /> : undefined}
      closeLabel="chi tiết"
      onClose={onClose}
      footer={
        myTurn && d ? (
          <div className="flex gap-2 px-4 pt-2 pb-4">
            <Button className="flex-1" loading={approving} onClick={() => onApprove(d.request)}>
              <Check size={16} strokeWidth={1.75} aria-hidden="true" />
              Duyệt
            </Button>
            <Button className="flex-1" variant="soft" disabled={approving} onClick={() => onReturn(d.request)}>
              Trả lại
            </Button>
          </div>
        ) : undefined
      }
    >
      {!d ? <DetailSkeleton /> : <DetailBody detail={d} people={people} meNodeId={meNodeId} />}
    </SidePanel>
  )
}

/**
 * The status under the title. Once the chain is known it names the step the
 * request waits on ("Chờ Kế toán trưởng"), or says it is the viewer's turn.
 */
function headerPill(request: ApprovalRequest, detail: ApprovalRequestDetail | undefined, myTurn: boolean): Pill {
  if (request.status !== 'pending') return requestPill({ request })
  if (myTurn) return { tone: 'wait', label: 'Chờ bạn' }
  const step = detail?.steps?.find((s) => s.step_order === request.current_step)
  return step?.name ? { tone: 'idle', label: `Chờ ${step.name}` } : requestPill({ request })
}

function DetailBody({ detail, people, meNodeId }: { detail: ApprovalRequestDetail; people: PeopleDirectory; meNodeId: string | undefined }) {
  const { request } = detail
  const fields: FormFieldDefinition[] = detail.form_fields ?? []
  const values = parseFormData(request.form_data_json)
  const amount = amountOf(fields, values)
  const requester = personFor(people, request.created_by, request.created_by_name)
  const chain = chainOf({ request, steps: detail.steps, assignments: detail.assignments, meNodeId, canAct: detail.can_act, people })
  const amountField = fields.find((f) => f.field_type === 'currency' && (values[f.label] ?? '').trim() !== '')
  const shown = fields.filter((f) => f !== amountField && (values[f.label] ?? '').trim() !== '')
  const longText = shown.filter((f) => f.field_type === 'textarea')
  const short = shown.filter((f) => f.field_type !== 'textarea')

  return (
    <div className="grid gap-5 px-3 pt-1 pb-3">
      {amount !== null && <div className="font-display text-2xl font-semibold tnum">{formatMoney(amount)}</div>}

      <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm m-0">
        <dt className="text-ink-muted">Người gửi</dt>
        <dd className="m-0 min-w-0">
          <PersonChip className="max-w-full" name={requester.name} hueKey={requester.hueKey} avatarUrl={requester.avatarUrl} />
          {requester.role && <span className="block text-xs text-ink-muted">{requester.role}</span>}
        </dd>
        {request.department_name && (
          <>
            <dt className="text-ink-muted">Phòng</dt>
            <dd className="m-0">{request.department_name}</dd>
          </>
        )}
        <dt className="text-ink-muted">Gửi lúc</dt>
        <dd className="m-0 tnum">{formatDateTime(request.created_at)}</dd>
        {request.completed_at && (
          <>
            <dt className="text-ink-muted">Hoàn tất lúc</dt>
            <dd className="m-0 tnum">{formatDateTime(request.completed_at)}</dd>
          </>
        )}
        {short.map((f) => (
          <FieldRow key={f.label} field={f} value={values[f.label]!} />
        ))}
      </dl>

      {longText.map((f) => (
        <section key={f.label} className="grid gap-2">
          <h3 className="m-0 text-label text-ink-muted">{f.label}</h3>
          <p className="m-0 text-sm leading-relaxed whitespace-pre-line">{values[f.label]}</p>
        </section>
      ))}

      <section className="grid gap-2.5">
        <h3 className="m-0 text-label text-ink-muted">Chuỗi phê duyệt</h3>
        <ApprovalChain items={chain} />
      </section>

      <section className="grid gap-2.5">
        <h3 className="m-0 text-label text-ink-muted">Nhật ký</h3>
        <AuditList requestId={request.id} people={people} />
      </section>
    </div>
  )
}

function FieldRow({ field, value }: { field: FormFieldDefinition; value: string }) {
  const text =
    field.field_type === 'date' ? formatDate(value) || value
    : field.field_type === 'currency' && Number.isFinite(Number(value)) ? formatMoney(Number(value))
    : value
  return (
    <>
      <dt className="text-ink-muted">{field.label}</dt>
      <dd className="m-0 tnum">{text}</dd>
    </>
  )
}

function DetailSkeleton() {
  return (
    <div className="grid gap-4 px-3 pt-1 pb-3" aria-busy="true" aria-label="Đang tải đề nghị">
      <div className="skeleton h-8 w-40 rounded-sm" />
      <div className="skeleton h-4 w-2/3 rounded-sm" />
      <div className="skeleton h-4 w-1/2 rounded-sm" />
      <div className="skeleton h-20 rounded-sm" />
    </div>
  )
}
