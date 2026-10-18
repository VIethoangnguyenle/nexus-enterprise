import type { CallKind } from '../api/calls'

/**
 * Where to send `navigate` to open a call's screen. The workspace in the
 * address is carried along (as every in-app link does), and `answer` says how
 * a ring or a start picks the call up: with the camera or without. A ring for
 * a conversation of another workspace names that workspace (`workspaceId`), so
 * the session is re-scoped to it; without one the open workspace is kept.
 */
export function callRoute(callId: string, answer?: CallKind, workspaceId?: string) {
  return {
    to: '/calls/$callId' as const,
    params: { callId },
    search: (prev: { ws?: string }) => ({ ws: workspaceId || prev.ws, ...(answer ? { answer } : {}) }),
  }
}
