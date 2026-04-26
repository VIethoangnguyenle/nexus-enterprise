import { useState, useRef } from 'react'
import { useDocStore } from '../store'

export default function UploadModal({ onClose, onUploaded }) {
  const [title, setTitle] = useState('')
  const [file, setFile] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [dragActive, setDragActive] = useState(false)
  const fileInputRef = useRef()
  const { uploadDocument } = useDocStore()

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!file) { setError('Please select a file'); return }
    setLoading(true)
    setError('')
    try {
      await uploadDocument(title, file)
      onUploaded()
    } catch (err) {
      setError(err.response?.data?.error || 'Upload failed')
    } finally {
      setLoading(false)
    }
  }

  const handleDrag = (e) => {
    e.preventDefault()
    e.stopPropagation()
    if (e.type === 'dragenter' || e.type === 'dragover') setDragActive(true)
    else if (e.type === 'dragleave') setDragActive(false)
  }

  const handleDrop = (e) => {
    e.preventDefault()
    e.stopPropagation()
    setDragActive(false)
    if (e.dataTransfer.files?.[0]) {
      setFile(e.dataTransfer.files[0])
      if (!title) setTitle(e.dataTransfer.files[0].name.replace(/\.[^/.]+$/, ''))
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal fade-in" onClick={e => e.stopPropagation()}>
        <h2>Upload Document</h2>

        {error && <div className="error-msg">{error}</div>}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label htmlFor="upload-title">Document Title</label>
            <input
              id="upload-title"
              type="text"
              value={title}
              onChange={e => setTitle(e.target.value)}
              placeholder="Enter document title"
              required
              autoFocus
            />
          </div>

          <div
            className={`upload-zone ${dragActive ? 'active' : ''}`}
            onDragEnter={handleDrag}
            onDragLeave={handleDrag}
            onDragOver={handleDrag}
            onDrop={handleDrop}
            onClick={() => fileInputRef.current?.click()}
          >
            <input
              ref={fileInputRef}
              type="file"
              style={{ display: 'none' }}
              onChange={e => {
                if (e.target.files?.[0]) {
                  setFile(e.target.files[0])
                  if (!title) setTitle(e.target.files[0].name.replace(/\.[^/.]+$/, ''))
                }
              }}
            />
            {file ? (
              <div>
                <div style={{ fontSize: '2rem', marginBottom: '0.5rem' }}>📄</div>
                <div style={{ fontWeight: 600, color: 'var(--text-primary)' }}>{file.name}</div>
                <div style={{ fontSize: '0.8rem', marginTop: '0.25rem' }}>
                  {(file.size / 1024).toFixed(1)} KB
                </div>
              </div>
            ) : (
              <div>
                <div style={{ fontSize: '2rem', marginBottom: '0.5rem' }}>📂</div>
                <div>Drop a file here or click to browse</div>
                <div style={{ fontSize: '0.8rem', marginTop: '0.25rem' }}>Any file type supported</div>
              </div>
            )}
          </div>

          <div className="modal-actions">
            <button type="button" className="btn btn-secondary" onClick={onClose}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={loading}>
              {loading ? <span className="spinner" /> : 'Upload'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
