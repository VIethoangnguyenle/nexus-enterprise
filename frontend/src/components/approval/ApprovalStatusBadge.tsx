import { Badge } from '../primitives'

type ApprovalStatus = 'pending' | 'approved' | 'rejected' | 'cancelled'

const statusMap: Record<ApprovalStatus, { variant: 'warning' | 'success' | 'danger' | 'neutral'; label: string }> = {
  pending:   { variant: 'warning', label: 'Pending' },
  approved:  { variant: 'success', label: 'Approved' },
  rejected:  { variant: 'danger',  label: 'Rejected' },
  cancelled: { variant: 'neutral', label: 'Cancelled' },
}

interface ApprovalStatusBadgeProps {
  status: ApprovalStatus
  className?: string
}

/** Maps approval status to a semantic Badge with correct color variant. */
export function ApprovalStatusBadge({ status, className }: ApprovalStatusBadgeProps) {
  const config = statusMap[status] || statusMap.pending
  return (
    <Badge variant={config.variant} className={className}>
      {config.label}
    </Badge>
  )
}
