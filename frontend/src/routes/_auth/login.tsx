import { createFileRoute, useNavigate, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useLogin } from '../../hooks/useAuth'
import { Button, Input, Spinner } from '../../components/primitives'

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
    <div className="bg-bg-tertiary/80 backdrop-blur-xl border border-border rounded-[var(--radius-lg)]
      p-8 shadow-lg animate-fade-in">
      {/* Logo area */}
      <div className="flex flex-col items-center mb-8">
        <div className="flex items-center justify-center w-12 h-12 rounded-xl
          bg-gradient-to-br from-accent to-[#8b5cf6] mb-4">
          <span className="text-white font-bold text-lg">N</span>
        </div>
        <h1 className="text-2xl font-bold text-text-primary tracking-tight mb-1">
          NGAC Platform
        </h1>
        <p className="text-sm text-text-secondary">Sign in to continue</p>
      </div>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {login.error && (
          <div className="bg-danger-bg border border-danger/20 text-danger px-4 py-3
            rounded-[var(--radius-sm)] text-sm">
            {login.error.message}
          </div>
        )}

        <Input
          label="Username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoFocus
          required
          placeholder="Enter your username"
        />

        <Input
          label="Password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          placeholder="Enter your password"
        />

        <Button
          type="submit"
          disabled={login.isPending}
          size="lg"
          className="w-full mt-2"
        >
          {login.isPending ? <Spinner size="sm" /> : 'Sign In'}
        </Button>
      </form>

      <p className="text-center mt-6 text-sm text-text-secondary">
        Don&apos;t have an account?{' '}
        <Link to="/register" className="text-accent-hover font-medium hover:underline">
          Register
        </Link>
      </p>
    </div>
  )
}
