import { apiFetchIfChanged } from './client'
import type { Channel, ChannelUnread, ConversationActivity, Message } from './messaging'
import type { ChatSection } from '../lib/chat-sections'
import { onSessionEnd } from '../lib/session-end'

/**
 * The chat navigator's whole state in one answer (docs/specs/chat-activity-feed): what
 * six separate requests used to fetch when the app opens. Each part is exactly
 * what its own endpoint answers.
 */
export interface ChatState {
  channels: Channel[]
  dms: Channel[]
  unread: ChannelUnread[]
  activity: ConversationActivity[]
  sections: ChatSection[]
  mention_unread_count: number
  unread_thread_count: number
  /** Followed threads with unread replies, newest first. */
  unread_threads: Message[]
}

interface Held {
  /** Whose copy: a copy is never offered to another person or workspace. */
  owner: string
  etag: string | null
  state: ChatState
  /** When the server last confirmed this copy (a 200 or a 304), in ms. */
  confirmedAt: number
}

/**
 * A copy the server confirmed this recently is not asked about again. Screens
 * of the start-up mount at different moments (the workspace is known only after
 * its own list arrives); this is what keeps them to one request together.
 */
export const RECENT_MS = 3000

let held: Held | null = null
let inflight: { owner: string; promise: Promise<Loaded> } | null = null

/** What `loadChatState` found: the state, and whether it differs from the copy held before. */
export interface Loaded {
  state: ChatState
  changed: boolean
}

/**
 * Fetches the chat state of the workspace the session speaks for.
 *
 * It sends the version of the copy it already holds; a 304 answers with that
 * copy and `changed: false`. Callers that ask while a request is in flight share
 * it, which is what makes the start-up one request however many screens ask.
 * Only one copy is kept, so a different person or workspace starts clean.
 *
 * `revalidate` asks the server however recently the copy was confirmed, and
 * does not share a request already in flight: after a reconnect, an answer the
 * server gave before the subscription was live may predate a change.
 */
export function loadChatState(owner: string, workspaceId: string, opts: { revalidate?: boolean } = {}): Promise<Loaded> {
  if (!opts.revalidate && inflight?.owner === owner) return inflight.promise
  if (held && held.owner !== owner) held = null
  if (!opts.revalidate && held && Date.now() - held.confirmedAt < RECENT_MS) {
    return Promise.resolve({ state: held.state, changed: false })
  }

  const promise = (async (): Promise<Loaded> => {
    const mine = held
    const answer = await apiFetchIfChanged<ChatState>(`/workspaces/${workspaceId}/chat-state`, mine?.etag ?? null)
    if (!answer.changed) {
      if (!mine) throw new Error('chat state: 304 without a copy to keep')
      mine.confirmedAt = Date.now()
      return { state: mine.state, changed: false }
    }
    held = { owner, etag: answer.etag, state: answer.data, confirmedAt: Date.now() }
    return { state: answer.data, changed: true }
  })()
  const mine = { owner, promise }
  inflight = mine
  const done = () => {
    if (inflight === mine) inflight = null
  }
  promise.then(done, done)
  return promise
}

/** Forgets the copy held (a test, or a session that ended). */
export function forgetChatState() {
  held = null
  inflight = null
}

// The copy belongs to the person who fetched it.
onSessionEnd(forgetChatState)
