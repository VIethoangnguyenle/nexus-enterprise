import { useState } from 'react'
import { useDocStore, useAuthStore } from '../store'

export default function DocumentCard({ doc, onShare, onAccessCheck, onRefresh }) {
  const { approveDocument, publishDocument, unpublishDocument, deleteDocument } = useDocStore()
  const user = useAuthStore(s => s.user)
  const [actionLoading, setActionLoading] = useState(null)

  const isOwner = doc.owner_id === user?.id

  const handleAction = async (action, fn) => {
    setActionLoading(action)
    try {
      await fn()
      onRefresh()
    } catch (err) {
      alert(err.response?.data?.error || `${action} failed`)
    } finally {
      setActionLoading(null)
    }
  }

  return (
    <div className="doc-card">
      <div className="doc-card-header">
        <div className="doc-title">{doc.title}</div>
        <div style={{ display: 'flex', gap: 4, flexShrink: 0 }}>
          <span className={`doc-badge ${doc.status === 'approved' ? 'badge-approved' : 'badge-draft'}`}>
            {doc.status}
          </span>
          {doc.is_public && <span className="doc-badge badge-public">Public</span>}
        </div>
      </div>
      <div className="doc-meta">
        <span>📄 {doc.filename}</span>
        <span>👤 {doc.owner_name || 'Unknown'}</span>
        <span>📅 {new Date(doc.created_at).toLocaleDateString()}</span>
      </div>
      <div className="doc-actions">
        {/* Approve button — only on draft docs */}
        {doc.status === 'draft' && (
          <button
            className="btn btn-success btn-sm"
            disabled={actionLoading === 'approve'}
            onClick={() => handleAction('approve', () => approveDocument(doc.id))}
          >
            {actionLoading === 'approve' ? <span className="spinner" /> : '✓ Approve'}
          </button>
        )}

        {/* Share button — only on approved docs */}
        {doc.status === 'approved' && isOwner && (
          <button className="btn btn-primary btn-sm" onClick={onShare}>
            ↗ Share
          </button>
        )}

        {/* Publish/Unpublish toggle — only on approved docs */}
        {doc.status === 'approved' && isOwner && !doc.is_public && (
          <button
            className="btn btn-secondary btn-sm"
            disabled={actionLoading === 'publish'}
            onClick={() => handleAction('publish', () => publishDocument(doc.id))}
          >
            {actionLoading === 'publish' ? <span className="spinner" /> : '🌐 Make Public'}
          </button>
        )}
        {doc.is_public && isOwner && (
          <button
            className="btn btn-secondary btn-sm"
            disabled={actionLoading === 'unpublish'}
            onClick={() => handleAction('unpublish', () => unpublishDocument(doc.id))}
          >
            {actionLoading === 'unpublish' ? <span className="spinner" /> : '🔒 Make Private'}
          </button>
        )}

        {/* Access Check */}
        <button className="btn btn-secondary btn-sm" onClick={onAccessCheck}>
          🔍 Check Access
        </button>

        {/* Delete — owner only */}
        {isOwner && (
          <button
            className="btn btn-danger btn-sm"
            disabled={actionLoading === 'delete'}
            onClick={() => {
              if (confirm('Delete this document?'))
                handleAction('delete', () => deleteDocument(doc.id))
            }}
          >
            {actionLoading === 'delete' ? <span className="spinner" /> : '✕ Delete'}
          </button>
        )}
      </div>
    </div>
  )
}
