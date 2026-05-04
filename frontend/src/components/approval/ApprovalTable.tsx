import { Check, X } from 'lucide-react'
import { ApprovalStatusBadge } from './ApprovalStatusBadge'
import { IconButton } from '../primitives'
import type { ApprovalRequest } from '../../api/approval'

interface ApprovalTableProps {
  items: ApprovalRequest[]
  showActions?: boolean
  showCheckboxes?: boolean
  selectedIds: Set<string>
  onToggleSelect: (id: string) => void
  onSelectAll: () => void
  onRowClick: (item: ApprovalRequest) => void
  onApprove?: (id: string) => void
  onReject?: (id: string) => void
}

/** Data table for approval requests — follows DriveFileList density pattern. */
export function ApprovalTable({
  items,
  showActions = false,
  showCheckboxes = false,
  selectedIds,
  onToggleSelect,
  onSelectAll,
  onRowClick,
  onApprove,
  onReject,
}: ApprovalTableProps) {
  const allSelected = items.length > 0 && items.every(i => selectedIds.has(i.id))

  return (
    <div className="flex flex-col flex-1 min-h-0 px-4 md:px-6 pb-4">
      <div className="bg-surface-container-low border border-outline-variant rounded-lg overflow-hidden flex flex-col flex-1">
        {/* Header row */}
        <div className="grid grid-cols-12 gap-2 px-4 py-3 border-b border-outline-variant
          bg-surface-container text-label-caps font-label-caps text-on-surface-variant uppercase tracking-wider">
          {showCheckboxes && (
            <div className="col-span-1 flex items-center">
              <input
                type="checkbox"
                checked={allSelected}
                onChange={onSelectAll}
                className="w-4 h-4 accent-primary cursor-pointer"
                aria-label="Select all"
              />
            </div>
          )}
          <div className={showCheckboxes ? 'col-span-3' : 'col-span-4'}>Request</div>
          <div className="col-span-2 hidden md:block">Status</div>
          <div className="col-span-2 hidden md:block">Step</div>
          <div className="col-span-2 hidden md:block">Date</div>
          {showActions && <div className="col-span-2 hidden md:block text-right">Actions</div>}
        </div>

        {/* Rows */}
        <div className="flex-1 overflow-y-auto">
          {items.map(item => (
            <div
              key={item.id}
              onClick={() => onRowClick(item)}
              className={`grid grid-cols-12 gap-2 px-4 py-3 border-b border-outline-variant/50
                cursor-pointer transition-colors hover:bg-surface-container
                ${selectedIds.has(item.id) ? 'bg-primary-fixed/10' : ''}`}
            >
              {showCheckboxes && (
                <div className="col-span-1 flex items-center">
                  <input
                    type="checkbox"
                    checked={selectedIds.has(item.id)}
                    onChange={e => { e.stopPropagation(); onToggleSelect(item.id) }}
                    onClick={e => e.stopPropagation()}
                    className="w-4 h-4 accent-primary cursor-pointer"
                    aria-label={`Select ${item.template_name}`}
                  />
                </div>
              )}
              <div className={showCheckboxes ? 'col-span-3' : 'col-span-4'}>
                <div className="text-sm font-medium text-on-surface truncate">{item.template_name}</div>
                <div className="text-xs text-on-surface-variant truncate">{item.entity_type}</div>
              </div>
              <div className="col-span-2 hidden md:flex items-center">
                <ApprovalStatusBadge status={item.status} />
              </div>
              <div className="col-span-2 hidden md:flex items-center text-sm text-on-surface-variant">
                {item.status === 'pending'
                  ? `Step ${item.current_step}/${item.assignments?.length || '?'}`
                  : '—'}
              </div>
              <div className="col-span-2 hidden md:flex items-center text-sm text-on-surface-variant">
                {formatRelativeDate(item.created_at)}
              </div>
              {showActions && (
                <div className="col-span-2 hidden md:flex items-center justify-end gap-1">
                  <IconButton
                    onClick={e => { e.stopPropagation(); onApprove?.(item.id) }}
                    aria-label="Approve"
                    title="Approve"
                    className="text-success hover:bg-success-bg"
                  >
                    <Check size={16} />
                  </IconButton>
                  <IconButton
                    onClick={e => { e.stopPropagation(); onReject?.(item.id) }}
                    aria-label="Reject"
                    title="Reject"
                    className="text-danger hover:bg-danger-bg"
                  >
                    <X size={16} />
                  </IconButton>
                </div>
              )}
              {/* Mobile: show status inline */}
              <div className="col-span-8 md:hidden flex items-center justify-end gap-2">
                <ApprovalStatusBadge status={item.status} />
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

/** Converts ISO date string to relative format. */
function formatRelativeDate(iso: string): string {
  if (!iso) return '—'
  const date = new Date(iso)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'Just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 7) return `${days}d ago`
  return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}
