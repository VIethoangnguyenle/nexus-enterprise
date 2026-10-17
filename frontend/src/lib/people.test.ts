import { describe, expect, it, vi } from 'vitest'
import type { Contact } from '../hooks/useContacts'
import { avatarOf, buildDirectory, displayName, EMPTY_DIRECTORY, matchesPerson, normalize, UNKNOWN_PERSON } from './people'

const contact = (over: Partial<Contact>): Contact =>
  ({ user_id: 'u1', ngac_node_id: 'n1', username: 'Duc.Le', display_name: 'Lê Văn Đức', title: 'Trưởng phòng', department: 'Vận hành', avatar_url: '', ...over }) as Contact

describe('people directory', () => {
  const dir = buildDirectory([contact({}), contact({ user_id: 'u2', ngac_node_id: 'n2', username: 'an', display_name: '  ', title: '', department: '' })])

  it('indexes by user id, node id and lower-cased username', () => {
    expect(dir.byUserId.get('u1')?.name).toBe('Lê Văn Đức')
    expect(dir.byNodeId.get('n1')?.userId).toBe('u1')
    expect(dir.byUsername.get('duc.le')?.userId).toBe('u1')
  })

  it('joins title and department as the role, and tolerates either being empty', () => {
    expect(dir.byUserId.get('u1')?.role).toBe('Trưởng phòng · Vận hành')
    expect(dir.byUserId.get('u2')?.role).toBe('')
  })

  it('never uses an id as a name: a blank display name becomes the neutral word', () => {
    expect(dir.byUserId.get('u2')?.name).toBe(UNKNOWN_PERSON)
  })

  it('displayName prefers the directory, then the sent username, then the neutral word', () => {
    expect(displayName(dir, 'u1')).toBe('Lê Văn Đức')
    expect(displayName(dir, 'missing', 'DUC.LE')).toBe('Lê Văn Đức')
    expect(displayName(dir, 'missing', ' bob ')).toBe('bob')
    expect(displayName(dir, 'missing')).toBe(UNKNOWN_PERSON)
  })
})

describe('people search', () => {
  const p = buildDirectory([contact({})]).list[0]!

  it('normalises accents and đ', () => {
    expect(normalize('  Đức Lê ')).toBe('duc le')
  })

  it('matches name, role and username without accents; empty query matches all', () => {
    expect(matchesPerson(p, 'duc')).toBe(true)
    expect(matchesPerson(p, 'van hanh')).toBe(true)
    expect(matchesPerson(p, 'duc.le')).toBe(true)
    expect(matchesPerson(p, '')).toBe(true)
    expect(matchesPerson(p, 'zzz')).toBe(false)
  })
})

describe('buildDirectory without departments (personal workspace)', () => {
  it('leaves the department out of the role line and out of search', () => {
    const [p] = buildDirectory([contact({ department: 'Vận hành' })], { departments: false }).list
    expect(p!.role).toBe('Trưởng phòng')
    expect(matchesPerson(p!, 'vận hành')).toBe(false)
  })

  it('keeps it by default', () => {
    const [p] = buildDirectory([contact({ department: 'Vận hành' })]).list
    expect(p!.role).toBe('Trưởng phòng · Vận hành')
  })
})

describe('people the directory does not list (hidden or removed)', () => {
  const resolved = {
    byUserId: new Map([['u9', { userId: 'u9', nodeId: 'n9', name: 'Phạm Ẩn', avatarUrl: '/api/auth/avatars/x' }]]),
    byNodeId: new Map([['n9', { userId: 'u9', nodeId: 'n9', name: 'Phạm Ẩn', avatarUrl: '/api/auth/avatars/x' }]]),
  }

  it('are named and pictured from the resolver, but not visible and not in the list', () => {
    const dir = buildDirectory([contact({})], { resolved })
    expect(displayName(dir, 'u9')).toBe('Phạm Ẩn')
    expect(avatarOf(dir, 'u9')).toBe('/api/auth/avatars/x')
    expect(dir.byNodeId.get('n9')?.name).toBe('Phạm Ẩn')
    expect(dir.isVisible('u9')).toBe(false)
    expect(dir.isVisible('u1')).toBe(true)
    expect(dir.list.map((p) => p.userId)).toEqual(['u1'])
  })

  it('a listed person wins over a resolved copy of the same id', () => {
    const dir = buildDirectory([contact({})], {
      resolved: { byUserId: new Map([['u1', { userId: 'u1', nodeId: 'n1', name: 'Tên cũ', avatarUrl: '' }]]), byNodeId: new Map() },
    })
    expect(displayName(dir, 'u1')).toBe('Lê Văn Đức')
    expect(dir.isVisible('u1')).toBe(true)
  })

  it('a resolved person without a saved name reads as the neutral word, never an id', () => {
    const dir = buildDirectory([], {
      resolved: { byUserId: new Map([['u8', { userId: 'u8', nodeId: 'n8', name: '', avatarUrl: '' }]]), byNodeId: new Map() },
    })
    expect(displayName(dir, 'u8')).toBe(UNKNOWN_PERSON)
  })

  it('a miss asks the resolver once for the id; a hit or a username match asks for nothing', () => {
    const request = vi.fn()
    const dir = buildDirectory([contact({})], { request })
    displayName(dir, 'u1')
    displayName(dir, 'missing', 'Duc.Le')
    avatarOf(dir, 'u1')
    expect(request).not.toHaveBeenCalled()
    displayName(dir, 'u7')
    avatarOf(dir, 'u7')
    expect(request).toHaveBeenCalledWith('u7')
    displayName(dir, undefined, 'bob')
    displayName(dir, '')
    expect(request).toHaveBeenCalledTimes(2)
  })

  it('a directory with no resolver can be asked without harm', () => {
    expect(() => displayName(buildDirectory([]), 'u7')).not.toThrow()
  })
})

describe('a directory knows whether it is ready', () => {
  it('is not before the contacts arrive (and when they fail), and is once it is built from them', () => {
    expect(EMPTY_DIRECTORY.ready).toBe(false)
    expect(buildDirectory([]).ready).toBe(true)
  })
})

describe('the id maps ask for what they lack', () => {
  const dir = (request = vi.fn(), requestNode = vi.fn()) => ({ request, requestNode, d: buildDirectory([contact({})], { request, requestNode }) })

  it('byUserId.get queues a missing user id; a hit, has() and a username lookup do not', () => {
    const { request, d } = dir()
    d.byUserId.get('u1'); d.byUserId.has('u7'); d.byUsername.get('zzz')
    expect(request).not.toHaveBeenCalled()
    d.byUserId.get('u7')
    expect(request).toHaveBeenCalledWith('u7')
  })

  it('byNodeId.get queues a missing node id by the node path, not as a user id', () => {
    const { request, requestNode, d } = dir()
    d.byNodeId.get('n7')
    expect(requestNode).toHaveBeenCalledWith('n7')
    expect(request).not.toHaveBeenCalled()
    d.byNodeId.get('n1')
    expect(requestNode).toHaveBeenCalledTimes(1)
  })

  it('still behave as maps: size, iteration, set', () => {
    const { d } = dir()
    expect(d.byUserId.size).toBe(1)
    expect([...d.byUserId.keys()]).toEqual(['u1'])
  })
})
