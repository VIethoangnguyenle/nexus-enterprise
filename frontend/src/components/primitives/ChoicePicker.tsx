import { useId, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { X } from 'lucide-react'
import { normalize } from '../../lib/people'
import { IconButton } from './IconButton'

export interface Choice {
  /** Opaque key handed back to the caller. Never rendered. */
  id: string
  name: string
}

interface ChoicePickerProps {
  /** Accessible name of the search box, e.g. "Chọn vai trò". */
  label: string
  choices: Choice[]
  value: Choice | null
  onChange: (next: Choice | null) => void
  /** Small icon before the picked name. */
  icon: ReactNode
  placeholder?: string
  /** Shown when nothing matches or there is nothing to pick. */
  emptyText: string
  loading?: boolean
  invalid?: boolean
}

const LIMIT = 8

/**
 * Picker for one role or department (DESIGN.md §6): a search box over the names
 * and a list to choose from. The pick shows as a chip with a remove button.
 * There is no way to type an id.
 *
 * Keyboard: ↓/↑ move, Enter picks, Esc closes the list (and only it).
 */
export function ChoicePicker({
  label, choices, value, onChange, icon, placeholder, emptyText, loading, invalid,
}: ChoicePickerProps) {
  const listId = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)

  const matches = useMemo(() => {
    const q = normalize(query)
    return choices.filter((c) => !q || normalize(c.name).includes(q)).slice(0, LIMIT)
  }, [choices, query])

  const pick = (c: Choice) => {
    onChange(c)
    setQuery('')
    setOpen(false)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setOpen(true)
      if (matches.length) setActive((i) => (i + 1) % matches.length)
    } else if (e.key === 'ArrowUp' && matches.length) {
      e.preventDefault()
      setActive((i) => (i - 1 + matches.length) % matches.length)
    } else if (e.key === 'Enter') {
      const c = matches[active]
      if (open && c) {
        e.preventDefault()
        pick(c)
      }
    } else if (e.key === 'Escape' && open) {
      // Close the list without letting the dialog or panel behind it close too.
      e.preventDefault()
      e.stopPropagation()
      e.nativeEvent.stopImmediatePropagation()
      setOpen(false)
    }
  }

  if (value) {
    return (
      <span className="inline-flex items-center gap-1.5 h-7 pl-2 pr-1 rounded-full bg-sunk text-small-ui text-ink max-w-full">
        <span className="text-ink-muted shrink-0" aria-hidden="true">{icon}</span>
        <span className="truncate">{value.name}</span>
        <IconButton size="sm" aria-label={`Bỏ ${value.name}`} onClick={() => onChange(null)}>
          <X size={14} strokeWidth={1.75} />
        </IconButton>
      </span>
    )
  }

  return (
    <div className="grid gap-1.5">
      <div
        className={`flex items-center min-h-10 px-3 rounded-md bg-raised cursor-text focus-within:field-focus
          ${invalid ? 'field-danger' : 'field-line'}`}
        onClick={() => inputRef.current?.focus()}
      >
        <input
          ref={inputRef}
          role="combobox"
          aria-label={label}
          aria-expanded={open}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-invalid={invalid || undefined}
          aria-activedescendant={open && matches[active] ? `${listId}-${active}` : undefined}
          autoComplete="off"
          value={query}
          placeholder={placeholder ?? 'Tìm theo tên'}
          onChange={(e) => {
            setQuery(e.target.value)
            setOpen(true)
            setActive(0)
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => setOpen(false)}
          onKeyDown={onKeyDown}
          className="flex-1 min-w-20 h-7 bg-transparent border-none outline-none text-base text-ink placeholder:text-ink-muted"
        />
      </div>
      {open && (
        <div id={listId} role="listbox" aria-label="Gợi ý" className="grid p-1 rounded-surface bg-overlay shadow-overlay">
          {loading ? (
            <div className="skeleton h-8 m-1 rounded-sm" aria-busy="true" />
          ) : matches.length === 0 ? (
            <div className="px-2.5 py-2 text-sm text-ink-muted">{emptyText}</div>
          ) : (
            matches.map((c, i) => (
              <div
                key={c.id}
                id={`${listId}-${i}`}
                role="option"
                aria-selected={i === active}
                // Keep focus in the box: pick on mousedown, before blur closes the list.
                onMouseDown={(e) => {
                  e.preventDefault()
                  pick(c)
                }}
                onMouseEnter={() => setActive(i)}
                className={`flex items-center gap-2.5 px-2.5 py-2 rounded-md cursor-pointer text-sm
                  ${i === active ? 'bg-hover' : ''}`}
              >
                <span className="text-ink-muted shrink-0" aria-hidden="true">{icon}</span>
                <span className="truncate text-ink">{c.name}</span>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  )
}
