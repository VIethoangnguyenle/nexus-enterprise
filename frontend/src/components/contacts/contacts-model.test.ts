import { describe, expect, it } from 'vitest'
import type { Channel } from '../../api/messaging'
import type { Department } from '../../api/admin'
import type { Contact } from '../../hooks/useContacts'
import { CONTACTS, DEPT_ID } from '../../test/chat-fixtures'
import {
  NO_FILTER, OWNERS_LABEL, buildTree, colleaguesOf, filterContacts, givenNameOf, isFiltering, nameOf,
  categoriesOf, pathLabel, pathOf, pathSegments, privateGroups, rootLabel, roleLine, rolesOf, searchSubline, sortContacts,
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

  it('narrows to who is online', () => {
    const online = filterContacts(CONTACTS, { ...NO_FILTER, onlineOnly: true }, (c) => c.username === 'lan')
    expect(online.map(nameOf)).toEqual(['Nguyễn Thu Lan'])

    const both = filterContacts(CONTACTS, { query: 'vinh', onlineOnly: true }, (c) => c.username === 'lan')
    expect(both).toEqual([])
  })

  it('knows when a filter is active', () => {
    expect(isFiltering(NO_FILTER)).toBe(false)
    expect(isFiltering({ ...NO_FILTER, query: '  ' })).toBe(false)
    expect(isFiltering({ ...NO_FILTER, onlineOnly: true })).toBe(true)
  })

  it('lists colleagues without the person themselves, and none for someone with no department', () => {
    const lan = CONTACTS.find((c) => c.username === 'lan')!
    expect(colleaguesOf(CONTACTS, lan).map(nameOf)).toEqual(['Lê Quang Vinh'])
    const hoa = CONTACTS.find((c) => c.username === 'hoa')!
    expect(colleaguesOf(CONTACTS, hoa)).toEqual([])
  })

  it('sorts by given name (the last word) the Vietnamese way, then by full name', () => {
    expect(sortContacts(CONTACTS).map(nameOf)).toEqual([
      'Trần Minh Đức', 'Lê Thị Hoa', 'Nguyễn Thu Lan', 'Trần Bảo Ngọc', 'Lê Quang Vinh', 'Phạm Hải Yến',
    ])
  })
})

const person = (id: string, name: string, over: Partial<Contact> = {}): Contact => ({
  user_id: id, ngac_node_id: `node-${id}`, username: id, display_name: name, email: `${id}@x.vn`, title: '',
  department: '', department_id: '', roles: [], location: '', avatar_url: '', is_online: false, ...over,
})
const dept = (id: string, name: string, parent_id = '', over: Partial<Department> = {}): Department =>
  ({ id, name, parent_id, member_count: 0, ...over })

describe('sorting by given name', () => {
  it('takes the last word of the name and ignores accents when comparing', () => {
    expect(givenNameOf('Nguyễn Văn An')).toBe('An')
    expect(givenNameOf('  Hoa ')).toBe('Hoa')
    expect(givenNameOf('')).toBe('')
  })

  it('puts "Nguyễn Văn An" before "Trần Thị Bình"', () => {
    const names = sortContacts([person('a', 'Trần Thị Bình'), person('b', 'Nguyễn Văn An')]).map(nameOf)
    expect(names).toEqual(['Nguyễn Văn An', 'Trần Thị Bình'])
  })

  it('goes by the given name, not the surname: "Trần Văn An" comes before "Nguyễn Thị Bình"', () => {
    const names = sortContacts([person('a', 'Nguyễn Thị Bình'), person('b', 'Trần Văn An')]).map(nameOf)
    expect(names).toEqual(['Trần Văn An', 'Nguyễn Thị Bình'])
  })

  it('orders equal given names by full name', () => {
    const names = sortContacts([person('a', 'Trần Minh An'), person('b', 'Lê Văn An'), person('c', 'Đỗ Quốc An')]).map(nameOf)
    expect(names).toEqual(['Đỗ Quốc An', 'Lê Văn An', 'Trần Minh An'])
  })

  it('does not change the list it was given', () => {
    const list = [person('a', 'Trần Thị Bình'), person('b', 'Nguyễn Văn An')]
    sortContacts(list)
    expect(list.map(nameOf)).toEqual(['Trần Thị Bình', 'Nguyễn Văn An'])
  })
})

describe('the department tree', () => {
  // Khối Tài chính › Đối soát, and Khối Vận hành › Đối soát: two departments with one name.
  const departments = [
    dept('tc', 'Khối Tài chính'),
    dept('vh', 'Khối Vận hành'),
    dept('ds1', 'Đối soát', 'tc'),
    dept('ds2', 'Đối soát', 'vh'),
    dept('kt', 'Kế toán', 'tc'),
  ]
  const people = [
    person('an', 'Nguyễn Văn An', { department_id: 'ds1', department: 'Đối soát' }),
    person('binh', 'Trần Thị Bình', { department_id: 'ds2', department: 'Đối soát' }),
    person('chi', 'Lê Chi', { department_id: 'ds2', department: 'Đối soát' }),
    person('dung', 'Phạm Dũng', { department_id: 'tc', department: 'Khối Tài chính' }),
    person('em', 'Võ Em'),
  ]
  const tree = buildTree(departments, people)

  it('keeps each person under the department they sit in, even when two share a name', () => {
    expect(tree.byId.get('ds1')!.people.map(nameOf)).toEqual(['Nguyễn Văn An'])
    expect(tree.byId.get('ds2')!.people.map(nameOf)).toEqual(['Trần Thị Bình', 'Lê Chi'])
  })

  it('roots are the departments with no parent present, children sorted by name', () => {
    expect(tree.roots.map((n) => n.id)).toEqual(['tc', 'vh'])
    expect(tree.byId.get('tc')!.children.map((n) => n.id)).toEqual(['ds1', 'kt'])
  })

  it('counts the people of the whole subtree and the sub-departments one level down', () => {
    const tc = tree.byId.get('tc')!
    expect(tc.people.map(nameOf)).toEqual(['Phạm Dũng'])
    expect(tc.peopleCount).toBe(2)
    expect(tc.subDepartmentCount).toBe(2)
    expect(tree.byId.get('vh')!.peopleCount).toBe(2)
    expect(tree.byId.get('kt')!.peopleCount).toBe(0)
    expect(tree.byId.get('kt')!.subDepartmentCount).toBe(0)
  })

  it('people with no department stay at the organisation root', () => {
    expect(tree.unassigned.map(nameOf)).toEqual(['Võ Em'])
  })

  it('a person whose department the caller cannot see also stays at the organisation root', () => {
    const t = buildTree(departments, [person('x', 'Ẩn Danh', { department_id: 'pháp-chế' })])
    expect(t.unassigned.map(nameOf)).toEqual(['Ẩn Danh'])
  })

  it('a limited caller whose parent department is not in the list gets their own department as a root', () => {
    const t = buildTree([dept('ds1', 'Đối soát', 'tc')], [person('an', 'Nguyễn Văn An', { department_id: 'ds1' })])
    expect(t.roots.map((n) => n.id)).toEqual(['ds1'])
    expect(t.roots[0]!.peopleCount).toBe(1)
  })

  it('carries the hidden mark an administrator sees', () => {
    const t = buildTree([dept('p', 'Pháp chế', '', { hidden: true }), dept('q', 'Quỹ')], [])
    expect(t.byId.get('p')!.hidden).toBe(true)
    expect(t.byId.get('q')!.hidden).toBe(false)
  })

  it('survives a loop in the parents without hanging, and its people are not lost: they sit at the root', () => {
    const t = buildTree(
      [dept('a', 'A', 'b'), dept('b', 'B', 'a'), dept('ok', 'Ổn')],
      [person('x', 'Võ Xuân', { department_id: 'a' }), person('y', 'Lê Yến', { department_id: 'ok' })],
    )
    expect(pathOf(t, 'a').length).toBeLessThanOrEqual(2)
    expect(t.unassigned.map(nameOf)).toEqual(['Võ Xuân'])
    expect(t.byId.get('ok')!.people.map(nameOf)).toEqual(['Lê Yến'])
  })

  it('names the whole path by ids, so the two same-named departments read differently', () => {
    expect(pathOf(tree, 'ds1').map((n) => n.name)).toEqual(['Khối Tài chính', 'Đối soát'])
    expect(pathLabel(tree, 'ds2')).toBe('Khối Vận hành › Đối soát')
    expect(pathLabel(tree, 'ds1')).toBe('Khối Tài chính › Đối soát')
    expect(pathOf(tree, 'unknown')).toEqual([])
    expect(pathLabel(tree, '')).toBe('')
  })

  it('a search result reads "title · full path"', () => {
    expect(searchSubline(person('an', 'An', { title: 'Chuyên viên', department_id: 'ds1' }), tree)).toBe('Chuyên viên · Khối Tài chính › Đối soát')
    expect(searchSubline(person('x', 'X', { title: 'Chuyên viên' }), tree)).toBe('Chuyên viên')
    expect(searchSubline(person('y', 'Y', { department_id: 'ds2' }), tree)).toBe('Khối Vận hành › Đối soát')
  })

  it('a person\'s path is the tree\'s, or just their own department\'s name while the tree is not there', () => {
    const an = person('an', 'An', { department_id: 'ds1', department: 'Đối soát' })
    expect(pathSegments(tree, an)).toEqual([{ id: 'tc', name: 'Khối Tài chính' }, { id: 'ds1', name: 'Đối soát' }])
    // The tree failed to load, or has not yet: the leaf is still known by name and id.
    expect(pathSegments(undefined, an)).toEqual([{ id: 'ds1', name: 'Đối soát' }])
    // A department the caller cannot see in the tree keeps its name too.
    expect(pathSegments(tree, person('z', 'Z', { department_id: 'gone', department: 'Pháp chế' }))).toEqual([{ id: 'gone', name: 'Pháp chế' }])
    expect(pathSegments(tree, person('e', 'E'))).toEqual([])
  })

  it('names the root of the tree: the organisation, or the scope of someone who sees only part of it', () => {
    expect(rootLabel('NovaPay', false)).toBe('Toàn NovaPay')
    expect(rootLabel('NovaPay', true)).toBe('Phạm vi của bạn')
  })
})

describe('colleagues by department id', () => {
  it('are the others of the same department, never one that merely shares its name', () => {
    const me = person('me', 'Tôi', { department_id: 'ds1', department: 'Đối soát' })
    const same = person('same', 'Cùng Phòng', { department_id: 'ds1', department: 'Đối soát' })
    const namesake = person('other', 'Trùng Tên', { department_id: 'ds2', department: 'Đối soát' })
    expect(colleaguesOf([me, same, namesake], me).map(nameOf)).toEqual(['Cùng Phòng'])
  })

  it('are nobody for a person with no department', () => {
    const a = person('a', 'A'); const b = person('b', 'B')
    expect(colleaguesOf([a, b], a)).toEqual([])
  })

  it('works on the fixture directory', () => {
    const lan = CONTACTS.find((c) => c.username === 'lan')!
    expect(lan.department_id).toBe(DEPT_ID['Đối soát'])
  })
})

describe('roles', () => {
  const owners = { id: 'r-own', name: '', kind: 'owners' as const }
  const members = { id: 'r-mem', name: '', kind: 'members' as const }
  const kt = { id: 'r-kt', name: 'Kế toán viên', kind: 'custom' as const }
  const ax = { id: 'r-ax', name: 'Ân xá', kind: 'custom' as const }
  const people = [
    person('a', 'Nguyễn Văn An', { roles: [members, kt] }),
    person('b', 'Trần Thị Bình', { roles: [members, owners] }),
    person('c', 'Lê Chi', { roles: [members, kt, ax] }),
    person('d', 'Phạm Dũng', { roles: [members] }),
  ]

  it('lists Quản trị viên first, then the custom roles by name, each with its holders', () => {
    const groups = rolesOf(people)
    expect(groups.map((g) => g.name)).toEqual([OWNERS_LABEL, 'Ân xá', 'Kế toán viên'])
    expect(groups[0]!.kind).toBe('owners')
    expect(groups[2]!.people.map(nameOf)).toEqual(['Nguyễn Văn An', 'Lê Chi'])
  })

  it('has no "Members" role: everyone holds it, so it says nothing', () => {
    expect(rolesOf(people).map((g) => g.id)).not.toContain(members.id)
  })

  it('a role nobody visible holds is not listed, and a payload without roles is no error', () => {
    expect(rolesOf([person('x', 'X')])).toEqual([])
    expect(rolesOf([{ ...person('x', 'X'), roles: undefined as unknown as Contact['roles'] }])).toEqual([])
  })
})

describe('groups', () => {
  const ch = (id: string, type: string): Channel => ({ id, name: id, channel_type: type, workspace_id: 'w' })

  it('are the private groups: not #general (everyone), not a direct message', () => {
    expect(privateGroups([ch('general', 'workspace'), ch('nhom', 'private'), ch('dm', 'dm')]).map((c) => c.id)).toEqual(['nhom'])
    expect(privateGroups(undefined)).toEqual([])
  })
})

describe('the categories', () => {
  const base = { isPersonal: false, workspaceName: 'NovaPay', limited: false, memberCount: 16, roleCount: 4, groupCount: 3, rootCount: 3 }
  const view = (over: Partial<Parameters<typeof categoriesOf>[0]> = {}) => categoriesOf({ ...base, ...over }).map((c) => [c.id, c.title, c.sub])

  it('an organisation leads with its structure: Phòng ban, Vai trò, Nhóm của tôi; nothing for the second wave', () => {
    expect(view()).toEqual([
      ['org', 'Phòng ban', 'NovaPay · 3 khối'],
      ['roles', 'Vai trò', '4 vai trò'],
      ['groups', 'Nhóm của tôi', '3 nhóm'],
    ])
    const titles = categoriesOf(base).map((c) => c.title).join('|')
    expect(titles).not.toMatch(/Tất cả thành viên|Bên ngoài|Danh bạ chia sẻ/)
  })

  it('a personal workspace has Thành viên and Nhóm của tôi, no departments or roles', () => {
    expect(view({ isPersonal: true })).toEqual([
      ['members', 'Thành viên', '16 người trong không gian này'],
      ['groups', 'Nhóm của tôi', '3 nhóm'],
    ])
  })

  it('says what a limited caller sees, and does not claim a count of departments it has not loaded', () => {
    expect(view({ limited: true })[0]).toEqual(['org', 'Phòng ban', 'Chỉ phòng của bạn'])
    expect(view({ rootCount: undefined })[0]).toEqual(['org', 'Phòng ban', 'NovaPay'])
  })

  it('hides a category with nothing in it, but not one that is still unknown', () => {
    expect(view({ roleCount: 0 }).map((c) => c[0])).toEqual(['org', 'groups'])
    expect(view({ groupCount: 0 }).map((c) => c[0])).toEqual(['org', 'roles'])
    expect(view({ groupCount: undefined })[2]).toEqual(['groups', 'Nhóm của tôi', 'Nhóm chat bạn đang ở'])
    expect(view({ roleCount: undefined })[1]).toEqual(['roles', 'Vai trò', 'Ai giữ vai trò nào'])
  })

  it('counts one group and one role in the singular-free Vietnamese', () => {
    expect(view({ roleCount: 1, groupCount: 1 })).toEqual([
      ['org', 'Phòng ban', 'NovaPay · 3 khối'], ['roles', 'Vai trò', '1 vai trò'], ['groups', 'Nhóm của tôi', '1 nhóm'],
    ])
  })
})
