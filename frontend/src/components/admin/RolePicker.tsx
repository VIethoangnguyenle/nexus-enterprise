import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { KeyRound, Plus } from 'lucide-react'
import type { RoleSummary } from '../../api/admin'
import { roleName } from '../../lib/admin-model'
import { normalize } from '../../lib/people'
import { Pressable, SearchField } from '../primitives'

interface RolePickerProps {
  /** What the trigger says: "Thêm vai trò". */
  label: string
  /** Roles that can still be chosen. */
  roles: RoleSummary[]
  onPick: (role: RoleSummary) => void
  disabled?: boolean
  /** Shown when there is nothing to choose. */
  emptyText?: string
}

/**
 * Adds one role to a person: a button that opens a search box and the roles by
 * name, each with how many people hold it. In place, not floating, so it works
 * inside a side panel; Esc closes the list and only it. There is no id to type.
 */
export function RolePicker({ label, roles, onPick, disabled, emptyText = 'Không còn vai trò nào để thêm.' }: RolePickerProps) {
  const listId = useId()
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')

  const matches = useMemo(() => {
    const q = normalize(query)
    return roles.filter((r) => !q || normalize(roleName(r)).includes(q))
  }, [roles, query])

  const close = () => {
    setOpen(false)
    setQuery('')
  }

  useEffect(() => {
    if (!open) return
    const onPointer = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) close()
    }
    document.addEventListener('pointerdown', onPointer)
    return () => document.removeEventListener('pointerdown', onPointer)
  }, [open])

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'Escape' || !open) return
    e.preventDefault()
    e.stopPropagation()
    e.nativeEvent.stopImmediatePropagation()
    close()
    triggerRef.current?.focus()
  }

  return (
    <div ref={rootRef} className="grid gap-1.5 justify-items-start min-w-0" onKeyDown={onKeyDown}>
      <Pressable
        ref={triggerRef}
        aria-expanded={open}
        aria-controls={listId}
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        className="inline-flex items-center gap-1.5 h-8 px-2 rounded-md text-small-ui font-semibold text-ink hover:bg-hover"
      >
        <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
        {label}
      </Pressable>
      {open && (
        <div id={listId} className="grid gap-1 p-1.5 rounded-surface bg-overlay shadow-overlay w-full">
          <SearchField label="Tìm vai trò" tone="sunk" value={query} onChange={(e) => setQuery(e.target.value)} autoFocus />
          <div className="max-h-60 overflow-y-auto grid gap-px content-start">
            {matches.length === 0 ? (
              <p className="m-0 px-2.5 py-2 text-sm text-ink-muted">{roles.length === 0 ? emptyText : 'Không có vai trò nào khớp.'}</p>
            ) : (
              matches.map((r) => (
                <Pressable
                  key={r.id}
                  onClick={() => {
                    onPick(r)
                    close()
                    triggerRef.current?.focus()
                  }}
                  className="flex items-center gap-2.5 min-h-10 px-2.5 py-1.5 rounded-surface text-sm hover:bg-hover"
                >
                  <KeyRound size={16} strokeWidth={1.75} className="text-ink-muted shrink-0" aria-hidden="true" />
                  <span className="grid min-w-0">
                    <span className="truncate font-semibold">{roleName(r)}</span>
                    <span className="text-xs text-ink-muted tnum">{r.member_count} thành viên</span>
                  </span>
                </Pressable>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
