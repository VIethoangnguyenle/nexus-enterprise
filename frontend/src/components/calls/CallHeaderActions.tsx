import { useNavigate } from '@tanstack/react-router'
import { Phone, Video } from 'lucide-react'
import type { CallKind } from '../../api/calls'
import { useOpenCall, useStartCall } from '../../hooks/useCalls'
import { useElapsed } from '../../hooks/useElapsed'
import { toMillis } from '../../lib/format'
import { callRoute } from '../../lib/call-nav'
import { stashTicket } from '../../stores/call.store'
import { Button, IconButton } from '../primitives'

interface CallHeaderActionsProps {
  channelId: string
  kind: 'dm' | 'space'
}

/**
 * Call buttons in the conversation header (DESIGN.md §6, mockup §1):
 * - DM: IconButtons "Gọi thoại" and "Gọi video", before the info button;
 * - group: pill "Bắt đầu cuộc gọi"; while the group has a call open it becomes
 *   "Tham gia" beside a pill "Đang gọi · mm:ss" with a pulsing dot (static under
 *   reduced motion).
 * Whether a call is open comes from the server and follows realtime call events.
 * A refused start (no right to write, calls off) is told by the mutation's toast.
 */
export function CallHeaderActions({ channelId, kind }: CallHeaderActionsProps) {
  const navigate = useNavigate()
  const start = useStartCall()
  const { data: open } = useOpenCall(channelId)

  const startCall = (callKind: CallKind) =>
    start.mutate(
      { channelId, kind: callKind },
      {
        // A direct call goes straight in; a group shows the pre-join first.
        onSuccess: (ticket) => {
          // The ticket that came with starting is the one the call screen joins with: no second one is minted.
          stashTicket(ticket)
          void navigate(callRoute(ticket.call.id, kind === 'dm' ? callKind : undefined))
        },
      },
    )

  if (kind === 'dm') {
    return (
      <>
        <IconButton aria-label="Gọi thoại" title="Gọi thoại" disabled={start.isPending} onClick={() => startCall('audio')}>
          <Phone size={18} strokeWidth={1.75} />
        </IconButton>
        <IconButton aria-label="Gọi video" title="Gọi video" disabled={start.isPending} onClick={() => startCall('video')}>
          <Video size={18} strokeWidth={1.75} />
        </IconButton>
      </>
    )
  }

  if (open) {
    const startedMs = toMillis(open.started_at ?? open.created_at) || null
    return (
      <>
        <LivePill startMs={startedMs} />
        <Button
          variant="success"
          size="sm"
          onClick={() => void navigate(callRoute(open.id))}
        >
          <Video size={16} strokeWidth={1.75} aria-hidden="true" />
          Tham gia
        </Button>
      </>
    )
  }

  return (
    <Button
      variant="tonal"
      size="sm"
      disabled={start.isPending}
      onClick={() => startCall('video')}
      className="!rounded-full !h-9 !px-3.5 max-sm:!px-2.5"
    >
      <Video size={16} strokeWidth={1.75} className="text-accent" aria-hidden="true" />
      {/* A phone header has room for the icon only; the name stays for screen readers. */}
      <span className="max-sm:sr-only">Bắt đầu cuộc gọi</span>
    </Button>
  )
}

/** "Đang gọi · mm:ss" with a pulsing dot. The clock is not announced each second. */
function LivePill({ startMs }: { startMs: number | null }) {
  const elapsed = useElapsed(startMs)
  return (
    // On a phone "Tham gia" alone says a call is on; the clock would push the name out.
    <span className="max-sm:hidden inline-flex items-center gap-1.5 h-5.5 px-2 rounded-full bg-success-wash text-ink text-xs font-semibold">
      <span className="w-2 h-2 rounded-full bg-success animate-pulse motion-reduce:animate-none" aria-hidden="true" />
      Đang gọi · <span className="tnum">{elapsed}</span>
    </span>
  )
}
