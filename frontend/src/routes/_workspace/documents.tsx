import { createFileRoute } from '@tanstack/react-router'
import { useDocuments } from '../../hooks/useDocuments'
import { useWorkspaces } from '../../hooks/useWorkspaces'

export const Route = createFileRoute('/_workspace/documents')({ component: DocumentsPage })

function DocumentsPage() {
  const { data: wsData } = useWorkspaces()
  const wsId = wsData?.workspaces?.[0]?.id || ''
  const { data, isLoading } = useDocuments(wsId)
  const docs = data?.documents || []

  return (
    <div className="fade-in" style={{ padding: '1.5rem' }}>
      <div className="page-header"><div><h1 className="page-title">Documents</h1></div></div>
      {isLoading ? <div className="loading-center"><div className="spinner" /></div> :
       docs.length > 0 ? (
        <div className="card"><table className="data-table"><thead><tr><th>Name</th><th>State</th><th>Created</th></tr></thead><tbody>
          {docs.map(d => <tr key={d.id}><td className="asset-name-cell">{d.name}</td><td><span className="state-badge" style={{background:'#6366f1'}}>{d.state}</span></td><td className="text-muted">{d.created_at ? new Date(d.created_at).toLocaleDateString() : ''}</td></tr>)}
        </tbody></table></div>
      ) : <div className="empty-state-large"><div className="empty-icon">📄</div><h3>No documents</h3><p>Upload your first document to get started.</p></div>}
    </div>
  )
}
