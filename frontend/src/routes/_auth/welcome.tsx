import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useAuthStore } from '../../stores/auth.store'
import { LayoutGrid } from 'lucide-react'

export const Route = createFileRoute('/_auth/welcome')({
  component: WelcomeBackPage,
})

/** "Welcome Back" splash matching .stitch/designs/welcome-back.html.
 *  Key tokens: max-w-md, w-48 h-48 rounded-full avatar area, h-unit progress bar,
 *  shadow-[0_16px_32px_-12px_rgba(0,0,0,0.08)], surface-variant track. */
function WelcomeBackPage() {
  const user = useAuthStore((s) => s.user)
  const navigate = useNavigate()
  const [progress, setProgress] = useState(0)

  useEffect(() => {
    const interval = setInterval(() => {
      setProgress((p) => Math.min(p + 8, 100))
    }, 100)

    const timer = setTimeout(() => {
      navigate({ to: '/documents' })
    }, 2000)

    return () => { clearInterval(interval); clearTimeout(timer) }
  }, [navigate])

  return (
    <div className="flex flex-col items-center justify-center min-h-screen bg-background animate-fade-in">
      {/* Hero circle — Stitch: w-48 h-48 rounded-full shadow with surface-container-lowest */}
      <div className="w-48 h-48 mb-8 rounded-full bg-surface-container-lowest
        shadow-[0_16px_32px_-12px_rgba(0,0,0,0.08)] flex items-center justify-center overflow-hidden">
        <div className="w-20 h-20 bg-primary rounded-2xl flex items-center justify-center">
          <LayoutGrid size={40} className="text-on-primary" />
        </div>
      </div>

      {/* Title — Stitch: font-h1 text-h1 tracking-tight */}
      <h1 className="text-h1 text-on-surface mb-4 tracking-tight text-center">
        Welcome back{user?.username ? `, ${user.username}` : ''}
      </h1>

      {/* Subtitle stack — Stitch: flex-col gap-space-xs mb-space-xl */}
      <div className="flex flex-col gap-1 mb-10 text-center">
        <p className="text-body text-on-surface-variant">Logging you in...</p>
        <p className="text-small text-outline">Setting up your workspace...</p>
      </div>

      {/* Progress bar — Stitch: max-w-[240px] h-unit surface-variant track, primary fill */}
      <div className="w-full max-w-[240px] h-1 bg-surface-variant rounded-full overflow-hidden">
        <div
          className="h-full bg-primary rounded-full transition-all duration-100 ease-out"
          style={{ width: `${progress}%` }}
        />
      </div>
    </div>
  )
}
