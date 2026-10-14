import { Stamp } from 'lucide-react'
import { Link } from '@tanstack/react-router'
import { useApprovalRequest } from '../../hooks/useApproval'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { usePeople } from '../../hooks/usePeople'
import { amountOfRequest, chainOf, entityLabel, requestPill, requestTitle, type Pill } from '../../lib/approval-model'
import { statusOf } from '../../lib/errors'
import { avatarOf } from '../../lib/people'
import { formatMoney } from '../../lib/format'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar } from '../primitives'
import { StatusPill } from '../approval/StatusPill'
import { CardSkeleton, ChatCard, UnavailableCard } from './ChatCard'

const icon = <Stamp size={16} strokeWidth={1.75} aria-hidden="true" />

/**
 * An approval request in the conversation (DESIGN.md "Đề nghị phê duyệt"): its
 * title, kind and amount, the chain of approvers with where each stands, and the
 * overall status. The request is read from the approval service under the
 * viewer's own access; without it the card shows no title, only that it cannot
 * be viewed.
 */
export function ApprovalCard({ requestId }: { requestId: string }) {
  const detail = useApprovalRequest(requestId)
  const meNodeId = useAuthStore((s) => s.user?.ngac_node_id)
  const { workspaceId } = useActiveWorkspace()
  const people = usePeople(workspaceId)

  if (detail.isLoading) return <CardSkeleton />
  const d = detail.data
  if (!d) {
    const denied = [403, 404].includes(statusOf(detail.error) ?? 0)
    return denied ? (
      <UnavailableCard tone="warning" icon={icon} title="Đề nghị không xem được" note="Bạn không có quyền xem đề nghị này." />
    ) : (
      <UnavailableCard tone="warning" icon={icon} title="Chưa tải được đề nghị" />
    )
  }

  const { request } = d
  const amount = amountOfRequest(request)
  const chain = chainOf({ request, steps: d.steps, assignments: d.assignments, meNodeId, canAct: d.can_act, people })
  const waiting = d.steps?.find((s) => s.step_order === request.current_step)
  const pill: Pill =
    request.status !== 'pending'
      ? requestPill({ request })
      : d.can_act
        ? { tone: 'wait', label: 'Chờ bạn duyệt' }
        : { tone: 'idle', label: waiting ? `Chờ ${waiting.approver_name || waiting.name} duyệt` : 'Đang chờ duyệt' }
  const submitter = request.created_by_name?.trim()

  return (
    <ChatCard
      tone="warning"
      icon={icon}
      title={requestTitle(request)}
      sub={['Đề nghị phê duyệt', entityLabel(request.entity_type), amount !== null ? formatMoney(amount) : undefined]
        .filter(Boolean)
        .join(' · ')}
    >
      <ol aria-label="Chuỗi phê duyệt" className="flex flex-wrap items-center gap-x-1.5 gap-y-1 m-0 p-0 list-none text-xs">
        {submitter && (
          <li className="inline-flex items-center gap-1.5">
            <Avatar name={submitter} hueKey={request.created_by} src={avatarOf(people, request.created_by)} size={20} />
            <span className="font-medium text-ink">{submitter}</span>
            <small className="text-success">Đã gửi</small>
          </li>
        )}
        {chain.map((c) => (
          <li key={c.key} className="inline-flex items-center gap-1.5">
            {(submitter || c !== chain[0]) && <span aria-hidden="true" className="text-ink-muted">›</span>}
            <Avatar name={c.name} hueKey={c.hueKey} src={c.avatarUrl} size={20} />
            <span className={`font-medium ${c.pill.tone === 'idle' ? 'text-ink-muted' : 'text-ink'}`}>{c.name}</span>
            <small className={c.pill.tone === 'ok' ? 'text-success' : c.pill.tone === 'wait' ? 'text-warning' : 'text-ink-muted'}>
              {c.pill.label === 'Chưa tới' ? 'Tiếp theo' : c.pill.label}
            </small>
          </li>
        ))}
      </ol>
      <div className="flex items-center gap-2 min-h-8">
        <StatusPill pill={pill} />
        <Link
          to="/approval"
          search={{ request: request.id }}
          className="press ml-auto inline-flex items-center justify-center min-h-11 md:min-h-8 px-3 rounded-md bg-transparent
            text-sm font-semibold text-ink no-underline focus-ring hover:bg-hover"
        >
          Xem đề nghị
        </Link>
      </div>
    </ChatCard>
  )
}
