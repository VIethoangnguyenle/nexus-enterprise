import { useState, useEffect } from 'react'
import { useDocStore, useAuthStore } from '../store'
import DocumentCard from '../components/DocumentCard'
import UploadModal from '../components/UploadModal'
import ShareModal from '../components/ShareModal'
import AccessModal from '../components/AccessModal'

export default function Dashboard() {
  const { documents, loading, error, fetchDocuments } = useDocStore()
  const user = useAuthStore(s => s.user)
  const [activeTab, setActiveTab] = useState('my')
  const [showUpload, setShowUpload] = useState(false)
  const [shareDoc, setShareDoc] = useState(null)
  const [accessDoc, setAccessDoc] = useState(null)

  useEffect(() => {
    fetchDocuments()
  }, [])

  const myDocs = documents.filter(d => d.owner_id === user?.id)
  const sharedDocs = documents.filter(d => d.owner_id !== user?.id && !d.is_public)
  const publicDocs = documents.filter(d => d.is_public)

  const tabDocs = activeTab === 'my' ? myDocs
    : activeTab === 'shared' ? sharedDocs
    : publicDocs

  const tabs = [
    { key: 'my', label: 'My Documents', count: myDocs.length },
    { key: 'shared', label: 'Shared With Me', count: sharedDocs.length },
    { key: 'public', label: 'Public Documents', count: publicDocs.length },
  ]

  return (
    <div className="dashboard fade-in">
      <div className="dashboard-header">
        <h1>Document Dashboard</h1>
        <button className="btn btn-primary" onClick={() => setShowUpload(true)}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
            <line x1="8" y1="2" x2="8" y2="14" />
            <line x1="2" y1="8" x2="14" y2="8" />
          </svg>
          Upload Document
        </button>
      </div>

      <div className="tabs">
        {tabs.map(t => (
          <button
            key={t.key}
            className={`tab ${activeTab === t.key ? 'active' : ''}`}
            onClick={() => setActiveTab(t.key)}
          >
            {t.label}
            <span style={{ marginLeft: 6, opacity: 0.7, fontSize: '0.75rem' }}>({t.count})</span>
          </button>
        ))}
      </div>

      {loading && (
        <div className="empty-state">
          <div className="spinner" style={{ width: 32, height: 32 }} />
        </div>
      )}

      {error && <div className="error-msg">{error}</div>}

      {!loading && tabDocs.length === 0 && (
        <div className="empty-state">
          <h3>{activeTab === 'my' ? 'No documents yet' : activeTab === 'shared' ? 'No shared documents' : 'No public documents'}</h3>
          <p>{activeTab === 'my' ? 'Upload your first document to get started.' : 'Documents shared with you will appear here.'}</p>
        </div>
      )}

      <div className="doc-grid">
        {tabDocs.map(doc => (
          <DocumentCard
            key={doc.id}
            doc={doc}
            onShare={() => setShareDoc(doc)}
            onAccessCheck={() => setAccessDoc(doc)}
            onRefresh={fetchDocuments}
          />
        ))}
      </div>

      {showUpload && (
        <UploadModal
          onClose={() => setShowUpload(false)}
          onUploaded={() => { setShowUpload(false); fetchDocuments() }}
        />
      )}

      {shareDoc && (
        <ShareModal
          doc={shareDoc}
          onClose={() => setShareDoc(null)}
          onShared={() => { setShareDoc(null); fetchDocuments() }}
        />
      )}

      {accessDoc && (
        <AccessModal
          doc={accessDoc}
          onClose={() => setAccessDoc(null)}
        />
      )}
    </div>
  )
}
