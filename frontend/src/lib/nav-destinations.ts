import {
  MessageSquare, Calendar, FolderOpen, Users, Package, ClipboardCheck, Settings, ShieldCheck,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { isConversationPath, validateWorkspaceSearch } from './workspace'

export type ModuleId = 'messaging' | 'calendar' | 'documents' | 'drive' | 'assets' | 'contacts' | 'approval' | 'admin' | 'settings'

export interface NavDestination {
  id: ModuleId
  icon: LucideIcon
  /** The name on the desktop menu. */
  label: string
  /** The name where the phone menu uses another word for the same place. */
  phoneLabel?: string
  to: string
}

/** DESIGN.md §5: six main destinations, then Thông báo, Quản trị and Cài đặt at the foot. */
export const mainNavItems: NavDestination[] = [
  { id: 'messaging', icon: MessageSquare, label: 'Tin nhắn', phoneLabel: 'Chat', to: '/channels' },
  { id: 'calendar', icon: Calendar, label: 'Lịch', to: '/calendar' },
  { id: 'drive', icon: FolderOpen, label: 'Tài liệu', to: '/drive' },
  { id: 'approval', icon: ClipboardCheck, label: 'Phê duyệt', to: '/approval' },
  { id: 'assets', icon: Package, label: 'Tài sản', to: '/assets' },
  { id: 'contacts', icon: Users, label: 'Danh bạ', to: '/contacts' },
]
export const footNavItems: NavDestination[] = [
  { id: 'admin', icon: ShieldCheck, label: 'Quản trị', to: '/admin' },
  { id: 'settings', icon: Settings, label: 'Cài đặt', to: '/settings' },
]

/** What a personal workspace does not offer. */
const PERSONAL_HIDDEN: readonly ModuleId[] = ['approval', 'assets']

/** The main destinations this kind of workspace offers, in menu order. */
export function mainDestinations(isPersonal: boolean): NavDestination[] {
  return isPersonal ? mainNavItems.filter((i) => !PERSONAL_HIDDEN.includes(i.id)) : mainNavItems
}

/** The foot destinations this kind of workspace offers; a personal workspace has nothing to administer. */
export function footDestinations(isPersonal: boolean): NavDestination[] {
  return isPersonal ? footNavItems.filter((i) => i.id !== 'admin') : footNavItems
}

/** What the number on a destination counts, for the hidden text read after its name. */
export const countUnit = (id: string) => (id === 'approval' ? 'chờ bạn' : 'chưa đọc')

/** The module a path belongs to; Văn bản is a group inside Tài liệu, so its pages belong to Tài liệu. */
export function moduleOfPath(pathname: string): ModuleId | undefined {
  if (pathname.includes('/admin')) return 'admin'
  if (pathname.includes('/contacts')) return 'contacts'
  if (pathname.includes('/drive')) return 'drive'
  if (pathname.includes('/approval')) return 'approval'
  if (pathname.includes('/channels')) return 'messaging'
  if (pathname.includes('/calendar')) return 'calendar'
  if (pathname.includes('/assets')) return 'assets'
  if (pathname.includes('/settings')) return 'settings'
  if (pathname.includes('/documents')) return 'drive'
  return undefined
}

/** The hint the phone search bar shows for a module (DESIGN.md §5, Điều hướng di động). */
const SEARCH_HINT: Partial<Record<ModuleId, string>> = {
  messaging: 'Tìm trong chat',
  calendar: 'Tìm sự kiện',
  drive: 'Tìm tài liệu',
  documents: 'Tìm tài liệu',
  approval: 'Tìm đề nghị',
  assets: 'Tìm tài sản',
  contacts: 'Tìm người',
  settings: 'Tìm cài đặt',
  admin: 'Tìm cài đặt',
}
export const searchHintOf = (module: ModuleId | undefined): string =>
  (module && SEARCH_HINT[module]) || 'Tìm kiếm'

/** The module's name on the phone, for the middle of the search bar when its screen has no search of its own. */
export function phoneNameOf(module: ModuleId | undefined): string {
  const all = [...mainNavItems, ...footNavItems]
  const item = all.find((i) => i.id === module)
  return item?.phoneLabel ?? item?.label ?? 'Nexus'
}

/** The list of conversations (`/channels`), where the Chat bottom bar lives. */
export const isChatListPath = (pathname: string) => /^\/channels\/?$/.test(pathname)

/**
 * A screen that carries its own header with a way back on a phone (one
 * conversation, one document, the calendar editor), so the search bar steps aside for it.
 */
export function isPhoneDetailPath(pathname: string): boolean {
  return isConversationPath(pathname) || /^\/documents\/[^/]+\/?$/.test(pathname) || /^\/calendar\/(new|event\/[^/]+)\/?$/.test(pathname)
}

/** The tabs of the phone Chat list; the desktop ignores them. */
export const CHAT_TABS = ['home', 'direct', 'sections', 'more'] as const
export type ChatTab = (typeof CHAT_TABS)[number]

export interface ChannelsSearch {
  ws?: string
  /** Absent means Trang chủ. */
  tab?: Exclude<ChatTab, 'home'>
  /** The Tìm trong chat screen is open. */
  search?: 1
  /** The Thread screen is open, on Có tin mới (`new`) or Đang theo dõi (`followed`). */
  threads?: 'new' | 'followed'
}

/** A full-screen layer over the Chat list; each owns a history entry so the system back gesture closes it. */
export type ChatOverlay = { kind: 'search' } | { kind: 'threads'; chip: 'unread' | 'following' }

/**
 * `validateSearch` for `/channels`: an unknown tab opens Trang chủ rather than an error page, and an
 * unknown layer is dropped. The desktop has no such layers and ignores both.
 */
export function validateChannelsSearch(search: Record<string, unknown>): ChannelsSearch {
  const out: ChannelsSearch = { ...validateWorkspaceSearch(search) }
  if (search.tab === 'direct' || search.tab === 'sections' || search.tab === 'more') out.tab = search.tab
  if (search.search === 1 || search.search === '1' || search.search === true) out.search = 1
  if (search.threads === 'new' || search.threads === 'followed') out.threads = search.threads
  return out
}

/** The layer to show over the Chat list; the search screen wins if both are asked for. */
export function chatOverlayOf(s: { search?: unknown; threads?: unknown }): ChatOverlay | null {
  if (s.search === 1) return { kind: 'search' }
  if (s.threads === 'new') return { kind: 'threads', chip: 'unread' }
  if (s.threads === 'followed') return { kind: 'threads', chip: 'following' }
  return null
}

/** The tab to show; Trang chủ unless the address names another that exists. */
export const chatTabOf = (s: { tab?: unknown }): ChatTab =>
  (CHAT_TABS as readonly unknown[]).includes(s.tab) ? (s.tab as ChatTab) : 'home'
