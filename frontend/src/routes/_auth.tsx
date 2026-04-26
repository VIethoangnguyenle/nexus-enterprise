import { createFileRoute, Outlet, Navigate } from '@tanstack/react-router'
import { useAuthStore } from '../stores/auth.store'

export const Route = createFileRoute('/_auth')({
  component: AuthLayout,
})

function AuthLayout() {
  const isAuth = useAuthStore((s) => !!s.token)
  if (isAuth) return <Navigate to="/documents" />
  return (
    <div className="auth-page">
      <Outlet />
    </div>
  )
}
