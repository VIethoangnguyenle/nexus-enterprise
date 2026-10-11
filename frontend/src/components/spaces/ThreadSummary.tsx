import type { CSSProperties } from 'react'
import type { Message } from '../../api/messaging'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { threadSummary } from '../../lib/stream-model'
import { Avatar, Pressable } from '../primitives'

interface ThreadSummaryProps {
  message: Message
  people: PeopleDirectory
  /** The thread is open in the side panel. */
  open?: boolean
  /** Your own message: the row sits on the right edge. */
  mine?: boolean
  /** Realtime: a reply from someone else just landed; washed in their hue. */
  washStyle?: CSSProperties
  onOpen: () => void
  /** On a panel (raised) the faces' ring matches it; on the page it matches the base. */
  ring?: 'base' | 'raised'
}

/**
 * The row under a message that has replies (DESIGN.md "Hàng tóm tắt thread"):
 * up to three repliers' faces, "N trả lời", the time of the last reply, and a
 * "M mới" pill only while the reader follows the thread. The numbers are the
 * server's; this draws them. Nothing here is an id: faces are named through the
 * people directory.
 */
export function ThreadSummary({ message, people, open, mine, washStyle, onOpen, ring = 'base' }: ThreadSummaryProps) {
  const s = threadSummary(message, (id) => displayName(people, id))
  if (!s) return null
  return (
    <Pressable
      onClick={onOpen}
      aria-label={s.label}
      aria-expanded={open ?? false}
      style={washStyle}
      className={`flex items-center gap-2 max-w-full min-h-9 md:min-h-8 mt-1 mb-0.5 pl-2 pr-2.5 rounded-md text-small
        text-ink-muted transition-colors duration-quick hover:bg-raised
        ${mine ? '-mr-2' : '-ml-2'}
        ${open ? 'bg-accent-wash hover:bg-accent-wash' : ''} ${washStyle ? 'rt-wash' : ''}`}
    >
      {s.faces.length > 0 && (
        <span className="flex items-center shrink-0" aria-hidden="true">
          {s.faces.map((f, i) => (
            <Avatar
              key={f.id}
              name={f.name}
              hueKey={f.id}
              src={people.byUserId.get(f.id)?.avatarUrl}
              size={20}
              className={`ring-2 ${ring === 'raised' ? 'ring-raised' : 'ring-base'} ${i > 0 ? '-ml-1.5' : ''}`}
            />
          ))}
        </span>
      )}
      <span className="font-semibold text-accent whitespace-nowrap">{s.count} trả lời</span>
      {s.last && (
        <span className="truncate tnum">
          <span className="max-sm:hidden">Lần cuối </span>
          {s.last}
        </span>
      )}
      {s.unread > 0 && (
        <span className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent text-on-accent text-2xs font-semibold tnum shrink-0">
          {s.unread} mới
        </span>
      )}
    </Pressable>
  )
}
