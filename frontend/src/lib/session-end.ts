/**
 * Per-user client state that must not outlive the session.
 *
 * `auth.store` cannot import the stores that hold chat content (drafts, last
 * messages) without pulling the component graph into the auth module, so each
 * such store registers its reset here and `logout()` runs them all.
 */
const resets = new Set<() => void>()

/** Registers a reset to run when the user signs out; returns the unregister. */
export function onSessionEnd(reset: () => void): () => void {
  resets.add(reset)
  return () => { resets.delete(reset) }
}

/** Runs every registered reset. One failing reset must not leave the others holding the previous user's data. */
export function endSession(): void {
  for (const reset of resets) {
    try { reset() } catch (err) { console.error('[session] reset failed', err) }
  }
}
