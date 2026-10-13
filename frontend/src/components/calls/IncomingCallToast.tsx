import { useRef } from 'react'
import { PhoneOff, Phone, Video } from 'lucide-react'
import { useFocusIn } from '../../hooks/useFocusIn'
import { useIncomingCallActions } from '../../hooks/useIncomingCallActions'
import { Avatar, Button } from '../primitives'
import type { IncomingCallInfo } from '../../stores/incoming-call.store'

interface Props {
  incoming: IncomingCallInfo
  onClear: (callId: string) => void
}

/**
 * Desktop floating card for an incoming call. Positioned top-right by the
 * layer that mounts it; this component owns only its own 360 px card.
 *
 * Mockup §3: `--color-overlay`, radius 16, 360 wide, avatar 56 with
 * spreading success ring, three buttons.
 */
export function IncomingCallToast({ incoming, onClear }: Props) {
  const { kind, callerName, callerAvatarUrl, mode } = incoming
  const { decline, answer, join, dismiss } = useIncomingCallActions(incoming, onClear)
  const ref = useRef<HTMLDivElement>(null)
  // The answer that matches the call takes the keyboard when the card appears, and gives it back when it goes.
  useFocusIn(ref)

  if (mode === 'answered_elsewhere') {
    return (
      <div
        ref={ref}
        role="alertdialog"
        aria-label="Cuộc gọi đã được trả lời"
        tabIndex={-1}
        className="p-4 rounded-2xl bg-overlay shadow-overlay animate-fade-in"
        style={{ width: 360 }}
      >
        <p className="text-sm text-ink-muted text-center">Đã trả lời trên thiết bị khác</p>
      </div>
    )
  }

  if (mode === 'invited') {
    return (
      <div
        ref={ref}
        role="alertdialog"
        aria-label="Lời mời cuộc gọi"
        className="p-4 rounded-2xl bg-overlay shadow-overlay grid gap-3.5 animate-fade-in"
        style={{ width: 360 }}
      >
        <div className="grid grid-cols-[56px_minmax(0,1fr)] gap-3 items-center">
          <Avatar name={callerName} src={callerAvatarUrl || undefined} size={52} />
          <div className="min-w-0">
            <p className="font-semibold text-ink truncate">{callerName}</p>
            <p className="text-sm text-ink-muted">mời bạn vào cuộc gọi</p>
          </div>
        </div>
        <div className="grid grid-cols-2 gap-2">
          <Button variant="soft" size="sm" onClick={dismiss}>
            Bỏ qua
          </Button>
          <Button variant="success" size="sm" onClick={join} data-primary>
            Tham gia
          </Button>
        </div>
      </div>
    )
  }

  // mode === 'ringing'
  const callText = kind === 'video' ? 'đang gọi video cho bạn…' : 'đang gọi thoại cho bạn…'
  return (
    <div
      ref={ref}
      role="alertdialog"
      aria-label={kind === 'video' ? 'Cuộc gọi video đến' : 'Cuộc gọi thoại đến'}
      className="p-4 rounded-2xl bg-overlay shadow-overlay grid gap-3.5 animate-fade-in"
      style={{ width: 360 }}
    >
      <div className="grid grid-cols-[56px_minmax(0,1fr)] gap-3 items-center">
        {/* Avatar 56 with spreading success ring — reduced motion: static ring */}
        <span className="relative">
          <Avatar name={callerName} src={callerAvatarUrl || undefined} size={52} />
          <span
            aria-hidden="true"
            className="absolute rounded-full border-2 border-success
              motion-safe:animate-[ring-spread_1.4s_ease-out_infinite]
              motion-reduce:opacity-100"
            style={{ inset: -4 }}
          />
        </span>
        <div className="min-w-0">
          <p className="font-semibold text-ink truncate">{callerName}</p>
          <p className="text-sm text-ink-muted">{callText}</p>
        </div>
      </div>
      <div className="grid grid-cols-3 gap-2">
        <Button variant="danger" size="sm" onClick={decline}>
          <PhoneOff size={16} strokeWidth={1.75} />
          Từ chối
        </Button>
        <Button variant="soft" size="sm" onClick={() => answer('audio')} data-primary={kind === 'audio' ? '' : undefined}>
          <Phone size={16} strokeWidth={1.75} />
          Thoại
        </Button>
        <Button variant="success" size="sm" onClick={() => answer('video')} data-primary={kind === 'video' ? '' : undefined}>
          <Video size={16} strokeWidth={1.75} />
          Video
        </Button>
      </div>
    </div>
  )
}
