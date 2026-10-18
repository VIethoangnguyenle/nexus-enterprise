import type { Contact } from '../../hooks/useContacts'
import { memo } from 'react'
import { Avatar, Pressable } from '../primitives'
import { Marked } from '../patterns/Marked'
import { nameOf, type PathSegment } from './contacts-model'
import { PathTrail } from './PathTrail'

interface PersonRowProps {
  person: Contact
  online: boolean
  /** The department path after the title; given where the row is outside the tree (search, role, group). */
  path?: PathSegment[]
  selected?: boolean
  /** The signed-in user: the name is followed by "(bạn)". */
  isMe?: boolean
  /** The search the row was found by: its words are marked in the name, title and path. */
  query?: string
  /** Receives the row's own button, so closing the profile can give focus back to it. */
  onSelect: (person: Contact, opener: HTMLElement) => void
  /** The phone's row: 60 high with avatar 40 (DESIGN.md "Danh bạ" › Điện thoại); elsewhere 52 with 32. */
  tall?: boolean
}

/** A person in the list (DESIGN.md "Danh bạ"): avatar 32, name, title · path, and the status in words. */
function PersonRowView({ person, online, path = [], selected, isMe, query, onSelect, tall }: PersonRowProps) {
  const name = nameOf(person)
  const last = path[path.length - 1]
  return (
    <Pressable
      aria-current={selected ? 'true' : undefined}
      onClick={(e) => onSelect(person, e.currentTarget)}
      className={`press flex items-center gap-3 w-full min-w-0 ${tall ? 'min-h-15' : 'min-h-13'} px-2 py-2 rounded-lg border-none text-left cursor-pointer
        focus-ring transition-colors duration-quick ${selected ? 'bg-accent-wash' : 'bg-transparent hover:bg-hover'}`}
    >
      <Avatar name={name} hueKey={person.user_id} src={person.avatar_url || undefined} size={tall ? 40 : 32} online={online} />
      <span className="grid flex-1 min-w-0 gap-px">
        <span className="truncate font-medium text-ink">
          <Marked text={name} query={query} />
          {isMe && <span className="font-normal text-ink-muted"> (bạn)</span>}
        </span>
        {(person.title || last) && (
          <span className="flex min-w-0 gap-1 overflow-hidden text-small text-ink-muted">
            {person.title && (
              // With a path after it the title gives way first: the last segment of the path is never cut.
              <span className={last ? 'min-w-16 shrink-[3] truncate' : 'truncate'}>
                <span><Marked text={person.title} query={query} /></span>
                {last && <span aria-hidden="true"> ·</span>}
              </span>
            )}
            {last && <PathTrail segments={path} query={query} />}
          </span>
        )}
      </span>
      <span className={`shrink-0 text-xs font-medium ${online ? 'text-success' : 'text-ink-muted'}`}>
        {online ? 'Trực tuyến' : 'Ngoại tuyến'}
      </span>
    </Pressable>
  )
}

const samePath = (a: PathSegment[] = [], b: PathSegment[] = []) =>
  a.length === b.length && a.every((seg, i) => seg.id === b[i]!.id && seg.name === b[i]!.name)

/**
 * A row is redrawn only when something it shows changed: typing in the box re-renders the list around it, and a
 * row whose person, status, path and marks are the same stays as it is.
 */
export const PersonRow = memo(PersonRowView, (a, b) =>
  a.person === b.person && a.online === b.online && a.selected === b.selected && a.isMe === b.isMe
  && a.query === b.query && a.onSelect === b.onSelect && a.tall === b.tall && samePath(a.path, b.path))
