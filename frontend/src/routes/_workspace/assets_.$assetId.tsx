import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useAsset, useAssetTransitions, useAssetHistory, useTransitionAsset } from '../../hooks/useAssets'
import { useState } from 'react'

export const Route = createFileRoute('/_workspace/assets_/$assetId')({ component: AssetDetail })

function AssetDetail() {
  const { assetId } = Route.useParams()
  const navigate = useNavigate()
  const { data: asset, isLoading } = useAsset(assetId)
  const { data: transData } = useAssetTransitions(assetId)
  const { data: histData } = useAssetHistory(assetId)
  const transition = useTransitionAsset()
  const [comment, setComment] = useState('')

  const STATE_COLORS: Record<string,string> = { requested:'#f59e0b', approved:'#10b981', assigned:'#3b82f6', in_use:'#8b5cf6', returned:'#6b7280', disposed:'#ef4444' }

  if (isLoading) return <div className="loading-center"><div className="spinner" /></div>
  if (!asset) return <div className="empty-state-large"><h3>Asset not found</h3></div>

  return (
    <div className="fade-in" style={{ padding: '1.5rem' }}>
      <button className="btn btn-ghost btn-sm" onClick={() => navigate({ to: '/assets' })}>← Back</button>
      <div className="page-header" style={{ marginTop: '0.5rem' }}>
        <div><h1 className="page-title">{asset.name}</h1>
          <span className="state-badge" style={{background:STATE_COLORS[asset.state]||'#6b7280'}}>{asset.state}</span></div></div>

      <div className="dashboard-grid">
        <div className="card"><div className="card-header"><h3>Details</h3></div><div className="card-body">
          <div className="detail-grid">
            <div className="detail-row"><span className="detail-label">Type</span><span className="detail-value">{asset.type_name||asset.type_id}</span></div>
            <div className="detail-row"><span className="detail-label">Assigned To</span><span className="detail-value">{asset.assigned_to||'—'}</span></div>
          </div></div></div>

        <div className="card transition-card"><div className="card-header"><h3>Actions</h3></div><div className="card-body">
          {(transData?.transitions||[]).length > 0 ? <>
            <div className="transition-actions">
              {transData!.transitions.map(t => (
                <button key={t.action} className="btn-action" disabled={transition.isPending}
                  onClick={() => transition.mutate({ id: assetId, action: t.action, comment })}>{t.action} → <span className="transition-target">{t.to}</span></button>
              ))}
            </div>
            <input className="transition-comment" style={{marginTop:'0.75rem'}} placeholder="Comment (optional)" value={comment} onChange={e=>setComment(e.target.value)} />
          </> : <p className="text-muted">No transitions available</p>}
        </div></div>
      </div>

      <div className="card" style={{ marginTop: '1rem' }}><div className="card-header"><h3>History</h3></div><div className="card-body">
        <div className="timeline">{(histData?.history||[]).map((h:any,i:number) => (
          <div key={i} className="timeline-item"><div className="timeline-dot" style={{background:STATE_COLORS[h.to_state]||'#6b7280'}} />
            <div className="timeline-content"><div className="timeline-header"><span className="timeline-action">{h.action}</span><span className="timeline-date">{h.created_at?new Date(h.created_at).toLocaleDateString():''}</span></div>
              <div className="timeline-body"><span className="state-badge state-badge-sm" style={{background:STATE_COLORS[h.from_state]||'#6b7280'}}>{h.from_state}</span><span className="timeline-arrow">→</span><span className="state-badge state-badge-sm" style={{background:STATE_COLORS[h.to_state]||'#6b7280'}}>{h.to_state}</span></div>
              {h.comment && <p className="timeline-comment">{h.comment}</p>}</div></div>
        ))}</div></div></div>
    </div>
  )
}
