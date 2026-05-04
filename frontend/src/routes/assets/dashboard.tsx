import { createFileRoute } from '@tanstack/react-router'
import { useAssetSummary, useAssetTypes } from '../../hooks/useAssets'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { Heading, Text } from '../../components/primitives'
import { Card } from '../../components/composites'
import { Package, CheckCircle, Clock, Tag } from 'lucide-react'
import type { ReactNode } from 'react'

export const Route = createFileRoute('/assets/dashboard')({ component: AssetDashboard })

function AssetDashboard() {
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data: summary, isLoading: summaryLoading, error: summaryError, refetch: refetchSummary } = useAssetSummary(wsId)
  const { data: typesData, isLoading: typesLoading, error: typesError, refetch: refetchTypes } = useAssetTypes(wsId)

  const isLoading = summaryLoading || typesLoading
  const error = summaryError || typesError

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState title="Failed to load dashboard" message={error.message} onRetry={() => { refetchSummary(); refetchTypes() }} />

  const stats: { label: string; value: number; icon: ReactNode; color: string }[] = [
    { label: 'Total Assets', value: summary?.total || 0, icon: <Package size={20} />, color: 'bg-primary-container text-on-primary-container' },
    { label: 'In Use', value: summary?.in_use || 0, icon: <CheckCircle size={20} />, color: 'bg-tertiary-container text-on-tertiary-container' },
    { label: 'Pending', value: summary?.pending || 0, icon: <Clock size={20} />, color: 'bg-secondary-container text-on-secondary-container' },
    { label: 'Types', value: typesData?.types?.length || 0, icon: <Tag size={20} />, color: 'bg-surface-variant text-on-surface-variant' },
  ]

  return (
    <div className="animate-fade-in">
      <div className="mb-6">
        <h1 className="font-h1 text-h1 text-on-surface">Asset Dashboard</h1>
        <p className="text-sm text-on-surface-variant mt-1">Overview of all managed assets</p>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4">
        {stats.map((s, i) => (
          <Card key={i} className="hover:-translate-y-0.5 transition-transform duration-200">
            <Card.Body className="flex items-center gap-4">
              <div className={`w-11 h-11 rounded-md flex items-center justify-center
                text-section ${s.color}`}>
                {s.icon}
              </div>
              <div>
                <div className="font-h2 text-h2 text-on-surface">{s.value}</div>
                <div className="text-[11px] font-bold uppercase tracking-widest text-on-surface-variant">{s.label}</div>
              </div>
            </Card.Body>
          </Card>
        ))}
      </div>
    </div>
  )
}
