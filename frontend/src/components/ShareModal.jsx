import { useState, useEffect } from 'react'
import { useDocStore, useAdminStore } from '../store'

export default function ShareModal({ doc, onClose, onShared }) {
  const [departments, setDepartments] = useState([])
  const [selectedDept, setSelectedDept] = useState('')
  const [operations, setOperations] = useState(['read'])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [shares, setShares] = useState([])
  const [sharesLoading, setSharesLoading] = useState(true)
  const { shareDocument, revokeShare, getShares } = useDocStore()
  const { fetchDepartments } = useAdminStore()

  useEffect(() => {
    fetchDepartments().then(setDepartments).catch(() => {})
    loadShares()
  }, [])

  const loadShares = async () => {
    setSharesLoading(true)
    try {
      const data = await getShares(doc.id)
      setShares(data)
    } catch { }
    setSharesLoading(false)
  }

  const toggleOp = (op) => {
    setOperations(prev =>
      prev.includes(op)
        ? prev.filter(o => o !== op)
        : [...prev, op]
    )
  }

  const handleShare = async () => {
    if (!selectedDept) { setError('Please select a department'); return }
    setLoading(true)
    setError('')
    try {
      await shareDocument(doc.id, selectedDept, operations)
      await loadShares()
      setSelectedDept('')
    } catch (err) {
      setError(err.response?.data?.error || 'Share failed')
    } finally {
      setLoading(false)
    }
  }

  const handleRevoke = async (shareOaId) => {
    try {
      await revokeShare(doc.id, shareOaId)
      await loadShares()
    } catch (err) {
      alert(err.response?.data?.error || 'Revoke failed')
    }
  }

  const allOps = ['read', 'write', 'upload', 'approve', 'share']

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal fade-in" onClick={e => e.stopPropagation()}>
        <h2>Share: {doc.title}</h2>

        {error && <div className="error-msg">{error}</div>}

        <div className="form-group">
          <label htmlFor="share-dept">Share with Department</label>
          <select
            id="share-dept"
            value={selectedDept}
            onChange={e => setSelectedDept(e.target.value)}
          >
            <option value="">Select a department...</option>
            {departments.map(d => (
              <option key={d.id} value={d.id}>{d.name} ({d.company})</option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label>Operations</label>
          <div className="checkbox-group">
            {allOps.map(op => (
              <label key={op} className="checkbox-label">
                <input
                  type="checkbox"
                  checked={operations.includes(op)}
                  onChange={() => toggleOp(op)}
                />
                {op}
              </label>
            ))}
          </div>
        </div>

        <button
          className="btn btn-primary"
          onClick={handleShare}
          disabled={loading}
          style={{ width: '100%' }}
        >
          {loading ? <span className="spinner" /> : 'Share Document'}
        </button>

        {/* Active shares */}
        <div style={{ marginTop: '1.5rem' }}>
          <label style={{ fontSize: '0.8rem', fontWeight: 500, color: 'var(--text-secondary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
            Active Shares
          </label>
          {sharesLoading ? (
            <div style={{ textAlign: 'center', padding: '1rem' }}><span className="spinner" /></div>
          ) : shares.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '1rem', color: 'var(--text-muted)', fontSize: '0.85rem' }}>
              No active shares
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', marginTop: '0.5rem' }}>
              {shares.map(s => (
                <div key={s.id} style={{
                  display: 'flex', justifyContent: 'space-between', alignItems: 'center',
                  background: 'var(--bg-glass)', border: '1px solid var(--border-glass)',
                  borderRadius: 'var(--radius-sm)', padding: '0.75rem 1rem',
                }}>
                  <div>
                    <div style={{ fontWeight: 600, fontSize: '0.85rem' }}>{s.target_ua_name}</div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                      {s.operations?.join(', ')}
                    </div>
                  </div>
                  <button
                    className="btn btn-danger btn-sm"
                    onClick={() => handleRevoke(s.share_oa_id)}
                  >
                    Revoke
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="modal-actions">
          <button className="btn btn-secondary" onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  )
}
