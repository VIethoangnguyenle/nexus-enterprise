import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useAssets } from '../../hooks/useAssets'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { EmptyState } from '../../components/EmptyState'
import { ASSET_STATE_COLORS } from '../../lib/constants'
import { Button, Badge, Heading } from '../../components/primitives'
import { Card } from '../../components/composites'

export const Route = createFileRoute('/_assets/list')({ component: AssetList })

const stateToBadge = (state: string) => {
  const map: Record<string, 'success' | 'warning' | 'danger' | 'info' | 'primary' | 'default'> = {
    in_use: 'success', available: 'info', pending: 'warning', retired: 'danger', maintenance: 'warning',
  }
  return map[state] || 'default'
}

function AssetList() {
  const navigate = useNavigate()
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data, isLoading, error, refetch } = useAssets(wsId)

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState title="Failed to load assets" message={error.message} onRetry={() => refetch()} />

  const assets = data?.assets || []

  return (
    <div className="animate-fade-in">
      <div className="flex items-center justify-between mb-6">
        <Heading as="h2">Assets</Heading>
        <Button onClick={() => navigate({ to: '/assets/request/new' })}>+ New Request</Button>
      </div>

      {assets.length > 0 ? (
        <Card>
          <table className="w-full border-collapse">
            <thead>
              <tr>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">Name</th>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">Type</th>
                <th className="text-left px-4 py-2.5 text-[0.7rem] font-semibold text-text-muted uppercase tracking-wider border-b border-border">State</th>
              </tr>
            </thead>
            <tbody>
              {assets.map(a => (
                <tr key={a.id}
                  onClick={() => navigate({ to: '/assets/$assetId', params: { assetId: a.id } })}
                  className="border-b border-border/50 hover:bg-bg-hover transition-colors cursor-pointer">
                  <td className="px-4 py-3 text-sm font-medium text-text-primary">{a.name}</td>
                  <td className="px-4 py-3 text-sm text-text-muted">{a.type_name || ''}</td>
                  <td className="px-4 py-3">
                    <Badge variant={stateToBadge(a.state)}>{a.state}</Badge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      ) : <EmptyState icon="📦" title="No assets" description="Request your first asset to get started." action={{ label: '+ New Request', onClick: () => navigate({ to: '/assets/request/new' }) }} />}
    </div>
  )
}
