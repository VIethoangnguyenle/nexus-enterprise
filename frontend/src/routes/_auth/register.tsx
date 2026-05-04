import { createFileRoute, Navigate } from '@tanstack/react-router'

export const Route = createFileRoute('/_auth/register')({
  component: RegisterRedirect,
})

/** Registration is now unified with login via OTP. Redirect to login. */
function RegisterRedirect() {
  return <Navigate to="/login" />
}
