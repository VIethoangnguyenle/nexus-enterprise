import { Mail } from 'lucide-react'
import type { Invitation } from '../../api/admin'
import { useRevokeInvitation } from '../../hooks/useAdmin'
import { formatDate, formatRelative } from '../../lib/format'
import { Button, toast } from '../primitives'

interface InvitationsListProps {
  workspaceId: string
  invitations: Invitation[]
}

/** "Mời bởi Lê Thị Hoa · vai trò Kế toán · phòng Đối soát": who asked and what rides along. */
function detail(i: Invitation): string {
  return [
    i.inviter_name ? `Mời bởi ${i.inviter_name}` : '',
    i.role_name ? `vai trò ${i.role_name}` : '',
    i.department_name ? `phòng ${i.department_name}` : '',
  ].filter(Boolean).join(' · ')
}

/**
 * Lời mời đang chờ: the offers nobody has answered yet. The address is shown
 * (it is who the offer is for), never an identifier. An offer can be withdrawn
 * with "Thu hồi"; the person can no longer accept it.
 */
export function InvitationsList({ workspaceId, invitations }: InvitationsListProps) {
  const revoke = useRevokeInvitation(workspaceId)
  if (invitations.length === 0) return null
  return (
    <section aria-label="Lời mời đang chờ" className="grid gap-1 px-5 pb-6">
      <h2 className="m-0 flex items-center gap-2 h-9 text-xs font-semibold text-ink-muted border-b border-line">
        <Mail size={14} strokeWidth={1.75} aria-hidden="true" />
        Lời mời đang chờ ({invitations.length})
      </h2>
      <ul className="m-0 p-0 list-none">
        {invitations.map((i) => (
          <li key={i.id} className="flex items-center gap-3 min-h-14 px-2.5 border-b border-line">
            <span className="grid min-w-0 flex-1 py-1.5">
              <span className="font-medium truncate">{i.email}</span>
              <small className="text-xs text-ink-muted truncate">{detail(i)}</small>
            </span>
            <span className="hidden sm:grid text-right text-xs text-ink-muted tnum">
              <span>Gửi {formatRelative(i.created_at)}</span>
              <span>Hết hạn {formatDate(i.expires_at)}</span>
            </span>
            <Button
              variant="ghost"
              size="sm"
              aria-label={`Thu hồi lời mời của ${i.email}`}
              loading={revoke.isPending && revoke.variables === i.id}
              onClick={() => revoke.mutate(i.id, { onSuccess: () => toast(`Đã thu hồi lời mời của ${i.email}`) })}
            >
              Thu hồi
            </Button>
          </li>
        ))}
      </ul>
    </section>
  )
}
