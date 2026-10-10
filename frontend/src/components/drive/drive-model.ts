import { Folder, type LucideIcon } from 'lucide-react'
import type { DriveItem, DriveShare } from '../../api/drive'
import { fileKind } from '../../lib/file-kind'
import { normalize, UNKNOWN_PERSON, type PeopleDirectory } from '../../lib/people'
import { toMillis } from '../../lib/format'

/** Case- and accent-insensitive name match ("hop dong" finds "Hợp đồng"). */
export function matchesName(item: DriveItem, query: string): boolean {
  const q = normalize(query)
  return !q || normalize(item.name).includes(q)
}

/** Folders first, then files, each by name the way Vietnamese is collated. */
export function sortItems(items: DriveItem[]): DriveItem[] {
  return [...items].sort((a, b) => {
    if (a.item_type !== b.item_type) return a.item_type === 'folder' ? -1 : 1
    return a.name.localeCompare(b.name, 'vi', { numeric: true, sensitivity: 'base' })
  })
}

export interface Owner {
  name: string
  /** Stable key for the person's colour. A user id when known; hashed, never shown. */
  hueKey: string
  avatarUrl?: string
}

/**
 * Who owns an item, as a person. `owner_id` is a user id on files and an NGAC
 * node id on folders, so the directory is asked both ways; the server's
 * `owner_name` covers owners outside the workspace. An id is never shown.
 */
export function ownerOf(item: DriveItem, people: PeopleDirectory): Owner {
  const person = people.byUserId.get(item.owner_id) ?? people.byNodeId.get(item.owner_id)
  return {
    name: item.owner_name || person?.name || UNKNOWN_PERSON,
    hueKey: person?.userId || item.owner_id || item.id,
    avatarUrl: person?.avatarUrl || undefined,
  }
}

export interface DriveKind {
  label: string
  icon: LucideIcon
  /** Utility classes for the icon tile. */
  tile: string
}

export function kindOf(item: Pick<DriveItem, 'item_type' | 'name' | 'mime_type'>): DriveKind {
  if (item.item_type === 'folder') {
    return { label: 'Thư mục', icon: Folder, tile: 'bg-accent-wash text-accent' }
  }
  return fileKind(item.name, item.mime_type)
}

/** The permission a share grants, in words. `write` is the only operation that edits. */
export function sharePermissionLabel(operations: string[] | undefined): string {
  return operations?.includes('write') ? 'Có thể sửa' : 'Có thể xem'
}

const UUID = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi

/**
 * Who a share is with. A person is looked up by the NGAC node the share
 * targets. Otherwise the server's label is used, with any id stripped out
 * ("<uuid>_Owners" becomes "Owners"); if nothing readable remains, a neutral
 * word stands in. An id is never shown.
 */
export function shareTargetName(share: DriveShare, people: PeopleDirectory): string {
  if (share.share_type === 'public') return 'Bất kỳ ai có liên kết'
  const person = people.byNodeId.get(share.target_ngac_id)
  if (person) return person.name
  const label = (share.target_label ?? '')
    .replace(UUID, '')
    .replace(/\(workspace\)/i, '')
    .replace(/^[\s_\-:]+|[\s_\-:]+$/g, '')
    .trim()
  if (label) return label
  return share.share_type === 'user' ? UNKNOWN_PERSON : 'Nhóm người dùng'
}

/** One token for "this version of this item": a change to either name or time is a new arrival. */
export function itemVersion(item: DriveItem): string {
  return `${item.id}@${item.name}@${toMillis(item.updated_at)}`
}

/** What a share allows, as the value the API takes. */
export function sharePermissionOf(operations: string[] | undefined): 'read' | 'write' {
  return operations?.includes('write') ? 'write' : 'read'
}
