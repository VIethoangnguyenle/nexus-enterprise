import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuthStore } from '../store'

export default function RegisterPage() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const register = useAuthStore(s => s.register)
  const navigate = useNavigate()

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    if (password !== confirm) {
      setError('Passwords do not match')
      return
    }
    if (password.length < 4) {
      setError('Password must be at least 4 characters')
      return
    }
    setLoading(true)
    try {
      await register(username, password)
      navigate('/')
    } catch (err) {
      setError(err.response?.data?.error || 'Registration failed')
    }
    setLoading(false)
  }

  return (
    <div className="auth-page">
      <div className="auth-card fade-in">
        <h1>Create account</h1>
        <p>Join the NGAC platform</p>
        {error && <div className="error-msg">{error}</div>}
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>Username</label>
            <input id="register-username" value={username} onChange={e => setUsername(e.target.value)} placeholder="Choose a username" autoFocus />
          </div>
          <div className="form-group">
            <label>Password</label>
            <input id="register-password" type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder="Create a password" />
          </div>
          <div className="form-group">
            <label>Confirm Password</label>
            <input id="register-confirm" type="password" value={confirm} onChange={e => setConfirm(e.target.value)} placeholder="Confirm password" />
          </div>
          <button id="register-submit" className="btn btn-primary" type="submit" disabled={loading}>
            {loading ? <span className="spinner" /> : 'Create Account'}
          </button>
        </form>
        <div className="auth-link">
          Already have an account? <Link to="/login">Sign in</Link>
        </div>
      </div>
    </div>
  )
}
