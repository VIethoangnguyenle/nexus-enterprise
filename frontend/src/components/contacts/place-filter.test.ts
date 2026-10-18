import { describe, expect, it } from 'vitest'
import type { Contact } from '../../hooks/useContacts'
import { buildTree } from './contacts-model'
import { deptChoices, inPlace, placeOfLevel, type PlaceFilter } from './place-filter'

const person = (id: string, dept: string, roles: string[] = []): Contact => ({
  user_id: id, ngac_node_id: `n-${id}`, username: id, display_name: `Người ${id}`, email: '', title: '', department: '',
  department_id: dept, roles: roles.map((r) => ({ id: r, name: r, kind: 'custom' as const })), location: '', avatar_url: '', is_online: false,
})
const DEPTS = [
  { id: 'k', name: 'Khối Vận hành', parent_id: '' },
  { id: 'v', name: 'Vận hành thanh toán', parent_id: 'k' },
  { id: 'd', name: 'Đối soát', parent_id: 'v' },
  { id: 't', name: 'Khối Tài chính', parent_id: '' },
]
const PEOPLE = [person('a', 'k'), person('b', 'v', ['pay']), person('c', 'd'), person('e', 't', ['pay']), person('f', '')]
const tree = buildTree(DEPTS as never, PEOPLE)

describe('inPlace', () => {
  it('a department keeps its people and everyone beneath it, nobody else', () => {
    const f: PlaceFilter = { kind: 'dept', id: 'v', label: 'Vận hành thanh toán' }
    expect(inPlace(PEOPLE, f, { tree }).map((c) => c.user_id)).toEqual(['b', 'c'])
  })

  it('a department the tree does not know yet keeps only the people placed in it', () => {
    expect(inPlace(PEOPLE, { kind: 'dept', id: 't', label: 'x' }, {}).map((c) => c.user_id)).toEqual(['e'])
  })

  it('a role keeps its holders', () => {
    expect(inPlace(PEOPLE, { kind: 'role', id: 'pay', label: 'Duyệt chi' }, { tree }).map((c) => c.user_id)).toEqual(['b', 'e'])
  })

  it('a group keeps its members the directory shows, and nobody while its members are unknown', () => {
    const f: PlaceFilter = { kind: 'group', id: 'g', label: 'Đối soát liên khối' }
    expect(inPlace(PEOPLE, f, { groupMembers: new Set(['c', 'f', 'hidden']) }).map((c) => c.user_id)).toEqual(['c', 'f'])
    expect(inPlace(PEOPLE, f, {})).toEqual([])
  })
})

describe('deptChoices', () => {
  it('lists every department with the path above it, in tree order (siblings by name)', () => {
    expect(deptChoices(tree)).toEqual([
      { id: 't', label: 'Khối Tài chính' },
      { id: 'k', label: 'Khối Vận hành' },
      { id: 'v', label: 'Khối Vận hành › Vận hành thanh toán' },
      { id: 'd', label: 'Khối Vận hành › Vận hành thanh toán › Đối soát' },
    ])
  })
})

describe('placeOfLevel', () => {
  it('is the department, role or group a level shows, and nothing at the top of a category', () => {
    expect(placeOfLevel({ kind: 'dept', title: 'Đối soát' }, { dept: 'd' })).toEqual({ kind: 'dept', id: 'd', label: 'Đối soát' })
    expect(placeOfLevel({ kind: 'role', title: 'Duyệt chi' }, { role: 'pay' })).toEqual({ kind: 'role', id: 'pay', label: 'Duyệt chi' })
    expect(placeOfLevel({ kind: 'group', title: 'Nhóm A' }, { chat: 'g' })).toEqual({ kind: 'group', id: 'g', label: 'Nhóm A' })
    expect(placeOfLevel({ kind: 'dept', title: 'NovaPay' }, {})).toBeUndefined()
    expect(placeOfLevel({ kind: 'members', title: 'Thành viên' }, {})).toBeUndefined()
  })
})
