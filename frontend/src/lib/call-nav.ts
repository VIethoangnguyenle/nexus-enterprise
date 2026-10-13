import type { CallKind } from '../api/calls'

/**
 * Where to send `navigate` to open a call's screen. The workspace in the
 * address is carried along (as every in-app link does), and `answer` says how
 * a ring or a start picks the call up: with the camera or without.
 */
export function callRoute(callId: string, answer?: CallKind) {
  return {
    to: '/calls/$callId' as const,
    params: { callId },
    search: (prev: { ws?: string }) => ({ ws: prev.ws, ...(answer ? { answer } : {}) }),
  }
}
