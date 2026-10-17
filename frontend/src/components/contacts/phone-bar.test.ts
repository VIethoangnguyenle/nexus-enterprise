import { describe, expect, it } from 'vitest'
import type { BrowseView } from '../../hooks/useContactsBrowse'
import { phoneBarOf } from './phone-bar'

const view = (over: Partial<BrowseView>): BrowseView => ({
  kind: 'dept', level: { kind: over.kind ?? 'dept', title: over.title ?? '', sub: over.sub ?? '' }, status: 'ready', retry: () => {}, all: [], limited: false, tree: undefined, title: '', sub: '', trail: [],
  atRoot: false, depts: [], people: [], roles: [], groups: [], members: [], ...over,
})
const seg = (id: string, name: string) => ({ id, name })

describe('what the bar of a phone screen says', () => {
  it('Phòng ban at the top names the organisation under it', () => {
    expect(phoneBarOf(view({ atRoot: true, title: 'NovaPay' }), 'NovaPay')).toEqual({ title: 'Phòng ban', above: [], text: 'NovaPay' })
  })

  it('a department names itself and, under it, the path above it', () => {
    const trail = [seg('k', 'Khối Vận hành'), seg('v', 'Vận hành thanh toán'), seg('d', 'Đối soát')]
    expect(phoneBarOf(view({ title: 'Đối soát', trail }), 'NovaPay')).toEqual({
      title: 'Đối soát', above: [seg('k', 'Khối Vận hành'), seg('v', 'Vận hành thanh toán')],
    })
  })

  it('a top-level department says the organisation', () => {
    expect(phoneBarOf(view({ title: 'Khối Vận hành', trail: [seg('k', 'Khối Vận hành')] }), 'NovaPay'))
      .toEqual({ title: 'Khối Vận hành', above: [], text: 'NovaPay' })
  })

  it('roles, groups, members and search each say what they hold', () => {
    expect(phoneBarOf(view({ kind: 'roles', title: 'Vai trò', sub: '4 vai trò' }), 'NovaPay')).toEqual({ title: 'Vai trò', above: [], text: '4 vai trò' })
    expect(phoneBarOf(view({ kind: 'role', title: 'Duyệt chi', sub: '2 người' }), 'NovaPay')).toEqual({ title: 'Duyệt chi', above: [], text: 'Vai trò · 2 người' })
    expect(phoneBarOf(view({ kind: 'groups', title: 'Nhóm của tôi', sub: '3 nhóm' }), 'NovaPay')).toEqual({ title: 'Nhóm của tôi', above: [], text: '3 nhóm' })
    expect(phoneBarOf(view({ kind: 'group', title: 'Đối soát liên khối', sub: '5 thành viên' }), 'NovaPay')).toEqual({ title: 'Đối soát liên khối', above: [], text: '5 thành viên' })
    expect(phoneBarOf(view({ kind: 'members', title: 'Thành viên', sub: '7 người trong Không gian của bạn' }), 'x')).toEqual({ title: 'Thành viên', above: [], text: '7 người trong Không gian của bạn' })
    expect(phoneBarOf(view({ kind: 'search' }), 'NovaPay')).toEqual({ title: 'Tìm trong danh bạ', above: [], text: 'Cả NovaPay' })
  })

  it('keeps the name of the level while something is typed over it', () => {
    const typed = view({ kind: 'search', title: '“la”', sub: '2 người khớp', level: { kind: 'dept', title: 'Đối soát', sub: '2 người' }, trail: [seg('v', 'Vận hành thanh toán'), seg('d', 'Đối soát')] })
    expect(phoneBarOf(typed, 'NovaPay')).toEqual({ title: 'Đối soát', above: [seg('v', 'Vận hành thanh toán')] })
  })

  it('search says the scope a limited caller sees, not the organisation', () => {
    expect(phoneBarOf(view({ kind: 'search', limited: true }), 'NovaPay')).toEqual({ title: 'Tìm trong danh bạ', above: [], text: 'Trong phạm vi bạn được xem' })
  })
})
