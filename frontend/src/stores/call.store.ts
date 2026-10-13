import { create } from 'zustand'
import type { CallKind, CallRefusal } from '../api/calls'

/**
 * Client state of the call you are in. What the call is (its people, its
 * status) is server data and lives in TanStack Query (`useCalls`); what the
 * room is doing lives in `lib/call-room`. This holds the rest: which call, how
 * far the join got, and how the stage is laid out.
 */

export interface CallSession {
  callId: string
  channelId: string
  kind: CallKind
  /** The conversation's name: shown in the bars, never an id. */
  title: string
  /** A direct message: no pre-join, no "chưa vào" list. */
  isDm: boolean
  /** When you came in (ms). */
  joinedAt: number
  /** When the call began (ms), for the clock: the server's start, else your own entry. */
  startedAt: number
}

/** `ended`: a summary is on screen. `removed`: the server cut you off because you lost access. */
export type CallPhase = 'connecting' | 'connected' | 'reconnecting' | 'failed' | 'ended' | 'removed'

export interface CallSummary { durationMs: number; people: number; sinceMs: number }

export type CallPanel = 'chat' | 'people'
export type CallLayoutMode = 'auto' | 'speaker' | 'grid'

interface CallState {
  session: CallSession | null
  phase: CallPhase | null
  /** Why the last join was refused, for the screen that asked. */
  refusal: CallRefusal | null
  summary: CallSummary | null
  panel: CallPanel | null
  layout: CallLayoutMode
  /** The tile you pinned large, by user id. */
  pinnedId: string | null
  /** When you last looked at the chat panel: messages after it are "new". */
  chatSeenAt: number
  /** The device settings dialog is open. */
  devicesOpen: boolean

  begin: (session: CallSession) => void
  setPhase: (phase: CallPhase) => void
  setRefusal: (r: CallRefusal | null) => void
  finish: (summary: CallSummary, phase: 'ended' | 'removed') => void
  clear: () => void
  setPanel: (p: CallPanel | null) => void
  setLayout: (l: CallLayoutMode) => void
  togglePin: (userId: string) => void
  markChatSeen: () => void
  setDevicesOpen: (open: boolean) => void
}

const IDLE = {
  session: null, phase: null, refusal: null, summary: null, panel: null, layout: 'auto' as CallLayoutMode,
  pinnedId: null, chatSeenAt: 0, devicesOpen: false,
}

export const useCallStore = create<CallState>()((set) => ({
  ...IDLE,
  begin: (session) => set({ ...IDLE, session, phase: 'connecting', chatSeenAt: Date.now() }),
  setPhase: (phase) => set({ phase }),
  setRefusal: (refusal) => set({ refusal }),
  finish: (summary, phase) => set({ summary, phase, panel: null, devicesOpen: false }),
  clear: () => set({ ...IDLE }),
  setPanel: (panel) => set({ panel }),
  setLayout: (layout) => set({ layout, pinnedId: null }),
  togglePin: (userId) => set((s) => ({ pinnedId: s.pinnedId === userId ? null : userId })),
  markChatSeen: () => set({ chatSeenAt: Date.now() }),
  setDevicesOpen: (devicesOpen) => set({ devicesOpen }),
}))

/** True while a room is open or opening: the call can be minimised, and the floating tile shows. */
export const isLive = (phase: CallPhase | null): boolean =>
  phase === 'connecting' || phase === 'connected' || phase === 'reconnecting'
