import { CircleAlert, Lock } from 'lucide-react'
import { useApprovalAudit } from '../../hooks/useApproval'
import { auditLine, personFor } from '../../lib/approval-model'
import { statusOf } from '../../lib/errors'
import { formatDateTime, formatRelative } from '../../lib/format'
import type { PeopleDirectory } from '../../lib/people'
import { Avatar, Button } from '../primitives'

/**
 * The trail of a request: who did what and when, newest last. Each line names
 * its actor with an avatar. A request whose trail the caller may not read
 * (403) says so instead of looking empty.
 */
export function AuditList({ requestId, people }: { requestId: string; people: PeopleDirectory }) {
  const audit = useApprovalAudit(requestId)

  if (audit.isLoading) {
    return (
      <div className="grid gap-3" aria-busy="true" aria-label="Đang tải nhật ký">
        {[0, 1, 2].map((i) => <div key={i} className="skeleton h-8 rounded-sm" />)}
      </div>
    )
  }
  if (audit.isError) {
    if (statusOf(audit.error) === 403) {
      return (
        <p role="status" className="m-0 flex items-start gap-2 text-sm text-ink-muted">
          <Lock size={16} strokeWidth={1.75} className="shrink-0 mt-0.5" aria-hidden="true" />
          Bạn không có quyền xem nhật ký của đề nghị này.
        </p>
      )
    }
    return (
      <div role="alert" className="grid justify-items-start gap-2 text-sm text-ink-muted">
        <span className="flex items-start gap-2">
          <CircleAlert size={16} strokeWidth={1.75} className="shrink-0 mt-0.5" aria-hidden="true" />
          Không tải được nhật ký. Kiểm tra kết nối rồi thử lại.
        </span>
        <Button variant="soft" size="sm" onClick={() => void audit.refetch()}>Thử lại</Button>
      </div>
    )
  }
  const entries = audit.data?.entries ?? []
  if (entries.length === 0) return <p className="m-0 text-sm text-ink-muted">Chưa có hoạt động nào.</p>

  return (
    <ol aria-label="Nhật ký" className="grid gap-3 m-0 p-0 list-none">
      {entries.map((e) => {
        const line = auditLine(e)
        const who = personFor(people, e.actor_node_id, e.actor_name)
        return (
          <li key={e.id} className="grid grid-cols-[24px_minmax(0,1fr)] gap-2.5 items-start">
            {line.system ? (
              <span className="grid place-items-center w-6 h-6 rounded-full bg-sunk text-ink-muted" aria-hidden="true">
                <span className="w-1.5 h-1.5 rounded-full bg-ink-muted" />
              </span>
            ) : (
              <Avatar name={who.name} hueKey={who.hueKey} src={who.avatarUrl} size={24} />
            )}
            <p className="m-0 text-sm leading-snug">
              {line.system ? line.text : <><b className="font-semibold">{who.name}</b> {line.text}</>}
              {line.comment && <q className="block text-ink-muted italic">{line.comment}</q>}
              <time className="block text-xs text-ink-muted tnum" title={formatDateTime(e.created_at)} dateTime={e.created_at}>
                {formatRelative(e.created_at)}
              </time>
            </p>
          </li>
        )
      })}
    </ol>
  )
}
