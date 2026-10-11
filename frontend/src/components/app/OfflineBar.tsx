import { CheckCircle2, RefreshCw, WifiOff } from 'lucide-react'
import { useConnectionStatus } from '../../hooks/useConnectionLost'
import { useAuthStore } from '../../stores/auth.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { Button } from '../primitives'

/**
 * The one app-wide connection bar (design/mockups/push.html §5), under the top
 * bar: warning while the socket or the network is down, "Đã kết nối lại" in the
 * success colour for two seconds, then gone. Cached screens stay as they were.
 */
export function OfflineBar() {
  const status = useConnectionStatus()
  const connect = useWebSocketStore((s) => s.connect)
  const token = useAuthStore((s) => s.accessToken)

  // The live region is always present and only its content changes, so screen readers announce it.
  return (
    <div role="status">
      {status === 'restored' && (
        <div className="flex items-center gap-2.5 min-h-11 pl-4 pr-2 py-1.5 shrink-0 text-sm font-semibold bg-success-wash text-ink">
          <CheckCircle2 size={16} strokeWidth={1.75} className="text-success shrink-0" aria-hidden />
          <span className="flex-1 min-w-0">Đã kết nối lại</span>
        </div>
      )}
      {status === 'lost' && (
        <div className="flex items-center gap-2.5 min-h-11 pl-4 pr-2 py-1.5 shrink-0 text-sm font-semibold bg-warning-wash text-ink">
          <WifiOff size={16} strokeWidth={1.75} className="text-warning shrink-0" aria-hidden />
          <span className="flex-1 min-w-0">Mất kết nối. Đang thử kết nối lại…</span>
          <Button
            variant="ghost"
            size="sm"
            className="min-h-9 inline-flex items-center gap-1.5"
            onClick={() => { if (token) connect(token) }}
          >
            <RefreshCw size={14} strokeWidth={1.75} aria-hidden />
            Thử lại
          </Button>
        </div>
      )}
    </div>
  )
}
