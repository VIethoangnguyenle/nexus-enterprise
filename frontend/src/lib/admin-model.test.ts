import { describe, expect, it } from 'vitest'
import type { Department, Member, PermissionArea } from '../api/admin'
import {
  areaLabel, changeCount, deptAncestors, changesBetween, childrenIndex, consequenceLine, deptDepth, deptPath, deptPathLabel,
  deptWithDescendants, grantsOf, matchesMember, matrixColumns, opLabel, roleLine, roleName, rolePills, setOp, statusLabel,
} from './admin-model'

const dept = (id: string, name: string, parent = ''): Department => ({ id, name, parent_id: parent, member_count: 0 })
const DEPTS = [
  dept('d-khoi', 'Khối Vận hành'),
  dept('d-vh', 'Vận hành thanh toán', 'd-khoi'),
  dept('d-ds', 'Đối soát', 'd-vh'),
  dept('d-kt', 'Kế toán', 'd-khoi'),
  dept('d-hc', 'Hành chính'),
]

const person = (over: Partial<Member>): Member => ({
  ngac_node_id: 'n', user_id: 'u', display_name: 'Nguyễn Thu Lan', email: 'lan@novapay.vn', avatar_url: '',
  title: 'Chuyên viên', status: 'active', is_owner: false, department: null, roles: [], ...over,
})

describe('words', () => {
  it('labels the eight operations in Vietnamese', () => {
    expect(['read', 'write', 'upload', 'approve', 'share', 'manage', 'invite', 'create_channel'].map(opLabel)).toEqual([
      'Xem', 'Sửa', 'Tải lên', 'Duyệt', 'Chia sẻ', 'Quản lý', 'Mời', 'Tạo nhóm chat',
    ])
  })

  it('never shows a code it has no word for', () => {
    expect(opLabel('teleport')).toBe('Quyền khác')
    expect(areaLabel('payroll')).toBe('Vùng khác')
  })

  it('words the built-in roles itself and uses the typed name for the rest', () => {
    expect(roleName({ kind: 'owners' })).toBe('Chủ sở hữu')
    expect(roleName({ kind: 'members' })).toBe('Thành viên')
    expect(roleName({ kind: 'custom', name: 'Kế toán' })).toBe('Kế toán')
    expect(roleName({ kind: 'custom' })).toBe('Vai trò')
    expect(roleLine({ kind: 'owners' })).toMatch(/Toàn quyền/)
  })

  it('has a word for each standing', () => {
    expect(['active', 'invited', 'disabled'].map(statusLabel)).toEqual(['Đang hoạt động', 'Đã mời', 'Đã khoá'])
  })
})

describe('people', () => {
  it('searches name, email and title without regard to accents or case', () => {
    const lan = person({})
    expect(matchesMember(lan, 'thu lan')).toBe(true)
    expect(matchesMember(lan, 'NGUYEN')).toBe(true)
    expect(matchesMember(lan, 'novapay')).toBe(true)
    expect(matchesMember(lan, 'chuyen vien')).toBe(true)
    expect(matchesMember(lan, 'đức')).toBe(false)
    expect(matchesMember(lan, '  ')).toBe(true)
  })

  it('shows at most two role pills, then a count', () => {
    const roles = (...names: string[]) => names.map((n, i) => ({ id: `r${i}`, name: n }))
    expect(rolePills(person({ roles: roles('Kế toán') }))).toEqual({ shown: ['Kế toán'], more: 0 })
    expect(rolePills(person({ is_owner: true, roles: roles('Kế toán', 'Kiểm soát', 'Tài sản') }))).toEqual({
      shown: ['Chủ sở hữu', 'Kế toán'], more: 2,
    })
  })

  it('shows the default role only for someone who holds nothing else', () => {
    expect(rolePills(person({}))).toEqual({ shown: ['Thành viên'], more: 0 })
    expect(rolePills(person({ is_owner: true })).shown).toEqual(['Chủ sở hữu'])
  })
})

describe('departments', () => {
  it('groups by parent and treats an unknown parent as a root', () => {
    const idx = childrenIndex([...DEPTS, dept('d-lost', 'Mồ côi', 'gone')])
    expect(idx.get('')!.map((d) => d.id)).toEqual(['d-khoi', 'd-hc', 'd-lost'])
    expect(idx.get('d-khoi')!.map((d) => d.id)).toEqual(['d-vh', 'd-kt'])
  })

  it('spells a path from the root', () => {
    expect(deptPath(DEPTS, 'd-ds')).toEqual(['Khối Vận hành', 'Vận hành thanh toán', 'Đối soát'])
    expect(deptPathLabel(DEPTS, 'd-ds')).toBe('Khối Vận hành › Vận hành thanh toán › Đối soát')
    expect(deptPath(DEPTS, 'nope')).toEqual([])
  })

  it('lists the departments above one, nearest first', () => {
    expect(deptAncestors(DEPTS, 'd-ds')).toEqual(['d-vh', 'd-khoi'])
    expect(deptAncestors(DEPTS, 'd-hc')).toEqual([])
    expect(deptAncestors(DEPTS, 'nope')).toEqual([])
  })

  it('survives a cycle in the data', () => {
    const loop = [dept('a', 'A', 'b'), dept('b', 'B', 'a')]
    expect(deptPath(loop, 'a').length).toBeLessThanOrEqual(2)
    expect(deptDepth(loop)).toBe(0)
  })

  it('counts levels', () => {
    expect(deptDepth(DEPTS)).toBe(3)
    expect(deptDepth([])).toBe(0)
    expect(deptDepth([dept('a', 'A')])).toBe(1)
  })

  it('names a department and everything beneath it', () => {
    expect([...deptWithDescendants(DEPTS, 'd-vh')].sort()).toEqual(['d-ds', 'd-vh'])
  })
})

describe('the permission matrix', () => {
  const AREAS: PermissionArea[] = [
    { area: 'documents', operations: ['read', 'write', 'upload', 'share'] },
    { area: 'management', operations: ['manage', 'invite'] },
  ]

  it('has a column for every operation any area offers, and only those', () => {
    expect(matrixColumns(AREAS)).toEqual(['read', 'write', 'upload', 'share', 'manage', 'invite'])
    expect(matrixColumns([{ area: 'documents', operations: ['read'] }])).toEqual(['read'])
    expect(matrixColumns([])).toEqual([])
  })

  it('keeps an operation it does not know after the familiar ones', () => {
    expect(matrixColumns([{ area: 'x', operations: ['teleport', 'read'] }])).toEqual(['read', 'teleport'])
  })

  it('turns an operation on and off in the area\'s own order, and refuses one the area does not offer', () => {
    let draft = grantsOf([{ area: 'documents', operations: ['read'] }])
    draft = setOp(draft, AREAS[0]!, 'share', true)
    draft = setOp(draft, AREAS[0]!, 'write', true)
    expect(draft.documents).toEqual(['read', 'write', 'share'])
    draft = setOp(draft, AREAS[0]!, 'read', false)
    expect(draft.documents).toEqual(['write', 'share'])
    expect(setOp(draft, AREAS[0]!, 'approve', true)).toBe(draft)
  })

  it('finds what changed, per area', () => {
    const saved = grantsOf([{ area: 'documents', operations: ['read', 'write'] }])
    const draft = { documents: ['read', 'share'], management: ['manage'] }
    expect(changesBetween(saved, draft, AREAS)).toEqual([
      { area: 'documents', added: ['share'], removed: ['write'] },
      { area: 'management', added: ['manage'], removed: [] },
    ])
    expect(changesBetween(saved, saved, AREAS)).toEqual([])
  })

  it('says who is affected and how', () => {
    const changes = [{ area: 'assets', added: ['approve'], removed: [] }]
    expect(consequenceLine('Kế toán', 14, changes)).toBe('14 người có vai trò Kế toán sẽ được Duyệt trên Tài sản.')
    expect(consequenceLine('Kế toán', 14, [{ area: 'documents', added: [], removed: ['write', 'share'] }])).toBe(
      '14 người có vai trò Kế toán sẽ không còn Sửa, Chia sẻ trên Tài liệu.',
    )
    expect(consequenceLine('Kế toán', 0, changes)).toMatch(/Chưa ai giữ vai trò Kế toán/)
    expect(consequenceLine('Kế toán', 3, [])).toBe('')
  })

  it('counts one change per operation', () => {
    expect(changeCount([{ area: 'a', added: ['read', 'write'], removed: ['share'] }, { area: 'b', added: [], removed: ['manage'] }])).toBe(4)
  })
})
