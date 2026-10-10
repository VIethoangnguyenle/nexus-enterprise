import { keys } from '../hooks/keys'

/**
 * Realtime: what a change somebody else made should refresh on this screen.
 *
 * Everything here is pure or injected so each rule is testable without a
 * socket: `planFor` maps one event to the query keys it makes stale,
 * `observeSeq` decides whether a workspace sequence number is in order, and
 * `createBatcher` coalesces the invalidations of a burst into one refetch.
 * Events carry ids and the kind of change, never content: the screen refetches
 * under its own authorization.
 */

export type QueryKey = readonly unknown[]

/** The fields of a DomainEvent frame the rules read. */
export interface DomainChange {
  domain: string
  kind: string
  workspaceId: string
  ids: readonly string[]
  parentId?: string
  oldParentId?: string
  channelId?: string
  actorUserId?: string
}

export interface Plan {
  /** Query prefixes to mark stale. */
  invalidate: QueryKey[]
  /** Entities that changed, for the "someone else just changed this" wash. */
  touched: string[]
}

/** Domains whose frames this module understands; anything else is ignored. */
export const DOMAINS = ['drive', 'channel', 'workspace', 'document', 'asset', 'permission'] as const

/**
 * Every query family that belongs to a workspace's data. After a reconnect or a
 * hole in the sequence the screen cannot know what it missed, so all of it is
 * refreshed.
 */
export function workspaceResyncKeys(wsId: string): QueryKey[] {
  return [
    keys.drive.everything(),
    keys.permissions.all(),
    keys.documents.all(wsId),
    keys.admin.everything(),
    keys.contacts.every(),
    keys.workspaces.all(),
    keys.assets.typesAll(),
    keys.assets.listsAll(),
    keys.assets.summaries(),
    keys.assets.activitiesAll(),
    keys.assets.requestsAll(),
    keys.assets.requestDetailsAll(),
    keys.approval.all(),
    keys.messaging.channelsAll(),
    keys.messaging.dms(),
    keys.messaging.messagesAll(),
    keys.messaging.unreadCounts(),
    keys.messaging.pinsAll(),
    keys.messaging.tasksAll(),
    keys.messaging.reactionsAll(),
    keys.messaging.threadsAll(),
    // The notification list and count, shown by the sidebar, the Thêm sheet and the panel.
    keys.notifications.all(),
  ]
}

/** The keys one event makes stale, and the entities it touched. */
export function planFor(e: DomainChange): Plan {
  const ws = e.workspaceId
  const inv: QueryKey[] = []
  const ids = [...e.ids]

  switch (e.domain) {
    case 'drive': {
      if (e.kind === 'share_created' || e.kind === 'share_revoked') {
        // The share list and the item's own record. Who can now see or do what
        // is told to those people by the permission event addressed to them;
        // every subscriber refetching permissions on each share is not wanted.
        for (const id of ids) inv.push(keys.drive.shares(id), keys.drive.item(id))
        break
      }
      // The folder the item sits in now, and the one it left.
      inv.push(keys.drive.folder(ws, e.parentId))
      if (e.kind === 'moved' && e.oldParentId !== undefined) inv.push(keys.drive.folder(ws, e.oldParentId))
      // A moved or trashed folder takes its descendants with it, and the event
      // names only the folder itself: every listing of the workspace is stale.
      if (e.kind === 'moved' || e.kind === 'deleted') inv.push(keys.drive.folders(ws))
      // What a person may do with an item follows from where it sits.
      if (e.kind === 'moved') inv.push(keys.permissions.all())
      for (const id of ids) {
        if (e.kind !== 'created') inv.push(keys.drive.item(id))
        // A removed or moved folder's own listing is gone or stale.
        if (e.kind === 'deleted') inv.push(keys.drive.folder(ws, id))
      }
      if (e.kind === 'created' || e.kind === 'deleted') inv.push(keys.drive.quota(ws))
      inv.push(keys.drive.sharedWithMe())
      break
    }

    case 'document': {
      inv.push(keys.documents.all(ws))
      if (e.kind === 'deleted') for (const id of ids) inv.push(keys.documents.detail(id))
      // Text documents sit in a drive folder as well.
      if (e.parentId !== undefined) inv.push(keys.drive.folder(ws, e.parentId))
      break
    }

    case 'channel': {
      inv.push(keys.messaging.channels(ws), keys.messaging.dms())
      for (const id of ids) inv.push(keys.messaging.channel(id), keys.messaging.members(id))
      if (e.channelId) inv.push(keys.messaging.channel(e.channelId), keys.messaging.members(e.channelId))
      break
    }

    case 'workspace': {
      inv.push(keys.admin.everything(), keys.contacts.every(), keys.workspaces.all(), keys.workspaces.details(ws))
      if (e.kind === 'member_removed' || e.kind === 'role_changed' || e.kind === 'department_changed') {
        inv.push(keys.permissions.all())
      }
      break
    }

    case 'asset': {
      inv.push(
        keys.assets.lists(ws),
        keys.assets.summary(ws),
        keys.assets.activities(ws),
        keys.assets.types(ws),
        keys.assets.requests(ws),
      )
      for (const id of ids) {
        if (e.kind === 'request_changed') {
          inv.push(keys.assets.request(id))
        } else {
          inv.push(keys.assets.asset(id), keys.assets.history(id), keys.assets.transitions(id))
        }
      }
      break
    }

    case 'permission': {
      // What this user may do changed, and with it what they may see. Their
      // permission answers are stale and so is every listing filtered by them.
      return { invalidate: workspaceResyncKeys(ws), touched: [] }
    }

    default:
      return { invalidate: [], touched: [] }
  }

  return { invalidate: inv, touched: ids }
}

// ---------------------------------------------------------------------------
// Sequence
// ---------------------------------------------------------------------------

export type SeqVerdict = 'apply' | 'duplicate' | 'resync'

export interface SeqState {
  /** Last sequence number seen, per workspace. */
  last: Record<string, number>
}

export const emptySeq = (): SeqState => ({ last: {} })

/** A subscription acknowledgement fixes where the stream stands. */
export function resetSeq(state: SeqState, wsId: string, seq: number): SeqState {
  return { last: { ...state.last, [wsId]: seq } }
}

/**
 * Judges one workspace-stream frame against the last seen number.
 *
 *  - the next number: apply it;
 *  - the same number again: a duplicate, ignore it;
 *  - anything else: a hole (events were missed) or a restart of the counter
 *    (the server lost it). Either way the screen cannot trust what it has, so
 *    it resynchronises and takes this number as the new position.
 *
 * Frames with no sequence (seq 0: user- and channel-addressed) are outside the
 * stream and always apply.
 */
export function observeSeq(state: SeqState, wsId: string, seq: number): { state: SeqState; verdict: SeqVerdict } {
  if (!seq) return { state, verdict: 'apply' }
  const last = state.last[wsId]
  if (last === undefined) return { state: resetSeq(state, wsId, seq), verdict: 'apply' }
  if (seq === last) return { state, verdict: 'duplicate' }
  const next = resetSeq(state, wsId, seq)
  return { state: next, verdict: seq === last + 1 ? 'apply' : 'resync' }
}

// ---------------------------------------------------------------------------
// Batching
// ---------------------------------------------------------------------------

/** One frame, as far as invalidations go. */
export const BATCH_MS = 100

export interface Batcher {
  add: (keysToInvalidate: readonly QueryKey[]) => void
  /** Run what is pending now (tests, and teardown). */
  flush: () => void
  cancel: () => void
}

/**
 * Coalesces invalidations: keys added within one window are de-duplicated and
 * handed to `run` together, so ten events in a burst cost one refetch of each
 * affected query rather than ten.
 */
export function createBatcher(run: (key: QueryKey) => void, delayMs = BATCH_MS): Batcher {
  const pending = new Map<string, QueryKey>()
  let timer: ReturnType<typeof setTimeout> | null = null

  const flush = () => {
    if (timer) clearTimeout(timer)
    timer = null
    const batch = [...pending.values()]
    pending.clear()
    for (const key of batch) run(key)
  }

  return {
    add(list) {
      if (list.length === 0) return
      for (const key of list) pending.set(JSON.stringify(key), key)
      if (!timer) timer = setTimeout(flush, delayMs)
    },
    flush,
    cancel() {
      if (timer) clearTimeout(timer)
      timer = null
      pending.clear()
    },
  }
}
