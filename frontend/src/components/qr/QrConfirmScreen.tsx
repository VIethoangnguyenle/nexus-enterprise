import { useEffect, useRef, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useNavigate, useRouter } from '@tanstack/react-router'
import { Building2, CircleCheck, Clock, Laptop, Lock, Monitor, ScanLine, ShieldAlert } from 'lucide-react'
import { qrApi, type QrScanResponse } from '../../api/auth'
import { ApiError } from '../../api/client'
import { useMe } from '../../hooks/useAuth'
import { describeDevice, isQrToken, rememberQrReturn } from '../../lib/qr-login'
import { useAuthStore } from '../../stores/auth.store'
import { NexusLogo } from '../brand/NexusLogo'
import { Avatar, Button, Spinner, Text } from '../primitives'
import { QrPhoneFrame } from './QrPhoneFrame'

/** The phone's confirm screen lives exactly as long as the code (two minutes). */
const QR_TTL_MS = 2 * 60 * 1000

type Outcome = 'unavailable' | 'denied' | 'failed'

function outcomeOf(err: unknown): Outcome {
  if (err instanceof ApiError && err.status === 410) return 'unavailable'
  if (err instanceof ApiError && err.status === 403) return 'denied'
  return 'failed'
}

const time = (iso: string) =>
  new Date(iso).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })

/**
 * The phone's side of sign-in by QR code (design/mockups/qr-login.html §4, §5).
 * Reached by the in-app scanner or by the phone's own camera opening the link,
 * so it may find the phone signed out.
 */
export function QrConfirmScreen({ token }: { token: string }) {
  const signedIn = useAuthStore((s) => !!s.user)
  const navigate = useNavigate()
  if (!isQrToken(token)) return <Unavailable onClose={() => void navigate({ to: '/' })} />
  if (!signedIn) {
    return (
      <QrPhoneFrame
        top={<NexusLogo size={32} />}
        icon={<Lock size={32} strokeWidth={1.75} />}
        title="Đăng nhập trên điện thoại trước"
        line="Mã QR này dùng tài khoản đang mở trên điện thoại. Đăng nhập xong, bạn quay lại đây để xác nhận."
        actions={
          <Button variant="primary" size="md" className="w-full h-11" onClick={() => {
            rememberQrReturn(token)
            void navigate({ to: '/login' })
          }}>
            Đăng nhập
          </Button>
        }
      />
    )
  }
  return <Confirm token={token} />
}

function Confirm({ token }: { token: string }) {
  const router = useRouter()
  const navigate = useNavigate()
  const me = useMe()
  const [outcome, setOutcome] = useState<Outcome | null>(null)
  const [done, setDone] = useState(false)
  // Kept here, not read off the mutation: the scan is started from an effect, and
  // a development double mount detaches the observer that started it.
  const [device, setDevice] = useState<QrScanResponse | null>(null)

  const leave = () => {
    if (router.history.canGoBack()) router.history.back()
    else void navigate({ to: '/' })
  }

  const scan = useMutation({
    meta: { silentError: true },
    mutationFn: () => qrApi.scan(token),
    onSuccess: setDevice,
    onError: (err) => setOutcome(outcomeOf(err)),
  })
  const approve = useMutation({
    meta: { silentError: true },
    mutationFn: () => qrApi.approve(token),
    onSuccess: () => setDone(true),
    onError: (err) => setOutcome(outcomeOf(err)),
  })
  const decline = useMutation({ meta: { silentError: true }, mutationFn: () => qrApi.decline(token) })

  // Opening the code is the scan; once, even when effects run twice.
  const scanned = useRef(false)
  useEffect(() => {
    if (scanned.current) return
    scanned.current = true
    scan.mutate()
  }, [scan])

  // The screen goes when the code does.
  useEffect(() => {
    if (!device || done) return
    const left = new Date(device.created_at).getTime() + QR_TTL_MS - Date.now()
    const id = window.setTimeout(() => setOutcome('unavailable'), Math.max(0, left))
    return () => window.clearTimeout(id)
  }, [device, done])

  const refuse = () => {
    // Closing counts as Huỷ: the computer is told, then the phone goes back.
    if (device && !outcome) decline.mutate(undefined, { onSettled: leave })
    else leave()
  }

  if (outcome === 'unavailable') return <Unavailable onClose={leave} />
  if (outcome === 'denied') {
    return (
      <QrPhoneFrame
        onClose={leave}
        tone="warning"
        icon={<Building2 size={32} strokeWidth={1.75} />}
        title={device?.tenant_name ? `Không đăng nhập được vào ${device.tenant_name}` : 'Không đăng nhập được vào tổ chức này'}
        line="Bạn không còn là thành viên của tổ chức này. Đổi sang tổ chức khác trên điện thoại rồi quét lại."
        actions={
          <Button variant="primary" size="md" className="w-full h-11" onClick={() => void navigate({ to: '/workspace-select' })}>
            Đổi tổ chức
          </Button>
        }
      />
    )
  }
  if (outcome === 'failed') {
    return (
      <QrPhoneFrame
        onClose={leave}
        tone="warning"
        icon={<Clock size={32} strokeWidth={1.75} />}
        title="Chưa xác nhận được"
        line="Kết nối gặp trục trặc. Thử lại sau giây lát."
        actions={
          <Button variant="primary" size="md" className="w-full h-11" onClick={() => { setOutcome(null); scan.mutate() }}>
            Thử lại
          </Button>
        }
      />
    )
  }
  if (done && device) {
    return (
      <QrPhoneFrame
        tone="success"
        icon={<CircleCheck size={32} strokeWidth={1.75} />}
        title="Đã đăng nhập trên máy tính"
        line={`${describeDevice(device.browser, device.os)} đã vào ${device.tenant_name} với tài khoản của bạn.`}
        actions={<Button variant="primary" size="md" className="w-full h-11" onClick={leave}>Xong</Button>}
      />
    )
  }
  if (!device) {
    return (
      <main className="min-h-dvh grid place-items-center bg-base" role="status">
        <Spinner />
      </main>
    )
  }
  return <Ask device={device} name={me.data?.display_name || me.data?.username || ''} avatar={me.data?.avatar_url}
    hueKey={me.data?.id} title={me.data?.title} busy={approve.isPending || decline.isPending}
    approving={approve.isPending} onApprove={() => approve.mutate()} onRefuse={refuse} />
}

function Ask({ device, name, avatar, hueKey, title, busy, approving, onApprove, onRefuse }: {
  device: QrScanResponse
  name: string
  avatar?: string
  hueKey?: string
  title?: string
  busy: boolean
  approving: boolean
  onApprove: () => void
  onRefuse: () => void
}) {
  const card = 'grid grid-cols-[2.5rem_minmax(0,1fr)] gap-3 items-center p-3 rounded-surface bg-raised ring-1 ring-line'
  return (
    <QrPhoneFrame
      onClose={onRefuse}
      icon={<Laptop size={32} strokeWidth={1.75} />}
      title="Đăng nhập Nexus trên máy tính?"
      actions={
        <>
          <Button variant="primary" size="md" className="w-full h-11" loading={approving} disabled={busy} onClick={onApprove}>
            Đăng nhập
          </Button>
          <Button variant="secondary" size="md" className="w-full h-11" disabled={busy} onClick={onRefuse}>Huỷ</Button>
        </>
      }
    >
      <div className="grid gap-2">
        <div className={card}>
          <span aria-hidden="true" className="grid place-items-center w-10 h-10 rounded-surface bg-accent-wash text-accent">
            <Monitor size={20} strokeWidth={1.75} />
          </span>
          <span className="grid min-w-0">
            <b className="font-semibold truncate">{describeDevice(device.browser, device.os)}</b>
            <span className="text-sm text-ink-muted">Tạo mã lúc {time(device.created_at)} · vừa quét</span>
          </span>
        </div>
        <div className={card}>
          <Avatar name={name || ' '} hueKey={hueKey} src={avatar || undefined} size={40} />
          <span className="grid min-w-0">
            <b className="font-semibold truncate">{name}</b>
            <span className="text-sm text-ink-muted truncate">{title ? `${device.tenant_name} · ${title}` : device.tenant_name}</span>
          </span>
        </div>
      </div>
      {/* Always shown, never dismissible: QR sign-in is phished by sending someone your own code. */}
      <div className="flex gap-2.5 items-start p-3 rounded-surface bg-warning-wash text-ink" role="note">
        <ShieldAlert size={16} strokeWidth={1.75} className="mt-0.5 shrink-0 text-warning" aria-hidden="true" />
        <Text variant="small">Chỉ đăng nhập nếu chính bạn đang ngồi trước máy tính này. Nếu ai đó gửi mã cho bạn quét, hãy huỷ.</Text>
      </div>
    </QrPhoneFrame>
  )
}

function Unavailable({ onClose }: { onClose: () => void }) {
  const navigate = useNavigate()
  return (
    <QrPhoneFrame
      onClose={onClose}
      tone="warning"
      icon={<Clock size={32} strokeWidth={1.75} />}
      title="Mã này không còn dùng được"
      line="Trên máy tính, chọn “Lấy mã mới” rồi quét lại."
      actions={
        <Button variant="primary" size="md" className="w-full h-11" onClick={() => void navigate({ to: '/scan', replace: true })}>
          <ScanLine size={16} strokeWidth={1.75} aria-hidden="true" />
          Quét lại
        </Button>
      }
    />
  )
}
