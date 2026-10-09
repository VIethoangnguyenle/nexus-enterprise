import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { completeGoogleSignIn } from '../api/auth'
import { Spinner } from '../components/primitives'

export const Route = createFileRoute('/auth/google/done')({
  component: GoogleDonePage,
})

/**
 * Landing page after Google redirects back through the auth service.
 *
 * Deliberately outside the `_auth` layout: that layout bounces anyone with a
 * persisted user to /documents, which would skip this page — and the persisted
 * user may be someone else entirely if a different account signed in before.
 * This page always re-reads who the new session belongs to.
 */
function GoogleDonePage() {
  const navigate = useNavigate()
  // Refresh tokens rotate: running the exchange twice (StrictMode re-runs
  // effects) would spend the cookie and then present it again.
  const started = useRef(false)

  useEffect(() => {
    if (started.current) return
    started.current = true

    void completeGoogleSignIn().then((ok) => {
      if (ok) {
        // Same next step as the OTP flow: workspace selection decides between
        // onboarding, a single workspace, or the picker.
        navigate({ to: '/workspace-select' as any, replace: true })
      } else {
        navigate({ to: '/login' as any, search: { error: 'google_session' } as any, replace: true })
      }
    })
  }, [navigate])

  return (
    <div className="flex flex-col items-center justify-center gap-4 min-h-screen bg-background">
      <Spinner />
      <p className="text-body text-on-surface-variant">Signing you in with Google…</p>
    </div>
  )
}
