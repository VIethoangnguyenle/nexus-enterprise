import { createFileRoute, Navigate } from '@tanstack/react-router'

/** Root index — redirect to Chat channels. The channels index route will auto-select
 *  the first available channel or show a welcome state. */
export const Route = createFileRoute('/')({
  component: () => <Navigate to="/channels" />,
})
