import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useAssetRequests, useApproveRequest, useRejectRequest } from '../../hooks/useAssets'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { useState } from 'react'

export const Route = createFileRoute('/_workspace/asset-requests')({ component: AssetRequests })

function AssetRequests() {
  const navigate = useNavigate()
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const [filter, setFilter] = useState('pending')
  const { data, isLoading } = useAssetRequests(wsId, filter ? { status: filter } : undefined)
  const approve = useApproveRequest(wsId)
  const reject = useRejectRequest(wsId)
  const requests = data?.requests || []

  const STATUS_COLORS: Record<string,string> = { pending:'#f59e0b', approved:'#10b981', rejected:'#ef4444' }

  return (
    <div className="fade-in" style={{ padding: '1.5rem' }}>
      <div className="page-header"><div><h1 className="page-title">Asset Requests</h1></div>
        <button className="btn btn-primary" onClick={() => navigate({ to: '/asset-request/new' })}>+ New Request</button></div>
      <div className="tabs" style={{maxWidth:400,marginBottom:'1.5rem'}}>
        {['pending','approved','rejected',''].map(s => <button key={s} className={`tab ${filter===s?'active':''}`} onClick={()=>setFilter(s)}>{s||'All'}</button>)}
      </div>
      {isLoading ? <div className="loading-center"><div className="spinner" /></div> :
       requests.length > 0 ? (
        <div className="request-list">{requests.map(r => (
          <div key={r.id} className="card request-card"><div className="card-body">
            <div className="request-header"><div className="request-info"><span className="request-type">{r.type_name||'Asset'}</span>
              <span className="state-badge" style={{background:STATUS_COLORS[r.status]||'#6b7280'}}>{r.status}</span></div></div>
            {r.justification && <p className="request-justification">{r.justification}</p>}
            {r.status==='pending' && <div className="request-actions">
              <button className="btn btn-success btn-sm" disabled={approve.isPending} onClick={()=>approve.mutate(r.id)}>✓ Approve</button>
              <button className="btn btn-danger btn-sm" onClick={()=>reject.mutate({id:r.id,reason:'Rejected'})}>✕ Reject</button></div>}
          </div></div>
        ))}</div>
      ) : <div className="empty-state-large"><div className="empty-icon">📋</div><h3>No requests</h3></div>}
    </div>
  )
}
