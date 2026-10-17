import type { ApprovalRequest } from '../../api/approval'
import { entityLabel, personFor, requestTitle } from '../approval-model'
import type { PeopleDirectory } from '../people'
import { matchesAll } from '../search-text'

/** How many requests Phê duyệt suggests per group at most (DESIGN.md "Gợi ý trong ô tìm"). */
export const RECENT_REQUESTS = 5

/**
 * Whether a request is one the Phê duyệt search box finds: every word of `query` in its title, the name of who
 * sent it or its kind, ignoring case and accents. Only what the open tab has loaded is ever searched.
 */
export function matchesRequest(r: ApprovalRequest, query: string, people: PeopleDirectory): boolean {
  const sender = personFor(people, r.created_by, r.created_by_name).name
  return matchesAll(`${requestTitle(r)} ${sender} ${entityLabel(r.entity_type)}`, query)
}
