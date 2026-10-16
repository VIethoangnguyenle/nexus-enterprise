import {
  Suspense, createContext, lazy, useCallback, useContext, useEffect, useMemo, useRef, useState, useSyncExternalStore,
  type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'
import { useNavigate, useRouterState } from '@tanstack/react-router'
import { callsApi, refusalOf, type Call, type CallJoin, type CallKind } from '../../api/calls'
import { callRoute } from '../../lib/call-nav'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import type { CallRoom, CallSnapshot } from '../../lib/call-room'
import { EMPTY_SNAPSHOT } from '../../lib/call-model'
import { loadCallPrefs, type CallPrefs } from '../../lib/call-prefs'
import { toMillis } from '../../lib/format'
import { avatarOf, displayName } from '../../lib/people'
import { useAuthStore } from '../../stores/auth.store'
import { isLive, useCallStore } from '../../stores/call.store'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { usePeople } from '../../hooks/usePeople'
import { usePhone } from '../../hooks/usePhone'
import { useCallPictureInPicture } from '../../hooks/useCallPictureInPicture'
import { RemoteAudio } from './media'

export interface JoinRequest {
  callId: string
  channelId: string
  kind: CallKind
  /** The conversation's name. */
  title: string
  isDm: boolean
  /** Join with the microphone and camera on. */
  audio: boolean
  video: boolean
  /** An entry ticket already in hand (from starting the call); otherwise one is asked for. */
  ticket?: CallJoin
  /** Devices chosen in the check; defaults to what was remembered. */
  prefs?: CallPrefs
}

interface CallApi {
  callRoom: CallRoom | null
  join: (req: JoinRequest) => Promise<void>
  leave: () => Promise<void>
  /** Try again after "Không kết nối được": a fresh ticket and a fresh room. */
  retry: () => Promise<void>
}

// The floating tile and its styles load when a call is first minimised or the tab is left.
const MiniCall = lazy(() => import('./MiniCall').then((m) => ({ default: m.MiniCall })))

const CallContext = createContext<CallApi | null>(null)

export function useCallApi(): CallApi {
  const ctx = useContext(CallContext)
  if (!ctx) throw new Error('useCallApi needs a CallProvider')
  return ctx
}

const NO_SUBSCRIBE = () => () => {}
const NO_SNAPSHOT = () => EMPTY_SNAPSHOT

/** The room's current state; an empty one while there is no room. */
export function useCallSnapshot(): CallSnapshot {
  const { callRoom } = useCallApi()
  return useSyncExternalStore(callRoom?.subscribe ?? NO_SUBSCRIBE, callRoom?.getSnapshot ?? NO_SNAPSHOT)
}

const startedAtOf = (join: CallJoin | undefined, fallback: number): number =>
  toMillis(join?.call.started_at) || toMillis(join?.call.created_at) || fallback

/**
 * Owns the call room for the whole workspace shell. The room lives here, above
 * every route, so going from the call to a conversation and back does not drop
 * it. While a call is open and its screen is not, a small tile floats over the
 * app (on a desktop that is hidden, Picture-in-Picture takes over). The SDK is
 * loaded here only when a call starts.
 */
export function CallProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const user = useAuthStore((s) => s.user)
  const { workspaceId } = useActiveWorkspace()
  const people = usePeople(workspaceId)
  const pictureOf = (userId: string) => avatarOf(people, userId)
  const phone = usePhone()
  const phase = useCallStore((s) => s.phase)
  const session = useCallStore((s) => s.session)

  const [callRoom, setCallRoom] = useState<CallRoom | null>(null)
  const roomRef = useRef<CallRoom | null>(null)
  const unsubRef = useRef<(() => void) | null>(null)
  const lastRef = useRef<JoinRequest | null>(null)
  /** Guards a slow join: a newer join or a leave makes the older one drop what it built. */
  const attempt = useRef(0)

  const selfId = user?.id ?? ''
  const selfName = displayName(people, user?.id, user?.username)

  const teardown = useCallback(() => {
    unsubRef.current?.()
    unsubRef.current = null
    roomRef.current?.dispose()
    roomRef.current = null
    setCallRoom(null)
  }, [])

  /** Mirrors the room's state into the store, and ends the call in the store when the room ends. */
  const follow = useCallback((cr: CallRoom) => {
    const sync = () => {
      const snap = cr.getSnapshot()
      const store = useCallStore.getState()
      if (store.phase === 'ended' || store.phase === 'removed') return
      if (snap.link === 'disconnected') {
        const joinedAt = store.session?.joinedAt ?? Date.now()
        const summary = { durationMs: Date.now() - joinedAt, people: snap.peak, sinceMs: joinedAt }
        if (snap.end === 'failed') store.setPhase('failed')
        else store.finish(summary, snap.end === 'removed' ? 'removed' : 'ended')
        if (snap.end !== 'failed') queueMicrotask(() => { if (roomRef.current === cr) teardown() })
      } else if (snap.link === 'reconnecting') store.setPhase('reconnecting')
      else if (snap.link === 'connected') store.setPhase('connected')
    }
    unsubRef.current?.()
    unsubRef.current = cr.subscribe(sync)
  }, [teardown])

  const join = useCallback(async (req: JoinRequest) => {
    const mine = ++attempt.current
    lastRef.current = req
    if (roomRef.current) {
      await roomRef.current.disconnect().catch(() => {})
      teardown()
    }
    const store = useCallStore.getState()
    const now = Date.now()
    store.begin({
      callId: req.callId, channelId: req.channelId, kind: req.kind, title: req.title, isDm: req.isDm,
      joinedAt: now, startedAt: startedAtOf(req.ticket, now),
    })

    let ticket = req.ticket
    try {
      ticket ??= await callsApi.token(req.callId)
    } catch (err) {
      if (attempt.current !== mine) return
      const refusal = refusalOf(err)
      if (refusal === 'unreachable') {
        useCallStore.getState().setRefusal(refusal, req.callId)
        useCallStore.getState().setPhase('failed')
      } else {
        // Refused for a reason the pre-join screen explains: leave the call and tell it why.
        useCallStore.getState().clear()
        useCallStore.getState().setRefusal(refusal, req.callId)
      }
      return
    }
    if (attempt.current !== mine) return
    // What the ticket says of the call is as fresh as anything: hanging up reads it to know whether it still rings.
    queryClient.setQueryData(keys.calls.detail(req.callId), ticket.call)
    useCallStore.setState((s) => (s.session ? { session: { ...s.session, startedAt: startedAtOf(ticket, now) } } : s))

    const prefs = req.prefs ?? loadCallPrefs(selfId)
    try {
      const { createCallRoom } = await import('../../lib/call-room')
      if (attempt.current !== mine) return
      const cr = createCallRoom({
        selfId, selfName, micId: prefs.micId, camId: prefs.camId, speakerId: prefs.speakerId,
      })
      roomRef.current = cr
      setCallRoom(cr)
      follow(cr)
      await cr.connect(ticket.url, ticket.token, { micOn: req.audio, camOn: req.video && req.kind === 'video' })
      if (attempt.current !== mine) { await cr.disconnect().catch(() => {}); return }
      useCallStore.getState().setPhase('connected')
    } catch {
      if (attempt.current !== mine) return
      useCallStore.getState().setRefusal('unreachable', req.callId)
      useCallStore.getState().setPhase('failed')
    }
  }, [follow, selfId, selfName, teardown])

  const leave = useCallback(async () => {
    attempt.current++
    const cr = roomRef.current
    const store = useCallStore.getState()
    // Hanging up a call you started before anyone picked up is cancelling it: the other person stops ringing.
    const open = store.session && queryClient.getQueryData<Call>(keys.calls.detail(store.session.callId))
    if (store.session && open && open.status === 'ringing' && open.started_by === selfId) {
      // A call that stopped ringing meanwhile answers 409, and a lost network cannot be helped: hanging up goes on regardless.
      await callsApi.cancel(store.session.callId).catch(() => {})
    }
    if (!cr) { store.clear(); return }
    const joinedAt = store.session?.joinedAt ?? Date.now()
    const snap = cr.getSnapshot()
    await cr.disconnect().catch(() => {})
    store.finish({ durationMs: Date.now() - joinedAt, people: Math.max(snap.peak, cr.getSnapshot().peak), sinceMs: joinedAt }, 'ended')
    teardown()
  }, [teardown, selfId])

  const retry = useCallback(async () => {
    const last = lastRef.current
    if (!last) return
    // A ticket from starting the call is spent by now: ask for a new one.
    await join({ ...last, ticket: undefined })
  }, [join])

  // Closing the shell (signing out) must not leave a room open behind it.
  useEffect(() => () => {
    attempt.current++
    void roomRef.current?.disconnect().catch(() => {})
    teardown()
  }, [teardown])

  const onCallRoute = pathname.startsWith('/calls/')
  const live = isLive(phase)
  const snapshot = useSyncExternalStore(callRoom?.subscribe ?? NO_SUBSCRIBE, callRoom?.getSnapshot ?? NO_SNAPSHOT)

  // Cut off for losing access while looking at another screen: bring up the call's own screen, which says why.
  useEffect(() => {
    if (phase === 'removed' && session && !onCallRoute) {
      void navigate(callRoute(session.callId))
    }
  }, [phase, session, onCallRoute, navigate])

  const open = useCallback(() => {
    if (!session) return
    void navigate(callRoute(session.callId))
  }, [navigate, session])

  const pip = useCallPictureInPicture(live && !phone)

  const api = useMemo<CallApi>(() => ({ callRoom, join, leave, retry }), [callRoom, join, leave, retry])

  return (
    <CallContext.Provider value={api}>
      {children}
      {callRoom && <RemoteAudio items={snapshot.audio} />}
      <Suspense fallback={null}>
        {live && session && !onCallRoute && !pip.window && (
          <MiniCall snapshot={snapshot} startedAt={session.startedAt} avatarOf={pictureOf} onOpen={open} onLeave={() => void leave()} />
        )}
        {pip.window && session && createPortal(
          <MiniCall pip snapshot={snapshot} startedAt={session.startedAt} avatarOf={pictureOf} onOpen={() => { window.focus(); pip.exit(); open() }} onLeave={() => { pip.exit(); void leave() }} />,
          pip.window.document.body,
        )}
      </Suspense>
    </CallContext.Provider>
  )
}
