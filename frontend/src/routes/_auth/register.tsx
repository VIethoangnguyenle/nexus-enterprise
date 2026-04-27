import { createFileRoute, useNavigate, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useRegister } from '../../hooks/useAuth'
import { Button, Input, Spinner } from '../../components/primitives'

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
    <div className="bg-bg-tertiary/80 backdrop-blur-xl border border-border rounded-[var(--radius-lg)]
      p-8 shadow-lg animate-fade-in">
      {/* Logo area */}
      <div className="flex flex-col items-center mb-8">
        <div className="flex items-center justify-center w-12 h-12 rounded-xl
          bg-gradient-to-br from-accent to-[#8b5cf6] mb-4">
          <span className="text-white font-bold text-lg">N</span>
        </div>
        <h1 className="text-2xl font-bold text-text-primary tracking-tight mb-1">
          Create Account
        </h1>
        <p className="text-sm text-text-secondary">Join the NGAC Platform</p>
      </div>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {register.error && (
          <div className="bg-danger-bg border border-danger/20 text-danger px-4 py-3
            rounded-[var(--radius-sm)] text-sm">
            {register.error.message}
          </div>
        )}

        <Input
          label="Username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoFocus
          required
          placeholder="Choose a username"
        />

        <Input
          label="Password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          placeholder="Create a password"
        />

        <Button
          type="submit"
          disabled={register.isPending}
          size="lg"
          className="w-full mt-2"
        >
          {register.isPending ? <Spinner size="sm" /> : 'Create Account'}
        </Button>
      </form>

      <p className="text-center mt-6 text-sm text-text-secondary">
        Already have an account?{' '}
        <Link to="/login" className="text-accent-hover font-medium hover:underline">
          Sign in
        </Link>
      </p>
    </div>
  )
}
