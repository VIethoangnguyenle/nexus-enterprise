import type { ActivityEntry } from '../../api/assets'
import { stepSegments } from '../asset-model'
import { toMillis } from '../format'
import { UNKNOWN_PERSON } from '../people'

/** How many assets Tài sản suggests per group at most (DESIGN.md "Gợi ý trong ô tìm"). */
export const RECENT_ASSETS = 5

/**
 * "Hoạt động gần đây": the latest step of each asset, newest first. A decision on a request is not an asset and is
 * left out. The feed is what the server returned for the caller, so an asset they can no longer see is not in it.
 */
export function recentAssetSteps(entries: ActivityEntry[], limit = RECENT_ASSETS): ActivityEntry[] {
  const seen = new Set<string>()
  const out: ActivityEntry[] = []
  for (const e of [...entries].sort((a, b) => toMillis(b.created_at) - toMillis(a.created_at))) {
    if (!e.asset_id || e.request_status || seen.has(e.asset_id)) continue
    seen.add(e.asset_id)
    out.push(e)
    if (out.length === limit) break
  }
  return out
}

/**
 * The step in one line, by name: "Trần Minh Đức đã giao cho Nguyễn Thu Lan". The asset is the row's title. `actor` is
 * the name the screen resolved (server name, then the directory), so the line and the avatar beside it agree.
 */
export function stepText(e: ActivityEntry, actor = e.actor_name?.trim() || UNKNOWN_PERSON): string {
  const what = stepSegments(e)
    .map((s) => (typeof s === 'string' ? s : 'b' in s ? s.b : s.pill.label))
    .join('')
  return `${actor} ${what}`
}
