import { useState, useEffect, useRef } from 'react'
import { Avatar } from '../primitives'
import { displayName, EMPTY_DIRECTORY, normalize, type PeopleDirectory } from '../../lib/people'

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
  onClose: () => void
}

/**
 * @mention autocomplete for the composer: channel members by display name,
 * keyboard driven (↑ ↓ Enter, Esc closes).
 */
export function MentionDropdown({ members, people = EMPTY_DIRECTORY, query, onSelect, onClose }: MentionDropdownProps) {
  const [selectedIndex, setSelectedIndex] = useState(0)
  const ref = useRef<HTMLDivElement>(null)

  const q = normalize(query)
  const filtered = members
    .map((m) => ({ m, name: displayName(people, m.user_id, m.username) }))
    .filter(({ m, name }) => normalize(m.username).includes(q) || normalize(name).includes(q))
    .slice(0, 8)

  useEffect(() => { setSelectedIndex(0) }, [query])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex((i) => Math.max(i - 1, 0))
      } else if (e.key === 'Enter') {
        const pick = filtered[selectedIndex]
        if (pick) {
          e.preventDefault()
          onSelect(pick.m)
        }
      } else if (e.key === 'Escape') {
        e.preventDefault()
        e.stopPropagation()
        onClose()
      }
    }
    document.addEventListener('keydown', handler, true)
    return () => document.removeEventListener('keydown', handler, true)
  }, [filtered, selectedIndex, onSelect, onClose])

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [onClose])

  return (
    <div
      ref={ref}
      role="listbox"
      aria-label="Nhắc tới"
      className="min-w-60 max-h-60 overflow-y-auto p-1 rounded-surface bg-overlay shadow-overlay animate-fade-in"
    >
      {filtered.length === 0 ? (
        <div className="px-2.5 py-2 text-sm text-ink-muted">Không có thành viên nào khớp.</div>
      ) : (
        filtered.map(({ m, name }, i) => (
          <div
            key={m.ngac_node_id || m.user_id || m.username}
            role="option"
            aria-selected={i === selectedIndex}
            onMouseDown={(e) => {
              e.preventDefault()
              onSelect(m)
            }}
            onMouseEnter={() => setSelectedIndex(i)}
            className={`flex items-center gap-2.5 px-2.5 py-1.5 rounded-md cursor-pointer text-sm
              ${i === selectedIndex ? 'bg-hover' : ''}`}
          >
            <Avatar name={name} hueKey={m.user_id || m.username} size={24} />
            <span className="truncate text-ink">{name}</span>
            <span className="ml-auto text-xs text-ink-muted">@{m.username}</span>
          </div>
        ))
      )}
    </div>
  )
}
