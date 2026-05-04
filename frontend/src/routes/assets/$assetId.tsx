import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useAsset, useAssetTransitions, useAssetHistory, useTransitionAsset } from '../../hooks/useAssets'
import { useState } from 'react'
import { LoadingState } from '../../components/LoadingState'
import { ErrorState } from '../../components/ErrorState'
import { ASSET_STATE_COLORS } from '../../lib/constants'
import type { AssetHistory } from '../../api/assets'
import { Button, Badge, Heading, Input, Spinner, Text } from '../../components/primitives'
import { Card, Timeline } from '../../components/composites'

export const Route = createFileRoute('/assets/$assetId')({ component: AssetDetail })

function AssetDetail() {
  const { assetId } = Route.useParams()
  const navigate = useNavigate()
  const { data: asset, isLoading, error, refetch } = useAsset(assetId)
  const { data: transData } = useAssetTransitions(assetId)
  const { data: histData } = useAssetHistory(assetId)
  const transition = useTransitionAsset()
  const [comment, setComment] = useState('')

  if (isLoading) return <LoadingState />
  if (error) return <ErrorState title="Failed to load asset" message={error.message} onRetry={() => refetch()} />
  if (!asset) return <ErrorState title="Asset not found" />

  const history: AssetHistory[] = histData?.history || []

  const timelineItems = history.map((h, i) => ({
    id: `${i}`,
    color: ASSET_STATE_COLORS[h.to_state] || '#6b7280',
    title: h.action,
    timestamp: h.created_at ? new Date(h.created_at).toLocaleDateString() : '',
    body: (
      <div className="flex items-center gap-2 flex-wrap">
        <Badge variant="default">{h.from_state}</Badge>
        <span className="text-on-surface-variant">→</span>
        <Badge variant="primary">{h.to_state}</Badge>
        {h.comment && <span className="text-xs text-on-surface-variant ml-2">— {h.comment}</span>}
      </div>
    ),
  }))

  return (
    <div className="animate-fade-in">
      <Button variant="ghost" size="sm" onClick={() => navigate({ to: '/assets/list' })}>← Back</Button>

      <div className="flex items-center gap-3 mt-3 mb-6">
        <h1 className="font-h1 text-h1 text-on-surface">{asset.name}</h1>
        <Badge variant="primary">{asset.state}</Badge>
      </div>

      {transition.error && (
        <div className="bg-error-container text-on-error-container px-4 py-2 rounded text-sm mb-4">
          {transition.error.message}
        </div>
      )}

      <div className="grid grid-cols-2 gap-4 mb-4">
        {/* Details card */}
        <Card>
          <Card.Header>Details</Card.Header>
          <Card.Body>
            <div className="flex flex-col gap-3">
              <div className="flex justify-between">
                <Text variant="caption" muted>Type</Text>
                <Text variant="body">{asset.type_name || asset.type_id}</Text>
              </div>
              <div className="flex justify-between">
                <Text variant="caption" muted>Assigned To</Text>
                <Text variant="body">{asset.assigned_to || '—'}</Text>
              </div>
            </div>
          </Card.Body>
        </Card>

        {/* Actions card */}
        <Card>
          <Card.Header>Actions</Card.Header>
          <Card.Body>
            {(transData?.transitions || []).length > 0 ? (
              <>
                <div className="flex flex-wrap gap-2 mb-3">
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
                <Input placeholder="Comment (optional)" value={comment} onChange={e => setComment(e.target.value)} />
              </>
            ) : <Text variant="body" muted>No transitions available</Text>}
          </Card.Body>
        </Card>
      </div>

      {/* History */}
      <Card>
        <Card.Header>History</Card.Header>
        <Card.Body>
          <Timeline items={timelineItems} />
        </Card.Body>
      </Card>
    </div>
  )
}
