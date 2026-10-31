import { createFileRoute, redirect } from '@tanstack/react-router'

/** Root index: on to Chat. The channels index picks the first channel or shows a
 *  welcome state; a signed-out visitor is sent on from there to the login page.
 *  Done in `beforeLoad` so nothing renders, and no code loads, for a page no one sees. */
export const Route = createFileRoute('/')({
  beforeLoad: () => {
    throw redirect({ to: '/channels', replace: true })
  },
})
