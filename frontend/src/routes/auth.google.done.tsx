import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { completeGoogleSignIn } from '../api/auth'
import { AuthShell } from '../components/auth/AuthShell'
import { Spinner, Text } from '../components/primitives'
import { afterSignIn } from '../lib/auth-flow'

export const Route = createFileRoute('/auth/google/done')({
  component: GoogleDonePage,
})

/**
 * Landing page after Google redirects back through the auth service.
 *
 * Deliberately outside the `_auth` layout: this page must always re-read who
 * the new session belongs to, and a persisted user may be someone else entirely
 * if a different account signed in before.
 */
function GoogleDonePage() {
  const navigate = useNavigate()
  // Refresh tokens rotate: running the exchange twice (StrictMode re-runs
  // effects) would spend the cookie and then present it again.
  const started = useRef(false)

  useEffect(() => {
    if (started.current) return
    started.current = true

    void completeGoogleSignIn().then((result) => {
      if (result) {
        // The same next step as a code sign-in: a profile if one is owed,
        // otherwise the choice of workspace.
        void navigate({ to: afterSignIn(result.needsProfile), replace: true })
      } else {
        void navigate({ to: '/login', search: { error: 'google_session' }, replace: true })
      }
    })
  }, [navigate])

  return (
    <AuthShell>
      <div className="grid justify-items-center gap-3 py-10 text-center" role="status">
        <Spinner />
        <Text variant="body" muted>Đang đăng nhập bằng Google…</Text>
      </div>
    </AuthShell>
  )
}
