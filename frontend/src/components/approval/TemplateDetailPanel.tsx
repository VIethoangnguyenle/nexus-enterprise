import { PeekPanel } from '../composites/PeekPanel'
import { Timeline } from '../composites/Timeline'
import { Badge, Button, Text } from '../primitives'
import type { ApprovalTemplate } from '../../api/approval'

interface TemplateDetailPanelProps {
  template: ApprovalTemplate
  onClose: () => void
  onEdit: (t: ApprovalTemplate) => void
}

/** Right-side detail panel for a template — metadata + form field list + step chain. */
export function TemplateDetailPanel({
  template,
  onClose,
  onEdit,
}: TemplateDetailPanelProps) {
  const stepTimeline = (template.steps || []).map((s) => ({
    id: s.id || String(s.step_order),
    color: 'var(--color-primary)',
    title: (
      <span className="flex items-center gap-2">
        <span>{s.name}</span>
        <Badge variant="neutral">{s.approver_type.replace('_', ' ')}</Badge>
      </span>
    ),
    timestamp: s.approver_value || '—',
    body: s.required_count > 1 ? (
      <span>Requires {s.required_count} approvals</span>
    ) : undefined,
  }))

  return (
    <PeekPanel
      title={template.name}
      onClose={onClose}
      width={400}
    >
      <div className="flex flex-col h-full">
        {/* Metadata */}
        <div className="px-4 py-4 space-y-2 border-b border-outline-variant">
          <DetailRow label="Entity Type" value={template.entity_type} />
          <div className="flex items-center gap-2">
            <Text variant="caption" muted className="min-w-[80px]">Status</Text>
            <Badge variant={template.is_active ? 'success' : 'neutral'}>
              {template.is_active ? 'Active' : 'Inactive'}
            </Badge>
          </div>
          <DetailRow label="Priority" value={String(template.priority)} />
          <DetailRow
            label="Created"
            value={template.created_at
              ? new Date(template.created_at).toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })
              : '—'}
          />
        </div>

        {/* Form fields */}
        {template.form_fields && template.form_fields.length > 0 && (
          <div className="px-4 py-4 border-b border-outline-variant">
            <Text variant="caption" muted className="uppercase tracking-wider mb-3 block">
              Form Fields ({template.form_fields.length})
            </Text>
            <div className="space-y-2">
              {template.form_fields.map((field, i) => (
                <div key={i} className="flex items-center gap-2 px-3 py-2 bg-surface-container rounded-md">
                  <Badge variant="neutral">{field.field_type}</Badge>
                  <Text variant="body" className="flex-1 truncate">{field.label}</Text>
                  {field.required && (
                    <Text variant="caption" className="text-danger shrink-0">Required</Text>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Step chain */}
        <div className="flex-1 overflow-y-auto px-4 py-4">
          <Text variant="caption" muted className="uppercase tracking-wider mb-3 block">
            Approval Steps ({stepTimeline.length})
          </Text>
          {stepTimeline.length > 0 ? (
            <Timeline items={stepTimeline} />
          ) : (
            <Text variant="body" muted>No steps configured</Text>
          )}
        </div>

        {/* Edit action */}
        <div className="px-4 py-3 border-t border-outline-variant flex-shrink-0">
          <Button
            onClick={() => onEdit(template)}
            className="w-full"
          >
            Edit Template
          </Button>
        </div>
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
