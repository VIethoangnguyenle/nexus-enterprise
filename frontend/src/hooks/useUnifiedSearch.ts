import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { assetApi, type Asset } from '../api/assets'
import type { ApprovalRequest } from '../api/approval'
import { calendarApi, type Occurrence } from '../api/calendar'
import type { DriveItem } from '../api/drive'
import type { Message } from '../api/messaging'
import { searchApprovals, searchDrive } from '../api/search'
import {
  approvalQuery, filterDrive, filterEvents, type SearchFilters,
} from '../components/search/search-filters'
import type { Conversation } from '../lib/conversations'
import type { Person } from '../lib/people'
import { conversationsMatching, peopleMatching } from '../lib/suggestions/chat'
import { useAuthStore } from '../stores/auth.store'
import { keys } from './keys'
import { useConversations } from './useConversations'
import { useMessageSearch } from './useMessageSearch'
import { usePeople } from './usePeople'

/** Each service gets this long to answer; a slower one leaves its group empty rather than holding the panel. */
export const GROUP_TIMEOUT_MS = 3000
/** A pause in typing this long before the services are asked. */
const DEBOUNCE_MS = 250
/** Words shorter than this are matched in the browser only (people, conversations). */
export const SERVER_MIN_CHARS = 2
/** Most rows a module's own list asks for. */
const PAGE = 50
/** People and conversations matched in the browser, at most. */
const LOCAL_LIMIT = 20

/**
 * The keys of the searches only this screen makes. Nothing writes them: a result is never edited here, and
 * the drive, the approval service and the asset list invalidate their own lists.
 */
export const searchKeys = {
  drive: (wsId: string, q: string) => ['search', 'drive', wsId, q] as const,
  approvals: (wsId: string, q: string, filters: object) => ['search', 'approvals', wsId, q, filters] as const,
}

/** A promise that gives up after `ms`, so one slow service cannot keep its group waiting. */
export function withTimeout<T>(p: Promise<T>, ms: number = GROUP_TIMEOUT_MS): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const id = setTimeout(() => reject(new Error('timeout')), ms)
    p.then(
      (v) => { clearTimeout(id); resolve(v) },
      (e: unknown) => { clearTimeout(id); reject(e) },
    )
  })
}

/** The value after it has stayed the same for `ms`. */
function useSettled(value: string, ms: number): string {
  const [settled, setSettled] = useState(value)
  useEffect(() => {
    const id = setTimeout(() => setSettled(value), ms)
    return () => clearTimeout(id)
  }, [value, ms])
  return settled
}

export interface GroupResult<T> {
  items: T[]
  /** Every match the caller may read; more than `items` when the service pages. */
  total: number
  /** Asked, not answered yet: the group keeps its place with a skeleton. */
  loading: boolean
}

export interface UnifiedResults {
  /** The words the services were asked about (the typed ones, once typing pauses). */
  term: string
  people: GroupResult<Person>
  conversations: GroupResult<Conversation>
  messages: GroupResult<Message>
  documents: GroupResult<DriveItem>
  /** Every document found before the chips narrowed them: the chips offer what is in here. */
  documentsUnfiltered: DriveItem[]
  approvals: GroupResult<ApprovalRequest>
  events: GroupResult<Occurrence>
  assets: GroupResult<Asset>
  conversationList: Conversation[]
}

const EMPTY = { items: [], total: 0, loading: false }

/** A query's answer as a group: a failure (refusal, timeout, outage) is an empty group, never an error on screen. */
function asGroup<T>(q: { data?: T[]; isFetching: boolean; isError: boolean }, asked: boolean, total?: number): GroupResult<T> {
  if (!asked || q.isError) return EMPTY
  const items = q.data ?? []
  return { items, total: total ?? items.length, loading: q.isFetching && !q.data }
}

/**
 * Every kind of thing one search finds (DESIGN.md §6 "Tìm cho mọi tác vụ"), asked of the service that owns it,
 * in parallel and without a gateway: people and conversations are matched in the browser from the lists the
 * person may read; messages, documents, requests, events and assets come from their services, each already
 * filtered by read permission there. Each group arrives on its own; a personal workspace asks for no requests
 * and no assets.
 */
export function useUnifiedSearch({ query, workspaceId, isPersonal, filters, enabled }: {
  query: string
  workspaceId: string
  isPersonal: boolean
  filters: SearchFilters
  enabled: boolean
}): UnifiedResults {
  const me = useAuthStore((s) => s.user?.id ?? '')
  const typed = query.trim()
  const term = useSettled(typed, DEBOUNCE_MS)
  const settled = term === typed
  const ask = enabled && !!workspaceId && term.length >= SERVER_MIN_CHARS

  const people = usePeople(workspaceId)
  const { conversations } = useConversations(workspaceId)
  const messages = useMessageSearch(query, {}, { enabled: enabled && typed !== '', limit: PAGE })

  const drive = useQuery({
    queryKey: searchKeys.drive(workspaceId, term),
    queryFn: ({ signal }) => withTimeout(searchDrive(workspaceId, term, PAGE, signal)),
    enabled: ask,
    select: (d) => d.items ?? [],
    retry: false,
  })
  const approvalFilters = useMemo(() => approvalQuery(filters.approval), [filters.approval])
  const approvals = useQuery({
    queryKey: searchKeys.approvals(workspaceId, term, approvalFilters),
    queryFn: ({ signal }) => withTimeout(searchApprovals(term, approvalFilters, PAGE, signal)),
    enabled: ask && !isPersonal,
    retry: false,
  })
  const events = useQuery({
    queryKey: keys.calendar.search(workspaceId, term),
    queryFn: () => withTimeout(calendarApi.searchEvents(workspaceId, term)),
    enabled: ask,
    select: (d) => d.occurrences ?? [],
    retry: false,
  })
  const assetParams = {
    search: term, type_id: filters.assets.type, state: filters.assets.state, assigned_to: filters.assets.holder, limit: PAGE,
  }
  const assets = useQuery({
    queryKey: keys.assets.list(workspaceId, assetParams),
    queryFn: () => withTimeout(assetApi.list(workspaceId, assetParams)),
    enabled: ask && !isPersonal,
    retry: false,
  })

  const persons = useMemo(
    () => (enabled && typed ? peopleMatching(people.list, typed, me, LOCAL_LIMIT) : []),
    [enabled, people.list, typed, me],
  )
  const spaces = useMemo(
    () => (enabled && typed ? conversationsMatching(conversations, typed, new Set(persons.map((p) => p.userId)), LOCAL_LIMIT) : []),
    [enabled, conversations, typed, persons],
  )
  const documentsUnfiltered = useMemo(() => drive.data ?? [], [drive.data])
  const documents = useMemo(() => filterDrive(documentsUnfiltered, filters.drive), [documentsUnfiltered, filters.drive])
  const eventList = useMemo(() => filterEvents(events.data ?? [], filters.calendar), [events.data, filters.calendar])

  // While typing has not paused, a server group shows its skeleton rather than the answer to older words.
  const waiting = <T,>(g: GroupResult<T>, asked: boolean): GroupResult<T> => (asked && !settled ? { items: [], total: 0, loading: true } : g)
  const askedServer = enabled && typed.length >= SERVER_MIN_CHARS

  return {
    term,
    people: { items: persons, total: persons.length, loading: false },
    conversations: { items: spaces, total: spaces.length, loading: false },
    messages: messages.isError || !messages.active
      ? EMPTY
      : { items: messages.messages, total: messages.messages.length, loading: messages.waiting },
    documents: waiting(asGroup({ ...drive, data: drive.data ? documents : undefined }, ask), askedServer),
    documentsUnfiltered,
    approvals: waiting(
      asGroup({ ...approvals, data: approvals.data?.items }, ask && !isPersonal, approvals.data?.total),
      askedServer && !isPersonal,
    ),
    events: waiting(asGroup({ ...events, data: events.data ? eventList : undefined }, ask), askedServer),
    assets: waiting(
      asGroup({ ...assets, data: assets.data?.assets }, ask && !isPersonal, assets.data?.total),
      askedServer && !isPersonal,
    ),
    conversationList: conversations,
  }
}
