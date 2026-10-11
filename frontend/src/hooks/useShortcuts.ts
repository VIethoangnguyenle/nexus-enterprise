import { useCallback, useRef } from 'react'
import { useInfiniteQuery, useQueries, useQuery, type InfiniteData } from '@tanstack/react-query'
import { messagingApi, type Channel, type ChannelUnread, type Message } from '../api/messaging'
import { shortcutsApi, type FeedMessage, type FeedPage } from '../api/shortcuts'
import { toast } from '../components/primitives'
import { feedCursor } from '../lib/shortcut-feed'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

/** Items per feed page; the screens add "Tải thêm" for the next. */
export const FEED_PAGE = 25

type MessageList = { messages: Message[]; has_more: boolean }
type Thread = { messages: Message[] }
type Feed = InfiniteData<FeedPage, unknown>

// --- Pure cache patches ------------------------------------------------------

function withStar<T extends { starred?: boolean }>(item: T, starred: boolean): T {
  const { starred: _drop, ...rest } = item
  return (starred ? { ...rest, starred: true } : rest) as T
}

/** One message of a cached list flagged (or unflagged); the same list when it is not there. */
export function setMessageStarred<L extends { messages: Message[] }>(list: L | undefined, id: string, starred: boolean): L | undefined {
  if (!list || !list.messages.some((x) => x.id === id)) return list
  return { ...list, messages: list.messages.map((x) => (x.id === id ? withStar(x, starred) : x)) }
}

/** One conversation of the unread list flagged; the same list when it is not there. */
export function setUnreadListStarred<L extends { channels: ChannelUnread[] }>(list: L | undefined, channelId: string, starred: boolean): L | undefined {
  if (!list || !list.channels.some((c) => c.channel_id === channelId)) return list
  // Starring stamps the time now, so the section lists it first before the server answers.
  const patch = (c: ChannelUnread): ChannelUnread => {
    const { starred_at: _drop, ...rest } = withStar(c, starred)
    return starred ? { ...rest, starred_at: new Date().toISOString() } : rest
  }
  return { ...list, channels: list.channels.map((c) => (c.channel_id === channelId ? patch(c) : c)) }
}

/** A conversation's own record flagged. */
export function setChannelRecordStarred(ch: Channel | undefined, starred: boolean): Channel | undefined {
  return ch ? withStar(ch, starred) : ch
}

function mapFeed(feed: Feed | undefined, fn: (page: FeedPage) => FeedPage): Feed | undefined {
  if (!feed) return feed
  let changed = false
  const pages = feed.pages.map((p) => {
    const next = fn(p)
    if (next !== p) changed = true
    return next
  })
  return changed ? { ...feed, pages } : feed
}

/** A message gone from every page of a feed; the same feed when it is not there. */
export function dropFromFeed(feed: Feed | undefined, id: string): Feed | undefined {
  return mapFeed(feed, (p) => (p.messages.some((x) => x.id === id) ? { ...p, messages: p.messages.filter((x) => x.id !== id) } : p))
}

/** A mention no longer unread (the dot is gone); the same feed when it already was read or is absent. */
export function markFeedRead(feed: Feed | undefined, id: string): Feed | undefined {
  return mapFeed(feed, (p) =>
    p.messages.some((x) => x.id === id && x.unread)
      ? { ...p, messages: p.messages.map((x) => (x.id === id ? ({ ...x, unread: undefined } as FeedMessage) : x)) }
      : p)
}

// --- Queries -----------------------------------------------------------------

/** Every message that mentions me, newest first, a page at a time. Reading the list marks nothing read. */
export function useMentionsFeed(wsId: string) {
  return useInfiniteQuery({
    queryKey: keys.shortcuts.mentions(wsId),
    initialPageParam: undefined as Parameters<typeof shortcutsApi.listMentions>[0]['cursor'],
    queryFn: ({ pageParam }) => shortcutsApi.listMentions({ cursor: pageParam, limit: FEED_PAGE }),
    getNextPageParam: (page) => feedCursor(page, 'created_at'),
    enabled: !!wsId,
  })
}

/** The messages I starred, newest star first, a page at a time. */
export function useStarredFeed(wsId: string) {
  return useInfiniteQuery({
    queryKey: keys.shortcuts.starredMessages(wsId),
    initialPageParam: undefined as Parameters<typeof shortcutsApi.listStarred>[0]['cursor'],
    queryFn: ({ pageParam }) => shortcutsApi.listStarred({ cursor: pageParam, limit: FEED_PAGE }),
    getNextPageParam: (page) => feedCursor(page, 'starred_at'),
    enabled: !!wsId,
  })
}

/** The pill on the "Được nhắc tới" shortcut: unread mentions, counted by the server. */
export function useMentionsUnreadCount(wsId: string) {
  return useQuery({
    queryKey: keys.shortcuts.mentionsUnread(wsId),
    queryFn: () => shortcutsApi.mentionsUnreadCount(),
    enabled: !!wsId,
  })
}

/**
 * The root message of each thread a reply in the feed belongs to, for the line
 * "Trả lời trong thread của …". Shares the thread cache with the thread panel.
 */
export function useThreadRoots(items: FeedMessage[]): Map<string, Message | undefined> {
  const parents = [...new Set(items.map((m) => m.parent_message_id).filter((id): id is string => !!id))]
  const results = useQueries({
    queries: parents.map((id) => ({
      queryKey: keys.messaging.thread(id),
      queryFn: () => messagingApi.getThread(id),
      staleTime: 60_000,
    })),
  })
  return new Map(parents.map((id, i) => [id, results[i]?.data?.messages.find((x) => x.id === id)]))
}

// --- Opening a mention -------------------------------------------------------

/** Opening a mention reads it in the conversation: its dot goes out and the badge drops at once. No request here. */
export function useOpenMention(): (m: FeedMessage) => void {
  return useCallback((m) => {
    if (!m.unread) return
    queryClient.setQueriesData<Feed>({ queryKey: keys.shortcuts.mentionsAll() }, (old) => markFeedRead(old, m.id))
    queryClient.setQueriesData<number>({ queryKey: keys.shortcuts.mentionsUnreadAll() }, (n) => (typeof n === 'number' ? Math.max(0, n - 1) : n))
  }, [])
}

// --- Starring ----------------------------------------------------------------

/**
 * Stops the refetches in flight on these keys before an optimistic patch: a
 * response that left before the patch would land after it with the old flag.
 * Awaited only when something is fetching, so the patch is otherwise immediate.
 */
function cancelFetches(...queryKeys: (readonly unknown[])[]): Promise<unknown> | undefined {
  const busy = queryKeys.filter((queryKey) => queryClient.isFetching({ queryKey }) > 0)
  return busy.length > 0 ? Promise.all(busy.map((queryKey) => queryClient.cancelQueries({ queryKey }))) : undefined
}

const failure = (starred: boolean) =>
  `${starred ? 'Không gắn sao được' : 'Không bỏ gắn sao được'}. Kiểm tra kết nối rồi thử lại.`

export interface StarMessageInput {
  message: Pick<Message, 'id' | 'channel_id' | 'parent_message_id'>
  starred: boolean
  /** Feed pages to put back first: the undo of an unstar returns the card to where it was. */
  restoreFeeds?: [readonly unknown[], Feed | undefined][]
  /** No toast: this call is itself an undo or a retry. */
  quiet?: boolean
}

/**
 * Star or unstar one message. It moves at once in the stream, its thread and
 * the starred feed; a refusal puts everything back and says so, with Thử lại.
 * Starring offers "Xem" (`onView` opens Đã gắn sao); unstarring offers Hoàn tác.
 */
export function useStarMessage(opts: { onView?: () => void } = {}): (input: StarMessageInput) => Promise<void> {
  const onView = useRef(opts.onView)
  onView.current = opts.onView

  const toggle = useCallback(async function run(input: StarMessageInput): Promise<void> {
    const { message, starred } = input
    const parent = message.parent_message_id || undefined
    const listKey = keys.messaging.messages(message.channel_id)
    const threadKey = keys.messaging.thread(parent || message.id)
    const feedKey = keys.shortcuts.starredMessagesAll()
    const cancelling = cancelFetches(feedKey, threadKey, ...(parent ? [] : [listKey]))
    if (cancelling) await cancelling
    const before = { feeds: queryClient.getQueriesData<Feed>({ queryKey: feedKey }) }

    for (const [key, feed] of input.restoreFeeds ?? []) queryClient.setQueryData(key, feed)
    if (!parent) queryClient.setQueryData<MessageList>(listKey, (old) => setMessageStarred(old, message.id, starred))
    queryClient.setQueryData<Thread>(threadKey, (old) => setMessageStarred(old, message.id, starred))
    if (!starred) queryClient.setQueriesData<Feed>({ queryKey: feedKey }, (old) => dropFromFeed(old, message.id))

    try {
      await (starred ? shortcutsApi.starMessage(message.id) : shortcutsApi.unstarMessage(message.id))
      // Said once the server has agreed: a refusal must not follow a success toast with an error one.
      if (!input.quiet) {
        toast(starred ? 'Đã gắn sao tin' : 'Đã bỏ gắn sao tin', {
          action: starred
            ? onView.current && { label: 'Xem', onClick: () => onView.current?.() }
            : { label: 'Hoàn tác', onClick: () => void run({ message, starred: true, restoreFeeds: before.feeds, quiet: true }) },
        })
      }
      // A new star lands at the top of the feed; an unstar already left it, and a refetch would move an undo.
      if (starred) await queryClient.invalidateQueries({ queryKey: feedKey })
    } catch {
      // Undo only this star, item by item: whole-list snapshots would also undo
      // what the socket delivered while the request was out.
      if (!parent) queryClient.setQueryData<MessageList>(listKey, (old) => setMessageStarred(old, message.id, !starred))
      queryClient.setQueryData<Thread>(threadKey, (old) => setMessageStarred(old, message.id, !starred))
      // A card the unstar took out of the feed returns with the server's own copy.
      if (!starred) void queryClient.invalidateQueries({ queryKey: feedKey })
      toast.error(failure(starred), { action: { label: 'Thử lại', onClick: () => void run({ message, starred, quiet: true }) } })
    }
  }, [])
  return toggle
}

export interface StarConversationInput {
  channelId: string
  /** The name the toast speaks of. */
  name: string
  starred: boolean
  quiet?: boolean
}

/**
 * Star or unstar a conversation. The unread list (which carries `starred` for
 * the navigator) and the conversation's own record move at once; a refusal
 * puts them back. Sections are not touched: the server keeps the old one.
 */
export function useStarConversation(): (input: StarConversationInput) => Promise<void> {
  return useCallback(async function run(input: StarConversationInput): Promise<void> {
    const { channelId, name, starred } = input
    const unreadKey = keys.messaging.unreadCounts()
    const channelKey = keys.messaging.channel(channelId)
    const cancelling = cancelFetches(unreadKey, channelKey)
    if (cancelling) await cancelling
    queryClient.setQueryData<{ channels: ChannelUnread[] }>(unreadKey, (old) => setUnreadListStarred(old, channelId, starred))
    queryClient.setQueryData<Channel>(channelKey, (old) => setChannelRecordStarred(old, starred))

    try {
      await (starred ? shortcutsApi.starConversation(channelId) : shortcutsApi.unstarConversation(channelId))
      if (!input.quiet) {
        toast(starred ? `Đã gắn sao “${name}”` : `Đã bỏ gắn sao “${name}”`, {
          action: { label: 'Hoàn tác', onClick: () => void run({ channelId, name, starred: !starred, quiet: true }) },
        })
      }
    } catch {
      queryClient.setQueryData<{ channels: ChannelUnread[] }>(unreadKey, (old) => setUnreadListStarred(old, channelId, !starred))
      queryClient.setQueryData<Channel>(channelKey, (old) => setChannelRecordStarred(old, !starred))
      toast.error(failure(starred), { action: { label: 'Thử lại', onClick: () => void run({ channelId, name, starred, quiet: true }) } })
    }
    // Settled either way: the server's list carries the true order of the starred section.
    void queryClient.invalidateQueries({ queryKey: unreadKey })
  }, [])
}
