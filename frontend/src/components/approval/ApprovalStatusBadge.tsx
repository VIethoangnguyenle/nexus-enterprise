import { Badge } from '../primitives'

/**
 * The backend keeps two separate status vocabularies, and this badge renders
 * both — request rows in the approval table, assignment rows in the detail
 * panel's timeline.
 *
 *   Request.Status          — pending | approved | rejected | cancelled
 *   AssignmentRecord.Status — pending | approved | rejected | skipped | revoked
 *
 * They were previously modelled as one union carrying only the request side,
 * so `skipped` and `revoked` fell through to the fallback and a step that had
 * been skipped or revoked rendered as **Pending**. A workflow that had already
 * moved on looked like it was still waiting on someone.
 */
export type RequestStatus = 'pending' | 'approved' | 'rejected' | 'cancelled'
export type AssignmentStatus = 'pending' | 'approved' | 'rejected' | 'skipped' | 'revoked'
export type ApprovalStatus = RequestStatus | AssignmentStatus

type BadgeVariant = 'warning' | 'success' | 'danger' | 'neutral'

const statusMap: Record<ApprovalStatus, { variant: BadgeVariant; label: string }> = {
  pending:   { variant: 'warning', label: 'Pending' },
  approved:  { variant: 'success', label: 'Approved' },
  rejected:  { variant: 'danger',  label: 'Rejected' },
  cancelled: { variant: 'neutral', label: 'Cancelled' },
  // The step completed without this approver acting, because the quorum was
  // already met or the request was rejected elsewhere. Not an outcome the
  // approver chose, so it reads neutral rather than as a denial.
  skipped:   { variant: 'neutral', label: 'Skipped' },
  // The approver lost the role that granted them this assignment before they
  // acted on it.
  revoked:   { variant: 'neutral', label: 'Revoked' },
}

interface ApprovalStatusBadgeProps {
  status: ApprovalStatus | string
  className?: string
}

/** Maps approval status to a semantic Badge with correct color variant. */
export function ApprovalStatusBadge({ status, className }: ApprovalStatusBadgeProps) {
  // An unrecognised status shows as itself rather than being relabelled. The
  // previous fallback claimed "Pending", which turned any status the frontend
  // had not been taught about into a false statement about the workflow.
  const config = statusMap[status as ApprovalStatus] ?? { variant: 'neutral' as BadgeVariant, label: status }
  return (
    <Badge variant={config.variant} className={className}>
      {config.label}
    </Badge>
  )
}
