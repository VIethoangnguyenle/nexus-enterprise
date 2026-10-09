import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { queryClient } from '../lib/query-client'
import { keys } from '../hooks/keys'
import { useAuthStore } from './auth.store'
import { useWebSocketStore } from './websocket.store'
import { ServerEnvelope } from '../generated/proto/messaging/ws'

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
  send() {}
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

beforeEach(() => {
  queryClient.clear()
  vi.stubGlobal('WebSocket', FakeSocket)
  useAuthStore.setState({ tenantId: 'tenant-1' })
  useWebSocketStore.getState().connect('token')
})

afterEach(() => {
  vi.unstubAllGlobals()
  useWebSocketStore.setState({ ws: null })
})

describe('websocket events invalidate the keys the screens actually cache under', () => {
  it('reconnect resync refreshes cached polls', () => {
    seed(keys.messaging.poll('poll-1'))
    deliver({ oneofKind: 'authResponse', authResponse: { ok: true, userId: 'u', reason: '' } })
    expect(invalidated(keys.messaging.poll('poll-1'))).toBe(true)
  })

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
    deliver({
      oneofKind: 'driveObject',
      driveObject: { eventType: 'created', itemId: 'i-1', parentId: '', workspaceId: 'ws-1' },
    })
    expect(invalidated(keys.drive.folder('ws-1'))).toBe(true)
    expect(invalidated(keys.drive.folder('ws-1', 'folder-9'))).toBe(false)
    expect(invalidated(keys.drive.folder('ws-2'))).toBe(false)
  })

  it('a drive event inside a folder refreshes that folder only', () => {
    seed(keys.drive.folder('ws-1'))
    seed(keys.drive.folder('ws-1', 'folder-9'))
    deliver({
      oneofKind: 'driveObject',
      driveObject: { eventType: 'created', itemId: 'i-1', parentId: 'folder-9', workspaceId: 'ws-1' },
    })
    expect(invalidated(keys.drive.folder('ws-1', 'folder-9'))).toBe(true)
    expect(invalidated(keys.drive.folder('ws-1'))).toBe(false)
  })

  it('a drive event refreshes workspace quota on create and delete', () => {
    seed(keys.drive.quota('ws-1'))
    deliver({
      oneofKind: 'driveObject',
      driveObject: { eventType: 'deleted', itemId: 'i-1', parentId: '', workspaceId: 'ws-1' },
    })
    expect(invalidated(keys.drive.quota('ws-1'))).toBe(true)
  })

  it('an asset event reaches the summary cached under its workspace', () => {
    seed(keys.assets.summary('ws-1'))
    seed(keys.assets.list('ws-1', { status: 'active' }))
    seed(keys.assets.history('asset-1'))
    deliver({ oneofKind: 'assetUpdated', assetUpdated: { assetId: 'asset-1', newState: 'assigned' } })
    expect(invalidated(keys.assets.summary('ws-1'))).toBe(true)
    expect(invalidated(keys.assets.list('ws-1', { status: 'active' }))).toBe(true)
    expect(invalidated(keys.assets.history('asset-1'))).toBe(true)
  })

  it('an unread-count event refreshes both the chat and the notification counters', () => {
    seed(keys.messaging.unreadCounts())
    seed(keys.notifications.unreadCount())
    deliver({ oneofKind: 'unreadCount', unreadCount: { channelId: 'c-1', count: 3 } } as ServerEnvelopeInit)
    expect(invalidated(keys.messaging.unreadCounts())).toBe(true)
    expect(invalidated(keys.notifications.unreadCount())).toBe(true)
  })

  it('a permission event refreshes that object for the current tenant only', () => {
    seed(keys.permissions.object('tenant-1', 'node-1'))
    seed(keys.permissions.object('tenant-1', 'node-2'))
    deliver({ oneofKind: 'drivePerm', drivePerm: { itemId: 'node-1', workspaceId: 'ws-1' } })
    expect(invalidated(keys.permissions.object('tenant-1', 'node-1'))).toBe(true)
    expect(invalidated(keys.permissions.object('tenant-1', 'node-2'))).toBe(false)
  })

  it('a pin event refreshes the pins of that channel', () => {
    seed(keys.messaging.pins('c-1'))
    deliver({ oneofKind: 'pinEvent', pinEvent: { channelId: 'c-1', messageId: 'm-1', action: 'pin', userId: 'u' } } as ServerEnvelopeInit)
    expect(invalidated(keys.messaging.pins('c-1'))).toBe(true)
  })

  it('a moved drive item leaves its old folder too, which the event does not name', () => {
    seed(keys.drive.folder('ws-1', 'old-folder'))
    seed(keys.drive.folder('ws-2', 'other-ws-folder'))
    deliver({
      oneofKind: 'driveObject',
      driveObject: { eventType: 'moved', itemId: 'i-1', parentId: 'new-folder', workspaceId: 'ws-1' },
    })
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
})
