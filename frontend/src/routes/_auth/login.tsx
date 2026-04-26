import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useLogin } from '../../hooks/useAuth'

export const Route = createFileRoute('/_auth/login')({
  component: LoginPage,
})

function LoginPage() {
  const navigate = useNavigate()
  const login = useLogin()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    login.mutate(
      { username, password },
      { onSuccess: () => navigate({ to: '/documents' }) },
    )
  }

  return (
    <div className="auth-card fade-in">
      <div className="auth-header">
        <h1>NGAC Platform</h1>
        <p>Sign in to continue</p>
      </div>
      <form onSubmit={handleSubmit}>
        {login.error && <div className="error-msg">{login.error.message}</div>}
        <div className="form-group">
          <label>Username</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoFocus required />
        </div>
        <div className="form-group">
          <label>Password</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </div>
        <button className="btn btn-primary" type="submit" disabled={login.isPending} style={{ width: '100%' }}>
          {login.isPending ? <span className="spinner" /> : 'Sign In'}
        </button>
      </form>
      <div className="auth-link">
        Don't have an account? <a onClick={() => navigate({ to: '/register' })}>Register</a>
      </div>
    </div>
  )
}
