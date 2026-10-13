import { create } from 'zustand'
import type { CallKind } from '../api/calls'
import { RING_MS } from '../lib/call-model'
import { onSessionEnd } from '../lib/session-end'

export interface IncomingCallInfo {
  callId: string
  channelId: string
  kind: CallKind
  callerName: string
  callerAvatarUrl: string
  mode: 'ringing' | 'invited' | 'answered_elsewhere'
}

interface IncomingCallState {
  incoming: IncomingCallInfo | null
  ring: (info: Omit<IncomingCallInfo, 'mode'>) => void
  invite: (info: Omit<IncomingCallInfo, 'mode'>) => void
  answeredElsewhere: (callId: string) => void
  clear: (callId: string) => void
}

/** Pending auto-clear timer; cancelled and replaced by each ring/invite/answeredElsewhere. */
let autoClearTimer: ReturnType<typeof setTimeout> | null = null

/** How long "answered elsewhere" stays on screen. */
const ANSWERED_ELSEWHERE_MS = 3_000

function cancelTimer() {
  if (autoClearTimer) clearTimeout(autoClearTimer)
  autoClearTimer = null
}

export const useIncomingCallStore = create<IncomingCallState>()((set, get) => ({
  incoming: null,

  ring: (info) => {
    cancelTimer()
    set({ incoming: { ...info, mode: 'ringing' } })
    autoClearTimer = setTimeout(() => {
      if (get().incoming?.callId === info.callId) set({ incoming: null })
    }, RING_MS)
  },

  invite: (info) => {
    cancelTimer()
    set({ incoming: { ...info, mode: 'invited' } })
    // Invitations stay until dismissed — no auto-clear.
  },

  answeredElsewhere: (callId) => {
    const current = get().incoming
    if (!current || current.callId !== callId) return
    cancelTimer()
    set({ incoming: { ...current, mode: 'answered_elsewhere' } })
    autoClearTimer = setTimeout(() => {
      if (get().incoming?.callId === callId) set({ incoming: null })
    }, ANSWERED_ELSEWHERE_MS)
  },

  clear: (callId) => {
    if (get().incoming?.callId !== callId) return
    cancelTimer()
    set({ incoming: null })
  },
}))

// A ring that outlives the person it was for would greet whoever signs in next, and its timer would clear theirs.
onSessionEnd(() => {
  cancelTimer()
  useIncomingCallStore.setState({ incoming: null })
})
