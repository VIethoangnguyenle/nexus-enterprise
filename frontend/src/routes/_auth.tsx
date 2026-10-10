import { createFileRoute, Outlet, useRouterState } from '@tanstack/react-router'
import { motion } from 'motion/react'
import { AuthShell } from '../components/auth/AuthShell'
import { useMotionPresets } from '../lib/motion'

export const Route = createFileRoute('/_auth')({
  component: AuthLayout,
})

/**
 * Frame shared by every sign-in screen. It does not decide who may be here:
 * the sign-in page turns away a signed-in person and the pages after it turn
 * away a signed-out one, each in its own `beforeLoad` (lib/auth-guards.ts).
 * A rule kept here would also fire the moment a sign-in succeeds and race the
 * screen's own choice of where to go next.
 */
function AuthLayout() {
  const m = useMotionPresets()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  return (
    <AuthShell wide={pathname === '/workspace-select'}>
      {/* Route change: the content fades in, no slide (DESIGN.md §7). */}
      <motion.div key={pathname} initial={m.route.initial} animate={m.route.animate} className="grid gap-5">
        <Outlet />
      </motion.div>
    </AuthShell>
  )
}
