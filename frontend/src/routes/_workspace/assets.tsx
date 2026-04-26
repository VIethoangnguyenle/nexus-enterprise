import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useAssets } from '../../hooks/useAssets'
import { useWorkspaces } from '../../hooks/useWorkspaces'

export const Route = createFileRoute('/_workspace/assets')({ component: AssetList })

function AssetList() {
  const navigate = useNavigate()
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data, isLoading } = useAssets(wsId)
  const assets = data?.assets || []

  const STATE_COLORS: Record<string,string> = { requested:'#f59e0b', approved:'#10b981', assigned:'#3b82f6', in_use:'#8b5cf6', returned:'#6b7280', disposed:'#ef4444' }

  return (
    <div className="fade-in" style={{ padding: '1.5rem' }}>
      <div className="page-header"><div><h1 className="page-title">Assets</h1></div>
        <button className="btn btn-primary" onClick={() => navigate({ to: '/asset-request/new' })}>+ New Request</button></div>
      {isLoading ? <div className="loading-center"><div className="spinner" /></div> :
       assets.length > 0 ? (
        <div className="card"><table className="data-table"><thead><tr><th>Name</th><th>Type</th><th>State</th></tr></thead><tbody>
          {assets.map(a => <tr key={a.id} className="clickable-row" onClick={() => navigate({ to: '/assets/$assetId', params: { assetId: a.id } })}>
            <td className="asset-name-cell">{a.name}</td><td className="text-muted">{a.type_name||''}</td>
            <td><span className="state-badge" style={{background:STATE_COLORS[a.state]||'#6b7280'}}>{a.state}</span></td></tr>)}
        </tbody></table></div>
      ) : <div className="empty-state-large"><div className="empty-icon">📦</div><h3>No assets</h3></div>}
    </div>
  )
}
