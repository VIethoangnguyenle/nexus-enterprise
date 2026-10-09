import { useId, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { Avatar } from './Avatar'
import { PersonChip } from './PersonChip'
import { matchesPerson, type Person } from '../../lib/people'

interface PeoplePickerProps {
  /** Accessible name of the search box, e.g. "Thêm người". */
  label: string
  /** Everyone who can be picked. */
  people: Person[]
  value: Person[]
  onChange: (next: Person[]) => void
  /** User ids that may not be picked (yourself, existing members). */
  exclude?: Set<string>
  /** Stop offering suggestions once this many are picked. */
  max?: number
  placeholder?: string
  autoFocus?: boolean
}

const LIMIT = 6

/**
 * Picker for people (DESIGN.md §6): a search box that matches names, titles
 * and departments, with picks shown as PersonChips. There is no way to type
 * an id; the ids live only inside `Person` for the caller's API calls.
 *
 * Keyboard: ↑/↓ move, Enter picks, Backspace on an empty box removes the last
 * pick, Esc closes the suggestions (and only them).
 */
export function PeoplePicker({
  label, people, value, onChange, exclude, max, placeholder, autoFocus,
}: PeoplePickerProps) {
  const listId = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)

  const picked = useMemo(() => new Set(value.map((p) => p.userId)), [value])
  const full = max !== undefined && value.length >= max
  const suggestions = useMemo(() => {
    if (!query.trim() || full) return []
    return people
      .filter((p) => !picked.has(p.userId) && !exclude?.has(p.userId) && matchesPerson(p, query))
      .slice(0, LIMIT)
  }, [people, picked, exclude, query, full])

  const showList = open && query.trim().length > 0 && !full

  const pick = (p: Person) => {
    onChange([...value, p])
    setQuery('')
    setActive(0)
    inputRef.current?.focus()
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown' && suggestions.length) {
      e.preventDefault()
      setOpen(true)
      setActive((i) => (i + 1) % suggestions.length)
    } else if (e.key === 'ArrowUp' && suggestions.length) {
      e.preventDefault()
      setActive((i) => (i - 1 + suggestions.length) % suggestions.length)
    } else if (e.key === 'Enter') {
      const s = suggestions[active]
      if (showList && s) {
        e.preventDefault()
        pick(s)
      }
    } else if (e.key === 'Escape' && showList) {
      // Close the list without letting the dialog behind it close too.
      e.preventDefault()
      e.stopPropagation()
      e.nativeEvent.stopImmediatePropagation()
      setOpen(false)
    } else if (e.key === 'Backspace' && !query && value.length) {
      onChange(value.slice(0, -1))
    }
  }

  return (
    <div className="grid gap-1.5">
      <div
        className="flex flex-wrap items-center gap-1.5 min-h-10 px-3 py-1.5 rounded-md bg-raised field-line
          focus-within:field-focus cursor-text"
        onClick={() => inputRef.current?.focus()}
      >
        {value.map((p) => (
          <PersonChip
            key={p.userId || p.nodeId}
            variant="pill"
            name={p.name}
            hueKey={p.userId}
            avatarUrl={p.avatarUrl}
            onRemove={() => onChange(value.filter((v) => v !== p))}
          />
        ))}
        {!full && (
          <input
            ref={inputRef}
            role="combobox"
            aria-label={label}
            aria-expanded={showList}
            aria-controls={listId}
            aria-autocomplete="list"
            aria-activedescendant={showList && suggestions[active] ? `${listId}-${active}` : undefined}
            autoComplete="off"
            autoFocus={autoFocus}
            value={query}
            placeholder={value.length ? '' : placeholder ?? 'Tìm theo tên hoặc phòng ban'}
            onChange={(e) => {
              setQuery(e.target.value)
              setOpen(true)
              setActive(0)
            }}
            onFocus={() => setOpen(true)}
            onBlur={() => setOpen(false)}
            onKeyDown={onKeyDown}
            className="flex-1 min-w-32 h-7 bg-transparent border-none outline-none text-base text-ink
              placeholder:text-ink-muted"
          />
        )}
      </div>
      {showList && (
        <div
          id={listId}
          role="listbox"
          aria-label="Gợi ý"
          className="grid p-1 rounded-surface bg-overlay shadow-overlay"
        >
          {suggestions.length === 0 ? (
            <div className="px-2.5 py-2 text-sm text-ink-muted">Không tìm thấy ai khớp “{query.trim()}”.</div>
          ) : (
            suggestions.map((p, i) => (
              <div
                key={p.userId || p.nodeId}
                id={`${listId}-${i}`}
                role="option"
                aria-selected={i === active}
                // Keep focus in the box: pick on mousedown, before blur closes the list.
                onMouseDown={(e) => {
                  e.preventDefault()
                  pick(p)
                }}
                onMouseEnter={() => setActive(i)}
                className={`grid grid-cols-[24px_minmax(0,1fr)_auto] items-center gap-2.5 px-2.5 py-2 rounded-md
                  cursor-pointer text-sm ${i === active ? 'bg-hover' : ''}`}
              >
                <Avatar name={p.name} hueKey={p.userId} src={p.avatarUrl} size={24} />
                <span className="truncate text-ink">{p.name}</span>
                {p.role && <span className="truncate text-xs text-ink-muted">{p.role}</span>}
              </div>
            ))
          )}
        </div>
      )}
    </div>
  )
}
