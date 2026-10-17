/** How long a workspace the person left themselves is remembered. */
const SELF_LEAVE_MS = 2 * 60_000
const leftByChoice = new Map<string, number>()

/**
 * Notes that the person is leaving this workspace on purpose, before the request is made.
 * The server then moves their session and closes their sockets just as it does for a
 * removal; the leave flow already tells them and takes them to the picker, so that
 * move must not be announced as "you are no longer a member". Kept apart from the
 * handler that reads it so the leave hook can import it without pulling in the app.
 */
export function noteSelfLeave(workspaceId: string): void {
  leftByChoice.set(workspaceId, Date.now() + SELF_LEAVE_MS)
}

/** Whether the person left this workspace on purpose a moment ago. */
export function leftOnPurpose(workspaceId: string): boolean {
  const until = leftByChoice.get(workspaceId)
  if (until === undefined) return false
  if (Date.now() > until) {
    leftByChoice.delete(workspaceId)
    return false
  }
  return true
}

/** Forgets every remembered leave (a new session starts clean). */
export function clearSelfLeaves(): void {
  leftByChoice.clear()
}
