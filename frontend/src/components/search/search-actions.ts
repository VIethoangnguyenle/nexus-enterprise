import { CalendarPlus, ClipboardPlus, MessageSquare, Repeat, Settings, type LucideIcon } from 'lucide-react'
import { fold } from '../../lib/search-text'
import type { SettingsSection } from '../settings/settings-nav'
import { SCOPE_LABEL, type SearchScope } from './search-scopes'

/** What choosing an action does; the search screen carries it out. */
export type ActionKind = 'create-request' | 'new-event' | 'message' | 'message-person' | 'settings' | 'switch-workspace'

export interface SearchAction {
  /** Unique among the actions shown together. */
  id: string
  kind: ActionKind
  label: string
  icon: LucideIcon
  /** Words that go with it: the title of the new event, the request's subject. */
  subject?: string
  /** `message-person`: who. A user id, never shown. */
  userId?: string
  /** `settings`: the module's settings page; none opens Cài đặt itself. */
  section?: SettingsSection
}

export interface ActionContext {
  isPersonal: boolean
  /** The caller has a template to make a request from (only in an organization). */
  canCreateRequest: boolean
  /** The scope the search was opened in: its own create action and settings come first. */
  home: SearchScope
}

/** The module whose settings an action opens; Tất cả has the general ones. */
const settingsLabel = (scope: SearchScope) => (scope === 'all' || scope === 'contacts' ? 'Cài đặt' : `Cài đặt ${SCOPE_LABEL[scope]}`)

const createRequest = (subject?: string): SearchAction => ({
  id: 'create-request', kind: 'create-request', icon: ClipboardPlus, subject,
  label: subject ? `Tạo đề nghị “${subject}”` : 'Tạo đề nghị',
})
const newEvent = (subject?: string): SearchAction => ({
  id: 'new-event', kind: 'new-event', icon: CalendarPlus, subject,
  label: subject ? `Sự kiện mới “${subject}”` : 'Sự kiện mới',
})
const message = (): SearchAction => ({ id: 'message', kind: 'message', icon: MessageSquare, label: 'Nhắn tin' })
/** The settings page of each module scope; Tất cả and Người have none of their own. */
const SECTION_OF: Partial<Record<SearchScope, SettingsSection>> = {
  messaging: 'chat', calendar: 'calendar', drive: 'drive', approval: 'approval', assets: 'assets',
}
const settings = (scope: SearchScope): SearchAction => ({
  id: `settings-${scope}`, kind: 'settings', icon: Settings, label: settingsLabel(scope), section: SECTION_OF[scope],
})
const switchWorkspace = (): SearchAction => ({ id: 'switch-workspace', kind: 'switch-workspace', icon: Repeat, label: 'Đổi workspace' })

/** "Làm nhanh" under an empty box: four tiles, by the module it was opened in and what the caller may do. */
export function quickActions(ctx: ActionContext): SearchAction[] {
  const out: SearchAction[] = []
  if (!ctx.isPersonal && ctx.canCreateRequest) out.push(createRequest())
  out.push(newEvent(), message(), settings(ctx.home))
  return out
}

/** The command words of an action, folded, for matching typed words against. */
interface Command {
  words: string
  /** Builds the action; `rest` is what was typed after the command words. */
  make: (rest: string) => SearchAction
  /** A command that takes words after it (Tạo đề nghị "…"); the others only match as a whole. */
  takesSubject: boolean
}

function commands(ctx: ActionContext): Command[] {
  const list: Command[] = []
  if (!ctx.isPersonal && ctx.canCreateRequest) {
    list.push({ words: 'tao de nghi', make: (rest) => createRequest(rest || undefined), takesSubject: true })
  }
  list.push({ words: 'su kien moi', make: (rest) => newEvent(rest || undefined), takesSubject: true })
  list.push({ words: 'nhan tin', make: () => message(), takesSubject: false })
  const scopes: SearchScope[] = ['all', 'messaging', 'calendar', 'drive', ...(ctx.isPersonal ? [] : ['approval', 'assets'] as SearchScope[])]
  for (const s of scopes) list.push({ words: fold(settingsLabel(s)), make: () => settings(s), takesSubject: false })
  list.push({ words: 'doi workspace', make: () => switchWorkspace(), takesSubject: false })
  return list
}

const words = (s: string) => fold(s).split(/\s+/).filter(Boolean)

/** Every typed word starts a word of the command, in any order ("cai dat lich", "tao de"). */
function startsCommand(typed: string[], command: string[]): boolean {
  return typed.length > 0 && typed.every((t) => command.some((c) => c.startsWith(t)))
}

/**
 * The actions typed words call for (DESIGN.md §6, Hành động): a command whose words they start ("tạo đề" → Tạo
 * đề nghị), a create command followed by its subject ("sự kiện mới họp quý" → Sự kiện mới "họp quý"), the
 * create action of the module the search was opened in with the words as its subject, and "Nhắn tin cho" the
 * first person the words name. Nothing the workspace kind or the caller's rights rule out.
 */
export function typedActions(query: string, ctx: ActionContext, person?: { userId: string; name: string }): SearchAction[] {
  const typed = words(query)
  if (typed.length === 0) return []
  const shown = new Map<string, SearchAction>()
  const add = (a: SearchAction) => { if (!shown.has(a.id)) shown.set(a.id, a) }
  const raw = query.trim().split(/\s+/)

  for (const c of commands(ctx)) {
    const cw = c.words.split(' ')
    const head = typed.slice(0, cw.length)
    if (c.takesSubject && typed.length > cw.length && head.join(' ') === c.words) {
      add(c.make(raw.slice(cw.length).join(' ')))
    } else if (startsCommand(typed, cw)) {
      add(c.make(''))
    }
  }
  // The module's own create action takes the words as its subject.
  if (ctx.home === 'approval' && !ctx.isPersonal && ctx.canCreateRequest) add(createRequest(query.trim()))
  if (ctx.home === 'calendar') add(newEvent(query.trim()))
  if (person) add({ id: `message-${person.userId}`, kind: 'message-person', icon: MessageSquare, label: `Nhắn tin cho ${person.name}`, userId: person.userId })
  return [...shown.values()]
}
