import { useEffect, useState } from 'react'
import { useDocStore, useWorkspaceStore } from '../store'

export default function DocumentsView() {
  const current = useWorkspaceStore(s => s.current)
  const { documents, loading, error, fetchDocuments, uploadDocument, approveDocument, publishDocument } = useDocStore()
  const [showUpload, setShowUpload] = useState(false)
  const [title, setTitle] = useState('')
  const [filename, setFilename] = useState('')
  const [tab, setTab] = useState('all')

  useEffect(() => {
    if (current) fetchDocuments(current.id)
  }, [current?.id])

  const handleUpload = async () => {
    if (!title.trim() || !current) return
    try {
      await uploadDocument(current.id, title.trim(), filename || title.trim() + '.txt')
      setShowUpload(false)
      setTitle('')
      setFilename('')
    } catch (err) {
      console.error(err)
    }
  }

  const handleApprove = async (docId) => {
    try {
      await approveDocument(docId)
      if (current) fetchDocuments(current.id)
    } catch (err) {
      console.error(err)
    }
  }

  const handlePublish = async (docId) => {
    try {
      await publishDocument(docId)
      if (current) fetchDocuments(current.id)
    } catch (err) {
      console.error(err)
    }
  }

  const filtered = (documents || []).filter(d => {
    if (tab === 'draft') return d.status === 'draft'
    if (tab === 'approved') return d.status === 'approved'
    return true
  })

  return (
    <>
      <div className="content-topbar">
        <div className="topbar-title">
          <span>📄</span> Documents
        </div>
        <div className="topbar-actions">
          <button className="btn btn-primary btn-sm" onClick={() => setShowUpload(true)}>+ Upload</button>
        </div>
      </div>

      <div className="content-body">
        <div className="tabs">
          {['all', 'draft', 'approved'].map(t => (
            <button key={t} className={`tab ${tab === t ? 'active' : ''}`} onClick={() => setTab(t)}>
              {t.charAt(0).toUpperCase() + t.slice(1)}
            </button>
          ))}
        </div>

        {loading && <div style={{ textAlign: 'center', padding: '2rem' }}><span className="spinner" /></div>}
        {error && <div className="error-msg">{error}</div>}

        {!loading && filtered.length === 0 && (
          <div className="empty-state">
            <div className="icon">📁</div>
            <h3>No documents yet</h3>
            <p>Upload your first document to get started</p>
            <button className="btn btn-primary" onClick={() => setShowUpload(true)}>Upload Document</button>
          </div>
        )}

        <div className="doc-grid">
          {filtered.map(doc => (
            <div key={doc.id} className="doc-card fade-in">
              <div className="doc-card-header">
                <span className="doc-title">{doc.title}</span>
                <span className={`doc-badge badge-${doc.status || 'draft'}`}>
                  {doc.status || 'draft'}
                </span>
              </div>
              <div className="doc-meta">
                <span>📎 {doc.filename}</span>
                <span>👤 {doc.owner_name || doc.ownerName || 'Unknown'}</span>
              </div>
              <div className="doc-actions">
                {(doc.status === 'draft' || !doc.status) && (
                  <button className="btn btn-success btn-sm" onClick={() => handleApprove(doc.id)}>✓ Approve</button>
                )}
                <button className="btn btn-secondary btn-sm" onClick={() => handlePublish(doc.id)}>🌐 Publish</button>
              </div>
            </div>
          ))}
        </div>
      </div>

      {showUpload && (
        <div className="modal-overlay" onClick={() => setShowUpload(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <h2>Upload Document</h2>
            <div className="form-group">
              <label>Title</label>
              <input value={title} onChange={e => setTitle(e.target.value)} placeholder="Document title" autoFocus />
            </div>
            <div className="form-group">
              <label>Filename</label>
              <input value={filename} onChange={e => setFilename(e.target.value)} placeholder="report.pdf" />
            </div>
            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setShowUpload(false)}>Cancel</button>
              <button className="btn btn-primary" onClick={handleUpload}>Upload</button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
