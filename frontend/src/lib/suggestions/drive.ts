import type { DriveItem } from '../../api/drive'
import { toMillis } from '../format'

/** How many files Tài liệu suggests per group at most (DESIGN.md "Gợi ý trong ô tìm"). */
export const RECENT_FILES = 5

/**
 * The files (never folders) of a listing, latest edit first: "Sửa gần đây trong <thư mục>" and "Được chia sẻ với
 * tôi". The listing is what the server returned for the caller, so a file they can no longer read is never in it.
 */
export function recentlyEdited(items: DriveItem[], limit = RECENT_FILES): DriveItem[] {
  return items
    .filter((i) => i.item_type === 'file')
    .sort((a, b) => toMillis(b.updated_at ?? b.created_at) - toMillis(a.updated_at ?? a.created_at))
    .slice(0, limit)
}
