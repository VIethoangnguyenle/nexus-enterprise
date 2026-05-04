import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState, useMemo } from 'react'
import { useAssets, useAsset, useAssetTransitions, useAssetHistory, useTransitionAsset } from '../../hooks/useAssets'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { EmptyState } from '../../components/EmptyState'
import { ASSET_STATE_COLORS } from '../../lib/constants'
import { Button, Badge, Heading, Text, Input, Spinner } from '../../components/primitives'
import { DataTable, PeekPanel, Timeline } from '../../components/composites'
import { Package } from 'lucide-react'
import type { AssetHistory } from '../../api/assets'

export const Route = createFileRoute('/assets/list')({ component: AssetList })

const STATE_OPTIONS = ['all', 'available', 'in_use', 'pending', 'maintenance', 'retired'] as const

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
  const [stateFilter, setStateFilter] = useState<string>('all')
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const assets = data?.assets || []
  const filtered = useMemo(() =>
    stateFilter === 'all' ? assets : assets.filter(a => a.state === stateFilter),
    [assets, stateFilter],
  )

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState title="Failed to load assets" message={error.message} onRetry={() => refetch()} />

  return (
    <div className="flex h-full animate-fade-in">
      {/* Main content */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Top bar */}
        <div className="h-[52px] px-4 md:px-6 flex items-center justify-between border-b border-outline-variant flex-shrink-0">
          <h1 className="font-h3 text-h3 text-on-surface">Assets</h1>
          <Button size="sm" onClick={() => navigate({ to: '/assets/request/new' })}>+ New</Button>
        </div>

        {/* Filter bar */}
        <div className="px-4 md:px-6 py-2 flex items-center gap-2 border-b border-outline-variant flex-shrink-0 overflow-x-auto scrollbar-none">
          {STATE_OPTIONS.map((s) => (
            <button
              key={s}
              onClick={() => setStateFilter(s)}
              className={`h-8 px-3 text-sm rounded-full border cursor-pointer
                transition-colors capitalize
                ${stateFilter === s
                  ? 'bg-primary-container text-on-primary-container border-primary-container font-medium'
                  : 'bg-transparent text-on-surface-variant border-outline-variant hover:bg-surface-container'}`}
            >
              {s === 'all' ? 'All' : s.replace('_', ' ')}
            </button>
          ))}
          <span className="text-xs text-on-surface-variant ml-auto">{filtered.length} items</span>
        </div>

        {/* Table */}
        {filtered.length > 0 ? (
          <DataTable
            columns={[
              { id: 'name', header: 'Name', cell: (a) => <span className="font-medium text-on-surface">{a.name}</span> },
              { id: 'type', header: 'Type', cell: (a) => <span className="text-on-surface-variant">{a.type_name || ''}</span> },
              { id: 'state', header: 'State', cell: (a) => <Badge variant={stateToBadge(a.state)}>{a.state}</Badge> },
              { id: 'assigned', header: 'Assigned To', cell: (a) => <span className="text-on-surface-variant">{a.assigned_to || '—'}</span> },
            ]}
            data={filtered}
            keyExtractor={(a) => a.id}
            selectedKey={selectedId}
            onRowClick={(a) => setSelectedId(a.id === selectedId ? null : a.id)}
          />
        ) : (
          <div className="flex-1 flex items-center justify-center">
            <EmptyState
              icon={<Package size={40} className="text-outline" strokeWidth={1.5} />}
              title="No assets"
              description={stateFilter === 'all'
                ? 'Request your first asset to get started.'
                : `No assets with state "${stateFilter.replace('_', ' ')}".`}
              action={{ label: '+ New Request', onClick: () => navigate({ to: '/assets/request/new' }) }}
            />
          </div>
        )}
      </div>

      {/* Peek panel for selected asset */}
      {selectedId && (
        <AssetPeekPanel
          assetId={selectedId}
          onClose={() => setSelectedId(null)}
          onOpenFull={() => navigate({ to: '/assets/$assetId', params: { assetId: selectedId } })}
        />
      )}
    </div>
  )
}

/** Peek panel showing asset details, transitions, and history inline. */
function AssetPeekPanel({ assetId, onClose, onOpenFull }: { assetId: string; onClose: () => void; onOpenFull: () => void }) {
  const { data: asset } = useAsset(assetId)
  const { data: transData } = useAssetTransitions(assetId)
  const { data: histData } = useAssetHistory(assetId)
  const transition = useTransitionAsset()
  const [comment, setComment] = useState('')

  if (!asset) return null

  const history: AssetHistory[] = histData?.history || []
  const timelineItems = history.slice(0, 5).map((h, i) => ({
    id: `${i}`,
    color: ASSET_STATE_COLORS[h.to_state] || '#6b7280',
    title: h.action,
    timestamp: h.created_at ? new Date(h.created_at).toLocaleDateString() : '',
    body: (
      <div className="flex items-center gap-1 flex-wrap">
        <Badge variant="default">{h.from_state}</Badge>
        <span className="text-on-surface-variant">→</span>
        <Badge variant="primary">{h.to_state}</Badge>
      </div>
    ),
  }))

  return (
    <PeekPanel title={asset.name} onClose={onClose}>
      <div className="p-4 space-y-4">
        {/* State + type */}
        <div className="space-y-3">
          <div>
            <span className="text-[11px] font-bold uppercase tracking-widest text-on-surface-variant">State</span>
            <div className="mt-1"><Badge variant={stateToBadge(asset.state)}>{asset.state}</Badge></div>
          </div>
          <div>
            <span className="text-[11px] font-bold uppercase tracking-widest text-on-surface-variant">Type</span>
            <p className="text-sm text-on-surface mt-1">{asset.type_name || asset.type_id}</p>
          </div>
          <div>
            <span className="text-[11px] font-bold uppercase tracking-widest text-on-surface-variant">Assigned To</span>
            <p className="text-sm text-on-surface mt-1">{asset.assigned_to || '—'}</p>
          </div>
        </div>

        {/* Actions */}
        {(transData?.transitions || []).length > 0 && (
          <div className="border-t border-outline-variant pt-3">
            <span className="text-[11px] font-bold uppercase tracking-widest text-on-surface-variant">Actions</span>
            <div className="flex flex-wrap gap-2 mt-2">
              {transData!.transitions.map(t => (
                <Button
                  key={t.action}
                  variant="secondary"
                  size="sm"
                  disabled={transition.isPending}
                  onClick={() => transition.mutate({ id: assetId, action: t.action, comment })}
                >
                  {transition.isPending ? <Spinner size="sm" /> : <>{t.action} → <span className="text-primary">{t.to}</span></>}
                </Button>
              ))}
            </div>
            <Input placeholder="Comment (optional)" value={comment} onChange={e => setComment(e.target.value)} className="mt-2" />
          </div>
        )}

        {/* Recent history */}
        {timelineItems.length > 0 && (
          <div className="border-t border-outline-variant pt-3">
            <span className="text-[11px] font-bold uppercase tracking-widest text-on-surface-variant">Recent History</span>
            <div className="mt-2">
              <Timeline items={timelineItems} />
            </div>
          </div>
        )}

        {/* Full details link */}
        <div className="border-t border-outline-variant pt-3">
          <Button variant="ghost" size="sm" onClick={onOpenFull} className="w-full">
            Open full details →
          </Button>
        </div>
      </div>
    </PeekPanel>
  )
}

