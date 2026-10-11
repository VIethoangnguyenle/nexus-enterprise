import { useState } from 'react'
import { Bell } from 'lucide-react'
import { Button, toast } from './primitives'
import {
  getOneSignalAppId,
  isPushSupported,
  getNotificationPermission,
  isPromptDismissed,
  recordPromptDismiss,
  requestPushPermission,
  isIosNotInstalled,
} from '../lib/push'

export function PushPrompt() {
  const [dismissed, setDismissed] = useState(false)
  const [enabling, setEnabling] = useState(false)
  const appId = getOneSignalAppId()
  const supported = isPushSupported()
  const permission = getNotificationPermission()
  const isDismissed = dismissed || isPromptDismissed()

  if (!appId || !supported || permission !== 'default' || isDismissed) {
    return null
  }

  const iosNotInstalled = isIosNotInstalled()

  const handleDismiss = () => {
    recordPromptDismiss()
    setDismissed(true)
  }

  const handleEnable = async () => {
    if (enabling) return
    setEnabling(true)
    try {
      const res = await requestPushPermission()
      if (res === 'granted') {
        toast('Đã bật thông báo trên thiết bị này')
      } else {
        toast.error('Trình duyệt đang chặn thông báo. Mở lại trong Cài đặt → Thông báo')
      }
    } catch {
      toast.error('Không bật được thông báo. Thử lại sau.')
    } finally {
      setEnabling(false)
      setDismissed(true)
    }
  }

  return (
    <div
      role="region"
      aria-label="Nhận thông báo"
      className="m-2 md:m-3 p-3.5 rounded-2xl bg-raised border border-line shadow-sm flex flex-col gap-3 animate-fade-in"
    >
      <div className="flex gap-3 items-start">
        <div
          aria-hidden="true"
          className="w-10 h-10 rounded-xl flex items-center justify-center shrink-0 bg-accent-wash text-accent"
        >
          <Bell size={20} strokeWidth={1.75} />
        </div>
        <div className="grid gap-0.5 min-w-0">
          <strong className="text-sm font-semibold text-ink">Nhận thông báo khi có tin mới?</strong>
          <p className="text-xs text-ink-muted leading-relaxed">
            Tin nhắn trực tiếp, khi được nhắc tên và trả lời trong thread sẽ báo ngay cả khi app đang đóng.
          </p>
        </div>
      </div>

      {iosNotInstalled ? (
        <div className="flex items-center justify-between gap-3 pt-1 border-t border-line/60">
          <span className="text-xs font-medium text-ink">
            Thêm Nexus vào màn hình chính để nhận thông báo trên iPhone.
          </span>
          <Button variant="ghost" size="sm" onClick={handleDismiss}>
            Để sau
          </Button>
        </div>
      ) : (
        <div className="flex justify-end gap-2 pt-1">
          <Button variant="ghost" size="sm" onClick={handleDismiss}>
            Để sau
          </Button>
          <Button variant="primary" size="sm" onClick={handleEnable} disabled={enabling}>
            {enabling ? 'Đang bật…' : 'Bật thông báo'}
          </Button>
        </div>
      )}
    </div>
  )
}
