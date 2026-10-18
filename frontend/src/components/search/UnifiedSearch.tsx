import { useMemo, useRef, useState, type FormEvent } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { ArrowLeft, Search, X } from 'lucide-react'
import type { Asset } from '../../api/assets'
import type { ApprovalRequest } from '../../api/approval'
import type { Occurrence } from '../../api/calendar'
import type { DriveItem } from '../../api/drive'
import type { Message } from '../../api/messaging'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { approvalPendingOptions, approvalTemplatesOptions } from '../../hooks/useApproval'
import { assetsQueryOptions } from '../../hooks/useAssets'
import { useSharedWithMe } from '../../hooks/useDrive'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useOpenPerson } from '../../hooks/useOpenPerson'
import { usePeople } from '../../hooks/usePeople'
import { useRecentDirects } from '../../hooks/useRecentDirects'
import { useRecentSearches } from '../../hooks/useRecentSearches'
import { feedSearch } from '../../hooks/useShortcutScreen'
import { useSuggestionPanel, type SuggestionGroup, type SuggestionItem } from '../../hooks/useSuggestionPanel'
import { useUnifiedSearch, type GroupResult } from '../../hooks/useUnifiedSearch'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { spanOf, toDateKey } from '../../lib/calendar-format'
import type { Conversation } from '../../lib/conversations'
import { DURATION, EASE, useMotionPresets } from '../../lib/motion'
import type { Person } from '../../lib/people'
import { useAuthStore } from '../../stores/auth.store'
import { IconButton, Pressable, ScrollFade, SearchField } from '../primitives'
import { Marked } from '../patterns/Marked'
import { useSettingsNav } from '../settings/settings-nav'
import { SearchSuggestions, type GroupMore } from '../patterns/SearchSuggestions'
import { quickActions, typedActions, type ActionContext, type SearchAction } from './search-actions'
import { NO_FILTERS, type SearchFilters } from './search-filters'
import {
  actionRow, approvalRow, assetRow, conversationRow, documentRow, eventRow, messageRow, personRow, type RowContext,
} from './search-rows'
import {
  GROUP_LABEL, SCOPE_LABEL, SCOPE_OF_GROUP, boxLabelOf, groupsOf, isScopeOffered, recentScopeOf, scopesFor,
  type ResultGroup, type SearchScope,
} from './search-scopes'
import { SearchFilterChips } from './SearchFilterChips'

/** Rows a group of Tất cả shows; "Xem tất cả" moves to its scope for the rest. */
const GROUP_ROWS = 3
/** Rows of a "Gợi ý" group under an empty box. */
const HINT_ROWS = 3
/** Skeleton rows while a module's own list is on its way. */
const SKELETON_ROWS = 4

interface Props {
  /** The scope of the module it was opened from; the first chip and the default. */
  home: SearchScope
  onClose: () => void
  /** "Đổi workspace": the shell closes this and opens its workspace sheet. */
  onSwitchWorkspace: () => void
  /** "Tạo đề nghị": the shell closes this and opens the request form. */
  onCreateRequest: () => void
}

/**
 * The one search of every module on a phone (DESIGN.md §6 "Tìm cho mọi tác vụ", mockup `mobile-shell.html` §6): a
 * full-screen layer over the module, its pill where the shell's pill is. Chips choose the scope — the module it
 * was opened from, Tất cả, the other modules (a personal workspace has no Phê duyệt or Tài sản) — and keep the
 * words when they change; then the filter chips of that module. An empty box offers Tìm gần đây of the scope,
 * Gợi ý and Làm nhanh; typed words in Tất cả give one group per kind, each as it arrives, at most three rows and
 * "Xem tất cả · N"; in a module, its own list and a way to search everywhere. Chat is the existing Tìm trong chat
 * screen: choosing it hands the words over. Everything shown was answered by the service that owns it, after its
 * read check; no id ever reaches the screen. A modal layer: focus goes to the box, Esc and ← close.
 */
export function UnifiedSearch({ home, onClose, onSwitchWorkspace, onCreateRequest }: Props) {
  const m = useMotionPresets()
  const navigate = useNavigate()
  const settingsNav = useSettingsNav()
  const { workspaceId } = useActiveWorkspace()
  const { isPersonal } = useWorkspaceType()
  const me = useAuthStore((s) => s.user?.id ?? '')
  const surfaceRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  useModalFocus({ active: true, surfaceRef, onClose, initialFocusRef: inputRef })

  const [query, setQuery] = useState('')
  const [chosen, setChosen] = useState<SearchScope>(home)
  const [filters, setFilters] = useState<SearchFilters>(NO_FILTERS)
  // The workspace kind can arrive after the layer opens: a scope it rules out falls back to Tất cả.
  const scope: SearchScope = isScopeOffered(chosen, isPersonal) ? chosen : 'all'
  const scopes = scopesFor(home, isPersonal)
  const typed = query.trim()

  const people = usePeople(workspaceId)
  const results = useUnifiedSearch({ query, workspaceId, isPersonal, filters, enabled: true })
  const recent = useRecentSearches(recentScopeOf(scope))
  const person = useOpenPerson(results.conversationList)
  const templates = useQuery({ ...approvalTemplatesOptions(), enabled: !isPersonal })
  const actx: ActionContext = {
    isPersonal,
    canCreateRequest: !isPersonal && (templates.data?.templates?.length ?? 0) > 0,
    home: isScopeOffered(home, isPersonal) ? home : 'all',
  }

  const leave = (go: () => void) => {
    onClose()
    go()
  }
  const conversationsById = useMemo(() => new Map(results.conversationList.map((c) => [c.id, c])), [results.conversationList])
  const ctx: RowContext = {
    term: typed,
    people,
    conversationsById,
    pick: (go) => () => {
      recent.add(typed)
      leave(go)
    },
  }

  // --- Where each result leads ---
  const openPerson = (p: Person) => void navigate({ to: '/contacts', search: { person: p.userId } })
  const openConversation = (c: Conversation) => void navigate({ to: '/channels/$channelId', params: { channelId: c.id } })
  const openMessage = (msg: Message) =>
    void navigate({ to: '/channels/$channelId', params: { channelId: msg.channel_id }, search: feedSearch(msg) })
  const openDocument = (it: DriveItem) => {
    const folder = it.item_type === 'folder' ? it.id : it.parent_id
    void navigate({ to: '/drive', search: folder ? { folder } : {} })
  }
  const openRequest = (r: ApprovalRequest) => void navigate({ to: '/approval', search: { request: r.id } })
  const openEvent = (o: Occurrence) =>
    void navigate({ to: '/calendar', search: { mode: 'day', date: toDateKey(spanOf(o).start) } })
  const openAsset = (a: Asset) => void navigate({ to: '/assets', search: { section: 'list', asset: a.id } })

  /** Chat's own search screen, with the words: its filters and results are the existing ones. */
  const toChat = () => {
    if (typed) recent.add(typed)
    leave(() => void navigate({ to: '/channels', search: { search: 1, ...(typed ? { q: typed } : {}) }, state: { chatOverlay: true } }))
  }

  const runAction = (a: SearchAction) => {
    if (typed) recent.add(typed)
    switch (a.kind) {
      case 'create-request': return leave(onCreateRequest)
      case 'new-event': return leave(() => void navigate({ to: '/calendar/new', search: a.subject ? { title: a.subject } : {} }))
      case 'message': return chooseScope('contacts')
      case 'message-person': {
        const p = a.userId ? people.byUserId.get(a.userId) : undefined
        return p ? leave(() => person.open(p)) : undefined
      }
      case 'settings': {
        const section = a.section
        return leave(() => (section ? settingsNav.open(section) : void navigate({ to: '/settings' })))
      }
      case 'switch-workspace': return leave(onSwitchWorkspace)
    }
  }

  // --- The rows of every kind ---
  const firstPerson = results.people.items[0]
  const rowsOf: Record<ResultGroup, GroupResult<SuggestionItem>> = {
    actions: (() => {
      const items = typedActions(typed, actx, firstPerson ? { userId: firstPerson.userId, name: firstPerson.name } : undefined).map((a) => actionRow(a, runAction))
      return { items, total: items.length, loading: false }
    })(),
    people: mapGroup(results.people, (p) => personRow(p, ctx, openPerson)),
    conversations: mapGroup(results.conversations, (c) => conversationRow(c, ctx, openConversation)),
    messages: mapGroup(results.messages, (msg) => messageRow(msg, ctx, openMessage)),
    documents: mapGroup(results.documents, (it) => documentRow(it, ctx, openDocument)),
    approvals: mapGroup(results.approvals, (r) => approvalRow(r, ctx, openRequest)),
    events: mapGroup(results.events, (o) => eventRow(o, ctx, openEvent)),
    assets: mapGroup(results.assets, (a) => assetRow(a, ctx, openAsset)),
  }

  // A suggestion is not a search: opening one keeps nothing in Tìm gần đây.
  const hintCtx: RowContext = { term: '', people, conversationsById, pick: (go) => () => leave(go) }
  const hints = useHints({ scope, workspaceId, me, isPersonal, enabled: typed === '', ctx: hintCtx, openRequest, openDocument, openAsset, openConversation })
  const groups: SuggestionGroup[] = typed === ''
    ? [recent.group((q) => {
        setQuery(q)
        inputRef.current?.focus()
      }), ...(hints ? [hints] : [])]
    : scope === 'all'
      ? groupsOf('all', isPersonal).map((g) => ({
          label: GROUP_LABEL[g], loading: rowsOf[g].loading, items: rowsOf[g].items.slice(0, GROUP_ROWS),
        }))
      : []
  const panel = useSuggestionPanel({ value: query, groups, variant: 'screen', anchorRef: inputRef, initiallyOpen: true, typing: true })

  const groupOfLabel = (label: string) => (Object.keys(GROUP_LABEL) as ResultGroup[]).find((g) => GROUP_LABEL[g] === label)
  const more = (g: { label: string }): GroupMore | undefined => {
    const key = groupOfLabel(g.label)
    const target = key ? SCOPE_OF_GROUP[key] : null
    if (typed === '' || !key || !target || rowsOf[key].total === 0) return undefined
    const total = rowsOf[key].total
    return { label: total > GROUP_ROWS ? `Xem tất cả · ${total}` : 'Xem tất cả', onSelect: () => chooseScope(target) }
  }

  function chooseScope(next: SearchScope) {
    if (next === 'messaging') return toChat()
    setChosen(next)
    // Choosing a row shut the panel; the new scope opens it again, and the box keeps its words and focus.
    panel.inputProps.onFocus()
    inputRef.current?.focus()
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (!typed) return
    recent.add(typed)
    // The keyboard goes away; the results are already under the box.
    inputRef.current?.blur()
  }

  const fade = m.reduced ? DURATION.reducedFade : DURATION.quick
  const label = boxLabelOf(scope)
  const own = scope === 'all' ? [] : groupsOf(scope, isPersonal)
  const elsewhere = groupsOf('all', isPersonal)
    .filter((g) => g !== 'actions' && !own.includes(g))
    .reduce((n, g) => n + rowsOf[g].total, 0)

  return createPortal(
    <div
      ref={surfaceRef}
      role="dialog"
      aria-modal="true"
      aria-label="Tìm kiếm"
      className="fixed inset-x-0 top-0 h-app z-modal flex flex-col min-h-0 bg-base lg:hidden"
    >
      <form role="search" onSubmit={submit} className="relative shrink-0 px-3 pb-1 pt-[calc(env(safe-area-inset-top,0px)+0.75rem)]">
        <SearchField
          pill
          ref={inputRef}
          label={label}
          value={query}
          {...panel.inputProps}
          onChange={(e) => setQuery(e.target.value)}
          enterKeyHint="search"
          leading={
            <motion.span initial={{ opacity: 0, rotate: m.reduced ? 0 : 90 }} animate={{ opacity: 1, rotate: 0 }} transition={{ duration: fade, ease: EASE.out }}>
              <IconButton type="button" aria-label="Đóng tìm kiếm" title="Quay lại" shape="round" size="lg" className="w-11 h-11" onClick={onClose}>
                <ArrowLeft size={20} strokeWidth={1.75} />
              </IconButton>
            </motion.span>
          }
          trailing={query ? (
            <IconButton
              type="button"
              aria-label="Xoá chữ"
              title="Xoá chữ"
              shape="round"
              size="lg"
              className="w-11 h-11"
              onClick={() => {
                setQuery('')
                inputRef.current?.focus()
              }}
            >
              <X size={18} strokeWidth={1.75} />
            </IconButton>
          ) : undefined}
        />
      </form>

      <ScrollFade role="toolbar" aria-label="Phạm vi tìm" className="shrink-0 flex gap-2 overflow-x-auto px-3 pt-2.5 pb-1.5 [scrollbar-width:none]">
        {scopes.map((s) => (
          <Pressable
            key={s}
            aria-pressed={s === scope}
            onClick={() => chooseScope(s)}
            className={`shrink-0 h-9 px-3 rounded-full text-[0.8125rem] font-semibold whitespace-nowrap transition-colors duration-quick
              ${s === scope ? 'bg-ink text-(--color-base)' : 'bg-sunk text-ink active:bg-hover'}`}
          >
            {SCOPE_LABEL[s]}
          </Pressable>
        ))}
        <SearchFilterChips
          scope={scope}
          filters={filters}
          onChange={setFilters}
          workspaceId={workspaceId}
          people={people}
          documents={results.documentsUnfiltered}
          isPersonal={isPersonal}
        />
      </ScrollFade>

      <div className="relative flex-1 min-h-0 overflow-y-auto overscroll-y-contain pb-[env(safe-area-inset-bottom,0px)]">
        {typed === '' || scope === 'all' ? (
          <>
            <SearchSuggestions
              panel={panel}
              label="Kết quả tìm kiếm"
              more={more}
              emptyText={typed ? `Không có gì khớp “${typed}”.` : 'Tìm người, trò chuyện, tài liệu và hơn thế.'}
              emptyHint={typed ? 'Thử từ khác hoặc viết ngắn hơn.' : undefined}
            />
            {typed === '' && <QuickActions actions={quickActions(actx)} onRun={runAction} />}
          </>
        ) : (
          <ScopedList
            rows={own.flatMap((g) => rowsOf[g].items)}
            loading={own.some((g) => rowsOf[g].loading)}
            typed={typed}
            elsewhere={elsewhere}
            onEverywhere={() => chooseScope('all')}
          />
        )}
      </div>
    </div>,
    document.body,
  )
}

function mapGroup<T>(g: GroupResult<T>, row: (t: T) => SuggestionItem): GroupResult<SuggestionItem> {
  return { items: g.items.map(row), total: g.total, loading: g.loading }
}

/** "Làm nhanh": a grid of four tiles under an empty box. */
function QuickActions({ actions, onRun }: { actions: SearchAction[]; onRun: (a: SearchAction) => void }) {
  if (actions.length === 0) return null
  return (
    <section aria-labelledby="unified-search-quick" className="pb-6">
      <h2 id="unified-search-quick" className="m-0 px-4 pt-4 pb-1 text-sm font-semibold text-ink-muted">Làm nhanh</h2>
      <div className="grid grid-cols-4 gap-1 px-2 py-1">
        {actions.map((a) => {
          const Icon = a.icon
          return (
            <Pressable key={a.id} onClick={() => onRun(a)} className="grid justify-items-center gap-1.5 py-2 rounded-lg text-xs font-medium text-ink text-center active:bg-hover">
              <span aria-hidden="true" className="grid place-items-center size-12 rounded-xl bg-sunk">
                <Icon size={22} strokeWidth={1.75} />
              </span>
              {a.label}
            </Pressable>
          )
        })}
      </div>
    </section>
  )
}

/** A module's own results, every row; then a way to search everywhere. */
function ScopedList({ rows, loading, typed, elsewhere, onEverywhere }: {
  rows: SuggestionItem[]
  loading: boolean
  typed: string
  elsewhere: number
  onEverywhere: () => void
}) {
  return (
    <div role="region" aria-label="Kết quả tìm kiếm" aria-busy={loading || undefined} className="grid pb-6">
      <p aria-live="polite" className="sr-only">{loading ? 'Đang tìm…' : rows.length ? `${rows.length} kết quả` : 'Không có kết quả'}</p>
      {rows.length === 0 && loading && Array.from({ length: SKELETON_ROWS }, (_, i) => (
        <div key={i} aria-hidden="true" className="grid grid-cols-[auto_1fr] items-center gap-3 min-h-13 px-4">
          <span className="skeleton size-9 rounded-md" />
          <span className="skeleton h-3.5 w-2/3 rounded-sm" />
        </div>
      ))}
      {rows.length === 0 && !loading && (
        <p className="m-0 px-4 py-8 text-center text-sm text-ink-muted">Không có kết quả nào khớp “{typed}”.</p>
      )}
      {rows.map((item) => (
        <Pressable
          key={item.key}
          aria-label={item.label}
          onClick={item.onSelect}
          className="grid grid-cols-[36px_minmax(0,1fr)] items-center gap-3 min-h-13 px-4 py-1.5 text-left text-ink active:bg-hover"
        >
          <span className="grid place-items-center">{item.icon}</span>
          <RowText item={item} />
        </Pressable>
      ))}
      <Pressable
        onClick={onEverywhere}
        className="flex items-center gap-2.5 mx-4 mt-2 px-3.5 py-3 rounded-xl bg-sunk text-left font-medium text-ink"
      >
        <Search size={18} strokeWidth={1.75} aria-hidden="true" />
        <span className="flex-1 min-w-0">{elsewhere > 0 ? `Có ${elsewhere} kết quả ở phân hệ khác` : 'Không thấy điều cần tìm?'}</span>
        <span className="shrink-0 text-sm font-semibold text-accent">Tìm ở mọi nơi</span>
      </Pressable>
    </div>
  )
}

function RowText({ item }: { item: SuggestionItem }) {
  return (
    <span className="min-w-0">
      <span className="block truncate font-medium text-[0.9375rem]"><Marked text={item.title} query={item.mark} /></span>
      {item.meta && <span className="block truncate text-[0.8125rem] text-ink-muted">{item.meta}</span>}
    </span>
  )
}

/** "Gợi ý" under an empty box: what the module would offer first (DESIGN.md "Gợi ý trong ô tìm", Ô trống). */
function useHints({ scope, workspaceId, me, isPersonal, enabled, ctx, openRequest, openDocument, openAsset, openConversation }: {
  scope: SearchScope
  workspaceId: string
  me: string
  isPersonal: boolean
  enabled: boolean
  ctx: RowContext
  openRequest: (r: ApprovalRequest) => void
  openDocument: (it: DriveItem) => void
  openAsset: (a: Asset) => void
  openConversation: (c: Conversation) => void
}): SuggestionGroup | null {
  const org = !isPersonal
  const pending = useQuery({ ...approvalPendingOptions(), enabled: enabled && org && scope === 'approval' })
  const shared = useSharedWithMe(enabled && scope === 'drive')
  const held = useQuery({ ...assetsQueryOptions(workspaceId, { assigned_to: me, limit: HINT_ROWS }), enabled: enabled && org && scope === 'assets' && !!me })
  const directs = useRecentDirects(workspaceId, enabled && scope === 'contacts')
  const why = (item: SuggestionItem, ...parts: (string | undefined)[]): SuggestionItem =>
    ({ ...item, meta: parts.filter(Boolean).join(' · ') })

  let items: SuggestionItem[] = []
  if (scope === 'approval' && org) {
    items = (pending.data?.rows ?? []).slice(0, HINT_ROWS)
      .map(({ request }) => why(approvalRow(request, ctx, openRequest), 'Chờ bạn', request.created_by_name))
  } else if (scope === 'drive') {
    items = (shared.data?.items ?? []).slice(0, HINT_ROWS)
      .map((it) => why(documentRow(it, ctx, openDocument), 'Được chia sẻ với bạn', it.owner_name))
  } else if (scope === 'assets' && org) {
    items = (held.data?.assets ?? []).slice(0, HINT_ROWS)
      .map((a) => why(assetRow(a, ctx, openAsset), 'Bạn đang giữ', a.type_name))
  } else if (scope === 'contacts') {
    items = directs.directs.slice(0, HINT_ROWS).map((c) => why(conversationRow(c, ctx, openConversation), 'Nhắn gần đây'))
  }
  const loading = (scope === 'approval' && pending.isLoading) || (scope === 'drive' && shared.isLoading) ||
    (scope === 'assets' && held.isLoading) || (scope === 'contacts' && directs.isLoading)
  if (!enabled || (items.length === 0 && !loading)) return null
  return { label: 'Gợi ý', items, loading: loading && items.length === 0 }
}
