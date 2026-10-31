import { createFileRoute } from '@tanstack/react-router'
import { QrScanScreen } from '../components/qr/QrScanScreen'
import { requireSignedIn } from '../lib/auth-guards'

/**
 * The in-app QR scanner, full screen and outside the workspace shell
 * (design/mockups/qr-login.html §3). Scanning approves a computer in the
 * person's name, so it needs a session.
 */
export const Route = createFileRoute('/scan')({
  beforeLoad: requireSignedIn,
  component: QrScanScreen,
})
