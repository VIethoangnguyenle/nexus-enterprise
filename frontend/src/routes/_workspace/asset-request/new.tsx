import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useAssetTypes, useCreateAssetRequest } from '../../../hooks/useAssets'
import { useWorkspaces } from '../../../hooks/useWorkspaces'
import { useState } from 'react'

export const Route = createFileRoute('/_workspace/asset-request/new')({ component: AssetRequestNew })

function AssetRequestNew() {
  const navigate = useNavigate()
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data: typesData } = useAssetTypes(wsId)
  const createReq = useCreateAssetRequest(wsId)
  const [typeId, setTypeId] = useState('')
  const [justification, setJustification] = useState('')
  const [urgency, setUrgency] = useState('normal')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    createReq.mutate({ type_id: typeId, justification, urgency }, { onSuccess: () => navigate({ to: '/asset-requests' }) })
  }

  return (
    <div className="fade-in" style={{ padding: '1.5rem' }}>
      <button className="btn btn-ghost btn-sm" onClick={() => navigate({ to: '/asset-dashboard' })}>← Back</button>
      <h1 className="page-title" style={{ marginTop: '0.5rem' }}>Request Asset</h1>
      <div className="card" style={{ maxWidth: 640, marginTop: '1rem' }}><div className="card-body">
        <form onSubmit={handleSubmit}>
          {createReq.error && <div className="error-msg">{createReq.error.message}</div>}
          <div className="form-group"><label>Asset Type</label>
            <select value={typeId} onChange={e => setTypeId(e.target.value)} className="filter-select" style={{width:'100%'}}>
              <option value="">Select...</option>
              {(typesData?.types||[]).map(t => <option key={t.id} value={t.id}>{t.name} ({t.category})</option>)}
            </select></div>
          <div className="form-group"><label>Urgency</label>
            <div className="urgency-selector">{['low','normal','high','critical'].map(u => (
              <button key={u} type="button" className={`urgency-btn ${urgency===u?'active':''} urgency-${u}`} onClick={()=>setUrgency(u)}>{u}</button>
            ))}</div></div>
          <div className="form-group"><label>Justification</label>
            <textarea value={justification} onChange={e=>setJustification(e.target.value)} rows={4} placeholder="Explain why..." /></div>
          <button className="btn btn-primary" type="submit" disabled={createReq.isPending||!typeId||!justification.trim()}>
            {createReq.isPending ? <span className="spinner" /> : 'Submit Request'}</button>
        </form></div></div>
    </div>
  )
}
