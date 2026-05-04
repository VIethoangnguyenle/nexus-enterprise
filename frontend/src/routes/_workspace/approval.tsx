import { createFileRoute } from '@tanstack/react-router'
import { useState, useCallback, useMemo } from 'react'
import {
  useApprovalPending,
  useApprovalHistory,
  useApprovalMyRequests,
  useApprovalDepartment,
  useApprovalTemplates,
  useApprove,
  useReject,
  useBatchApprove,
} from '../../hooks/useApproval'
import { DataTable } from '../../components/composites/DataTable'
import { ApprovalDetailPanel } from '../../components/approval/ApprovalDetailPanel'
import { ApprovalStatusBadge } from '../../components/approval/ApprovalStatusBadge'
import { BatchActionBar } from '../../components/approval/BatchActionBar'
import { CreateRequestModal } from '../../components/approval/CreateRequestModal'
import { CreateTemplateModal } from '../../components/approval/CreateTemplateModal'
import { TemplateDetailPanel } from '../../components/approval/TemplateDetailPanel'
import { Tabs } from '../../components/composites/Tabs'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { EmptyState } from '../../components/EmptyState'
import { Button, Badge, Heading } from '../../components/primitives'
import { IconButton } from '../../components/primitives'
import { ClipboardCheck, History, FileText, Building2, Settings, Plus, Check, X, Pencil } from 'lucide-react'
import { ResponsiveDetailPanel } from '../../components/composites/ResponsiveDetailPanel'
import type { ApprovalRequest, ApprovalTemplate, RequestWithAssignment } from '../../api/approval'

export const Route = createFileRoute('/_workspace/approval')({
  component: ApprovalPage,
})

type TabId = 'pending' | 'history' | 'my-requests' | 'department' | 'templates'

/** Approval module — 5-tab list view with detail panel, batch actions, template management, and create request. */
function ApprovalPage() {
  const [activeTab, setActiveTab] = useState<TabId>('pending')
  const [selectedItem, setSelectedItem] = useState<ApprovalRequest | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [showCreateRequest, setShowCreateRequest] = useState(false)
  const [showCreateTemplate, setShowCreateTemplate] = useState(false)
  const [editTemplate, setEditTemplate] = useState<ApprovalTemplate | null>(null)
  const [selectedTemplate, setSelectedTemplate] = useState<ApprovalTemplate | null>(null)

  // Cursor pagination state per tab
  const [historyCursor, setHistoryCursor] = useState<string | undefined>()
  const [myRequestsCursor, setMyRequestsCursor] = useState<string | undefined>()
  const [deptCursor, setDeptCursor] = useState<string | undefined>()

  // Queries
  const pending = useApprovalPending()
  const history = useApprovalHistory(historyCursor)
  const myRequests = useApprovalMyRequests(myRequestsCursor)
  const department = useApprovalDepartment(deptCursor)
  const templates = useApprovalTemplates(undefined, false)

  // Mutations
  const approveMut = useApprove()
  const rejectMut = useReject()
  const batchApproveMut = useBatchApprove()

  // Active tab data
  const activeQuery = useMemo(() => {
    switch (activeTab) {
      case 'pending': return pending
      case 'history': return history
      case 'my-requests': return myRequests
      case 'department': return department
      case 'templates': return templates
    }
  }, [activeTab, pending, history, myRequests, department, templates])

  // Normalize items: pending/history return {request, assignment}, my-requests/department return flat requests
  const items: ApprovalRequest[] = useMemo(() => {
    if (activeTab === 'templates') return []
    const raw = (activeQuery.data as { items?: unknown[] })?.items || []
    if (raw.length === 0) return []
    // Detect wrapped vs flat: wrapped items have .request field
    const first = raw[0] as Record<string, unknown>
    if ('request' in first) {
      // Unwrap RequestWithAssignment → ApprovalRequest
      return (raw as RequestWithAssignment[]).map(rwa => ({
        ...rwa.request,
        assignments: rwa.assignment ? [rwa.assignment] : [],
      }))
    }
    return raw as ApprovalRequest[]
  }, [activeTab, activeQuery.data])
  const templateList: ApprovalTemplate[] = activeTab === 'templates' ? (templates.data as { templates?: ApprovalTemplate[] })?.templates || [] : []
  const pendingCount = pending.data?.total || 0

  // Tab definitions with pending badge
  const tabs = useMemo(() => [
    { id: 'pending' as const, label: pendingCount > 0 ? `Pending (${pendingCount})` : 'Pending', icon: <ClipboardCheck size={16} /> },
    { id: 'history' as const, label: 'History', icon: <History size={16} /> },
    { id: 'my-requests' as const, label: 'My Requests', icon: <FileText size={16} /> },
    { id: 'department' as const, label: 'Department', icon: <Building2 size={16} /> },
    { id: 'templates' as const, label: 'Templates', icon: <Settings size={16} /> },
  ], [pendingCount])

  // Selection handlers
  const handleToggleSelect = useCallback((id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }, [])

  const handleSelectAll = useCallback(() => {
    setSelectedIds(prev =>
      prev.size === items.length ? new Set() : new Set(items.map(i => i.id))
    )
  }, [items])

  const handleClearSelection = useCallback(() => setSelectedIds(new Set()), [])

  // Action handlers
  const handleApprove = useCallback((id: string, comment = '') => {
    approveMut.mutate({ requestId: id, comment }, {
      onSuccess: () => setSelectedItem(null),
    })
  }, [approveMut])

  const handleReject = useCallback((id: string, comment: string) => {
    rejectMut.mutate({ requestId: id, comment }, {
      onSuccess: () => setSelectedItem(null),
    })
  }, [rejectMut])

  const handleBatchApprove = useCallback(() => {
    batchApproveMut.mutate({ requestIds: Array.from(selectedIds) }, {
      onSuccess: () => setSelectedIds(new Set()),
    })
  }, [batchApproveMut, selectedIds])

  const handleRowClick = useCallback((item: ApprovalRequest) => {
    setSelectedItem(item)
  }, [])

  // Tab change resets selection
  const handleTabChange = useCallback((id: string) => {
    setActiveTab(id as TabId)
    setSelectedIds(new Set())
    setSelectedItem(null)
    setSelectedTemplate(null)
  }, [])

  // DataTable columns for requests
  const requestColumns = useMemo(() => [
    {
      id: 'request',
      header: 'Request',
      cell: (row: ApprovalRequest) => (
        <div>
          <div className="text-small font-medium text-on-surface truncate">{row.template_name}</div>
          <div className="text-caption text-on-surface-variant truncate">{row.entity_type}</div>
        </div>
      ),
      className: 'w-[30%]',
    },
    {
      id: 'status',
      header: 'Status',
      cell: (row: ApprovalRequest) => <ApprovalStatusBadge status={row.status} />,
      className: 'w-[15%] hidden md:table-cell',
    },
    {
      id: 'step',
      header: 'Step',
      cell: (row: ApprovalRequest) => (
        <span className="text-small text-on-surface-variant">
          {row.status === 'pending'
            ? `Step ${row.current_step}/${row.assignments?.length || '?'}`
            : '—'}
        </span>
      ),
      className: 'w-[15%] hidden md:table-cell',
    },
    {
      id: 'date',
      header: 'Date',
      cell: (row: ApprovalRequest) => (
        <span className="text-small text-on-surface-variant">{formatRelativeDate(row.created_at)}</span>
      ),
      className: 'w-[15%] hidden md:table-cell',
    },
    ...(activeTab === 'pending' ? [{
      id: 'actions',
      header: 'Actions',
      cell: (row: ApprovalRequest) => (
        <div className="flex items-center justify-end gap-1">
          <IconButton
            aria-label="Approve"
            onClick={(e: React.MouseEvent) => { e.stopPropagation(); handleApprove(row.id) }}
            size="sm"
            className="text-success hover:bg-success/10"
          >
            <Check size={14} />
          </IconButton>
          <IconButton
            aria-label="Reject"
            onClick={(e: React.MouseEvent) => { e.stopPropagation(); setSelectedItem(row) }}
            size="sm"
            className="text-danger hover:bg-danger/10"
          >
            <X size={14} />
          </IconButton>
        </div>
      ),
      className: 'w-[15%] text-right hidden md:table-cell',
    }] : []),
  ], [activeTab, handleApprove])

  // DataTable columns for templates
  const templateColumns = useMemo(() => [
    {
      id: 'name',
      header: 'Template Name',
      cell: (row: ApprovalTemplate) => (
        <span className="text-small font-medium text-on-surface">{row.name}</span>
      ),
      className: 'w-[30%]',
    },
    {
      id: 'entity_type',
      header: 'Type',
      cell: (row: ApprovalTemplate) => (
        <Badge variant="neutral">{row.entity_type}</Badge>
      ),
      className: 'w-[15%]',
    },
    {
      id: 'fields',
      header: 'Fields',
      cell: (row: ApprovalTemplate) => (
        <span className="text-small text-on-surface-variant">{row.form_fields?.length || 0} fields</span>
      ),
      className: 'w-[10%] hidden md:table-cell',
    },
    {
      id: 'steps',
      header: 'Steps',
      cell: (row: ApprovalTemplate) => (
        <span className="text-small text-on-surface-variant">{row.step_count ?? row.steps?.length ?? 0} steps</span>
      ),
      className: 'w-[10%] hidden md:table-cell',
    },
    {
      id: 'status',
      header: 'Status',
      cell: (row: ApprovalTemplate) => (
        <Badge variant={row.is_active ? 'success' : 'neutral'}>
          {row.is_active ? 'Active' : 'Inactive'}
        </Badge>
      ),
      className: 'w-[10%] hidden md:table-cell',
    },
    {
      id: 'actions',
      header: '',
      cell: (row: ApprovalTemplate) => (
        <IconButton
          aria-label="Edit template"
          onClick={(e: React.MouseEvent) => { e.stopPropagation(); setEditTemplate(row) }}
          size="sm"
        >
          <Pencil size={14} />
        </IconButton>
      ),
      className: 'w-[5%] text-right',
    },
  ], [])

  // Pagination
  const nextCursor = activeTab !== 'templates'
    ? (activeQuery.data as { next_cursor?: string } | undefined)?.next_cursor
    : undefined

  // Empty state config per tab
  const emptyConfig: Record<TabId, { title: string; desc: string }> = {
    pending:       { title: 'No pending approvals', desc: 'Items assigned to you will appear here' },
    history:       { title: 'No approval history', desc: 'Items you\'ve acted on will appear here' },
    'my-requests': { title: 'No requests submitted', desc: 'Approval requests you create will appear here' },
    department:    { title: 'No department requests', desc: 'Department-scoped requests will appear here' },
    templates:     { title: 'No templates configured', desc: 'Create a template to get started' },
  }

  return (
    <div className="flex h-full">
      {/* Main content */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Header */}
        <div className="px-4 md:px-6 pt-4 md:pt-6 pb-0 flex items-center justify-between">
          <Heading as="h2">Approvals</Heading>
          <Button onClick={() => setShowCreateRequest(true)}>
            <Plus size={16} />
            <span className="hidden sm:inline ml-1">New Request</span>
          </Button>
        </div>

        {/* Tabs */}
        <div className="px-4 md:px-6 pt-4 overflow-x-auto">
          <Tabs tabs={tabs} activeId={activeTab} onChange={handleTabChange} className="whitespace-nowrap" />
        </div>

        {/* Content — flex-1 wrapper ensures empty states center vertically */}
        <div className="flex-1 flex flex-col min-h-0">
          {activeQuery.isLoading ? (
            <div className="flex-1 flex items-center justify-center">
              <LoadingState />
            </div>
          ) : activeQuery.error ? (
            (() => {
              const errMsg = (activeQuery.error as Error).message || ''
              const isNotProvisioned = errMsg.includes('not provisioned') || errMsg.includes('404')
              return isNotProvisioned ? (
                <div className="flex-1 flex items-center justify-center">
                  <EmptyState
                    icon={<ClipboardCheck size={40} className="text-outline" strokeWidth={1.5} />}
                    title="Approvals not configured"
                    description="The approval workflow hasn't been set up for this workspace yet. Contact your administrator."
                  />
                </div>
              ) : (
                <div className="flex-1 flex items-center justify-center">
                  <ErrorState
                    title="Failed to load approvals"
                    message={errMsg}
                    onRetry={() => activeQuery.refetch()}
                  />
                </div>
              )
            })()
          ) : activeTab === 'templates' ? (
            /* Templates tab */
            templateList.length === 0 ? (
              <div className="flex-1 flex items-center justify-center">
                <EmptyState
                  icon={<Settings size={40} className="text-outline" strokeWidth={1.5} />}
                  title={emptyConfig.templates.title}
                  description={emptyConfig.templates.desc}
                  action={{
                    label: 'Create Template',
                    onClick: () => setShowCreateTemplate(true),
                  }}
                />
              </div>
            ) : (
              <div className="flex-1 px-4 md:px-6 py-4 overflow-auto">
                <div className="flex justify-end mb-3">
                  <Button variant="ghost" onClick={() => setShowCreateTemplate(true)}>
                    <Plus size={14} />
                    New Template
                  </Button>
                </div>
                <DataTable
                  columns={templateColumns}
                  data={templateList}
                  keyExtractor={(t) => t.id}
                  onRowClick={(t) => setSelectedTemplate(t)}
                  selectedKey={selectedTemplate?.id}
                />
              </div>
            )
          ) : items.length === 0 ? (
            <div className="flex-1 flex items-center justify-center">
              <EmptyState
                icon={<ClipboardCheck size={40} className="text-outline" strokeWidth={1.5} />}
                title={emptyConfig[activeTab].title}
                description={emptyConfig[activeTab].desc}
              />
            </div>
          ) : (
            <>
              <div className="flex-1 px-4 md:px-6 py-4 overflow-auto">
                <DataTable
                  columns={requestColumns}
                  data={items}
                  keyExtractor={(item) => item.id}
                  onRowClick={handleRowClick}
                  selectedKey={selectedItem?.id}
                />
              </div>

              {/* Load more for paginated tabs */}
              {nextCursor && (
                <div className="flex justify-center py-4">
                  <Button
                    variant="ghost"
                    onClick={() => {
                      if (activeTab === 'history') setHistoryCursor(nextCursor)
                      if (activeTab === 'my-requests') setMyRequestsCursor(nextCursor)
                      if (activeTab === 'department') setDeptCursor(nextCursor)
                    }}
                  >
                    Load More
                  </Button>
                </div>
              )}
            </>
          )}
        </div>
      </div>

      {/* Detail panel */}
      {selectedItem && (
        <ResponsiveDetailPanel>
          <ApprovalDetailPanel
            item={selectedItem}
            onClose={() => setSelectedItem(null)}
            onApprove={handleApprove}
            onReject={handleReject}
            isApproving={approveMut.isPending}
            isRejecting={rejectMut.isPending}
          />
        </ResponsiveDetailPanel>
      )}

      {/* Template detail panel */}
      {selectedTemplate && (
        <ResponsiveDetailPanel>
          <TemplateDetailPanel
            template={selectedTemplate}
            onClose={() => setSelectedTemplate(null)}
            onEdit={(t) => { setEditTemplate(t); setSelectedTemplate(null) }}
          />
        </ResponsiveDetailPanel>
      )}

      {/* Batch action bar */}
      <BatchActionBar
        selectedCount={selectedIds.size}
        onBatchApprove={handleBatchApprove}
        onClear={handleClearSelection}
        isProcessing={batchApproveMut.isPending}
      />

      {/* Modals */}
      {showCreateRequest && (
        <CreateRequestModal
          onClose={() => setShowCreateRequest(false)}
          defaultScopeOaId=""
          defaultDepartmentId=""
        />
      )}
      {(showCreateTemplate || editTemplate) && (
        <CreateTemplateModal
          onClose={() => { setShowCreateTemplate(false); setEditTemplate(null) }}
          editTemplate={editTemplate}
        />
      )}
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
