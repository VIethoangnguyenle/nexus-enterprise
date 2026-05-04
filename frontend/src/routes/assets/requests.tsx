import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useAssetRequests, useApproveRequest, useRejectRequest } from '../../hooks/useAssets'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { useState } from 'react'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { EmptyState } from '../../components/EmptyState'
import { REQUEST_STATUS_COLORS } from '../../lib/constants'
import { Button, Badge, Heading, Spinner, Text } from '../../components/primitives'
import { Card, Tabs } from '../../components/composites'
import { ClipboardList, Check, X } from 'lucide-react'

export const Route = createFileRoute('/assets/requests')({ component: AssetRequests })

const statusToBadge = (status: string) => {
  const map: Record<string, 'success' | 'warning' | 'danger' | 'info' | 'default'> = {
    pending: 'warning', approved: 'success', rejected: 'danger',
  }
  return map[status] || 'default'
}

function AssetRequests() {
  const navigate = useNavigate()
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const [filter, setFilter] = useState('pending')
  const { data, isLoading, error, refetch } = useAssetRequests(wsId, filter ? { status: filter } : undefined)
  const approve = useApproveRequest(wsId)
  const reject = useRejectRequest(wsId)
  const requests = data?.requests || []

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState title="Failed to load requests" message={error.message} onRetry={() => refetch()} />

  const mutationError = approve.error || reject.error
  const tabs = [
    { id: 'pending', label: 'Pending' },
    { id: 'approved', label: 'Approved' },
    { id: 'rejected', label: 'Rejected' },
    { id: '', label: 'All' },
  ]

  return (
    <div className="animate-fade-in">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-4 md:mb-6">
        <h1 className="font-h1 text-h1 text-on-surface">Asset Requests</h1>
        <Button onClick={() => navigate({ to: '/assets/request/new' })}>+ New Request</Button>
      </div>

      {mutationError && (
        <div className="bg-error-container text-on-error-container px-4 py-2 rounded text-sm mb-4">
          {mutationError.message}
        </div>
      )}

      <Tabs
        tabs={tabs}
        activeTab={filter}
        onTabChange={setFilter}
        className="mb-5"
      />

      {requests.length > 0 ? (
        <div className="flex flex-col gap-3">
          {requests.map(r => (
            <Card key={r.id} className="hover:border-border-focus transition-colors">
              <Card.Body className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex items-center gap-3">
                  <div>
                    <div className="flex items-center gap-2">
                      <Text variant="body" className="font-medium">{r.type_name || 'Asset'}</Text>
                      <Badge variant={statusToBadge(r.status)}>{r.status}</Badge>
                    </div>
                    {r.justification && (
                      <Text variant="caption" muted className="mt-1">{r.justification}</Text>
                    )}
                  </div>
                </div>
                {r.status === 'pending' && (
                  <div className="flex gap-2">
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={approve.isPending}
                      onClick={() => approve.mutate(r.id)}
                    >
                      {approve.isPending ? <Spinner size="sm" /> : <><Check size={12} className="inline" /> Approve</>}
                    </Button>
                    <Button
                      variant="danger"
                      size="sm"
                      disabled={reject.isPending}
                      onClick={() => reject.mutate({ id: r.id, reason: 'Rejected' })}
                    >
                      {reject.isPending ? <Spinner size="sm" /> : <><X size={12} className="inline" /> Reject</>}
                    </Button>
                  </div>
                )}
              </Card.Body>
            </Card>
          ))}
        </div>
      ) : <EmptyState icon={<ClipboardList size={40} className="text-outline" strokeWidth={1.5} />} title="No requests" description="No requests match the current filter." />}
    </div>
  )
}
