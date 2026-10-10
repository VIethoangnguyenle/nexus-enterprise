import { describe, expect, it } from 'vitest'
import type { DriveItem, DriveShare } from '../../api/drive'
import { buildDirectory } from '../../lib/people'
import { CONTACTS, U } from '../../test/chat-fixtures'
import {
  kindOf, matchesName, ownerOf, shareTargetName, sharePermissionLabel, sortItems,
} from './drive-model'

const people = buildDirectory(CONTACTS)
const base: DriveItem = {
  id: 'i', workspace_id: 'w', drive_context: 'workspace', item_type: 'file', name: 'a.txt',
  ngac_node_id: 'n', owner_id: U.duc.id, status: 'active', created_at: '', updated_at: '',
}
const file = (name: string, extra: Partial<DriveItem> = {}): DriveItem => ({ ...base, name, ...extra })
const folder = (name: string): DriveItem => ({ ...base, name, item_type: 'folder' })

describe('ownerOf', () => {
  it('finds a file owner by user id and a folder owner by node id', () => {
    expect(ownerOf(file('x.pdf'), people).name).toBe('Trần Minh Đức')
    expect(ownerOf(folder('F', ), people).name).toBe('Trần Minh Đức')
    expect(ownerOf({ ...folder('F'), owner_id: U.lan.node }, people)).toMatchObject({ name: 'Nguyễn Thu Lan', hueKey: U.lan.id })
  })

  it('prefers the name the server sent, which covers owners outside the workspace', () => {
    expect(ownerOf(file('x', { owner_id: 'stranger', owner_name: 'Phạm Văn Ngoài' }), people).name).toBe('Phạm Văn Ngoài')
  })

  it('never falls back to an id', () => {
    const o = ownerOf(file('x', { owner_id: 'system' }), people)
    expect(o.name).toBe('Thành viên')
  })
})

describe('shareTargetName', () => {
  const share = (extra: Partial<DriveShare>): DriveShare => ({
    id: 's', drive_item_id: 'i', share_type: 'user', target_ngac_id: 'nope', target_label: '', operations: ['read'], created_at: '', ...extra,
  })

  it('names a person from the directory, ignoring the server label', () => {
    expect(shareTargetName(share({ target_ngac_id: U.lan.node, target_label: `${U.lan.node}_Owners` }), people)).toBe('Nguyễn Thu Lan')
  })

  it('strips ids out of a server label and keeps what is readable', () => {
    expect(shareTargetName(share({ share_type: 'role', target_label: `${U.lan.node}_Owners` }), people)).toBe('Owners')
    expect(shareTargetName(share({ share_type: 'workspace', target_label: 'Khối Vận hành (workspace)' }), people)).toBe('Khối Vận hành')
  })

  it('uses a neutral word when only an id is left', () => {
    expect(shareTargetName(share({ target_label: U.lan.node }), people)).toBe('Thành viên')
    expect(shareTargetName(share({ share_type: 'role', target_label: U.lan.node }), people)).toBe('Nhóm người dùng')
  })

  it('names a public link', () => {
    expect(shareTargetName(share({ share_type: 'public', target_label: 'Anyone with link' }), people)).toBe('Bất kỳ ai có liên kết')
  })
})

describe('listing helpers', () => {
  it('puts folders first, then sorts by name the Vietnamese way with numbers in order', () => {
    const sorted = sortItems([file('b10.pdf'), file('b2.pdf'), folder('Đối soát'), file('Ăn.pdf'), folder('Báo cáo')])
    expect(sorted.map((i) => i.name)).toEqual(['Báo cáo', 'Đối soát', 'Ăn.pdf', 'b2.pdf', 'b10.pdf'])
  })

  it('matches names without regard to case or accents', () => {
    expect(matchesName(folder('Hợp đồng'), 'hop DONG')).toBe(true)
    expect(matchesName(folder('Hợp đồng'), 'báo')).toBe(false)
    expect(matchesName(folder('x'), '  ')).toBe(true)
  })

  it('labels kinds and permissions in words', () => {
    expect(kindOf(folder('F')).label).toBe('Thư mục')
    expect(kindOf(file('a.xlsx')).label).toBe('Bảng tính')
    expect(sharePermissionLabel(['write'])).toBe('Có thể sửa')
    expect(sharePermissionLabel(['read'])).toBe('Có thể xem')
    expect(sharePermissionLabel(undefined)).toBe('Có thể xem')
  })
})
