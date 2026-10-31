import { createFileRoute } from '@tanstack/react-router'
import { QrConfirmScreen } from '../components/qr/QrConfirmScreen'

/**
 * Where a sign-in QR code leads (design/mockups/qr-login.html §4). Outside both
 * the sign-in and the workspace layouts: the phone's own camera opens this link,
 * signed in or not, and the screen itself says what to do in either case.
 */
export const Route = createFileRoute('/qr/$token')({
  component: QrConfirmRoute,
})

function QrConfirmRoute() {
  const { token } = Route.useParams()
  return <QrConfirmScreen token={token} />
}
