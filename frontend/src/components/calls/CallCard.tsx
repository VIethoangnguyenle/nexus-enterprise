import { useNavigate } from '@tanstack/react-router'
import { Phone, PhoneMissed, Video } from 'lucide-react'
import type { Call, CallKind, CallStatus } from '../../api/calls'
import { useCall, useStartCall } from '../../hooks/useCalls'
import { useElapsed } from '../../hooks/useElapsed'
import { useAuthStore } from '../../stores/auth.store'
import { stashTicket } from '../../stores/call.store'
import { callRoute } from '../../lib/call-nav'
import { toDate, formatTime } from '../../lib/format'
import { Avatar, Button } from '../primitives'
import { CardSkeleton, UnavailableCard } from '../chat/ChatCard'

const MAX_FACES = 3

/** Format a duration for ended calls: "X phút Y giây", "X giờ Y phút", etc. */
function formatDurationLong(seconds: number): string {
  if (seconds < 60) return `${Math.floor(seconds)} giây`
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  if (m < 60) return s > 0 ? `${m} phút ${s} giây` : `${m} phút`
  const h = Math.floor(m / 60)
  const rm = m % 60
  return rm > 0 ? `${h} giờ ${rm} phút` : `${h} giờ`
}

type CallState = 'active' | 'ringing' | 'ended' | 'missed'

function callState(status: CallStatus): CallState {
  if (status === 'active' || status === 'ringing') return status === 'ringing' ? 'ringing' : 'active'
  if (status === 'missed' || status === 'declined' || status === 'cancelled') return 'missed'
  return 'ended'
}

const ICON_CLASS: Record<CallState, string> = {
  active: 'bg-success-wash text-success',
  ringing: 'bg-success-wash text-success',
  ended: 'bg-accent-wash text-accent',
  missed: 'bg-danger-wash text-danger',
}

function CallIcon({ kind, state }: { kind: CallKind; state: CallState }) {
  const Icon = state === 'missed' ? PhoneMissed : kind === 'audio' ? Phone : Video
  return <Icon size={20} strokeWidth={1.75} aria-hidden="true" />
}

interface CallCardProps {
  callId: string
  channelId: string
}

/**
 * A call card in the message stream (DESIGN.md §6, mockup §1 "Thẻ cuộc gọi"):
 * raised background, inner line border, radius 14, max width 420.
 *
 * States:
 * - active/ringing: success-wash icon tile, up to 3 avatars + "và N người",
 *   live mm:ss clock, "Tham gia" button (or "Mở lại cuộc gọi" if current user is in call)
 * - ended: accent-wash icon, N người · duration · HH:mm–HH:mm
 * - missed/declined/cancelled: danger-wash, "<caller> gọi lúc HH:mm" + "Gọi lại"
 */
export function CallCard({ callId, channelId }: CallCardProps) {
  const navigate = useNavigate()
  const meId = useAuthStore((s) => s.user?.id)

  const { data: call, isError } = useCall(callId)

  const start = useStartCall()

  // A call that cannot be read (no access, gone), or one that belongs to another conversation than the message
  // that links it (a message can name any call id), says so instead of showing a stranger's call or loading for ever.
  if (isError || (call && call.channel_id !== channelId)) {
    return (
      <UnavailableCard
        tone="warning"
        icon={<Phone size={16} strokeWidth={1.75} aria-hidden="true" />}
        title="Không xem được cuộc gọi này"
      />
    )
  }
  if (!call) return <CardSkeleton />

  const state = callState(call.status)
  const kind = call.kind
  const participants = call.participants ?? []
  const participantCount = call.taker_count ?? participants.length
  const userInCall = !!meId && participants.some((p) => p.user_id === meId)

  const startedAtMs = toDate(call.started_at)?.getTime() ?? 0
  const endedAtMs = toDate(call.ended_at)?.getTime() ?? 0
  const createdAtMs = toDate(call.created_at)?.getTime() ?? 0

  const handleJoin = () => {
    void navigate(callRoute(callId))
  }

  // Calling back a missed direct call goes straight in, as calling does; a
  // refusal is told by the mutation's toast.
  const handleCallback = () =>
    start.mutate(
      { channelId, kind },
      {
        onSuccess: (ticket) => {
          stashTicket(ticket)
          void navigate(callRoute(ticket.call.id, kind))
        },
      },
    )

  const kindLabel = kind === 'audio' ? 'thoại' : 'video'

  return (
    <div
      role="group"
      aria-label={`Cuộc gọi ${kindLabel}`}
      // On a phone the action takes its own full-width row under the text (mockup "Trên điện thoại"),
      // so the text keeps the card's width instead of wrapping beside the button.
      className="w-full max-w-105 rounded-lg bg-raised ring-1 ring-inset ring-line px-3.5 py-3
        grid gap-3 items-center grid-cols-[44px_minmax(0,1fr)] sm:grid-cols-[44px_minmax(0,1fr)_auto]"
    >
      {/* Icon tile */}
      <span className={`grid place-items-center w-11 h-11 rounded-xl ${ICON_CLASS[state]}`}>
        <CallIcon kind={kind} state={state} />
      </span>

      {/* Body */}
      <div className="grid min-w-0">
        {state === 'active' || state === 'ringing' ? (
          <ActiveBody
            call={call}
            state={state}
            meId={meId}
            participantCount={participantCount}
            participants={participants}
            startedAtMs={startedAtMs || createdAtMs}
          />
        ) : state === 'ended' ? (
          <EndedBody
            call={call}
            participants={participants}
            participantCount={participantCount}
            startedAtMs={startedAtMs}
            endedAtMs={endedAtMs}
          />
        ) : (
          <MissedBody call={call} />
        )}
      </div>

      {/* Action */}
      {(state === 'active' || state === 'ringing') && (
        <Button variant="success" size="sm" onClick={handleJoin} className="max-sm:col-span-2">
          {userInCall ? 'Mở lại cuộc gọi' : 'Tham gia'}
        </Button>
      )}
      {state === 'missed' && (
        <Button variant="primary" size="sm" onClick={handleCallback} loading={start.isPending} className="max-sm:col-span-2">
          <CallIcon kind={kind} state="ended" />
          Gọi lại
        </Button>
      )}
      {state === 'ended' && <span className="max-sm:hidden" />}
    </div>
  )
}

function ActiveBody({
  call,
  state,
  meId,
  participantCount,
  participants,
  startedAtMs,
}: {
  call: Call
  state: 'active' | 'ringing'
  meId: string | undefined
  participantCount: number
  participants: Call['participants'] & object
  startedAtMs: number
}) {
  const isCaller = !!meId && call.started_by === meId
  const isLive = state === 'active'

  return (
    <>
      <b className="font-semibold text-ink">
        {state === 'ringing' && isCaller ? 'Đang gọi…' : 'Cuộc gọi đang diễn ra'}
      </b>
      <span className="flex items-center gap-2 text-sm text-ink-muted mt-0.5">
        {participants.length > 0 && (
          <span className="flex -space-x-1.5 shrink-0">
            {participants.slice(0, MAX_FACES).map((p) => (
              <Avatar
                key={p.user_id}
                name={p.display_name ?? ''}
                hueKey={p.user_id}
                src={p.avatar_url || undefined}
                size={20}
                className="ring-2 ring-raised"
              />
            ))}
          </span>
        )}
        <span className="min-w-0 truncate">
          <ParticipantNames participants={participants} count={participantCount} />
          {isLive && (
            <>
              {' · '}
              <LiveClock startMs={startedAtMs} />
            </>
          )}
        </span>
      </span>
    </>
  )
}

function LiveClock({ startMs }: { startMs: number }) {
  const time = useElapsed(startMs > 0 ? startMs : null)
  return <span className="tnum">{time}</span>
}

function EndedBody({
  call,
  participants,
  participantCount,
  startedAtMs,
  endedAtMs,
}: {
  call: Call
  participants: Call['participants'] & object
  participantCount: number
  startedAtMs: number
  endedAtMs: number
}) {
  const durationSec = startedAtMs > 0 && endedAtMs > 0 ? Math.floor((endedAtMs - startedAtMs) / 1000) : 0
  const startTime = formatTime(call.started_at)
  const endTime = formatTime(call.ended_at)
  const kindLabel = call.kind === 'audio' ? 'thoại' : 'video'

  return (
    <>
      <b className="font-semibold text-ink">Cuộc gọi {kindLabel}</b>
      <span className="flex items-center gap-2 text-sm text-ink-muted mt-0.5">
        {participants.length > 0 && (
          <span className="flex -space-x-1.5 shrink-0">
            {participants.slice(0, MAX_FACES).map((p) => (
              <Avatar
                key={p.user_id}
                name={p.display_name ?? ''}
                hueKey={p.user_id}
                src={p.avatar_url || undefined}
                size={20}
                className="ring-2 ring-raised"
              />
            ))}
          </span>
        )}
        <span className="tnum min-w-0 truncate">
          {participantCount > 0 && <>{participantCount} người · </>}
          {durationSec > 0 && <>{formatDurationLong(durationSec)} · </>}
          {startTime && endTime && <>{startTime}–{endTime}</>}
        </span>
      </span>
    </>
  )
}

function MissedBody({ call }: { call: Call }) {
  const kindLabel = call.kind === 'audio' ? 'thoại' : 'video'
  const callerName = call.started_by_name ?? ''
  const createdTime = formatTime(call.created_at)

  return (
    <>
      <b className="font-semibold text-ink">Cuộc gọi {kindLabel} nhỡ</b>
      <span className="text-sm text-ink-muted mt-0.5">
        {callerName} gọi lúc {createdTime}
      </span>
    </>
  )
}

function ParticipantNames({ participants, count }: { participants: Call['participants'] & object; count: number }) {
  const names = participants.slice(0, MAX_FACES).map((p) => p.display_name ?? '').filter(Boolean)
  const extra = count - names.length
  if (names.length === 0) return null
  return (
    <>
      {names.join(', ')}
      {extra > 0 && ` và ${extra} người`}
    </>
  )
}
