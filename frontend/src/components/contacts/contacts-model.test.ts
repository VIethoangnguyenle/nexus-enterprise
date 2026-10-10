import { describe, expect, it } from 'vitest'
import { CONTACTS } from '../../test/chat-fixtures'
import {
  NO_FILTER, colleaguesOf, departmentsOf, filterContacts, isFiltering, nameOf, roleLine, sortContacts,
} from './contacts-model'

const offline = () => false

describe('contacts model', () => {
  it('names a person by display name, and never by a username or an id', () => {
    expect(nameOf({ display_name: 'Lê Thị Hoa' })).toBe('Lê Thị Hoa')
    expect(nameOf({ display_name: '  ' })).toBe('Thành viên')
    expect(nameOf({ display_name: '' })).toBe('Thành viên')
  })

  it('joins title and department, whichever exist', () => {
    expect(roleLine({ title: 'Trưởng phòng', department: 'Vận hành' })).toBe('Trưởng phòng · Vận hành')
    expect(roleLine({ title: '', department: 'Vận hành' })).toBe('Vận hành')
    expect(roleLine({ title: '', department: '' })).toBe('')
  })

  it('filters by name, title or email without caring about accents or case', () => {
    const find = (query: string) => filterContacts(CONTACTS, { ...NO_FILTER, query }, offline).map(nameOf)
    expect(find('duc')).toEqual(['Trần Minh Đức'])
    expect(find('TRUONG PHONG')).toEqual(['Trần Minh Đức'])
    expect(find('lan@novapay')).toEqual(['Nguyễn Thu Lan'])
    expect(find('khong ai')).toEqual([])
  })

  it('narrows to a department and to who is online', () => {
    const doisoat = filterContacts(CONTACTS, { ...NO_FILTER, department: 'Đối soát' }, offline)
    expect(doisoat.map(nameOf).sort()).toEqual(['Lê Quang Vinh', 'Nguyễn Thu Lan'])

    const online = filterContacts(CONTACTS, { ...NO_FILTER, onlineOnly: true }, (c) => c.username === 'lan')
    expect(online.map(nameOf)).toEqual(['Nguyễn Thu Lan'])

    const both = filterContacts(CONTACTS, { query: 'vinh', department: 'Đối soát', onlineOnly: true }, (c) => c.username === 'lan')
    expect(both).toEqual([])
  })

  it('knows when a filter is active', () => {
    expect(isFiltering(NO_FILTER)).toBe(false)
    expect(isFiltering({ ...NO_FILTER, query: '  ' })).toBe(false)
    expect(isFiltering({ ...NO_FILTER, onlineOnly: true })).toBe(true)
  })

  it('counts departments with people and sorts them', () => {
    expect(departmentsOf(CONTACTS)).toEqual([
      { name: 'Đối soát', count: 2 },
      { name: 'Kiểm soát', count: 1 },
      { name: 'Vận hành thanh toán', count: 2 },
    ])
  })

  it('lists colleagues without the person themselves, and none for someone with no department', () => {
    const lan = CONTACTS.find((c) => c.username === 'lan')!
    expect(colleaguesOf(CONTACTS, lan).map(nameOf)).toEqual(['Lê Quang Vinh'])
    const hoa = CONTACTS.find((c) => c.username === 'hoa')!
    expect(colleaguesOf(CONTACTS, hoa)).toEqual([])
  })

  it('sorts by name the Vietnamese way', () => {
    expect(sortContacts(CONTACTS).map(nameOf)).toEqual([
      'Lê Quang Vinh', 'Lê Thị Hoa', 'Nguyễn Thu Lan', 'Phạm Hải Yến', 'Trần Bảo Ngọc', 'Trần Minh Đức',
    ])
  })
})
