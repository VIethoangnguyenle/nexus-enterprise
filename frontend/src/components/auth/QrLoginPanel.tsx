import { useEffect, useState, type ReactNode } from 'react'
import { Clock, RefreshCw, WifiOff, X } from 'lucide-react'
import { useQrLogin, type QrPhase } from '../../hooks/useQrLogin'
import { qrLoginUrl } from '../../lib/qr-login'
import { Avatar, Button, Heading, Spinner, Text } from '../primitives'
import { QrCode } from './QrCode'

/** "1:52" until the code is swapped. */
function useCountdown(until: number | null): string {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (until === null) return
    const id = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [until])
  if (until === null) return ''
  const left = Math.max(0, Math.round((until - now) / 1000))
  return `${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`
}

function NewCode({ onClick }: { onClick: () => void }) {
  return (
    <Button variant="primary" size="sm" onClick={onClick}>
      <RefreshCw size={16} strokeWidth={1.75} aria-hidden="true" />
      Lấy mã mới
    </Button>
  )
}

/** What lies over the code once it is no longer simply waiting. */
function Veil({ phase, scanner, onRenew }: {
  phase: QrPhase
  scanner: { display_name: string; avatar_url: string } | null
  onRenew: () => void
}): ReactNode {
  switch (phase) {
    case 'loading':
      return <Spinner size="lg" />
    case 'scanned':
      return (
        <>
          {scanner && <Avatar name={scanner.display_name} src={scanner.avatar_url || undefined} size={40} />}
          <b className="font-semibold">{scanner ? `${scanner.display_name} đã quét` : 'Đã quét'}</b>
          <Text variant="small" muted>Xác nhận trên điện thoại</Text>
          <Button variant="link" size="link" onClick={onRenew}>Quét bằng điện thoại khác</Button>
        </>
      )
    case 'approving':
      return (
        <>
          <Spinner size="lg" />
          <b className="font-semibold">Đang đăng nhập…</b>
        </>
      )
    case 'declined':
      return (
        <>
          <X size={28} strokeWidth={1.75} className="text-warning" aria-hidden="true" />
          <b className="font-semibold">Đã huỷ trên điện thoại</b>
          <NewCode onClick={onRenew} />
        </>
      )
    case 'expired':
      return (
        <>
          <Clock size={28} strokeWidth={1.75} className="text-ink-muted" aria-hidden="true" />
          <b className="font-semibold">Mã đã hết hạn</b>
          <NewCode onClick={onRenew} />
        </>
      )
    case 'failed':
      return (
        <>
          <WifiOff size={28} strokeWidth={1.75} className="text-ink-muted" aria-hidden="true" />
          <b className="font-semibold">Chưa lấy được mã</b>
          <NewCode onClick={onRenew} />
        </>
      )
    default:
      return null
  }
}

/**
 * Sign in by QR code on a computer (design/mockups/qr-login.html §1, §2): the
 * code, a countdown to its swap, and each state laid over the code itself so the
 * frame never changes size.
 */
export function QrLoginPanel({ onSignedIn, onUseForm }: {
  onSignedIn: (needsProfile: boolean) => void
  onUseForm: () => void
}) {
  const qr = useQrLogin(onSignedIn)
  const countdown = useCountdown(qr.phase === 'pending' ? qr.expiresAt : null)
  const veiled = qr.phase !== 'pending'

  return (
    <>
      <Heading as="h1" look="page" className="text-center">Quét mã để đăng nhập</Heading>
      <div className="grid justify-items-center gap-4 text-center">
        <div className="relative w-54 h-54 p-3 rounded-surface bg-white ring-1 ring-line" aria-live="polite">
          {qr.token ? (
            <div className={`w-full h-full transition-[filter,opacity] duration-(--duration-base) ease-(--ease-out) ${veiled ? 'blur-xs opacity-35' : ''}`}>
              <QrCode value={qrLoginUrl(qr.token)} label="Mã QR đăng nhập" />
            </div>
          ) : (
            <div className="w-full h-full rounded-control bg-sunk" />
          )}
          {!veiled && (
            <span
              aria-hidden="true"
              className="absolute left-1/2 top-1/2 -ml-5 -mt-5 w-10 h-10 grid place-items-center rounded-surface
                bg-accent text-on-accent font-display font-bold ring-4 ring-white"
            >
              N
            </span>
          )}
          {veiled && (
            <div className="absolute inset-0 grid place-items-center content-center gap-2.5 p-4 rounded-surface bg-base/90 text-ink">
              <Veil phase={qr.phase} scanner={qr.scanner} onRenew={qr.renew} />
            </div>
          )}
        </div>

        {qr.phase === 'pending' && (
          <Text variant="small" muted className="flex items-center gap-2 tabular-nums">
            <span className="w-2 h-2 rounded-full bg-success" aria-hidden="true" />
            Mã tự làm mới sau {countdown}
          </Text>
        )}
        {qr.reconnecting && (
          <Text variant="small" muted className="flex items-center gap-2">
            <WifiOff size={14} strokeWidth={1.75} aria-hidden="true" />
            Mất kết nối, đang thử lại
          </Text>
        )}

        <Text variant="body" muted className="block max-w-[34ch]">
          Mở <b className="font-semibold text-ink">Nexus</b> trên điện thoại, chạm <b className="font-semibold text-ink">+</b> rồi{' '}
          <b className="font-semibold text-ink">Quét mã QR</b>. Hoặc hướng camera điện thoại vào mã.
        </Text>
      </div>
      <Text variant="small" muted className="block text-center">
        Chưa đăng nhập trên điện thoại?{' '}
        <Button variant="link" size="link" onClick={onUseForm}>Dùng email hoặc Google</Button>
      </Text>
    </>
  )
}
