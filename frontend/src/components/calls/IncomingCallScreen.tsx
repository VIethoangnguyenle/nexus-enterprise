import { useId, useRef } from 'react'
import { PhoneOff, Phone, Video } from 'lucide-react'
import { useFocusIn } from '../../hooks/useFocusIn'
import { useIncomingCallActions } from '../../hooks/useIncomingCallActions'
import { Avatar, Pressable } from '../primitives'
import type { IncomingCallInfo } from '../../stores/incoming-call.store'

interface Props {
  incoming: IncomingCallInfo
  onClear: (callId: string) => void
}

/**
 * Phone full-screen ringing overlay using stage tokens. Per mockup §3:
 * avatar 112, name 26/700, three round 68 px buttons (Từ chối stage-danger,
 * Thoại, Video success).
 */
export function IncomingCallScreen({ incoming, onClear }: Props) {
  const { kind, callerName, callerAvatarUrl, mode } = incoming
  const { decline, answer, join, dismiss } = useIncomingCallActions(incoming, onClear)
  const ref = useRef<HTMLDivElement>(null)
  const nameId = useId()
  useFocusIn(ref)

  // One modal surface for every mode: named by the caller, focus inside it while it covers the app.
  const frame = (extra: string, children: React.ReactNode) => (
    <div
      ref={ref}
      role="dialog"
      aria-modal="true"
      aria-labelledby={nameId}
      tabIndex={-1}
      className={`fixed inset-0 z-50 bg-stage flex ${extra} animate-fade-in outline-none`}
    >
      {children}
    </div>
  )

  if (mode === 'answered_elsewhere') {
    return frame('items-center justify-center', (
      <p id={nameId} className="text-on-stage-muted text-16">Đã trả lời trên thiết bị khác</p>
    ))
  }

  if (mode === 'invited') {
    return frame('flex-col items-center justify-center gap-6', (
      <>
        <div className="flex flex-col items-center gap-3.5 text-center px-6">
          <Avatar name={callerName} src={callerAvatarUrl || undefined} size={64} className="!w-28 !h-28 !text-2xl" />
          <p id={nameId} className="text-on-stage font-bold text-2xl leading-tight">{callerName}</p>
          <p className="text-on-stage-muted text-sm">mời bạn vào cuộc gọi</p>
        </div>
        <div className="flex justify-around w-full max-w-xs px-7 pb-12">
          <div className="grid justify-items-center gap-2 text-xs text-on-stage-muted">
            <Pressable
              aria-label="Bỏ qua"
              onClick={dismiss}
              className="rounded-full bg-stage-raised text-on-stage flex items-center justify-center"
              style={{ width: 68, height: 68 }}
            >
              <PhoneOff size={28} strokeWidth={1.75} />
            </Pressable>
            Bỏ qua
          </div>
          <div className="grid justify-items-center gap-2 text-xs text-on-stage-muted">
            <Pressable
              aria-label="Tham gia"
              onClick={join}
              data-primary
              className="rounded-full bg-success text-on-success flex items-center justify-center"
              style={{ width: 68, height: 68 }}
            >
              <Video size={28} strokeWidth={1.75} />
            </Pressable>
            Tham gia
          </div>
        </div>
      </>
    ))
  }

  // mode === 'ringing'
  const subtitle = kind === 'video' ? 'Cuộc gọi video đến · Nexus' : 'Cuộc gọi thoại đến · Nexus'
  return frame('flex-col items-center', (
    <>
      {/* Center content */}
      <div className="flex-1 flex flex-col items-center justify-center gap-3.5 text-center px-6">
        {/* Avatar 112 with spreading success ring */}
        <span className="relative">
          <Avatar name={callerName} src={callerAvatarUrl || undefined} size={64} className="!w-28 !h-28 !text-2xl" />
          <span
            aria-hidden="true"
            className="absolute rounded-full border-2 border-success
              motion-safe:animate-[ring-spread_1.4s_ease-out_infinite]
              motion-reduce:opacity-100"
            style={{ inset: -4 }}
          />
        </span>
        <p id={nameId} className="text-on-stage font-bold text-2xl leading-tight">{callerName}</p>
        <p className="text-on-stage-muted text-sm">{subtitle}</p>
      </div>

      {/* Action buttons */}
      <div className="flex justify-around w-full max-w-xs px-7 pb-12">
        <div className="grid justify-items-center gap-2 text-xs text-on-stage-muted">
          <Pressable
            aria-label="Từ chối"
            onClick={decline}
            className="rounded-full bg-stage-danger text-on-stage flex items-center justify-center"
            style={{ width: 68, height: 68 }}
          >
            <PhoneOff size={28} strokeWidth={1.75} />
          </Pressable>
          Từ chối
        </div>
        <div className="grid justify-items-center gap-2 text-xs text-on-stage-muted">
          <Pressable
            aria-label="Thoại"
            onClick={() => answer('audio')}
            data-primary={kind === 'audio' ? '' : undefined}
            className="rounded-full bg-stage-raised text-on-stage flex items-center justify-center"
            style={{ width: 68, height: 68 }}
          >
            <Phone size={28} strokeWidth={1.75} />
          </Pressable>
          Thoại
        </div>
        <div className="grid justify-items-center gap-2 text-xs text-on-stage-muted">
          <Pressable
            aria-label="Video"
            onClick={() => answer('video')}
            data-primary={kind === 'video' ? '' : undefined}
            className="rounded-full bg-success text-on-success flex items-center justify-center"
            style={{ width: 68, height: 68 }}
          >
            <Video size={28} strokeWidth={1.75} />
          </Pressable>
          Video
        </div>
      </div>
    </>
  ))
}
