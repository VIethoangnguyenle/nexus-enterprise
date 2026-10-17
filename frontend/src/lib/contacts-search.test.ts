import { describe, expect, it } from 'vitest'
import {
  categorySearch, closePersonSearch, deptSearch, groupSearch, homeSearch, openPersonSearch, roleSearch, validateContactsSearch,
} from './contacts-search'

describe('validateContactsSearch', () => {
  it('keeps the workspace and the places a link names', () => {
    expect(validateContactsSearch({ ws: 'w1', cat: 'org', dept: 'd1', person: 'u1', q: 'doi soat' })).toEqual({
      ws: 'w1', cat: 'org', dept: 'd1', person: 'u1', q: 'doi soat',
    })
    expect(validateContactsSearch({ cat: 'roles', role: 'r1' })).toEqual({ cat: 'roles', role: 'r1' })
    expect(validateContactsSearch({ cat: 'groups', chat: 'g1' })).toEqual({ cat: 'groups', chat: 'g1' })
  })

  it('drops what it does not know instead of refusing the link', () => {
    expect(validateContactsSearch({ cat: 'external', dept: 5, person: '', q: 7, extra: 'x' })).toEqual({})
    expect(validateContactsSearch({ cat: 'members' })).toEqual({ cat: 'members' })
    expect(validateContactsSearch({ cat: 'search' })).toEqual({ cat: 'search' })
  })

  it('keeps only the place that belongs to the category', () => {
    expect(validateContactsSearch({ cat: 'groups', dept: 'd1', role: 'r1', chat: 'g1' })).toEqual({ cat: 'groups', chat: 'g1' })
    expect(validateContactsSearch({ cat: 'members', dept: 'd1' })).toEqual({ cat: 'members' })
  })

  it('reads the category from the place when the link gives none', () => {
    expect(validateContactsSearch({ role: 'r1' })).toEqual({ cat: 'roles', role: 'r1' })
    expect(validateContactsSearch({ chat: 'g1' })).toEqual({ cat: 'groups', chat: 'g1' })
    expect(validateContactsSearch({ dept: 'd1' })).toEqual({ cat: 'org', dept: 'd1' })
    // A person alone is a profile over the default category.
    expect(validateContactsSearch({ person: 'u1' })).toEqual({ person: 'u1' })
  })
})

describe('moving around', () => {
  const at = { ws: 'w1', cat: 'org' as const, dept: 'd1', person: 'u1', q: 'x' }

  it('a category starts at its own top and forgets the search, and keeps the profile that is open', () => {
    expect(categorySearch(at, 'roles')).toEqual({ ws: 'w1', cat: 'roles', person: 'u1' })
  })

  it('a department, role or group is opened inside its category', () => {
    expect(deptSearch(at, 'd2')).toEqual({ ws: 'w1', cat: 'org', dept: 'd2', person: 'u1' })
    expect(deptSearch(at, undefined)).toEqual({ ws: 'w1', cat: 'org', person: 'u1' })
    expect(roleSearch(at, 'r1')).toEqual({ ws: 'w1', cat: 'roles', role: 'r1', person: 'u1' })
    expect(groupSearch(at, 'g1')).toEqual({ ws: 'w1', cat: 'groups', chat: 'g1', person: 'u1' })
  })

  it('home is the first screen of the phone: no category, no place, no search; only the workspace', () => {
    expect(homeSearch({ ws: 'w1', cat: 'org', dept: 'd1', person: 'u1', q: 'x' })).toEqual({ ws: 'w1' })
    expect(homeSearch({})).toEqual({})
  })

  it('a profile opens and closes without moving the list', () => {
    expect(openPersonSearch({ ws: 'w1', cat: 'org', dept: 'd1' }, 'u2')).toEqual({ ws: 'w1', cat: 'org', dept: 'd1', person: 'u2' })
    expect(closePersonSearch(at)).toEqual({ ws: 'w1', cat: 'org', dept: 'd1', q: 'x' })
  })
})
