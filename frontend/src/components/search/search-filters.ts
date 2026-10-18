import type { ApprovalStatus } from '../../api/search'
import type { DriveItem } from '../../api/drive'
import type { Occurrence } from '../../api/calendar'
import { fileKind } from '../../lib/file-kind'
import { toDate } from '../../lib/format'

/**
 * The filter chips of each scope (DESIGN.md §6 "Tìm cho mọi tác vụ"). Every value is an id from a picker or a
 * fixed word; none is typed by hand and none reaches the screen except through its name.
 */

/** A span of days counted back from today (Tài liệu, Phê duyệt). */
export type PastRange = 'today' | '7d' | '30d'
/** A span of days counted on from today (Lịch). */
export type NextRange = 'today' | 'next7' | 'next30'

export interface DriveFilters {
  /** `folder`, or the label of a file kind ("Bảng tính"). */
  kind?: string
  /** The owner's id, as the drive gives it; shown by `owner_name`. */
  editor?: string
  date?: PastRange
  /** The folder holding the item; `''` is the top of the drive. */
  folder?: string
}

export interface ApprovalFilters {
  status?: ApprovalStatus
  /** A template id: the request's Loại. */
  template?: string
  /** The requester's user id. */
  sender?: string
  date?: PastRange
}

export interface CalendarFilters {
  calendar?: string
  range?: NextRange
}

export interface AssetFilters {
  type?: string
  state?: string
  /** The holder's user id. */
  holder?: string
}

export interface SearchFilters {
  drive: DriveFilters
  approval: ApprovalFilters
  calendar: CalendarFilters
  assets: AssetFilters
}

export const NO_FILTERS: SearchFilters = { drive: {}, approval: {}, calendar: {}, assets: {} }

export const PAST_RANGES: { id: PastRange; label: string }[] = [
  { id: 'today', label: 'Hôm nay' }, { id: '7d', label: '7 ngày qua' }, { id: '30d', label: '30 ngày qua' },
]
export const NEXT_RANGES: { id: NextRange; label: string }[] = [
  { id: 'today', label: 'Hôm nay' }, { id: 'next7', label: '7 ngày tới' }, { id: 'next30', label: '30 ngày tới' },
]
export const APPROVAL_STATUSES: { id: ApprovalStatus; label: string }[] = [
  { id: 'pending', label: 'Chờ duyệt' }, { id: 'approved', label: 'Đã duyệt' },
  { id: 'rejected', label: 'Từ chối' }, { id: 'cancelled', label: 'Đã huỷ' },
]
export const statusLabel = (s: string) => APPROVAL_STATUSES.find((x) => x.id === s)?.label ?? 'Khác'

/** The kinds the Loại chip of Tài liệu offers: folders, then the file kinds of `lib/file-kind`. */
export const DRIVE_KINDS = ['Thư mục', 'Văn bản', 'Bảng tính', 'PDF', 'Trình chiếu', 'Ảnh', 'Video', 'Tệp'] as const
const FOLDER_KIND = 'Thư mục'

export const driveKindOf = (item: Pick<DriveItem, 'item_type' | 'name' | 'mime_type'>) =>
  item.item_type === 'folder' ? FOLDER_KIND : fileKind(item.name, item.mime_type).label

const DAY = 86_400_000

function startOfDay(d: Date): number {
  const s = new Date(d)
  s.setHours(0, 0, 0, 0)
  return s.getTime()
}

/** The first moment a past span covers. */
export function pastStart(range: PastRange, now: Date): number {
  const today = startOfDay(now)
  return range === 'today' ? today : today - (range === '7d' ? 6 : 29) * DAY
}

/** `YYYY-MM-DD` in the reader's zone, for a server that takes days. */
export function dayKey(ms: number): string {
  const d = new Date(ms)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

/** The drive items that pass the chips of Tài liệu. */
export function filterDrive(items: DriveItem[], f: DriveFilters, now: Date = new Date()): DriveItem[] {
  const since = f.date ? pastStart(f.date, now) : null
  return items.filter((it) => {
    if (f.kind && driveKindOf(it) !== f.kind) return false
    if (f.editor && it.owner_id !== f.editor) return false
    if (f.folder !== undefined && (it.parent_id ?? '') !== f.folder) return false
    if (since !== null) {
      const at = toDate(it.updated_at)?.getTime() ?? 0
      if (at < since) return false
    }
    return true
  })
}

/** The events that pass the chips of Lịch. */
export function filterEvents(list: Occurrence[], f: CalendarFilters, now: Date = new Date()): Occurrence[] {
  const from = startOfDay(now)
  const days = f.range === 'today' ? 1 : f.range === 'next7' ? 7 : f.range === 'next30' ? 30 : 0
  return list.filter((o) => {
    if (f.calendar && o.calendarId !== f.calendar) return false
    if (days) {
      const start = Date.parse(o.start)
      const end = Date.parse(o.end)
      // Any part of the event inside the span counts.
      if (!(end > from && start < from + days * DAY)) return false
    }
    return true
  })
}

/** The approval filters as the server takes them. */
export function approvalQuery(f: ApprovalFilters, now: Date = new Date()) {
  return {
    status: f.status,
    template_id: f.template,
    created_by: f.sender,
    from: f.date ? dayKey(pastStart(f.date, now)) : undefined,
    to: f.date ? dayKey(now.getTime()) : undefined,
  }
}

/** How many chips of a scope are set. */
export const countSet = (f: object) => Object.values(f).filter((v) => v !== undefined).length
