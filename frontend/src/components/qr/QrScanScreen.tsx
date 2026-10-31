import { useCallback, useEffect, useRef, useState, type ChangeEvent } from 'react'
import { useNavigate, useRouter } from '@tanstack/react-router'
import { Camera, Image as ImageIcon, X, Zap } from 'lucide-react'
import QrScanner from 'qr-scanner'
import { qrTokenFromScan } from '../../lib/qr-login'
import { Button, IconButton, Text } from '../primitives'
import { QrPhoneFrame } from './QrPhoneFrame'

const NOT_OURS = 'Đây không phải mã đăng nhập Nexus'

/**
 * The in-app scanner behind + → Quét mã QR and the account sheet
 * (design/mockups/qr-login.html §3). The camera runs only while the page is
 * visible; a code that is not a Nexus sign-in link is named as such and the
 * scanner keeps going. The camera surface is dark in both themes (the stage
 * tokens, as in calls).
 */
export function QrScanScreen() {
  const router = useRouter()
  const navigate = useNavigate()
  const video = useRef<HTMLVideoElement>(null)
  const scanner = useRef<QrScanner | null>(null)
  const file = useRef<HTMLInputElement>(null)
  const [blocked, setBlocked] = useState(false)
  const [flash, setFlash] = useState<'none' | 'off' | 'on'>('none')
  const [notOurs, setNotOurs] = useState(false)
  const leaving = useRef(false)

  const close = () => {
    if (router.history.canGoBack()) router.history.back()
    else void navigate({ to: '/' })
  }

  const handle = useCallback((value: string) => {
    if (leaving.current) return
    const token = qrTokenFromScan(value)
    if (token) {
      leaving.current = true
      scanner.current?.stop()
      // Replace the scanner, so Xong on the confirm screen returns to where it was opened from.
      void navigate({ to: '/qr/$token', params: { token }, replace: true })
      return
    }
    setNotOurs(true)
    navigator.vibrate?.(30)
  }, [navigate])

  const start = useCallback(async () => {
    const s = scanner.current
    if (!s) return
    try {
      await s.start()
      setBlocked(false)
      setFlash((await s.hasFlash()) ? 'off' : 'none')
    } catch {
      setBlocked(true)
    }
  }, [])

  useEffect(() => {
    if (!video.current) return
    const s = new QrScanner(video.current, (r) => handle(r.data), {
      preferredCamera: 'environment',
      returnDetailedScanResult: true,
      maxScansPerSecond: 5,
    })
    scanner.current = s
    void start()
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') s.stop()
      else if (!leaving.current) void start()
    }
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      document.removeEventListener('visibilitychange', onVisibility)
      s.destroy()
      scanner.current = null
    }
  }, [handle, start])

  // The "not ours" line goes after a moment; the scanner never stopped.
  useEffect(() => {
    if (!notOurs) return
    const id = window.setTimeout(() => setNotOurs(false), 3000)
    return () => window.clearTimeout(id)
  }, [notOurs])

  const fromImage = async (e: ChangeEvent<HTMLInputElement>) => {
    const picked = e.target.files?.[0]
    e.target.value = ''
    if (!picked) return
    try {
      const r = await QrScanner.scanImage(picked, { returnDetailedScanResult: true })
      handle(r.data)
    } catch {
      setNotOurs(true)
    }
  }

  const imageInput = (
    // A hidden picker opened by the "Chọn ảnh có mã" buttons, as AvatarEdit does.
    // eslint-disable-next-line no-restricted-syntax
    <input ref={file} type="file" accept="image/*" aria-label="Ảnh có mã QR" className="sr-only" tabIndex={-1} aria-hidden="true" onChange={(e) => void fromImage(e)} />
  )

  return (
    <main className="min-h-dvh grid grid-rows-[auto_1fr_auto] bg-stage text-on-stage">
      <div className="flex items-center justify-between px-2 pt-3.5 pb-2">
        <IconButton aria-label="Đóng" size="lg" onClick={close} className="w-11 h-11 text-on-stage hover:bg-stage-hover">
          <X size={18} strokeWidth={1.75} />
        </IconButton>
        <b className="font-semibold">Quét mã QR</b>
        {flash === 'none' ? <span className="w-11" aria-hidden="true" /> : (
          <IconButton
            aria-label={flash === 'on' ? 'Tắt đèn' : 'Bật đèn'}
            aria-pressed={flash === 'on'}
            size="lg"
            className="w-11 h-11 text-on-stage hover:bg-stage-hover"
            onClick={() => {
              void scanner.current?.toggleFlash().then(() => setFlash(scanner.current?.isFlashOn() ? 'on' : 'off'))
            }}
          >
            <Zap size={18} strokeWidth={1.75} />
          </IconButton>
        )}
      </div>
      <div className="grid place-items-center content-center gap-5 p-4">
        <div className="relative w-58 h-58 overflow-hidden rounded-overlay bg-stage-tile">
          <video ref={video} className="absolute inset-0 w-full h-full object-cover" muted playsInline />
          <span aria-hidden="true" className="absolute inset-0 rounded-overlay ring-2 ring-inset ring-on-stage/80" />
        </div>
        <div role="status" aria-live="polite">
          <Text variant="body" className="block max-w-[30ch] text-center text-on-stage-muted">
            {notOurs ? NOT_OURS : 'Hướng camera vào mã QR trên màn hình máy tính'}
          </Text>
        </div>
      </div>
      <div className="px-4 pt-3 pb-[calc(1.75rem+env(safe-area-inset-bottom,0px))]">
        <Button variant="ghost" size="md" className="w-full h-11 bg-stage-raised text-on-stage hover:bg-stage-hover" onClick={() => file.current?.click()}>
          <ImageIcon size={16} strokeWidth={1.75} aria-hidden="true" />
          Chọn ảnh có mã
        </Button>
      </div>
      {imageInput}
      {blocked && (
        // Over the camera view, which keeps its video element so Thử lại can start it again.
        <div className="fixed inset-0 z-10 overflow-auto">
          <QrPhoneFrame
            onClose={close}
            tone="warning"
            icon={<Camera size={32} strokeWidth={1.75} />}
            title="Chưa có quyền dùng camera"
            line="Mở cài đặt trang trong trình duyệt, cho phép Camera, rồi quay lại đây."
            actions={
              <>
                <Button variant="primary" size="md" className="w-full h-11" onClick={() => void start()}>Thử lại</Button>
                <Button variant="secondary" size="md" className="w-full h-11" onClick={() => file.current?.click()}>
                  <ImageIcon size={16} strokeWidth={1.75} aria-hidden="true" />
                  Chọn ảnh có mã
                </Button>
                {notOurs && <div role="status"><Text variant="small" muted className="block text-center">{NOT_OURS}</Text></div>}
              </>
            }
          />
        </div>
      )}
    </main>
  )
}
