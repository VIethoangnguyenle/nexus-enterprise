import { useMemo } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Search } from 'lucide-react'
import { Avatar } from '../components/primitives'
import { ConversationIcon } from '../components/spaces/ConversationIcon'
import { Kbd } from '../components/patterns/SearchSuggestions'
import type { Message } from '../api/messaging'
import { formatListTime, formatMessageTime } from '../lib/format'
import { messageText } from '../lib/message-model'
import { displayName, type PeopleDirectory, type Person } from '../lib/people'
import { snippet } from '../lib/search-text'
import { conversationSub, conversationsMatching, peopleMatching, recentConversations } from '../lib/suggestions/chat'
import type { Conversation } from '../lib/conversations'
import { useAuthStore } from '../stores/auth.store'
import { useMessageSearch } from './useMessageSearch'
import { useOpenPerson } from './useOpenPerson'
import { useRecentSearches } from './useRecentSearches'
import { feedSearch } from './useShortcutScreen'
import type { SuggestionGroup } from './useSuggestionPanel'

/** A message's opening, on one line. */
const SNIPPET = 90
/** Messages the server is asked for while typing: the suggestions show three. */
const MESSAGE_ROWS = 3

interface Options {
  query: string
  conversations: Conversation[]
  people: PeopleDirectory
  /** The phone draws 36px marks and no key hints. */
  phone: boolean
  /** False while nothing shows the suggestions, so no message search is made for them. */
  enabled: boolean
  /** A recent search was picked: put it in the box. */
  onReuse: (query: string) => void
  /** "Xem tất cả kết quả" or Enter: open the results for these words. */
  onSubmit: (query: string) => void
}

/**
 * The rows Chat's search box suggests (DESIGN.md "Gợi ý trong ô tìm"). Before anything is typed: Tìm gần đây and
 * Trò chuyện gần đây. Once a word is typed: Người, Nhóm và trò chuyện (both matched in the browser from the
 * lists the person may read), Tin nhắn (asked of the server after a pause) and always "Xem tất cả kết quả".
 * Choosing a row keeps the typed words as a recent search and opens what the row names.
 */
export function useChatSuggestions({ query, conversations, people, phone, enabled, onReuse, onSubmit }: Options) {
  const me = useAuthStore((s) => s.user?.id ?? '')
  const navigate = useNavigate()
  const recent = useRecentSearches('chat')
  const person = useOpenPerson(conversations)
  const typed = query.trim()
  const size = phone ? 36 : 32
  const search = useMessageSearch(query, {}, { enabled: enabled && typed !== '', limit: MESSAGE_ROWS })

  const byId = useMemo(() => new Map(conversations.map((c) => [c.id, c])), [conversations])
  const lately = useMemo(() => recentConversations(conversations), [conversations])
  const persons = useMemo(() => (typed ? peopleMatching(people.list, typed, me) : []), [people.list, typed, me])
  const spaces = useMemo(
    () => (typed ? conversationsMatching(conversations, typed, new Set(persons.map((p) => p.userId))) : []),
    [conversations, typed, persons],
  )

  const open = (go: () => void) => () => {
    recent.add(typed)
    go()
  }
  const openConversation = (c: Conversation) => void navigate({ to: '/channels/$channelId', params: { channelId: c.id } })
  const openMessage = (m: Message) =>
    void navigate({ to: '/channels/$channelId', params: { channelId: m.channel_id }, search: feedSearch(m) })

  const personRow = (p: Person) => ({
    key: p.userId,
    title: p.name,
    mark: typed,
    meta: p.role || undefined,
    label: p.role ? `${p.name}, ${p.role}` : p.name,
    icon: <Avatar name={p.name} hueKey={p.userId} src={p.avatarUrl} size={size} />,
    onSelect: open(() => person.open(p)),
  })
  const conversationRow = (c: Conversation) => {
    const sub = conversationSub(c)
    return {
      key: c.id,
      title: c.title,
      mark: typed,
      meta: sub,
      label: `${c.title}, ${sub}`,
      icon: <ConversationIcon conversation={c} size={size} />,
      onSelect: open(() => openConversation(c)),
    }
  }
  const messageRow = (m: Message) => {
    const sender = displayName(people, m.sender_id, m.sender_name)
    const where = byId.get(m.channel_id)?.title
    const text = snippet(messageText(m.content), typed, SNIPPET)
    return {
      key: m.id,
      title: text,
      mark: typed,
      meta: [sender, where, formatMessageTime(m.created_at)].filter(Boolean).join(' · '),
      label: `Tin của ${sender}${where ? ` trong ${where}` : ''}: ${text}`,
      icon: <Avatar name={sender} hueKey={m.sender_id} src={people.byUserId.get(m.sender_id)?.avatarUrl} size={size} />,
      onSelect: open(() => openMessage(m)),
    }
  }

  const everything: SuggestionGroup = {
    label: 'Xem tất cả',
    hideLabel: true,
    items: [{
      key: 'all',
      title: `Xem tất cả kết quả cho “${typed}”`,
      label: `Xem tất cả kết quả cho “${typed}”`,
      strong: true,
      icon: (
        <span className={`grid place-items-center rounded-md bg-sunk text-ink-muted ${phone ? 'size-9' : 'size-8'}`}>
          <Search size={16} strokeWidth={1.75} aria-hidden="true" />
        </span>
      ),
      trailing: phone ? undefined : <Kbd>Enter</Kbd>,
      onSelect: () => onSubmit(typed),
    }],
  }

  let groups: SuggestionGroup[]
  let note: string | undefined
  if (typed === '') {
    groups = [
      recent.group(onReuse),
      {
        label: 'Trò chuyện gần đây',
        items: lately.map((c) => ({
          key: c.id,
          title: c.title,
          meta: c.preview ? [c.previewAuthor, c.preview].filter(Boolean).join(': ') : undefined,
          icon: <ConversationIcon conversation={c} size={size} />,
          trailing: formatListTime(new Date(c.lastActivity)),
          onSelect: () => openConversation(c),
        })),
      },
    ]
  } else {
    const found = search.active && !search.isError
    groups = [
      { label: 'Người', items: persons.map(personRow) },
      { label: 'Nhóm và trò chuyện', items: spaces.map(conversationRow) },
      // A failed search leaves its group out; it is not reported among suggestions.
      ...(found ? [{ label: 'Tin nhắn', loading: search.waiting, items: search.messages.slice(0, MESSAGE_ROWS).map(messageRow) }] : []),
      everything,
    ]
    if (persons.length + spaces.length === 0 && !search.waiting && search.messages.length === 0) {
      note = 'Không có gợi ý. Nhấn Enter để tìm trong mọi tin nhắn.'
    }
  }

  return { groups, note, recent }
}

