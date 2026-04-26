import { createFileRoute } from '@tanstack/react-router'
import { useAssetTypes, useCreateAssetType } from '../../hooks/useAssets'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import { useState } from 'react'

export const Route = createFileRoute('/_workspace/asset-types')({ component: AssetTypeConfig })

function AssetTypeConfig() {
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data, isLoading } = useAssetTypes(wsId)
  const types = data?.types || []

  return (
    <div className="fade-in" style={{ padding: '1.5rem' }}>
      <div className="page-header"><div><h1 className="page-title">Asset Type Configuration</h1></div></div>
      {isLoading ? <div className="loading-center"><div className="spinner" /></div> :
       types.length > 0 ? (
        <div className="type-config-grid">{types.map(t => (
          <div key={t.id} className="card type-config-card"><div className="card-header">
            <div className="type-config-info"><span className="type-category-badge">{t.category}</span><h3 className="type-config-name">{t.name}</h3></div></div></div>
        ))}</div>
      ) : <div className="empty-state-large"><div className="empty-icon">🏷️</div><h3>No asset types</h3><p>Create your first asset type.</p></div>}
    </div>
  )
}
