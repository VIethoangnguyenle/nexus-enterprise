import { useState } from 'react'
import { Check, X } from 'lucide-react'
import { PeekPanel } from '../composites/PeekPanel'
import { Timeline } from '../composites/Timeline'
import { Button, Spinner, Text, Textarea } from '../primitives'
import { ApprovalStatusBadge } from './ApprovalStatusBadge'
import { FormDataDisplay } from './DynamicFormRenderer'
import { useApprovalAudit, useApprovalTemplate } from '../../hooks/useApproval'
import type { ApprovalRequest } from '../../api/approval'

interface ApprovalDetailPanelProps {
  item: ApprovalRequest
  onClose: () => void
  onApprove: (id: string, comment: string) => void
  onReject: (id: string, comment: string) => void
  isApproving?: boolean
  isRejecting?: boolean
}

/** Right-side detail panel for an approval request, showing info + step timeline + actions. */
export function ApprovalDetailPanel({
  item,
  onClose,
  onApprove,
  onReject,
  isApproving = false,
  isRejecting = false,
}: ApprovalDetailPanelProps) {
  const [comment, setComment] = useState('')
  const { data: auditData } = useApprovalAudit(item.id)
  const { data: template } = useApprovalTemplate(item.template_id)
  const isPending = item.status === 'pending'

  // Parse form data if available
  const formData: Record<string, string> = (() => {
    if (!item.form_data_json) return {}
    try { return JSON.parse(item.form_data_json) } catch { return {} }
  })()

  const timelineItems = (item.assignments || []).map(a => ({
    id: a.id,
    color: a.status === 'approved' ? 'var(--color-success)'
         : a.status === 'rejected' ? 'var(--color-danger)'
         : a.status === 'pending' ? 'var(--color-warning)'
         : 'var(--color-on-surface-variant)',
    title: (
      <span className="flex items-center gap-2">
        <span>{a.step_name || `Step ${a.step_order}`}</span>
        <ApprovalStatusBadge status={a.status} />
      </span>
    ),
    timestamp: a.acted_at
      ? new Date(a.acted_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
      : 'Waiting',
    body: a.comment ? (
      <span className="text-on-surface-variant italic">"{a.comment}"</span>
    ) : undefined,
  }))

  return (
    <PeekPanel
      title={item.template_name || 'Approval Request'}
      onClose={onClose}
      width={400}
    >
      <div className="flex flex-col h-full">
        {/* Details section */}
        <div className="px-4 py-4 space-y-3 border-b border-outline-variant">
          <div className="flex items-center gap-2">
            <Text variant="caption" muted>Status</Text>
            <ApprovalStatusBadge status={item.status} />
          </div>
          <DetailRow label="Entity Type" value={item.entity_type} />
          <DetailRow label="Entity ID" value={item.entity_id} />
          <DetailRow label="Department" value={item.department_id || '—'} />
          <DetailRow label="Created" value={
            item.created_at
              ? new Date(item.created_at).toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })
              : '—'
          } />
          {item.completed_at && (
            <DetailRow label="Completed" value={
              new Date(item.completed_at).toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })
            } />
          )}
        </div>

        {/* Form data — dynamic fields submitted with the request */}
        {template?.form_fields && template.form_fields.length > 0 && Object.keys(formData).length > 0 && (
          <div className="px-4 py-4 space-y-2 border-b border-outline-variant">
            <Text variant="caption" muted className="uppercase tracking-wider block">
              Request Details
            </Text>
            <FormDataDisplay fields={template.form_fields} data={formData} />
          </div>
        )}

        {/* Timeline section */}
        <div className="flex-1 overflow-y-auto px-4 py-4">
          <Text variant="caption" muted className="uppercase tracking-wider mb-3 block">
            Approval Steps
          </Text>
          {timelineItems.length > 0 ? (
            <Timeline items={timelineItems} />
          ) : (
            <Text variant="body" muted>No steps available</Text>
          )}

          {/* Audit entries */}
          {auditData?.entries && auditData.entries.length > 0 && (
            <div className="mt-4 pt-4 border-t border-outline-variant">
              <Text variant="caption" muted className="uppercase tracking-wider mb-2 block">
                Audit Log
              </Text>
              {auditData.entries.map(entry => (
                <div key={entry.id} className="py-2 text-xs text-on-surface-variant">
                  <span className="font-medium text-on-surface capitalize">{entry.action}</span>
                  {' — '}
                  {new Date(entry.created_at).toLocaleString('en-US', {
                    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
                  })}
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Action bar — only for pending items */}
        {isPending && (
          <div className="px-4 py-4 border-t border-outline-variant bg-surface-container space-y-3 flex-shrink-0">
            <Textarea
              value={comment}
              onChange={e => setComment(e.target.value)}
              placeholder="Add a comment..."
              className="w-full text-sm"
              rows={2}
            />
            <div className="flex gap-2">
              <Button
                onClick={() => onApprove(item.id, comment)}
                disabled={isApproving}
                className="flex-1 bg-success text-on-success hover:bg-success/90"
              >
                {isApproving ? <Spinner size="sm" /> : <Check size={16} />}
                <span className="ml-1">Approve</span>
              </Button>
              <Button
                variant="error"
                onClick={() => onReject(item.id, comment)}
                disabled={isRejecting || !comment.trim()}
                className="flex-1"
              >
                {isRejecting ? <Spinner size="sm" /> : <X size={16} />}
                <span className="ml-1">Reject</span>
              </Button>
            </div>
            {!comment.trim() && (
              <Text variant="caption" muted className="text-center">
                Comment required to reject
              </Text>
            )}
          </div>
        )}
      </div>
    </PeekPanel>
  )
}

/** Simple label/value detail row. */
function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center gap-2">
      <Text variant="caption" muted className="min-w-[80px]">{label}</Text>
      <Text variant="body" className="truncate">{value}</Text>
    </div>
  )
}
