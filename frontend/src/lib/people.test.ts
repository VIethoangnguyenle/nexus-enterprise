import { describe, expect, it } from 'vitest'
import type { Contact } from '../hooks/useContacts'
import { buildDirectory, displayName, matchesPerson, normalize, UNKNOWN_PERSON } from './people'

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
