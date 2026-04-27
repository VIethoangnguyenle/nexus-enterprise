import { createFileRoute, Outlet, Navigate } from '@tanstack/react-router'
import { useAuthStore } from '../stores/auth.store'

export const Route = createFileRoute('/_auth')({
  component: AuthLayoutRoute,
})

function AuthLayoutRoute() {
  const isAuth = useAuthStore((s) => !!s.token)
  if (isAuth) return <Navigate to="/documents" />

  return (
    <div className="flex items-center justify-center min-h-screen p-6 bg-bg-primary
      bg-[radial-gradient(ellipse_at_50%_0%,rgba(99,102,241,0.12),transparent_60%)]">
      {/* Constellation background dots */}
      <div className="fixed inset-0 pointer-events-none overflow-hidden">
        <div className="absolute w-1 h-1 rounded-full bg-accent/30 top-[15%] left-[20%] animate-pulse" />
        <div className="absolute w-1.5 h-1.5 rounded-full bg-accent/20 top-[30%] right-[25%] animate-pulse [animation-delay:1s]" />
        <div className="absolute w-1 h-1 rounded-full bg-accent/25 top-[60%] left-[10%] animate-pulse [animation-delay:2s]" />
        <div className="absolute w-0.5 h-0.5 rounded-full bg-accent/40 top-[45%] right-[15%] animate-pulse [animation-delay:0.5s]" />
        <div className="absolute w-1 h-1 rounded-full bg-accent/15 top-[80%] left-[40%] animate-pulse [animation-delay:1.5s]" />
      </div>

      <div className="relative z-10 w-full max-w-[420px] animate-slide-up">
        <Outlet />
      </div>
    </div>
  )
}
