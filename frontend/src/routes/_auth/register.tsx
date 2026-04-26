import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useRegister } from '../../hooks/useAuth'

export const Route = createFileRoute('/_auth/register')({
  component: RegisterPage,
})

function RegisterPage() {
  const navigate = useNavigate()
  const register = useRegister()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    register.mutate(
      { username, password },
      { onSuccess: () => navigate({ to: '/documents' }) },
    )
  }

  return (
    <div className="auth-card fade-in">
      <div className="auth-header">
        <h1>NGAC Platform</h1>
        <p>Create your account</p>
      </div>
      <form onSubmit={handleSubmit}>
        {register.error && <div className="error-msg">{register.error.message}</div>}
        <div className="form-group">
          <label>Username</label>
          <input value={username} onChange={(e) => setUsername(e.target.value)} autoFocus required />
        </div>
        <div className="form-group">
          <label>Password</label>
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </div>
        <button className="btn btn-primary" type="submit" disabled={register.isPending} style={{ width: '100%' }}>
          {register.isPending ? <span className="spinner" /> : 'Create Account'}
        </button>
      </form>
      <div className="auth-link">
        Already have an account? <a onClick={() => navigate({ to: '/login' })}>Sign in</a>
      </div>
    </div>
  )
}
