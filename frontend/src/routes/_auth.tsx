import { createFileRoute, Outlet, Navigate } from '@tanstack/react-router'
import { useAuthStore } from '../stores/auth.store'
import { LayoutGrid } from 'lucide-react'

export const Route = createFileRoute('/_auth')({
  component: AuthLayoutRoute,
})

/** Nexus Hub auth layout — centered card with ambient background from Stitch source.
 *  Source: .stitch/designs/login.html
 *  Key tokens: bg-background, bg-texture radial gradients, max-w-[440px] container. */
function AuthLayoutRoute() {
  const isAuth = useAuthStore((s) => !!s.token)
  if (isAuth) return <Navigate to="/documents" />

  return (
    <div className="flex items-center justify-center min-h-screen bg-background text-on-surface p-4
      bg-[radial-gradient(circle_at_100%_0%,#dee8ff_0%,transparent_40%),radial-gradient(circle_at_0%_100%,#e7eeff_0%,transparent_40%)]">
      <main className="w-full max-w-[440px] flex flex-col gap-8">
        {/* Logo Area — centered brand icon */}
        <div className="flex flex-col items-center gap-4 text-center">
          <div className="w-12 h-12 bg-primary rounded-xl flex items-center justify-center text-on-primary shadow-sm">
            <LayoutGrid size={22} />
          </div>
        </div>

        {/* Content Card */}
        <div className="bg-surface-container-lowest rounded-xl p-8
          shadow-[0_16px_32px_-12px_rgba(0,0,0,0.12)] border border-surface-container w-full">
          <Outlet />
        </div>

        {/* Footer Links */}
        <div className="flex items-center justify-center gap-6 text-body-sm text-on-surface-variant">
          <button className="bg-transparent border-none cursor-pointer text-on-surface-variant
            hover:text-primary transition-colors">
            Privacy Policy
          </button>
          <span className="w-1 h-1 rounded-full bg-outline-variant" />
          <button className="bg-transparent border-none cursor-pointer text-on-surface-variant
            hover:text-primary transition-colors">
            Terms of Service
          </button>
        </div>
      </main>
    </div>
  )
}
