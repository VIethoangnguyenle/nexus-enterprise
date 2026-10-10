import { describe, it, expect, vi, afterEach } from 'vitest'
import { keys } from '../hooks/keys'
import {
  createBatcher,
  emptySeq,
  observeSeq,
  planFor,
  resetSeq,
  workspaceResyncKeys,
  type DomainChange,
  type QueryKey,
} from './realtime'

const ev = (over: Partial<DomainChange>): DomainChange => ({
  domain: 'drive',
  kind: 'updated',
  workspaceId: 'ws-1',
  ids: ['x-1'],
  ...over,
})
const has = (plan: { invalidate: QueryKey[] }, key: QueryKey) =>
  plan.invalidate.some((k) => JSON.stringify(k) === JSON.stringify(key))

describe('planFor: drive', () => {
  it('a created item refreshes the folder it sits in and the quota, not other folders', () => {
    const p = planFor(ev({ kind: 'created', parentId: 'folder-9' }))
    expect(has(p, keys.drive.folder('ws-1', 'folder-9'))).toBe(true)
    expect(has(p, keys.drive.quota('ws-1'))).toBe(true)
    expect(has(p, keys.drive.folder('ws-1', 'other'))).toBe(false)
    expect(has(p, keys.drive.folder('ws-2', 'folder-9'))).toBe(false)
  })

  it('an item at the top level refreshes the root listing', () => {
    const p = planFor(ev({ kind: 'created', parentId: '' }))
    expect(has(p, keys.drive.folder('ws-1'))).toBe(true)
  })

  it('a rename refreshes the folder and the item, with no quota change', () => {
    const p = planFor(ev({ kind: 'updated', parentId: 'f' }))
    expect(has(p, keys.drive.folder('ws-1', 'f'))).toBe(true)
    expect(has(p, keys.drive.item('x-1'))).toBe(true)
    expect(has(p, keys.drive.quota('ws-1'))).toBe(false)
  })

  it('a move refreshes both the old and the new folder', () => {
    const p = planFor(ev({ kind: 'moved', parentId: 'new', oldParentId: 'old' }))
    expect(has(p, keys.drive.folder('ws-1', 'new'))).toBe(true)
    expect(has(p, keys.drive.folder('ws-1', 'old'))).toBe(true)
  })

  it('a move out of the root refreshes the root', () => {
    const p = planFor(ev({ kind: 'moved', parentId: 'new', oldParentId: '' }))
    expect(has(p, keys.drive.folder('ws-1'))).toBe(true)
  })

  it('a delete refreshes the listing, the quota and, for a folder, everything beneath it', () => {
    const p = planFor(ev({ kind: 'deleted', parentId: 'f' }))
    expect(has(p, keys.drive.folder('ws-1', 'f'))).toBe(true)
    expect(has(p, keys.drive.quota('ws-1'))).toBe(true)
    expect(has(p, keys.drive.folders('ws-1'))).toBe(true)
    expect(has(p, keys.drive.item('x-1'))).toBe(true)
  })

  it('a share change refreshes the shares and the item, nothing wider', () => {
    for (const kind of ['share_created', 'share_revoked']) {
      const p = planFor(ev({ kind, parentId: undefined }))
      expect(has(p, keys.drive.shares('x-1'))).toBe(true)
      expect(has(p, keys.drive.item('x-1'))).toBe(true)
      // Who can now see the item is told to those people by the permission
      // event; every subscriber refetching permissions for a share is not wanted.
      expect(has(p, keys.permissions.all())).toBe(false)
      expect(has(p, keys.drive.folders('ws-1'))).toBe(false)
      expect(has(p, keys.drive.sharedWithMe())).toBe(false)
    }
  })

  it('a move also drops permission answers, which depend on where the item sits', () => {
    expect(has(planFor(ev({ kind: 'moved', parentId: 'n', oldParentId: 'o' })), keys.permissions.all())).toBe(true)
    expect(has(planFor(ev({ kind: 'created', parentId: 'n' })), keys.permissions.all())).toBe(false)
  })
})

describe('planFor: document', () => {
  it('lists refresh for every kind; a delete also drops the open document', () => {
    for (const kind of ['created', 'updated', 'deleted']) {
      expect(has(planFor(ev({ domain: 'document', kind })), keys.documents.all('ws-1'))).toBe(true)
    }
    expect(has(planFor(ev({ domain: 'document', kind: 'deleted' })), keys.documents.detail('x-1'))).toBe(true)
    expect(has(planFor(ev({ domain: 'document', kind: 'updated' })), keys.documents.detail('x-1'))).toBe(false)
  })
})

describe('planFor: channel', () => {
  it('refreshes the channel lists, the channel and its members', () => {
    const p = planFor(ev({ domain: 'channel', kind: 'member_added', ids: ['c-1'], channelId: 'c-1' }))
    expect(has(p, keys.messaging.channels('ws-1'))).toBe(true)
    expect(has(p, keys.messaging.channel('c-1'))).toBe(true)
    expect(has(p, keys.messaging.members('c-1'))).toBe(true)
  })
})

describe('planFor: workspace', () => {
  it('a member change refreshes the people, contacts and workspace lists', () => {
    const p = planFor(ev({ domain: 'workspace', kind: 'member_added' }))
    expect(has(p, keys.admin.everything())).toBe(true)
    expect(has(p, keys.contacts.every())).toBe(true)
    expect(has(p, keys.workspaces.all())).toBe(true)
    expect(has(p, keys.permissions.all())).toBe(false)
  })

  it('role, department and removal also drop the permission cache', () => {
    for (const kind of ['role_changed', 'department_changed', 'member_removed']) {
      expect(has(planFor(ev({ domain: 'workspace', kind })), keys.permissions.all())).toBe(true)
    }
  })

  it('an accepted invitation refreshes the roster and the pending invitations', () => {
    const p = planFor(ev({ domain: 'workspace', kind: 'invitation_accepted' }))
    expect(has(p, keys.admin.everything())).toBe(true)
  })
})

describe('planFor: asset', () => {
  it('an asset change refreshes this workspace only', () => {
    const p = planFor(ev({ domain: 'asset', kind: 'updated', ids: ['a-1'] }))
    expect(has(p, keys.assets.lists('ws-1'))).toBe(true)
    expect(has(p, keys.assets.summary('ws-1'))).toBe(true)
    expect(has(p, keys.assets.history('a-1'))).toBe(true)
    expect(has(p, keys.assets.lists('ws-2'))).toBe(false)
  })

  it('a request change refreshes the request lists and that request', () => {
    const p = planFor(ev({ domain: 'asset', kind: 'request_changed', ids: ['r-1'] }))
    expect(has(p, keys.assets.requests('ws-1'))).toBe(true)
    expect(has(p, keys.assets.request('r-1'))).toBe(true)
    expect(has(p, keys.assets.history('r-1'))).toBe(false)
  })
})

describe('planFor: permission', () => {
  it('drops the permission cache and every listing the user could see differently', () => {
    const p = planFor(ev({ domain: 'permission', kind: 'changed', ids: [] }))
    expect(has(p, keys.permissions.all())).toBe(true)
    expect(has(p, keys.drive.everything())).toBe(true)
    expect(has(p, keys.assets.listsAll())).toBe(true)
    expect(p.touched).toEqual([])
  })
})

describe('planFor: unknown domain', () => {
  it('invalidates nothing', () => {
    expect(planFor(ev({ domain: 'mystery' })).invalidate).toEqual([])
  })
})

describe('planFor: touched entities', () => {
  it('reports the ids that changed, for the author wash', () => {
    expect(planFor(ev({ domain: 'workspace', kind: 'member_added', ids: ['u-1', 'u-2'] })).touched).toEqual(['u-1', 'u-2'])
  })
})

describe('workspaceResyncKeys', () => {
  it('covers each data family of the workspace', () => {
    const k = workspaceResyncKeys('ws-1')
    for (const want of [
      keys.drive.everything(),
      keys.permissions.all(),
      keys.documents.all('ws-1'),
      keys.admin.everything(),
      keys.approval.all(),
      keys.assets.listsAll(),
      keys.messaging.channelsAll(),
    ]) {
      expect(k.some((x) => JSON.stringify(x) === JSON.stringify(want))).toBe(true)
    }
  })
})

describe('observeSeq', () => {
  it('applies the first frame it sees and then each successor', () => {
    let s = emptySeq()
    const a = observeSeq(s, 'ws-1', 7)
    expect(a.verdict).toBe('apply')
    s = a.state
    const b = observeSeq(s, 'ws-1', 8)
    expect(b.verdict).toBe('apply')
  })

  it('ignores a repeat', () => {
    const s = resetSeq(emptySeq(), 'ws-1', 4)
    expect(observeSeq(s, 'ws-1', 4).verdict).toBe('duplicate')
  })

  it('asks for a resync on a hole, and moves on to the new position', () => {
    const s = resetSeq(emptySeq(), 'ws-1', 4)
    const r = observeSeq(s, 'ws-1', 9)
    expect(r.verdict).toBe('resync')
    expect(observeSeq(r.state, 'ws-1', 10).verdict).toBe('apply')
  })

  it('asks for a resync when the counter goes backwards (the server lost it)', () => {
    const s = resetSeq(emptySeq(), 'ws-1', 40)
    const r = observeSeq(s, 'ws-1', 2)
    expect(r.verdict).toBe('resync')
    expect(observeSeq(r.state, 'ws-1', 3).verdict).toBe('apply')
  })

  it('counts each workspace on its own', () => {
    let s = resetSeq(emptySeq(), 'ws-1', 4)
    s = resetSeq(s, 'ws-2', 100)
    expect(observeSeq(s, 'ws-1', 5).verdict).toBe('apply')
    expect(observeSeq(s, 'ws-2', 101).verdict).toBe('apply')
  })

  it('frames outside the stream (seq 0) always apply and move nothing', () => {
    const s = resetSeq(emptySeq(), 'ws-1', 4)
    const r = observeSeq(s, 'ws-1', 0)
    expect(r.verdict).toBe('apply')
    expect(r.state).toBe(s)
  })
})

describe('createBatcher', () => {
  afterEach(() => vi.useRealTimers())

  it('runs a burst once, de-duplicated, after the window', () => {
    vi.useFakeTimers()
    const run = vi.fn()
    const b = createBatcher(run, 100)
    b.add([keys.drive.folder('ws-1', 'f'), keys.drive.quota('ws-1')])
    b.add([keys.drive.folder('ws-1', 'f')])
    b.add([keys.drive.folder('ws-1', 'f'), keys.documents.all('ws-1')])
    expect(run).not.toHaveBeenCalled()

    vi.advanceTimersByTime(99)
    expect(run).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(run).toHaveBeenCalledTimes(3)
  })

  it('starts a fresh window after a flush', () => {
    vi.useFakeTimers()
    const run = vi.fn()
    const b = createBatcher(run, 100)
    b.add([keys.drive.quota('ws-1')])
    vi.advanceTimersByTime(100)
    b.add([keys.drive.quota('ws-1')])
    vi.advanceTimersByTime(100)
    expect(run).toHaveBeenCalledTimes(2)
  })

  it('flush runs what is pending immediately, and cancel drops it', () => {
    vi.useFakeTimers()
    const run = vi.fn()
    const b = createBatcher(run, 100)
    b.add([keys.drive.quota('ws-1')])
    b.flush()
    expect(run).toHaveBeenCalledTimes(1)
    b.add([keys.drive.quota('ws-2')])
    b.cancel()
    vi.advanceTimersByTime(500)
    expect(run).toHaveBeenCalledTimes(1)
  })
})
