import { createFileRoute, Outlet, Navigate } from '@tanstack/react-router'
import { useAuthStore } from '../stores/auth.store'
import { LayoutGrid } from 'lucide-react'
import { Button } from '../components/primitives'

export const Route = createFileRoute('/_auth')({
  component: AuthLayoutRoute,
})

/** Nexus Hub auth layout — centered card with ambient background from Stitch source.
 *  Source: .stitch/designs/login.html
 *  Key tokens: bg-background, bg-nexus-auth radial gradients, max-w-110 container. */
function AuthLayoutRoute() {
  const isAuth = useAuthStore((s) => !!s.token)
  if (isAuth) return <Navigate to="/documents" />

  return (
    <div className="flex items-center justify-center min-h-screen bg-background text-on-surface p-4
      bg-nexus-auth">
      <main className="w-full max-w-110 flex flex-col gap-8">
        {/* Logo Area — centered brand icon */}
        <div className="flex flex-col items-center gap-4 text-center">
          <div className="w-12 h-12 bg-primary rounded-xl flex items-center justify-center text-on-primary shadow-sm">
            <LayoutGrid size={22} />
          </div>
        </div>

        {/* Content Card */}
        <div className="bg-surface-container-lowest rounded-xl p-8
          shadow-lg border border-surface-container w-full">
          <Outlet />
        </div>

        {/* Footer Links */}
        <div className="flex items-center justify-center gap-6 text-body-sm text-on-surface-variant">
          <Button variant="ghost" size="sm">
            Privacy Policy
          </Button>
          <span className="w-1 h-1 rounded-full bg-outline-variant" />
          <Button variant="ghost" size="sm">
            Terms of Service
          </Button>
        </div>
      </main>
    </div>
  )
}
