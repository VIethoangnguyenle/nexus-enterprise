import { createFileRoute } from '@tanstack/react-router'
import { useAssetSummary, useAssetTypes } from '../../hooks/useAssets'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { Heading, Text } from '../../components/primitives'
import { Card } from '../../components/composites'

export const Route = createFileRoute('/_assets/dashboard')({ component: AssetDashboard })

function AssetDashboard() {
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data: summary, isLoading: summaryLoading, error: summaryError, refetch: refetchSummary } = useAssetSummary(wsId)
  const { data: typesData, isLoading: typesLoading, error: typesError, refetch: refetchTypes } = useAssetTypes(wsId)

  const isLoading = summaryLoading || typesLoading
  const error = summaryError || typesError

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState title="Failed to load dashboard" message={error.message} onRetry={() => { refetchSummary(); refetchTypes() }} />

  const stats = [
    { label: 'Total Assets', value: summary?.total || 0, icon: '📦', color: 'bg-accent/10 text-accent' },
    { label: 'In Use', value: summary?.in_use || 0, icon: '✅', color: 'bg-success-bg text-success' },
    { label: 'Pending', value: summary?.pending || 0, icon: '⏳', color: 'bg-warning-bg text-warning' },
    { label: 'Types', value: typesData?.types?.length || 0, icon: '🏷️', color: 'bg-[rgba(139,92,246,0.1)] text-[#a78bfa]' },
  ]

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <Heading as="h2">Asset Dashboard</Heading>
        <Text variant="body" muted className="mt-1">Overview of all managed assets</Text>
      </div>

      <div className="grid grid-cols-4 gap-4">
        {stats.map((s, i) => (
          <Card key={i} className="hover:-translate-y-0.5 transition-transform duration-200">
            <Card.Body className="flex items-center gap-4">
              <div className={`w-11 h-11 rounded-[var(--radius-md)] flex items-center justify-center
                text-lg ${s.color}`}>
                {s.icon}
              </div>
              <div>
                <div className="text-2xl font-bold text-text-primary">{s.value}</div>
                <div className="text-xs text-text-muted uppercase tracking-wider">{s.label}</div>
              </div>
            </Card.Body>
          </Card>
        ))}
      </div>
    </div>
  )
}
