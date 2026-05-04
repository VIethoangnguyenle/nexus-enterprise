import { useState, useEffect, useRef, forwardRef, useImperativeHandle } from 'react'
import { Avatar } from '../primitives'

interface MentionUser {
  user_id: string
  username: string
  ngac_node_id: string
}

interface MentionDropdownProps {
  members: MentionUser[]
  query: string
  onSelect: (member: MentionUser) => void
  onClose: () => void
  position?: { top: number; left: number }
}

/** Floating @mention autocomplete dropdown for ChatEditor.
 *  Shows filtered channel members matching the typed text after @.
 *  Design tokens: bg-surface-container-lowest border-outline-variant rounded-lg shadow-lg */
export function MentionDropdown({ members, query, onSelect, onClose, position }: MentionDropdownProps) {
  const [selectedIndex, setSelectedIndex] = useState(0)
  const ref = useRef<HTMLDivElement>(null)

  const filtered = members.filter((m) =>
    m.username.toLowerCase().includes(query.toLowerCase()),
  ).slice(0, 8)

  // Reset selection when query changes
  useEffect(() => { setSelectedIndex(0) }, [query])

  // Keyboard navigation
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex((i) => Math.max(i - 1, 0))
      } else if (e.key === 'Enter') {
        e.preventDefault()
        if (filtered[selectedIndex]) onSelect(filtered[selectedIndex])
      } else if (e.key === 'Escape') {
        e.preventDefault()
        onClose()
      }
    }
    document.addEventListener('keydown', handler, true)
    return () => document.removeEventListener('keydown', handler, true)
  }, [filtered, selectedIndex, onSelect, onClose])

  // Click outside to close
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [onClose])

  if (filtered.length === 0) {
    return (
      <div
        ref={ref}
        className="absolute z-50 bg-surface-container-lowest border border-outline-variant rounded-lg shadow-lg
          py-2 px-3 min-w-[200px]"
        style={position ? { bottom: position.top, left: position.left } : { bottom: '100%', left: 0 }}
      >
        <div className="text-xs text-on-surface-variant">No members found</div>
      </div>
    )
  }

  return (
    <div
      ref={ref}
      className="absolute z-50 bg-surface-container-lowest border border-outline-variant rounded-lg shadow-lg
        py-1 min-w-[200px] max-h-[200px] overflow-y-auto"
      style={position ? { bottom: position.top, left: position.left } : { bottom: '100%', left: 0 }}
    >
      {filtered.map((m, i) => (
        <button
          key={m.user_id || m.ngac_node_id}
          onClick={() => onSelect(m)}
          className={`w-full flex items-center gap-2 px-3 py-1.5 text-left border-none cursor-pointer
            transition-colors text-sm
            ${i === selectedIndex
              ? 'bg-surface-container text-on-surface'
              : 'bg-transparent text-on-surface hover:bg-surface-container/50'}`}
        >
          <Avatar name={m.username || '?'} size="sm" />
          <span className="truncate">{m.username}</span>
        </button>
      ))}
    </div>
  )
}
