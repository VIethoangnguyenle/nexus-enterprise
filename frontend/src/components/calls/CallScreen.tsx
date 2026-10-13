import { useCallback, useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { messagingApi } from '../../api/messaging'
import { statusOf } from '../../lib/errors'
import { loadCallPrefs, type CallPrefs } from '../../lib/call-prefs'
import { displayName } from '../../lib/people'
import { useAuthStore } from '../../stores/auth.store'
import { callRoute } from '../../lib/call-nav'
import { isLive, takeTicket, useCallStore } from '../../stores/call.store'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useCall, useMessagesSince } from '../../hooks/useCalls'
import { useCallShortcuts } from '../../hooks/useCallShortcuts'
import { useConversations } from '../../hooks/useConversations'
import { usePeople } from '../../hooks/usePeople'
import { usePhone } from '../../hooks/usePhone'
import { keys } from '../../hooks/keys'
import { useCallApi, useCallSnapshot } from './CallProvider'
import { CallStage } from './CallStage'
import {
  CallOverState, ConnectingState, EndedSummary, FailedState, NoAccessState, OtherCallNotice, RemovedState,
} from './CallStates'
import { PhoneCallView } from './PhoneCallView'
import { PreJoin } from './PreJoin'
import './call-stage.css'

const route = getRouteApi('/_workspace/calls/$callId')

/** The route's component: reads the address, then hands over to the screen. */
export function CallRoute() {
  const { callId } = route.useParams()
  const { answer } = route.useSearch()
  // Keyed by the call: another address is another call, with nothing carried over from the one before.
  return <CallScreen key={callId} callId={callId} answer={answer} />
}

const FINISHED = new Set(['ended', 'missed', 'declined', 'cancelled'])

/**
 * One call's screen. What it shows follows where you are in the call: the
 * check before joining (a group), a spinner (a direct call or an answer, which
 * join straight away), the stage, or how it ended. The room itself is owned by
 * `CallProvider`, so leaving this screen does not leave the call.
 */
export function CallScreen({ callId, answer }: { callId: string; answer?: 'video' | 'audio' }) {
  const navigate = useNavigate()
  const api = useCallApi()
  const snapshot = useCallSnapshot()
  const phone = usePhone()
  const user = useAuthStore((s) => s.user)
  const { workspaceId } = useActiveWorkspace()
  const people = usePeople(workspaceId)
  const session = useCallStore((s) => s.session)
  const phase = useCallStore((s) => s.phase)
  const summary = useCallStore((s) => s.summary)
  // A refusal counts only for the call it was about.
  const refusal = useCallStore((s) => (s.refusalCallId === callId ? s.refusal : null))
  const sameCall = session?.callId === callId

  const callQ = useCall(callId, { poll: !sameCall })
  const call = callQ.data
  const channelId = call?.channel_id ?? ''

  // The conversation's name and kind: from the lists the navigator already holds, else its own record.
  const { conversations, isLoading: listLoading } = useConversations(workspaceId)
  const listed = conversations.find((c) => c.id === channelId)
  const single = useQuery({
    queryKey: keys.messaging.channel(channelId),
    queryFn: () => messagingApi.getChannel(channelId),
    enabled: !!channelId && !listLoading && !listed,
  })
  const isDm = listed ? listed.kind === 'dm' : single.data?.channel_type === 'dm'
  const title = listed?.title
    ?? (isDm ? (call?.started_by === user?.id ? 'Cuộc gọi' : call?.started_by_name) : single.data?.name)
    ?? ''
  const known = !!call && (!!listed || single.isSuccess || single.isError)

  const selfName = displayName(people, user?.id, user?.username)
  const toChat = useCallback(() => {
    if (!channelId) { void navigate({ to: '/channels' }); return }
    void navigate({ to: '/channels/$channelId', params: { channelId }, search: (prev: { ws?: string }) => ({ ws: prev.ws }) })
  }, [navigate, channelId])

  const enter = useCallback((prefs: CallPrefs, audio: boolean, video: boolean) => {
    if (!call) return
    // The ticket that came with starting the call, if this screen is where that led.
    const ticket = takeTicket(callId) ?? undefined
    void api.join({
      callId, channelId: call.channel_id, kind: call.kind, title, isDm: !!isDm, audio, video, prefs, ticket,
    })
  }, [api, call, callId, title, isDm])

  // A direct call, or an answer from a notification, skips the check and joins with what was remembered.
  const direct = !!answer || !!isDm
  const tried = useRef(false)
  const open = !!call && !FINISHED.has(call.status)
  // Only this call's own session holds the screen back (its stage, or its summary); another call's leftovers do not.
  const blocked = sameCall || (!!session && isLive(phase))
  useEffect(() => {
    if (tried.current || !direct || !known || !open || blocked || refusal) return
    tried.current = true
    const prefs = loadCallPrefs(user?.id)
    const video = answer ? answer === 'video' : prefs.camOn && call.kind === 'video'
    enter(prefs, answer ? true : prefs.micOn, video)
  }, [direct, known, open, blocked, refusal, answer, user?.id, call, enter])

  const live = sameCall && isLive(phase)
  useCallShortcuts(api.callRoom, live && !phone)
  const messages = useMessagesSince(session?.channelId ?? channelId, summary?.sinceMs ?? 0, { includeMine: true })

  const leaveAndBack = useCallback(async () => {
    await api.leave()
    useCallStore.getState().clear()
    toChat()
  }, [api, toChat])

  const body = (() => {
    // Another call is running here.
    if (session && !sameCall && isLive(phase)) {
      // Wait for the call so "Rời và vào" has something to join.
      if (!call || !known) return <ConnectingState onCancel={toChat} />
      return (
        <OtherCallNotice
          currentTitle={session.title}
          onSwitch={() => { void api.join({ callId, channelId: call.channel_id, kind: call.kind, title, isDm: !!isDm, audio: true, video: call.kind === 'video', ticket: takeTicket(callId) ?? undefined }) }}
          onStay={() => void navigate(callRoute(session.callId))}
        />
      )
    }
    if (sameCall) {
      if (phase === 'ended' && summary) {
        return (
          <EndedSummary
            summary={summary} messages={messages} onBack={() => { useCallStore.getState().clear(); toChat() }}
            onRejoin={() => { tried.current = false; useCallStore.getState().clear() }}
          />
        )
      }
      if (phase === 'removed') return <RemovedState onBack={() => { useCallStore.getState().clear(); void navigate({ to: '/channels' }) }} />
      if (phase === 'failed') return <FailedState onRetry={() => void api.retry()} onLeave={() => void leaveAndBack()} />
      if (!api.callRoom || phase === 'connecting') return <ConnectingState onCancel={() => void leaveAndBack()} />
      const props = { callRoom: api.callRoom, snapshot, session: session!, call, onLeave: () => void api.leave() }
      return phone
        // Portalled to the body: the app's content area is its own stacking layer, under the phone's bottom bar and avatar.
        ? createPortal(<div className="fixed inset-0 z-modal flex flex-col bg-stage"><PhoneCallView {...props} onMinimize={toChat} /></div>, document.body)
        : <CallStage {...props} />
    }
    // No call of ours yet.
    if (callQ.isError) {
      const s = statusOf(callQ.error)
      return s === 403 ? <NoAccessState onBack={() => void navigate({ to: '/channels' })} /> : <CallOverState onBack={toChat} />
    }
    if (!call || !known) return <ConnectingState onCancel={toChat} />
    if (FINISHED.has(call.status) || refusal === 'call_ended') return <CallOverState onBack={toChat} />
    if (refusal === 'forbidden') return <NoAccessState onBack={() => void navigate({ to: '/channels' })} />
    if (direct && !refusal) return <ConnectingState onCancel={toChat} />
    return (
      <PreJoin
        call={call} selfId={user?.id} selfName={selfName} refusal={refusal} joining={false}
        onJoin={enter} onCancel={toChat}
      />
    )
  })()

  return <div className="flex-1 flex flex-col min-h-0 min-w-0" data-testid="call-screen">{body}</div>
}
