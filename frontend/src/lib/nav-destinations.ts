import {
  MessageSquare, Calendar, FolderOpen, Users, Package, ClipboardCheck, Settings, ShieldCheck, Bell,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { parseChatSearch, type ChatSearchUrl } from './chat-search'
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

/**
 * Where the phone's bottom bar is drawn: every list screen. Not on a screen with a header of its own and a
 * way back (`isPhoneDetailPath`, which includes the full screens `/channels/mentions` and `/channels/starred`),
 * not on the settings pages, and not on the administration screens (on a phone they open from Cài đặt
 * workspace and go back there).
 */
export function isModuleBarPath(pathname: string): boolean {
  if (isPhoneDetailPath(pathname)) return false
  return !/^\/(settings|admin)(\/|$)/.test(pathname)
}

/** A module the bottom bar can hold, as the `mobile.bar` setting names it. */
export const BAR_MODULE_IDS = ['chat', 'calendar', 'drive', 'approval', 'assets', 'contacts', 'notifications'] as const
export type BarModuleId = (typeof BAR_MODULE_IDS)[number]

export const BAR_SLOTS = 4

export interface BarModule {
  id: BarModuleId
  icon: LucideIcon
  /** The name under the icon, on the phone. */
  label: string
  /** Where it goes; Thông báo opens a screen instead. */
  to?: string
  /** The module the path belongs to, to mark the slot you are in. */
  module?: ModuleId
}

const BAR_ID_OF: Partial<Record<ModuleId, BarModuleId>> = {
  messaging: 'chat', calendar: 'calendar', drive: 'drive', approval: 'approval', assets: 'assets', contacts: 'contacts',
}

/** What the bar can hold in this kind of workspace, in menu order: the main destinations, then Thông báo. */
export function barModules(isPersonal: boolean): BarModule[] {
  const modules = mainDestinations(isPersonal).map((d) => ({
    id: BAR_ID_OF[d.id]!, icon: d.icon, label: d.phoneLabel ?? d.label, to: d.to, module: d.id,
  }))
  return [...modules, { id: 'notifications' as const, icon: Bell, label: 'Thông báo' }]
}

/** Chat · Lịch · Tài liệu, then Phê duyệt, or Danh bạ where there is nothing to approve. */
export function defaultBar(isPersonal: boolean): BarModuleId[] {
  return ['chat', 'calendar', 'drive', isPersonal ? 'contacts' : 'approval']
}

/**
 * The four slots to draw: the stored choice when it names four different modules this kind of workspace
 * offers, the defaults otherwise (unset, damaged, or naming one the workspace does not have).
 */
export function resolveBar(stored: unknown, isPersonal: boolean): BarModuleId[] {
  const allowed = barModules(isPersonal).map((m) => m.id)
  if (
    Array.isArray(stored) && stored.length === BAR_SLOTS && new Set(stored).size === BAR_SLOTS &&
    stored.every((id): id is BarModuleId => allowed.includes(id as BarModuleId))
  ) return stored
  return defaultBar(isPersonal)
}

/**
 * A screen that carries its own header with a way back on a phone (one
 * conversation, one document, the calendar editor), so the search bar steps aside for it.
 */
export function isPhoneDetailPath(pathname: string): boolean {
  return isConversationPath(pathname) || /^\/documents\/[^/]+\/?$/.test(pathname) || /^\/calendar\/(new|event\/[^/]+)\/?$/.test(pathname)
}

/** The chips of the phone Chat list, in order (Tất cả · Mục · Chưa đọc · Được nhắc · Trực tiếp · Nhóm); the desktop ignores them. */
export const CHAT_TABS = ['all', 'sections', 'unread', 'mentions', 'direct', 'spaces'] as const
export type ChatTab = (typeof CHAT_TABS)[number]

export interface ChannelsSearch extends ChatSearchUrl {
  ws?: string
  /** Absent means the chip the device remembers (Tất cả on a new one). */
  tab?: ChatTab
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
  if ((CHAT_TABS as readonly unknown[]).includes(search.tab)) out.tab = search.tab as ChatTab
  // Thêm, the old fourth tab, held the mentions first.
  else if (search.tab === 'more') out.tab = 'mentions'
  if (search.search === 1 || search.search === '1' || search.search === true) out.search = 1
  // The words and filters of the search screen (a phone's results live in this address).
  if (out.search) Object.assign(out, parseChatSearch(search))
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

/** The chip to show: the one the address names if it exists, else `remembered` (the one last chosen on this device). */
export const chatTabOf = (s: { tab?: unknown }, remembered: ChatTab = 'all'): ChatTab => {
  if (s.tab === 'more') return 'mentions'
  return (CHAT_TABS as readonly unknown[]).includes(s.tab) ? (s.tab as ChatTab) : remembered
}
