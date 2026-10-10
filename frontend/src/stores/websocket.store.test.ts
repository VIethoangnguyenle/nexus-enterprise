import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { queryClient } from '../lib/query-client'
import { keys } from '../hooks/keys'
import { useAuthStore } from './auth.store'
import { useWebSocketStore } from './websocket.store'
import { ClientEnvelope, ServerEnvelope } from '../generated/proto/messaging/ws'
import { BATCH_MS } from '../lib/realtime'

/** Stands in for the browser WebSocket so a test can push server frames into the store. */
class FakeSocket {
  static OPEN = 1
  static last: FakeSocket | null = null
  readyState = FakeSocket.OPEN
  binaryType = ''
  onopen: (() => void) | null = null
  onclose: ((e: { code: number }) => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((e: { data: ArrayBuffer }) => void) | null = null
  constructor() {
    FakeSocket.last = this
  }
  sent: ClientEnvelope[] = []
  send(data: Uint8Array) {
    this.sent.push(ClientEnvelope.fromBinary(data))
  }
  close() {}
}

function deliver(payload: ServerEnvelopeInit) {
  const bytes = ServerEnvelope.toBinary({ payload } as Parameters<typeof ServerEnvelope.toBinary>[0])
  const data = bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength) as ArrayBuffer
  // The store checks `instanceof ArrayBuffer` in the test realm.
  FakeSocket.last!.onmessage!({ data })
}
type ServerEnvelopeInit = NonNullable<Parameters<typeof ServerEnvelope.toBinary>[0]['payload']>

/** Seeds a query so there is something for an invalidation to mark. */
function seed(key: readonly unknown[]) {
  queryClient.setQueryData(key, { seeded: true })
}
const invalidated = (key: readonly unknown[]) => queryClient.getQueryState(key)?.isInvalidated === true

/** Lets the batched invalidations of the last events run. */
const settle = () => vi.advanceTimersByTime(BATCH_MS + 1)

beforeEach(() => {
  vi.useFakeTimers()
  queryClient.clear()
  vi.stubGlobal('WebSocket', FakeSocket)
  useAuthStore.setState({ tenantId: 'tenant-1', accessToken: 'token' })
  useWebSocketStore.getState().connect('token')
  FakeSocket.last!.onopen!()
})

afterEach(() => {
  useWebSocketStore.getState().disconnect()
  vi.unstubAllGlobals()
  vi.useRealTimers()
  useWebSocketStore.setState({ ws: null, recentChanges: {}, onlineUsers: {} })
})

const domainEvent = (over: Partial<{
  domain: string; kind: string; workspaceId: string; ids: string[]; parentId: string; oldParentId: string
  actorUserId: string; seq: string; channelId: string; tenantId: string
}>) => ({
  oneofKind: 'domainEvent' as const,
  domainEvent: {
    domain: 'drive', kind: 'updated', tenantId: 'tenant-1', workspaceId: 'ws-1', ids: ['i-1'],
    parentId: '', oldParentId: '', actorUserId: 'u-other', seq: '0', channelId: '', ...over,
  },
})
const authOk = () => deliver({ oneofKind: 'authResponse', authResponse: { ok: true, userId: 'u', reason: '' } })
const ackWorkspace = (seq = '0', workspaceId = 'ws-1') =>
  deliver({ oneofKind: 'workspaceSubscribed', workspaceSubscribed: { workspaceId, seq, denied: false } })

describe('websocket events invalidate the keys the screens actually cache under', () => {
  it('reconnect resync refreshes permissions and the workspace root listing', () => {
    seed(keys.permissions.object('tenant-1', 'node-1'))
    seed(keys.drive.folder('ws-1'))
    deliver({ oneofKind: 'authResponse', authResponse: { ok: true, userId: 'u', reason: '' } })
    expect(invalidated(keys.permissions.object('tenant-1', 'node-1'))).toBe(true)
    expect(invalidated(keys.drive.folder('ws-1'))).toBe(true)
  })

  it('a drive event with no parent refreshes the root listing', () => {
    seed(keys.drive.folder('ws-1'))
    seed(keys.drive.folder('ws-1', 'folder-9'))
    seed(keys.drive.folder('ws-2'))
    deliver(domainEvent({ kind: 'created', parentId: '' }))
    settle()
    expect(invalidated(keys.drive.folder('ws-1'))).toBe(true)
    expect(invalidated(keys.drive.folder('ws-1', 'folder-9'))).toBe(false)
    expect(invalidated(keys.drive.folder('ws-2'))).toBe(false)
  })

  it('a drive event inside a folder refreshes that folder only', () => {
    seed(keys.drive.folder('ws-1'))
    seed(keys.drive.folder('ws-1', 'folder-9'))
    deliver(domainEvent({ kind: 'created', parentId: 'folder-9' }))
    settle()
    expect(invalidated(keys.drive.folder('ws-1', 'folder-9'))).toBe(true)
    expect(invalidated(keys.drive.folder('ws-1'))).toBe(false)
  })

  it('a drive event refreshes workspace quota on create and delete', () => {
    seed(keys.drive.quota('ws-1'))
    deliver(domainEvent({ kind: 'deleted' }))
    settle()
    expect(invalidated(keys.drive.quota('ws-1'))).toBe(true)
  })

  it('an asset event reaches the summary and lists cached under its workspace', () => {
    seed(keys.assets.summary('ws-1'))
    seed(keys.assets.list('ws-1', { status: 'active' }))
    seed(keys.assets.history('asset-1'))
    seed(keys.assets.summary('ws-2'))
    deliver(domainEvent({ domain: 'asset', kind: 'updated', ids: ['asset-1'] }))
    settle()
    expect(invalidated(keys.assets.summary('ws-1'))).toBe(true)
    expect(invalidated(keys.assets.list('ws-1', { status: 'active' }))).toBe(true)
    expect(invalidated(keys.assets.history('asset-1'))).toBe(true)
    expect(invalidated(keys.assets.summary('ws-2'))).toBe(false)
  })

  it('an asset event also refreshes the feed, the types and the requests', () => {
    seed(keys.assets.activity('ws-1', 10))
    seed(keys.assets.types('ws-1'))
    seed(keys.assets.requestList('ws-1', { status: 'pending' }))
    deliver(domainEvent({ domain: 'asset', kind: 'request_changed', ids: ['req-1'] }))
    settle()
    expect(invalidated(keys.assets.activity('ws-1', 10))).toBe(true)
    expect(invalidated(keys.assets.types('ws-1'))).toBe(true)
    expect(invalidated(keys.assets.requestList('ws-1', { status: 'pending' }))).toBe(true)
  })

  it('an unread-count event refreshes both the chat and the notification counters', () => {
    seed(keys.messaging.unreadCounts())
    seed(keys.notifications.unreadCount())
    deliver({ oneofKind: 'unreadCount', unreadCount: { channelId: 'c-1', count: 3 } } as ServerEnvelopeInit)
    expect(invalidated(keys.messaging.unreadCounts())).toBe(true)
    expect(invalidated(keys.notifications.unreadCount())).toBe(true)
  })

  it('a permission event drops the permission cache and the listings the user sees through it', () => {
    seed(keys.permissions.object('tenant-1', 'node-1'))
    seed(keys.permissions.object('tenant-1', 'node-2'))
    seed(keys.drive.folder('ws-1'))
    deliver(domainEvent({ domain: 'permission', kind: 'changed', ids: [], actorUserId: '' }))
    settle()
    expect(invalidated(keys.permissions.object('tenant-1', 'node-1'))).toBe(true)
    expect(invalidated(keys.permissions.object('tenant-1', 'node-2'))).toBe(true)
    expect(invalidated(keys.drive.folder('ws-1'))).toBe(true)
  })

  it('a pin event refreshes the pins of that channel', () => {
    seed(keys.messaging.pins('c-1'))
    deliver({ oneofKind: 'pinEvent', pinEvent: { channelId: 'c-1', messageId: 'm-1', action: 'pin', userId: 'u' } } as ServerEnvelopeInit)
    expect(invalidated(keys.messaging.pins('c-1'))).toBe(true)
  })

  it('a moved drive item refreshes the folder it left, which the event names', () => {
    seed(keys.drive.folder('ws-1', 'old-folder'))
    seed(keys.drive.folder('ws-2', 'other-ws-folder'))
    deliver(domainEvent({ kind: 'moved', parentId: 'new-folder', oldParentId: 'old-folder' }))
    settle()
    expect(invalidated(keys.drive.folder('ws-1', 'old-folder'))).toBe(true)
    expect(invalidated(keys.drive.folder('ws-2', 'other-ws-folder'))).toBe(false)
  })

  it('a task update reaches the task list the screen caches with a status filter slot', () => {
    queryClient.setQueryData(keys.messaging.tasks('ch-1'), {
      tasks: [{ id: 't-1', channel_id: 'ch-1', title: 'Old', status: 'open', assignee_id: '' }],
    })
    deliver({
      oneofKind: 'taskUpdate',
      taskUpdate: { taskId: 't-1', channelId: 'ch-1', status: 'done', assigneeId: '', title: '' },
    } as ServerEnvelopeInit)
    const data = queryClient.getQueryData<{ tasks: { status: string }[] }>(keys.messaging.tasks('ch-1'))
    expect(data?.tasks[0]?.status).toBe('done')
  })

  it('remembers who acted on an approval request, and forgets it when the session ends', () => {
    deliver({
      oneofKind: 'approvalEvent',
      approvalEvent: { requestId: 'r-1', status: 'approved', action: 'approved', actorNodeId: 'n-duc', templateName: 'Tạm ứng' },
    } as ServerEnvelopeInit)
    expect(useWebSocketStore.getState().approvalActivity['r-1']).toMatchObject({ actorNodeId: 'n-duc', action: 'approved' })
    useWebSocketStore.getState().disconnect()
    expect(useWebSocketStore.getState().approvalActivity).toEqual({})
  })
})

describe('a burst of events costs one refetch', () => {
  it('holds invalidations for a frame and runs each key once', () => {
    const spy = vi.spyOn(queryClient, 'invalidateQueries')
    for (let i = 0; i < 10; i++) deliver(domainEvent({ kind: 'updated', parentId: 'f', ids: [`i-${i}`] }))
    expect(spy).not.toHaveBeenCalled()
    settle()
    const folderCalls = spy.mock.calls.filter(
      ([f]) => JSON.stringify(f?.queryKey) === JSON.stringify(keys.drive.folder('ws-1', 'f')),
    )
    expect(folderCalls).toHaveLength(1)
  })
})

describe('workspace subscription', () => {
  it('follows a workspace once authenticated, and again after a reconnect', () => {
    const release = useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    const subscribed = () =>
      FakeSocket.last!.sent.filter((e) => e.payload.oneofKind === 'subscribe' && e.payload.subscribe.workspaceId === 'ws-1')
    expect(subscribed()).toHaveLength(1)

    FakeSocket.last!.onclose!({ code: 1006 })
    vi.advanceTimersByTime(40_000)
    FakeSocket.last!.onopen!()
    authOk()
    expect(subscribed()).toHaveLength(1) // the new socket, which has its own record
    release()
  })

  it('releasing sends an unsubscribe', () => {
    authOk()
    const release = useWebSocketStore.getState().followWorkspace('ws-1')
    release()
    const sent = FakeSocket.last!.sent.map((e) => e.payload.oneofKind)
    expect(sent).toContain('unsubscribe')
  })
})

describe('resync', () => {
  it('a reconnect refreshes the workspace only after the subscription is acknowledged', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    seed(keys.documents.list('ws-1'))
    seed(keys.admin.members('ws-1'))
    authOk()
    settle()
    expect(invalidated(keys.documents.list('ws-1'))).toBe(false)

    ackWorkspace('12')
    settle()
    expect(invalidated(keys.documents.list('ws-1'))).toBe(true)
    expect(invalidated(keys.admin.members('ws-1'))).toBe(true)
  })

  it('a 30 second outage ends with every workspace query refreshed and the stream resumed', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    ackWorkspace('5')
    deliver(domainEvent({ domain: 'document', kind: 'created', seq: '6' }))
    settle()
    queryClient.clear()
    seed(keys.documents.list('ws-1'))
    seed(keys.drive.folder('ws-1', 'f'))
    seed(keys.approval.pending())

    const dropped = FakeSocket.last!
    dropped.onclose!({ code: 1006 })
    vi.advanceTimersByTime(30_000)
    expect(FakeSocket.last).not.toBe(dropped) // reconnected on its own
    FakeSocket.last!.onopen!()
    authOk()
    ackWorkspace('9') // three events happened while it was down
    settle()

    expect(invalidated(keys.documents.list('ws-1'))).toBe(true)
    expect(invalidated(keys.drive.folder('ws-1', 'f'))).toBe(true)
    expect(invalidated(keys.approval.pending())).toBe(true)

    // and the next event is the next in line, not a hole
    queryClient.clear()
    seed(keys.documents.list('ws-1'))
    deliver(domainEvent({ domain: 'document', kind: 'created', seq: '10' }))
    settle()
    expect(invalidated(keys.documents.list('ws-1'))).toBe(true)
  })

  it('a hole in the sequence refreshes everything for the workspace', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    ackWorkspace('5')
    queryClient.clear()
    seed(keys.admin.members('ws-1'))
    seed(keys.documents.list('ws-1'))

    deliver(domainEvent({ domain: 'document', kind: 'created', seq: '9' }))
    settle()

    expect(invalidated(keys.admin.members('ws-1'))).toBe(true)
    expect(invalidated(keys.documents.list('ws-1'))).toBe(true)
  })

  it('the next frame in order is only that event, not a resync', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    ackWorkspace('5')
    queryClient.clear()
    seed(keys.admin.members('ws-1'))
    seed(keys.documents.list('ws-1'))

    deliver(domainEvent({ domain: 'document', kind: 'created', seq: '6' }))
    settle()

    expect(invalidated(keys.documents.list('ws-1'))).toBe(true)
    expect(invalidated(keys.admin.members('ws-1'))).toBe(false)
  })

  it('a repeated frame is ignored', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    ackWorkspace('5')
    deliver(domainEvent({ domain: 'document', kind: 'created', seq: '6' }))
    settle()
    queryClient.clear()
    seed(keys.documents.list('ws-1'))

    deliver(domainEvent({ domain: 'document', kind: 'created', seq: '6' }))
    settle()
    expect(invalidated(keys.documents.list('ws-1'))).toBe(false)
  })

  it('frames outside the stream (user and channel addressed) never count as a hole', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    ackWorkspace('5')
    queryClient.clear()
    seed(keys.admin.members('ws-1'))
    deliver(domainEvent({ domain: 'channel', kind: 'renamed', seq: '0', channelId: 'c-1', ids: ['c-1'] }))
    settle()
    expect(invalidated(keys.admin.members('ws-1'))).toBe(false)
  })
})

describe('who changed it', () => {
  it('remembers another person as the author of the entities they touched', () => {
    useAuthStore.setState({ user: { id: 'me' } } as never)
    deliver(domainEvent({ domain: 'document', kind: 'updated', ids: ['d-1'], actorUserId: 'u-other' }))
    expect(useWebSocketStore.getState().recentChanges['d-1']).toMatchObject({ actorUserId: 'u-other' })
  })

  it('does not credit my own changes to anyone else', () => {
    useAuthStore.setState({ user: { id: 'me' } } as never)
    deliver(domainEvent({ domain: 'document', kind: 'updated', ids: ['d-1'], actorUserId: 'me' }))
    expect(useWebSocketStore.getState().recentChanges['d-1']).toBeUndefined()
  })
})

describe('presence', () => {
  it('applies a burst of changes together, once per frame', () => {
    const online = (userId: string, username: string, status: string) =>
      deliver({ oneofKind: 'presenceEvent', presenceEvent: { userId, username, status } })
    online('u-1', 'An', 'online')
    online('u-2', 'Bình', 'online')
    expect(useWebSocketStore.getState().onlineUsers).toEqual({})
    vi.advanceTimersByTime(101)
    expect(useWebSocketStore.getState().onlineUsers).toEqual({ 'u-1': 'An', 'u-2': 'Bình' })
    online('u-1', 'An', 'offline')
    vi.advanceTimersByTime(101)
    expect(useWebSocketStore.getState().onlineUsers).toEqual({ 'u-2': 'Bình' })
  })
})

describe('following a workspace across connections', () => {
  const subscribesTo = (sock: FakeSocket, ws: string) =>
    sock.sent.filter((e) => e.payload.oneofKind === 'subscribe' && e.payload.subscribe.workspaceId === ws).length

  it('keeps following after a disconnect and a reconnect (a token refresh does both)', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    useWebSocketStore.getState().disconnect()
    useWebSocketStore.getState().connect('fresh-token')
    FakeSocket.last!.onopen!()
    authOk()
    expect(subscribesTo(FakeSocket.last!, 'ws-1')).toBe(1)
  })

  it('stops following once the follower releases, even across a reconnect', () => {
    const release = useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    release()
    useWebSocketStore.getState().disconnect()
    useWebSocketStore.getState().connect('fresh-token')
    FakeSocket.last!.onopen!()
    authOk()
    expect(subscribesTo(FakeSocket.last!, 'ws-1')).toBe(0)
  })
})

describe('a refused workspace subscription', () => {
  const deny = () =>
    deliver({ oneofKind: 'workspaceSubscribed', workspaceSubscribed: { workspaceId: 'ws-1', seq: '0', denied: true } })

  it('still refetches what the session missed, so chat and notifications are not left stale', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    seed(keys.messaging.unreadCounts())
    seed(keys.notifications.all())
    authOk()
    deny()
    settle()
    expect(invalidated(keys.messaging.unreadCounts())).toBe(true)
    expect(invalidated(keys.notifications.all())).toBe(true)
  })

  it('tries again with a growing pause, a bounded number of times', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    const count = () =>
      FakeSocket.last!.sent.filter((e) => e.payload.oneofKind === 'subscribe' && e.payload.subscribe.workspaceId === 'ws-1').length
    expect(count()).toBe(1)
    deny()
    vi.advanceTimersByTime(5_100)
    expect(count()).toBe(2)
    deny()
    vi.advanceTimersByTime(15_100)
    expect(count()).toBe(3)
    deny()
    vi.advanceTimersByTime(45_100)
    expect(count()).toBe(4)
    deny()
    vi.advanceTimersByTime(300_000)
    expect(count()).toBe(4)
  })

  it('stops retrying once it is granted', () => {
    useWebSocketStore.getState().followWorkspace('ws-1')
    authOk()
    deny()
    ackWorkspace('4')
    vi.advanceTimersByTime(60_000)
    const n = FakeSocket.last!.sent.filter((e) => e.payload.oneofKind === 'subscribe' && e.payload.subscribe.workspaceId === 'ws-1').length
    expect(n).toBe(1)
  })
})

describe('reconnecting', () => {
  it('does not reconnect after the session ended', () => {
    const dropped = FakeSocket.last!
    useAuthStore.setState({ accessToken: 'a-token' })
    dropped.onclose!({ code: 1006 })
    useAuthStore.setState({ accessToken: null }) // logged out while the socket was down
    vi.advanceTimersByTime(60_000)
    expect(FakeSocket.last).toBe(dropped)
  })

  it('does not reconnect after an intentional disconnect either', () => {
    const dropped = FakeSocket.last!
    useAuthStore.setState({ accessToken: 'a-token' })
    dropped.onclose!({ code: 1006 })
    useWebSocketStore.getState().disconnect()
    vi.advanceTimersByTime(60_000)
    expect(FakeSocket.last).toBe(dropped)
  })
})

describe('permission events', () => {
  it('refresh again a little later, for a policy replica that had not caught up', () => {
    deliver(domainEvent({ domain: 'permission', kind: 'changed', ids: [], actorUserId: '' }))
    settle()
    seed(keys.permissions.object('tenant-1', 'node-1'))
    expect(invalidated(keys.permissions.object('tenant-1', 'node-1'))).toBe(false)
    vi.advanceTimersByTime(2_000 + BATCH_MS + 1)
    expect(invalidated(keys.permissions.object('tenant-1', 'node-1'))).toBe(true)
  })

  it('a share refreshes the share list and the item and leaves permissions alone', () => {
    seed(keys.permissions.object('tenant-1', 'node-1'))
    seed(keys.drive.shares('i-1'))
    deliver(domainEvent({ kind: 'share_created', ids: ['i-1'] }))
    settle()
    expect(invalidated(keys.drive.shares('i-1'))).toBe(true)
    expect(invalidated(keys.permissions.object('tenant-1', 'node-1'))).toBe(false)
  })
})
