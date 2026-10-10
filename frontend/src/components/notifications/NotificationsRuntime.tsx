import { useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { registerNavigator } from '../../lib/notification-arrivals'

/**
 * Lets code outside the React tree (a toast's "Xem") move around the app: it
 * hands the notification logic the router's `navigate` for as long as the
 * workspace shell is mounted.
 */
export function NotificationsRuntime() {
  const navigate = useNavigate()
  useEffect(() => {
    registerNavigator((t) => void navigate({ to: t.to, search: t.search as never }))
    return () => registerNavigator(null)
  }, [navigate])
  return null
}
