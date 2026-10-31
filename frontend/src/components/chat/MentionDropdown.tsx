import { useState, useEffect, useRef } from 'react'
import { Users } from 'lucide-react'
import { Avatar } from '../primitives'
import { avatarOf, displayName, EMPTY_DIRECTORY, normalize, type PeopleDirectory } from '../../lib/people'

interface MentionUser {
  user_id: string
  username: string
  ngac_node_id: string
}

interface MentionDropdownProps {
  members: MentionUser[]
  /** Resolves display names; the inserted mention is still @username. */
  people?: PeopleDirectory
  query: string
  onSelect: (member: MentionUser) => void
  /**
   * Offers "@Tất cả" first, for where the caller may mention everyone (never a DM; administrators in the
   * workspace-wide channel; the group's rule in a group). Absent: not offered.
   */
  onSelectAll?: () => void
  onClose: () => void
}

/** What "@Tất cả" answers to as it is typed: the composer's query is ASCII letters, so "tat ca" or "all". */
const ALL_WORDS = ['tatca', 'all']
const matchesAll = (q: string) => {
  const flat = q.replace(/\s+/g, '')
  return ALL_WORDS.some((w) => w.startsWith(flat))
}

type Option = { kind: 'all' } | { kind: 'member'; m: MentionUser; name: string }

/**
 * @mention autocomplete for the composer: "@Tất cả" where allowed, then the
 * channel members by display name, keyboard driven (↑ ↓ Enter, Esc closes).
 */
export function MentionDropdown({ members, people = EMPTY_DIRECTORY, query, onSelect, onSelectAll, onClose }: MentionDropdownProps) {
  const [selectedIndex, setSelectedIndex] = useState(0)
  const ref = useRef<HTMLDivElement>(null)

  const q = normalize(query)
  const options: Option[] = [
    ...(onSelectAll && matchesAll(q) ? [{ kind: 'all' } as const] : []),
    ...members
      .map((m) => ({ kind: 'member' as const, m, name: displayName(people, m.user_id, m.username) }))
      .filter(({ m, name }) => normalize(m.username).includes(q) || normalize(name).includes(q))
      .slice(0, 8),
  ]

  const pick = (o: Option) => (o.kind === 'all' ? onSelectAll?.() : onSelect(o.m))

  useEffect(() => { setSelectedIndex(0) }, [query])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIndex((i) => Math.min(i + 1, options.length - 1))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex((i) => Math.max(i - 1, 0))
      } else if (e.key === 'Enter') {
        const o = options[selectedIndex]
        if (o) {
          e.preventDefault()
          pick(o)
        }
      } else if (e.key === 'Escape') {
        e.preventDefault()
        e.stopPropagation()
        onClose()
      }
    }
    document.addEventListener('keydown', handler, true)
    return () => document.removeEventListener('keydown', handler, true)
  })

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [onClose])

  const row = (i: number) =>
    `flex items-center gap-2.5 px-2.5 py-1.5 rounded-md cursor-pointer text-sm ${i === selectedIndex ? 'bg-hover' : ''}`

  return (
    <div
      ref={ref}
      role="listbox"
      aria-label="Nhắc tới"
      className="min-w-60 max-h-60 overflow-y-auto p-1 rounded-surface bg-overlay shadow-overlay animate-fade-in"
    >
      {options.length === 0 ? (
        <div className="px-2.5 py-2 text-sm text-ink-muted">Không có thành viên nào khớp.</div>
      ) : (
        options.map((o, i) => (
          <div
            key={o.kind === 'all' ? 'all' : o.m.ngac_node_id || o.m.user_id || o.m.username}
            role="option"
            aria-selected={i === selectedIndex}
            onMouseDown={(e) => {
              e.preventDefault()
              pick(o)
            }}
            onMouseEnter={() => setSelectedIndex(i)}
            className={row(i)}
          >
            {o.kind === 'all' ? (
              <>
                <span className="grid place-items-center w-6 h-6 rounded-full bg-sunk text-ink-muted shrink-0" aria-hidden="true">
                  <Users size={14} strokeWidth={1.75} />
                </span>
                <span className="truncate text-ink font-medium">@Tất cả</span>
                <span className="ml-auto text-xs text-ink-muted">Nhắc mọi người</span>
              </>
            ) : (
              <>
                <Avatar name={o.name} hueKey={o.m.user_id || o.m.username} src={avatarOf(people, o.m.user_id, o.m.username)} size={24} />
                <span className="truncate text-ink">{o.name}</span>
                <span className="ml-auto text-xs text-ink-muted">@{o.m.username}</span>
              </>
            )}
          </div>
        ))
      )}
    </div>
  )
}
