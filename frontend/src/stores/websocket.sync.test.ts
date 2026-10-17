import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { queryClient } from '../lib/query-client'
import { keys } from '../hooks/keys'
import { useAuthStore } from './auth.store'
import { useWebSocketStore, getSyncSnapshot, restoreSyncSnapshot } from './websocket.store'
import { endSession } from '../lib/session-end'
import { ClientEnvelope, ServerEnvelope } from '../generated/proto/messaging/ws'
import { BATCH_MS } from '../lib/realtime'
import { chatStateCovers, revalidateChatState } from '../hooks/useChatState'

vi.mock('../hooks/useChatState', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../hooks/useChatState')>()),
  chatStateCovers: vi.fn(() => false),
  revalidateChatState: vi.fn(async () => false),
}))

/** Stands in for the browser WebSocket; keeps binary and JSON frames the client sent apart. */
class FakeSocket {
  static OPEN = 1
  static last: FakeSocket | null = null
  readyState = FakeSocket.OPEN
  binaryType = ''
  onopen: (() => void) | null = null
  onclose: ((e: { code: number }) => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((e: { data: ArrayBuffer | string }) => void) | null = null
  constructor() {
    FakeSocket.last = this
  }
  sent: ClientEnvelope[] = []
  texts: Record<string, unknown>[] = []
  send(data: Uint8Array | string) {
    if (typeof data === 'string') this.texts.push(JSON.parse(data))
    else this.sent.push(ClientEnvelope.fromBinary(data))
  }
  close() {}
}

type ServerEnvelopeInit = NonNullable<Parameters<typeof ServerEnvelope.toBinary>[0]['payload']>
function deliver(payload: ServerEnvelopeInit) {
  const bytes = ServerEnvelope.toBinary({ payload } as Parameters<typeof ServerEnvelope.toBinary>[0])
  const data = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) as ArrayBuffer
  FakeSocket.last!.onmessage!({ data })
}
/** A JSON control frame from the server. */
const control = (msg: Record<string, unknown>) => FakeSocket.last!.onmessage!({ data: JSON.stringify(msg) })

const seed = (key: readonly unknown[]) => queryClient.setQueryData(key, { seeded: true })
const invalidated = (key: readonly unknown[]) => queryClient.getQueryState(key)?.isInvalidated === true
const settle = () => vi.advanceTimersByTime(BATCH_MS + 1)

const docEvent = (seq: number, workspaceId = 'ws-1') => deliver({
  oneofKind: 'domainEvent',
  domainEvent: {
    domain: 'document', kind: 'created', tenantId: 'tenant-1', workspaceId, ids: ['d-1'],
    parentId: '', oldParentId: '', actorUserId: 'u-other', seq: String(seq), channelId: '',
  },
})
const authOk = () => deliver({ oneofKind: 'authResponse', authResponse: { ok: true, userId: 'u', reason: '' } })
const ack = (seq: number, workspaceId = 'ws-1') =>
  deliver({ oneofKind: 'workspaceSubscribed', workspaceSubscribed: { workspaceId, seq: String(seq), denied: false } })
const synced = (seq: number, replayed = 0) =>
  control({ type: 'synced', scope: 'workspace', workspace_id: 'ws-1', seq, replayed })
const plainSubscribes = () =>
  FakeSocket.last!.sent.filter((e) => e.payload.oneofKind === 'subscribe' && e.payload.subscribe.workspaceId === 'ws-1').length
const workspaceCursor = (ws = 'ws-1') => getSyncSnapshot().workspaces[ws]

beforeEach(() => {
  vi.useFakeTimers()
  queryClient.clear()
  restoreSyncSnapshot(null)
  vi.stubGlobal('WebSocket', FakeSocket)
  useAuthStore.setState({ tenantId: 'tenant-1', accessToken: 'token' })
  useWebSocketStore.getState().connect('token')
  FakeSocket.last!.onopen!()
})

afterEach(() => {
  useWebSocketStore.getState().disconnect()
  restoreSyncSnapshot(null)
  vi.unstubAllGlobals()
  vi.useRealTimers()
  useWebSocketStore.setState({ ws: null, recentChanges: {}, onlineUsers: {} })
})

/** Follows ws-1 at seq 5 with the user stream at 3, drops the socket, and reconnects. */
function reconnectWithCursor() {
  useWebSocketStore.getState().followWorkspace('ws-1')
  authOk()
  ack(5)
  control({ type: 'user_seq', seq: 3 })
  FakeSocket.last!.onclose!({ code: 1006 })
  vi.advanceTimersByTime(30_000)
  FakeSocket.last!.onopen!()
  authOk()
}

/** Follows ws-1 at seq 5 and forgets what was sent so far. */
function inSync() {
  useWebSocketStore.getState().followWorkspace('ws-1')
  authOk()
  ack(5)
  FakeSocket.last!.texts.length = 0
}

describe('reconnect with a cursor', () => {
  it('asks to sync and refreshes only what the replay touches', () => {
    reconnectWithCursor()
    expect(FakeSocket.last!.texts).toEqual([
      { type: 'user_sync', since: 3 },
      { type: 'sync', workspace_id: 'ws-1', since: 5 },
    ])
    expect(plainSubscribes()).toBe(0)
    seed(keys.documents.list('ws-1'))
    seed(keys.admin.members('ws-1'))
    seed(keys.approval.pending())
    settle()
    expect(invalidated(keys.admin.members('ws-1'))).toBe(false)

    docEvent(6)
    synced(6, 1)
    control({ type: 'synced', scope: 'user', seq: 3 })
    settle()
    expect(invalidated(keys.documents.list('ws-1'))).toBe(true)
    expect(invalidated(keys.admin.members('ws-1'))).toBe(false)
    expect(invalidated(keys.approval.pending())).toBe(false)
    expect(workspaceCursor()).toBe(6)

    docEvent(7) // the live stream continues from the answer
    expect(workspaceCursor()).toBe(7)
  })

  it('too_far refreshes every query family and continues from the answered position', () => {
    reconnectWithCursor()
    seed(keys.admin.members('ws-1'))
    seed(keys.approval.pending())
    control({ type: 'too_far', scope: 'workspace', workspace_id: 'ws-1', seq: 90 })
    settle()
    expect(invalidated(keys.admin.members('ws-1'))).toBe(true)
    expect(invalidated(keys.approval.pending())).toBe(true)
    expect(workspaceCursor()).toBe(90)

    queryClient.clear()
    seed(keys.documents.list('ws-1'))
    docEvent(91)
    settle()
    expect(invalidated(keys.documents.list('ws-1'))).toBe(true)
    expect(FakeSocket.last!.texts).toHaveLength(2) // no hole, so no further sync
  })

  it('too_far on the user stream refreshes everything and adopts its position', () => {
    reconnectWithCursor()
    seed(keys.approval.pending())
    control({ type: 'too_far', scope: 'user', seq: 40 })
    settle()
    expect(invalidated(keys.approval.pending())).toBe(true)
    expect(getSyncSnapshot().user).toBe(40)
  })

  it('a refused sync still refreshes what chat and notifications missed', () => {
    reconnectWithCursor()
    seed(keys.notifications.all())
    deliver({ oneofKind: 'workspaceSubscribed', workspaceSubscribed: { workspaceId: 'ws-1', seq: '0', denied: true } })
    settle()
    expect(invalidated(keys.notifications.all())).toBe(true)
  })

  it('without a user cursor it falls back to a plain subscribe and a full refresh', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    ack(5)
    FakeSocket.last!.onclose!({ code: 1006 })
    vi.advanceTimersByTime(30_000)
    FakeSocket.last!.onopen!()
    seed(keys.admin.members('ws-1'))
    authOk()
    expect(FakeSocket.last!.texts).toEqual([])
    expect(plainSubscribes()).toBe(1)
    ack(8)
    settle()
    expect(invalidated(keys.admin.members('ws-1'))).toBe(true)
  })
})

describe('no cursor yet', () => {
  it('subscribes plainly, refreshes on the acknowledgement, then starts tracking', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    seed(keys.admin.members('ws-1'))
    authOk()
    expect(FakeSocket.last!.texts).toEqual([])
    expect(plainSubscribes()).toBe(1)
    settle()
    expect(invalidated(keys.admin.members('ws-1'))).toBe(false)
    ack(12)
    settle()
    expect(invalidated(keys.admin.members('ws-1'))).toBe(true)
    expect(getSyncSnapshot().workspaces).toEqual({ 'ws-1': 12 })
  })

  it('user_seq advances the user cursor', () => {
    control({ type: 'user_seq', seq: 4 })
    control({ type: 'user_seq', seq: 5 })
    expect(getSyncSnapshot().user).toBe(5)
  })

  it('ignores control frames it does not understand', () => {
    control({ type: 'nonsense', seq: 1 })
    FakeSocket.last!.onmessage!({ data: 'not json' })
    expect(getSyncSnapshot().user).toBeUndefined()
  })
})

describe('a hole in the live stream', () => {
  it('asks once from the last good position, without refreshing everything', () => {
    inSync()
    seed(keys.admin.members('ws-1'))
    docEvent(9)
    settle()
    expect(FakeSocket.last!.texts).toEqual([{ type: 'sync', workspace_id: 'ws-1', since: 5 }])
    expect(invalidated(keys.admin.members('ws-1'))).toBe(false)

    for (const seq of [6, 7, 8, 9]) docEvent(seq) // the replay, in order
    synced(9, 4)
    settle()
    expect(FakeSocket.last!.texts).toHaveLength(1)
    expect(invalidated(keys.admin.members('ws-1'))).toBe(false)
    expect(workspaceCursor()).toBe(9)
  })

  it('frames already in flight while the sync is pending are not a second failure', () => {
    inSync()
    seed(keys.admin.members('ws-1'))
    docEvent(9)
    docEvent(10)
    settle()
    expect(FakeSocket.last!.texts).toHaveLength(1)
    expect(invalidated(keys.admin.members('ws-1'))).toBe(false)
  })

  it('a second hole after the sync was answered refreshes everything', () => {
    inSync()
    docEvent(9)
    synced(5)
    seed(keys.admin.members('ws-1'))
    docEvent(12)
    settle()
    expect(invalidated(keys.admin.members('ws-1'))).toBe(true)
    expect(FakeSocket.last!.texts).toHaveLength(1) // not asked a second time
  })

  it('a sync that cannot cover the gap refreshes everything', () => {
    inSync()
    seed(keys.admin.members('ws-1'))
    docEvent(9)
    control({ type: 'too_far', scope: 'workspace', workspace_id: 'ws-1', seq: 9 })
    settle()
    expect(invalidated(keys.admin.members('ws-1'))).toBe(true)
  })

  it('an answer that leaves the cursor short of its position refreshes everything', () => {
    inSync()
    seed(keys.admin.members('ws-1'))
    docEvent(9)
    docEvent(6) // 7 and 8 never arrived
    synced(9, 4)
    settle()
    expect(invalidated(keys.admin.members('ws-1'))).toBe(true)
  })

  it('a good frame after the answer makes the next hole a first one again', () => {
    inSync()
    docEvent(9)
    synced(5)
    docEvent(6)
    seed(keys.admin.members('ws-1'))
    docEvent(20)
    settle()
    expect(FakeSocket.last!.texts).toHaveLength(2)
    expect(invalidated(keys.admin.members('ws-1'))).toBe(false)
  })
})

describe('a hole in the user stream', () => {
  const userSeq = (seq: number) => control({ type: 'user_seq', seq })

  it('asks once from the last good position and keeps the cursor there', () => {
    inSync()
    userSeq(3)
    userSeq(4)
    userSeq(7) // 5 and 6 were lost
    expect(FakeSocket.last!.texts).toEqual([{ type: 'user_sync', since: 4 }])
    expect(getSyncSnapshot().user).toBe(4)
    userSeq(8) // in flight before the request was handled: not a second failure
    expect(FakeSocket.last!.texts).toHaveLength(1)
    expect(getSyncSnapshot().user).toBe(4)
  })

  it('consecutive frames advance the cursor without asking', () => {
    inSync()
    userSeq(3)
    userSeq(4)
    userSeq(5)
    expect(getSyncSnapshot().user).toBe(5)
    expect(FakeSocket.last!.texts).toEqual([])
  })

  it('the answer settles the cursor, and the next good frame continues from it', () => {
    inSync()
    userSeq(3)
    userSeq(6)
    control({ type: 'synced', scope: 'user', seq: 6 })
    expect(getSyncSnapshot().user).toBe(6)
    userSeq(7)
    expect(getSyncSnapshot().user).toBe(7)
    userSeq(9) // a first hole again
    expect(FakeSocket.last!.texts).toHaveLength(2)
  })

  it('a second hole before a good frame refreshes everything', () => {
    inSync()
    userSeq(3)
    userSeq(6)
    control({ type: 'synced', scope: 'user', seq: 6 })
    seed(keys.approval.pending())
    userSeq(9)
    settle()
    expect(invalidated(keys.approval.pending())).toBe(true)
    expect(FakeSocket.last!.texts).toHaveLength(1) // not asked a second time
    expect(getSyncSnapshot().user).toBe(9)
  })
})

describe('a sync nobody answers', () => {
  it('falls back to a plain subscribe and a full refresh', () => {
    reconnectWithCursor()
    seed(keys.admin.members('ws-1'))
    expect(plainSubscribes()).toBe(0)
    vi.advanceTimersByTime(60_000)
    expect(plainSubscribes()).toBe(1)
    settle()
    expect(invalidated(keys.admin.members('ws-1'))).toBe(false) // waits for the acknowledgement
    ack(8)
    settle()
    expect(invalidated(keys.admin.members('ws-1'))).toBe(true)
    expect(workspaceCursor()).toBe(8)
  })

  it('an answer in time cancels the fallback', () => {
    reconnectWithCursor()
    synced(5)
    vi.advanceTimersByTime(60_000)
    expect(plainSubscribes()).toBe(0)
  })

  it('a refusal in time is retried by the usual schedule, not by the fallback', () => {
    reconnectWithCursor()
    deliver({ oneofKind: 'workspaceSubscribed', workspaceSubscribed: { workspaceId: 'ws-1', seq: '0', denied: true } })
    vi.advanceTimersByTime(5_000)
    expect(plainSubscribes()).toBe(1)
  })

  it('leaving the workspace cancels the fallback', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    ack(5)
    control({ type: 'user_seq', seq: 3 })
    FakeSocket.last!.onclose!({ code: 1006 })
    vi.advanceTimersByTime(30_000)
    FakeSocket.last!.onopen!()
    authOk()
    useWebSocketStore.getState().disconnect()
    const before = FakeSocket.last!.sent.length
    vi.advanceTimersByTime(60_000)
    expect(FakeSocket.last!.sent.length).toBe(before)
  })
})

describe('lifetime of the cursors', () => {
  it('signing out clears them', () => {
    reconnectWithCursor()
    expect(getSyncSnapshot().user).toBe(3)
    endSession()
    expect(getSyncSnapshot()).toEqual({ owner: undefined, user: undefined, workspaces: {} })
  })

  it('another tenant on the same tab never inherits them', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    ack(5)
    control({ type: 'user_seq', seq: 3 })
    useWebSocketStore.getState().disconnect()
    useAuthStore.setState({ tenantId: 'tenant-2' })
    useWebSocketStore.getState().connect('token')
    FakeSocket.last!.onopen!()
    authOk()
    expect(FakeSocket.last!.texts).toEqual([])
    expect(getSyncSnapshot().user).toBeUndefined()
    expect(plainSubscribes()).toBe(1)
  })

  it("leaving a workspace drops only its own cursor", () => {
    const release = useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    ack(5)
    restoreSyncSnapshot({ ...getSyncSnapshot(), workspaces: { 'ws-1': 5, 'ws-2': 8 } })
    release()
    expect(getSyncSnapshot().workspaces).toEqual({ 'ws-2': 8 })
  })

  it('switching workspace drops the old cursor though the new one is already followed', () => {
    const releaseOld = useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    ack(5)
    useWebSocketStore.getState().followWorkspace('ws-2')
    ack(2, 'ws-2')
    releaseOld()
    expect(getSyncSnapshot().workspaces).toEqual({ 'ws-2': 2 })
  })

  it('the snapshot restores, and malformed data is dropped', () => {
    restoreSyncSnapshot({ owner: 'u|t', user: 7, workspaces: { a: 1, b: -1, c: 'x', d: 2.5 } })
    expect(getSyncSnapshot()).toEqual({ owner: 'u|t', user: 7, workspaces: { a: 1 } })
    restoreSyncSnapshot('junk')
    expect(getSyncSnapshot().workspaces).toEqual({})
  })
})

describe('the chat navigator after a reconnect', () => {
  const chatParts = [keys.messaging.channels('ws-1'), keys.messaging.dms(), keys.messaging.unreadCounts(), keys.messaging.conversationActivity()]

  beforeEach(() => vi.mocked(chatStateCovers).mockReturnValue(true))
  afterEach(() => vi.mocked(chatStateCovers).mockReturnValue(false))

  it('asks the chat state whether anything changed instead of refetching each part', async () => {
    vi.mocked(revalidateChatState).mockResolvedValueOnce(true)
    useWebSocketStore.getState().followWorkspace('ws-1')
    for (const key of chatParts) seed(key)
    seed(keys.admin.members('ws-1'))
    authOk()
    ack(12)
    await vi.advanceTimersByTimeAsync(BATCH_MS + 1)

    expect(revalidateChatState).toHaveBeenCalledWith('ws-1')
    for (const key of chatParts) expect(invalidated(key)).toBe(false)
    expect(invalidated(keys.admin.members('ws-1'))).toBe(true)
  })

  it('refetches each part when the chat state cannot answer', async () => {
    vi.mocked(revalidateChatState).mockResolvedValueOnce(false)
    useWebSocketStore.getState().followWorkspace('ws-1')
    for (const key of chatParts) seed(key)
    authOk()
    ack(12)
    await vi.advanceTimersByTimeAsync(BATCH_MS + 1)

    for (const key of chatParts) expect(invalidated(key)).toBe(true)
  })
})
