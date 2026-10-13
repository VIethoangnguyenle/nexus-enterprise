import { useEffect, useRef, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { Phone, PhoneMissed, Video } from 'lucide-react'
import { callsApi, type Call, type CallKind, type CallStatus } from '../../api/calls'
import { callKeys } from '../../hooks/keys/calls'
import { useStartCall } from '../../hooks/useCalls'
import { useAuthStore } from '../../stores/auth.store'
import { toDate, formatTime } from '../../lib/format'
import { Avatar, Button } from '../primitives'
import { CardSkeleton } from '../chat/ChatCard'

const MAX_FACES = 3

/** Format a duration in seconds to mm:ss */
function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

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

/** Live elapsed time from a start timestamp. */
function useLiveClock(startMs: number, active: boolean): string {
  const [elapsed, setElapsed] = useState(() => active && startMs > 0 ? Math.max(0, Math.floor((Date.now() - startMs) / 1000)) : 0)
  const rafRef = useRef(0)

  useEffect(() => {
    if (!active || startMs <= 0) return
    const tick = () => {
      setElapsed(Math.max(0, Math.floor((Date.now() - startMs) / 1000)))
      rafRef.current = requestAnimationFrame(tick)
    }
    // Update every second using requestAnimationFrame for smoothness
    const id = setInterval(() => {
      setElapsed(Math.max(0, Math.floor((Date.now() - startMs) / 1000)))
    }, 1000)
    tick()
    return () => {
      clearInterval(id)
      cancelAnimationFrame(rafRef.current)
    }
  }, [active, startMs])

  return formatDuration(elapsed)
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

  const { data: call, isLoading } = useQuery({
    queryKey: callKeys.detail(callId),
    queryFn: () => callsApi.get(callId),
  })

  const start = useStartCall()

  if (isLoading || !call) {
    return <CardSkeleton />
  }

  const state = callState(call.status)
  const kind = call.kind
  const participants = call.participants ?? []
  const participantCount = call.taker_count ?? participants.length
  const userInCall = !!meId && participants.some((p) => p.user_id === meId)

  const startedAtMs = toDate(call.started_at)?.getTime() ?? 0
  const endedAtMs = toDate(call.ended_at)?.getTime() ?? 0
  const createdAtMs = toDate(call.created_at)?.getTime() ?? 0

  const handleJoin = () => {
    void navigate({ to: '/calls/$callId', params: { callId }, search: {} })
  }

  // Calling back a missed direct call goes straight in, as calling does; a
  // refusal is told by the mutation's toast.
  const handleCallback = () =>
    start.mutate(
      { channelId, kind },
      { onSuccess: ({ call }) => void navigate({ to: '/calls/$callId', params: { callId: call.id }, search: { answer: kind } }) },
    )

  const kindLabel = kind === 'audio' ? 'thoại' : 'video'

  return (
    <div
      role="group"
      aria-label={`Cuộc gọi ${kindLabel}`}
      className="w-full max-w-105 rounded-lg bg-raised ring-1 ring-inset ring-line px-3.5 py-3
        grid gap-3 items-center"
      style={{ gridTemplateColumns: '44px minmax(0,1fr) auto' }}
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
        <Button variant="success" size="sm" onClick={handleJoin}>
          {userInCall ? 'Mở lại cuộc gọi' : 'Tham gia'}
        </Button>
      )}
      {state === 'missed' && (
        <Button variant="primary" size="sm" onClick={handleCallback} loading={start.isPending}>
          <CallIcon kind={kind} state="ended" />
          Gọi lại
        </Button>
      )}
      {state === 'ended' && <span />}
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
  const time = useLiveClock(startMs, true)
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
