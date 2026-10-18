import type { ModuleId } from '../../lib/nav-destinations'
import type { RecentScope } from '../../hooks/useRecentSearches'

/**
 * Where one search looks (DESIGN.md §6 "Tìm cho mọi tác vụ"): one module, or Tất cả. Chat is the existing Tìm
 * trong chat screen, with its own filters; choosing it hands the words over to that screen.
 */
export type SearchScope = 'all' | 'messaging' | 'drive' | 'contacts' | 'calendar' | 'approval' | 'assets'

/** The modules after Tất cả, in the order of the mockup. */
const MODULE_SCOPES: readonly Exclude<SearchScope, 'all'>[] = ['messaging', 'drive', 'contacts', 'calendar', 'approval', 'assets']

/** What a personal workspace does not have. */
const ORGANIZATION_ONLY: readonly SearchScope[] = ['approval', 'assets']

export const SCOPE_LABEL: Record<SearchScope, string> = {
  all: 'Tất cả',
  messaging: 'Chat',
  drive: 'Tài liệu',
  contacts: 'Người',
  calendar: 'Lịch',
  approval: 'Phê duyệt',
  assets: 'Tài sản',
}

/** The scope a module opens the search in; a page that is no module of its own (Cài đặt) searches everywhere. */
export function scopeOfModule(module: ModuleId | undefined): SearchScope {
  switch (module) {
    case 'messaging': case 'drive': case 'contacts': case 'calendar': case 'approval': case 'assets':
      return module
    case 'documents':
      return 'drive'
    default:
      return 'all'
  }
}

export const isScopeOffered = (scope: SearchScope, isPersonal: boolean) => !isPersonal || !ORGANIZATION_ONLY.includes(scope)

/** The scope chips: the one the search was opened in, then Tất cả, then every other module this workspace has. */
export function scopesFor(first: SearchScope, isPersonal: boolean): SearchScope[] {
  const start = isScopeOffered(first, isPersonal) ? first : 'all'
  const rest = MODULE_SCOPES.filter((s) => s !== start && isScopeOffered(s, isPersonal))
  return start === 'all' ? ['all', ...rest] : [start, 'all', ...rest]
}

/** Where a scope keeps its Tìm gần đây; Chat's list is the one Tìm trong chat already keeps. */
export const recentScopeOf = (scope: SearchScope): RecentScope => (scope === 'messaging' ? 'chat' : scope)

/** The box's placeholder and accessible name. */
export const boxLabelOf = (scope: SearchScope) => (scope === 'all' ? 'Tìm ở mọi nơi' : `Tìm trong ${SCOPE_LABEL[scope]}`)

/** The kinds a Tất cả search groups its results by, in display order. */
export type ResultGroup = 'actions' | 'people' | 'conversations' | 'messages' | 'documents' | 'approvals' | 'events' | 'assets'

export const RESULT_GROUPS: readonly ResultGroup[] = ['actions', 'people', 'conversations', 'messages', 'documents', 'approvals', 'events', 'assets']

export const GROUP_LABEL: Record<ResultGroup, string> = {
  actions: 'Hành động',
  people: 'Người',
  conversations: 'Trò chuyện',
  messages: 'Tin nhắn',
  documents: 'Tài liệu',
  approvals: 'Đề nghị',
  events: 'Sự kiện',
  assets: 'Tài sản',
}

/** The scope that "Xem tất cả" of a group moves to; actions have none. */
export const SCOPE_OF_GROUP: Record<ResultGroup, SearchScope | null> = {
  actions: null,
  people: 'contacts',
  conversations: 'messaging',
  messages: 'messaging',
  documents: 'drive',
  approvals: 'approval',
  events: 'calendar',
  assets: 'assets',
}

/** The groups a scope shows; Tất cả shows every one this workspace has. */
export function groupsOf(scope: SearchScope, isPersonal: boolean): ResultGroup[] {
  return RESULT_GROUPS.filter((g) => {
    const owner = SCOPE_OF_GROUP[g]
    if (owner && !isScopeOffered(owner, isPersonal)) return false
    return scope === 'all' || owner === scope
  })
}
