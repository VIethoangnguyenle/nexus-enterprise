import { useState } from 'react'
import { SearchX } from 'lucide-react'
import { useSearch } from '../../hooks/useMessaging'
import { formatListTime, formatDateTime } from '../../lib/format'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { Avatar, Pressable, SearchField } from '../primitives'
import { SidePanel } from './SidePanel'
import { EmptyState } from './EmptyState'

/** Plain text of a message body for result snippets. */
function plain(html: string): string {
  return html.replace(/<[^>]+>/g, ' ').replace(/&nbsp;/g, ' ').replace(/\s+/g, ' ').trim()
}

/** Search inside one conversation; a result opens its topic. */
export function SpaceSearchPanel({ channelId, title, people, onOpenTopic, onClose }: {
  channelId: string
  title: string
  people: PeopleDirectory
  onOpenTopic: (topicId: string) => void
  onClose: () => void
}) {
  const [query, setQuery] = useState('')
  const q = query.trim()
  const { data, isFetching } = useSearch(channelId, q)
  const results = data?.messages ?? []

  return (
    <SidePanel label="Tìm kiếm" title="Tìm trong cuộc trò chuyện" sub={title} closeLabel="tìm kiếm" onClose={onClose}>
      <SearchField
        label="Tìm tin nhắn"
        value={query}
        autoFocus
        onChange={(e) => setQuery(e.target.value)}
        className="mx-3 mb-2"
      />
      {q.length < 2 ? (
        <div className="px-4 py-2 text-small text-ink-muted">Gõ ít nhất 2 ký tự để tìm.</div>
      ) : isFetching && results.length === 0 ? (
        <div className="px-4 py-2 text-small text-ink-muted" aria-live="polite">Đang tìm…</div>
      ) : results.length === 0 ? (
        <EmptyState compact icon={<SearchX size={24} strokeWidth={1.75} />} text={`Không thấy tin nào có “${q}”.`} />
      ) : (
        <ul className="grid gap-0.5 list-none m-0 p-0" aria-label="Kết quả">
          {results.map((r) => {
            const name = displayName(people, r.sender_id, r.sender_name)
            return (
              <li key={r.id}>
                <Pressable
                  onClick={() => onOpenTopic(r.parent_message_id || r.id)}
                  className="w-full grid grid-cols-[24px_minmax(0,1fr)] gap-2.5 px-3 py-2 rounded-surface hover:bg-hover
                    transition-colors duration-quick"
                >
                  <Avatar name={name} hueKey={r.sender_id || name} size={24} />
                  <span className="grid gap-0.5 min-w-0">
                    <span className="flex items-baseline gap-2">
                      <span className="font-semibold text-ink truncate">{name}</span>
                      <time className="text-xs text-ink-muted tnum" title={formatDateTime(r.created_at)}>
                        {formatListTime(r.created_at)}
                      </time>
                    </span>
                    <span className="text-small text-ink-muted line-clamp-2">{plain(r.content)}</span>
                  </span>
                </Pressable>
              </li>
            )
          })}
        </ul>
      )}
    </SidePanel>
  )
}
