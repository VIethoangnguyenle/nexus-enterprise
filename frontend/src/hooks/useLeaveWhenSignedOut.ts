import { useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useAuthStore } from '../stores/auth.store'

/**
 * For screens that only make sense with a session (the sign-in group's profile,
 * workspace choice and new-workspace pages).
 *
 * `requireSignedIn` checks the session once, as the route is entered. When a
 * later request gets a 401 that a refresh cannot repair, the store is logged out
 * but the address stays, leaving a screen full of failed requests. This leaves
 * for sign-in the moment the user disappears from the store.
 *
 * It is a hook on those screens rather than a guard on the `_auth` layout,
 * because that layout also hosts /login itself.
 */
export function useLeaveWhenSignedOut(): void {
  const navigate = useNavigate()
  const signedIn = useAuthStore((s) => !!s.user)
  useEffect(() => {
    if (!signedIn) void navigate({ to: '/login', replace: true })
  }, [signedIn, navigate])
}
