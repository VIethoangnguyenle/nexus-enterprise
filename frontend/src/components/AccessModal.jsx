import { useState } from 'react'
import { useDocStore, useAuthStore } from '../store'

export default function AccessModal({ doc, onClose }) {
  const [operation, setOperation] = useState('read')
  const [result, setResult] = useState(null)
  const [loading, setLoading] = useState(false)
  const user = useAuthStore(s => s.user)
  const { checkAccess } = useDocStore()

  const handleCheck = async () => {
    setLoading(true)
    try {
      const decision = await checkAccess(user.id, doc.id, operation)
      setResult(decision)
    } catch (err) {
      setResult({ decision: 'ERROR', explanation: { reason: err.response?.data?.error || 'Check failed' } })
    } finally {
      setLoading(false)
    }
  }

  const ops = ['read', 'write', 'upload', 'approve', 'share']

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal fade-in" onClick={e => e.stopPropagation()}>
        <h2>Access Check: {doc.title}</h2>

        <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
          {ops.map(op => (
            <button
              key={op}
              className={`btn btn-sm ${operation === op ? 'btn-primary' : 'btn-secondary'}`}
              onClick={() => { setOperation(op); setResult(null) }}
            >
              {op}
            </button>
          ))}
        </div>

        <div style={{ marginBottom: '1rem', fontSize: '0.85rem', color: 'var(--text-secondary)' }}>
          Checking: <strong>{user?.username}</strong> → <strong>{operation}</strong> → <strong>{doc.title}</strong>
        </div>

        <button
          className="btn btn-primary"
          onClick={handleCheck}
          disabled={loading}
          style={{ width: '100%' }}
        >
          {loading ? <span className="spinner" /> : 'Check Access'}
        </button>

        {result && (
          <div className="explanation-card" style={{ marginTop: '1rem' }}>
            <div className={`explanation-decision ${result.decision === 'ALLOW' ? 'decision-allow' : 'decision-deny'}`}>
              <span style={{ fontSize: '1.5rem' }}>{result.decision === 'ALLOW' ? '✓' : '✕'}</span>
              {result.decision}
            </div>

            {result.explanation?.path && result.explanation.path.length > 0 && (
              <div style={{ marginBottom: '1rem' }}>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: '0.5rem' }}>
                  Traversal Path
                </div>
                <ul className="explanation-path">
                  {result.explanation.path.map((step, i) => (
                    <li key={i}>{step}</li>
                  ))}
                </ul>
              </div>
            )}

            {result.explanation?.policy_class && (
              <div style={{ marginBottom: '0.75rem', fontSize: '0.85rem' }}>
                <span style={{ color: 'var(--text-muted)' }}>Policy Class: </span>
                <span style={{ color: 'var(--accent-hover)', fontWeight: 600 }}>{result.explanation.policy_class}</span>
              </div>
            )}

            {result.explanation?.reason && (
              <div style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', padding: '0.75rem', background: 'var(--danger-bg)', borderRadius: 'var(--radius-sm)', border: '1px solid rgba(239,68,68,0.15)' }}>
                {result.explanation.reason}
              </div>
            )}

            {result.explanation?.constraint_denied && (
              <div style={{ marginTop: '0.75rem', fontSize: '0.85rem', padding: '0.75rem', background: 'var(--warning-bg)', borderRadius: 'var(--radius-sm)', border: '1px solid rgba(245,158,11,0.15)' }}>
                <div style={{ fontWeight: 600, color: 'var(--warning)' }}>
                  Constraint: {result.explanation.constraint_denied.name}
                </div>
                <div style={{ color: 'var(--text-secondary)', marginTop: '0.25rem' }}>
                  {result.explanation.constraint_denied.message}
                </div>
              </div>
            )}

            {result.explanation?.user_attributes?.length > 0 && (
              <div style={{ marginTop: '0.75rem', fontSize: '0.8rem' }}>
                <span style={{ color: 'var(--text-muted)' }}>User Attributes: </span>
                {result.explanation.user_attributes.map(ua => (
                  <span key={ua} className="doc-badge badge-approved" style={{ marginRight: 4 }}>{ua}</span>
                ))}
              </div>
            )}

            {result.explanation?.object_attributes?.length > 0 && (
              <div style={{ marginTop: '0.5rem', fontSize: '0.8rem' }}>
                <span style={{ color: 'var(--text-muted)' }}>Object Attributes: </span>
                {result.explanation.object_attributes.map(oa => (
                  <span key={oa} className="doc-badge badge-draft" style={{ marginRight: 4 }}>{oa}</span>
                ))}
              </div>
            )}

            {result.explanation?.constraints_checked?.length > 0 && (
              <div style={{ marginTop: '0.5rem', fontSize: '0.8rem' }}>
                <span style={{ color: 'var(--text-muted)' }}>Constraints evaluated: </span>
                {result.explanation.constraints_checked.join(', ')}
              </div>
            )}
          </div>
        )}

        <div className="modal-actions">
          <button className="btn btn-secondary" onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  )
}
