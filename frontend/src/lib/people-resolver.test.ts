import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from '../api/client'
import { clearResolvers, createPeopleResolver, resolverFor } from './people-resolver'

vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

/** Ids are UUIDs: the resolver never sends anything else. */
const uid = (n: number | string) => `00000000-0000-4000-8000-${String(n).padStart(12, '0')}`
const nid = (n: number | string) => `00000000-0000-4000-9000-${String(n).padStart(12, '0')}`

const wire = (n: number, over: Record<string, string> = {}) => ({
  user_id: uid(n), ngac_node_id: nid(n), display_name: `Người ${n}`, avatar_url: '', ...over,
})

/** What the server answers: the people it knows among the ids asked, unknown ids left out. */
const answer = (known: number[]) => (_url: string, init?: RequestInit) => {
  const body = JSON.parse(String(init?.body)) as { user_ids?: string[]; node_ids?: string[] }
  const asked = [...(body.user_ids ?? []), ...(body.node_ids ?? [])]
  const people = known.map((n) => wire(n)).filter((p) => asked.includes(p.user_id) || asked.includes(p.ngac_node_id))
  return Promise.resolve({ people })
}

const settle = () => vi.advanceTimersByTimeAsync(100)
const bodyOf = (call: number) => JSON.parse(String(api.mock.calls[call]![1]!.body)) as { user_ids?: string[]; node_ids?: string[] }

beforeEach(() => {
  vi.useFakeTimers()
  api.mockReset()
  clearResolvers()
})
afterEach(() => vi.useRealTimers())

describe('people resolver: batching', () => {
  it('turns three misses in one tick into one POST', async () => {
    api.mockImplementation(answer([1, 2, 3]))
    const r = createPeopleResolver('w1')
    r.request(uid(1)); r.request(uid(2)); r.request(uid(3))
    expect(api).not.toHaveBeenCalled() // enqueue only: nothing is fetched while rendering
    await settle()
    expect(api).toHaveBeenCalledTimes(1)
    const [url, init] = api.mock.calls[0]!
    expect(url).toBe('/workspaces/w1/contacts/resolve')
    expect(init?.method).toBe('POST')
    expect(bodyOf(0).user_ids).toEqual([uid(1), uid(2), uid(3)])
  })

  it('asks for users and node ids in the same call', async () => {
    api.mockImplementation(answer([1, 2]))
    const r = createPeopleResolver('w1')
    r.request(uid(1)); r.requestNode(nid(2))
    await settle()
    expect(api).toHaveBeenCalledTimes(1)
    expect(bodyOf(0)).toEqual({ user_ids: [uid(1)], node_ids: [nid(2)] })
    expect(r.getSnapshot().byNodeId.get(nid(2))?.name).toBe('Người 2')
  })

  it('splits 250 ids into two calls of at most 200', async () => {
    api.mockImplementation(answer([]))
    const r = createPeopleResolver('w1')
    for (let i = 0; i < 250; i++) r.request(uid(i))
    await settle()
    expect(api).toHaveBeenCalledTimes(2)
    expect(bodyOf(0).user_ids).toHaveLength(200)
    expect(bodyOf(1).user_ids).toHaveLength(50)
  })

  it('ignores an empty id and asks once for an id requested twice', async () => {
    api.mockImplementation(answer([1]))
    const r = createPeopleResolver('w1')
    r.request(''); r.request(uid(1)); r.request(uid(1))
    await settle()
    expect(bodyOf(0).user_ids).toEqual([uid(1)])
  })

  it('never sends what cannot be a person id (a bot name, a username): the server refuses the whole call for it', async () => {
    api.mockImplementation(answer([1]))
    const r = createPeopleResolver('w1')
    r.request('system'); r.requestNode('not-a-node'); r.request(uid(1))
    await settle()
    expect(api).toHaveBeenCalledTimes(1)
    expect(bodyOf(0)).toEqual({ user_ids: [uid(1)] })
  })
})

describe('people resolver: cache', () => {
  it('keeps a resolved person and does not ask again', async () => {
    api.mockImplementation(answer([1]))
    const r = createPeopleResolver('w1')
    r.request(uid(1))
    await settle()
    expect(r.getSnapshot().byUserId.get(uid(1))).toMatchObject({ userId: uid(1), nodeId: nid(1), name: 'Người 1' })
    r.request(uid(1))
    await settle()
    expect(api).toHaveBeenCalledTimes(1)
  })

  it('remembers "not found" too, so an unknown id is not asked for on every render', async () => {
    api.mockImplementation(answer([]))
    const r = createPeopleResolver('w1')
    r.request(uid(999))
    await settle()
    r.request(uid(999))
    await settle()
    expect(api).toHaveBeenCalledTimes(1)
    expect(r.getSnapshot().byUserId.has(uid(999))).toBe(false)
  })

  it('does not ask again while the first answer is on its way', async () => {
    let release: (v: unknown) => void = () => {}
    api.mockImplementation(() => new Promise((res) => { release = res }))
    const r = createPeopleResolver('w1')
    r.request(uid(1))
    await settle()
    r.request(uid(1))
    await settle()
    expect(api).toHaveBeenCalledTimes(1)
    release({ people: [wire(1)] })
    await settle()
    expect(r.getSnapshot().byUserId.get(uid(1))?.name).toBe('Người 1')
  })
})

describe('people resolver: notifications', () => {
  it('stays silent on a request and tells subscribers once, after the answer', async () => {
    api.mockImplementation(answer([1, 2]))
    const r = createPeopleResolver('w1')
    const seen = vi.fn()
    r.subscribe(seen)
    const before = r.getSnapshot()
    r.request(uid(1)); r.request(uid(2))
    expect(seen).not.toHaveBeenCalled()
    expect(r.getSnapshot()).toBe(before) // render must see a stable snapshot
    await settle()
    expect(seen).toHaveBeenCalledTimes(1)
    expect(r.getSnapshot()).not.toBe(before)
  })

  it('stops telling a subscriber that left', async () => {
    api.mockImplementation(answer([1]))
    const r = createPeopleResolver('w1')
    const seen = vi.fn()
    r.subscribe(seen)()
    r.request(uid(1))
    await settle()
    expect(seen).not.toHaveBeenCalled()
  })
})

describe('people resolver: failure', () => {
  it('retries a failed id on the next miss after a backoff, not before', async () => {
    api.mockRejectedValueOnce(new Error('boom')).mockImplementation(answer([1]))
    const r = createPeopleResolver('w1')
    r.request(uid(1))
    await settle()
    expect(api).toHaveBeenCalledTimes(1)
    expect(r.getSnapshot().byUserId.has(uid(1))).toBe(false) // the name stays unknown meanwhile

    r.request(uid(1)) // too soon
    await settle()
    expect(api).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(10_000)
    r.request(uid(1))
    await settle()
    expect(api).toHaveBeenCalledTimes(2)
    expect(r.getSnapshot().byUserId.get(uid(1))?.name).toBe('Người 1')
  })

  it('retries by itself when the backoff is over, without anyone asking again', async () => {
    api.mockRejectedValueOnce(new Error('boom')).mockImplementation(answer([1]))
    const r = createPeopleResolver('w1')
    const seen = vi.fn()
    r.subscribe(seen)
    r.request(uid(1))
    await settle()
    expect(api).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(3_000)
    expect(api).toHaveBeenCalledTimes(2)
    expect(r.getSnapshot().byUserId.get(uid(1))?.name).toBe('Người 1')
    expect(seen).toHaveBeenCalledTimes(1)
  })

  it('gives up retrying alone after a few failures, so an outage is not hammered', async () => {
    api.mockRejectedValue(new Error('down'))
    const r = createPeopleResolver('w1')
    r.request(uid(1))
    await vi.advanceTimersByTimeAsync(10 * 60_000)
    const calls = api.mock.calls.length
    expect(calls).toBeGreaterThan(1)
    expect(calls).toBeLessThanOrEqual(6)
    r.request(uid(1)) // a render that still needs the name may ask again
    await settle()
    expect(api.mock.calls.length).toBe(calls + 1)
  })

  it('clear cancels a retry that is waiting', async () => {
    api.mockRejectedValue(new Error('boom'))
    const r = createPeopleResolver('w1')
    r.request(uid(1))
    await settle()
    r.clear()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(api).toHaveBeenCalledTimes(1)
  })

  it('a failed chunk does not take the other chunk with it', async () => {
    api.mockRejectedValueOnce(new Error('boom')).mockImplementation(answer([]))
    const r = createPeopleResolver('w1')
    for (let i = 0; i < 250; i++) r.request(uid(i))
    await settle()
    expect(api).toHaveBeenCalledTimes(2)
  })
})

describe('people resolver: refresh and clear', () => {
  it('refresh asks again for everyone resolved, keeping the old name until the new one lands', async () => {
    api.mockImplementationOnce(answer([1]))
    const r = createPeopleResolver('w1')
    r.request(uid(1))
    await settle()

    let release: (v: unknown) => void = () => {}
    api.mockImplementationOnce(() => new Promise((res) => { release = res }))
    r.refresh()
    await settle()
    expect(api).toHaveBeenCalledTimes(2)
    expect(r.getSnapshot().byUserId.get(uid(1))?.name).toBe('Người 1')
    release({ people: [wire(1, { display_name: 'Tên mới' })] })
    await settle()
    expect(r.getSnapshot().byUserId.get(uid(1))?.name).toBe('Tên mới')
  })

  it('an answer that was in flight when refresh came is asked for again, not trusted', async () => {
    let release: (v: unknown) => void = () => {}
    api.mockImplementationOnce(() => new Promise((res) => { release = res }))
    const r = createPeopleResolver('w1')
    r.request(uid(1))
    await settle()
    r.refresh()
    api.mockImplementationOnce(() => Promise.resolve({ people: [wire(1, { display_name: 'Tên mới' })] }))
    release({ people: [wire(1)] })
    await settle()
    expect(api).toHaveBeenCalledTimes(2)
    expect(r.getSnapshot().byUserId.get(uid(1))?.name).toBe('Tên mới')
  })

  it('refresh with the changed ids asks only for those people that are held, none of the others', async () => {
    api.mockImplementation(answer([1, 2, 3]))
    const r = createPeopleResolver('w1')
    r.request(uid(1)); r.request(uid(2)); r.request(uid(3))
    await settle()
    api.mockClear()
    r.refresh([uid(2), uid(99)])
    await settle()
    expect(api).toHaveBeenCalledTimes(1)
    expect(bodyOf(0)).toEqual({ user_ids: [uid(2)] })
  })

  it('refresh by the node id of a held person works, and a person known by node only is refreshed too', async () => {
    api.mockImplementation(answer([1]))
    const r = createPeopleResolver('w1')
    r.requestNode(nid(1))
    await settle()
    api.mockClear()
    r.refresh()
    await settle()
    expect(api).toHaveBeenCalledTimes(1)
    expect(bodyOf(0).user_ids).toEqual([uid(1)])
  })

  it('clear forgets everyone, lets subscribers know and ignores an answer that was in flight', async () => {
    let release: (v: unknown) => void = () => {}
    api.mockImplementationOnce(() => new Promise((res) => { release = res }))
    const r = createPeopleResolver('w1')
    const seen = vi.fn()
    r.subscribe(seen)
    r.request(uid(1))
    await settle()
    r.clear()
    expect(seen).toHaveBeenCalled()
    release({ people: [wire(1)] })
    await settle()
    expect(r.getSnapshot().byUserId.size).toBe(0)
  })

  it('keeps one store per workspace, and signing out empties them all', async () => {
    api.mockImplementation(answer([1]))
    const a = resolverFor('w1')
    expect(resolverFor('w1')).toBe(a)
    expect(resolverFor('w2')).not.toBe(a)
    a.request(uid(1))
    await settle()
    expect(a.getSnapshot().byUserId.size).toBe(1)

    clearResolvers()
    expect(a.getSnapshot().byUserId.size).toBe(0)
    expect(resolverFor('w1').getSnapshot().byUserId.size).toBe(0)
  })
})
