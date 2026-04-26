import { createFileRoute } from '@tanstack/react-router'
import { useAssetSummary, useAssetTypes } from '../../hooks/useAssets'
import { useWorkspaces } from '../../hooks/useWorkspaces'

export const Route = createFileRoute('/_workspace/asset-dashboard')({ component: AssetDashboard })

function AssetDashboard() {
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data: summary } = useAssetSummary(wsId)
  const { data: typesData } = useAssetTypes(wsId)

  return (
    <div className="fade-in" style={{ padding: '1.5rem' }}>
      <div className="page-header"><div><h1 className="page-title">Asset Dashboard</h1><p className="page-subtitle">Overview of all managed assets</p></div></div>
      <div className="stats-grid">
        {[{label:'Total Assets',value:summary?.total||0,icon:'📦',bg:'rgba(99,102,241,0.1)'},
          {label:'In Use',value:summary?.in_use||0,icon:'✅',bg:'rgba(16,185,129,0.1)'},
          {label:'Pending',value:summary?.pending||0,icon:'⏳',bg:'rgba(245,158,11,0.1)'},
          {label:'Types',value:typesData?.types?.length||0,icon:'🏷️',bg:'rgba(139,92,246,0.1)'}
        ].map((s,i)=>(
          <div key={i} className="stat-card"><div className="stat-icon" style={{background:s.bg}}>{s.icon}</div><div><div className="stat-value">{s.value}</div><div className="stat-label">{s.label}</div></div></div>
        ))}
      </div>
    </div>
  )
}
