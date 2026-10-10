import { useCallback, useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { Building2, ChevronDown } from 'lucide-react'
import type { Department } from '../../api/admin'
import { childrenIndex, deptAncestors, deptPathLabel } from '../../lib/admin-model'
import { normalize } from '../../lib/people'
import { Pressable, SearchField } from '../primitives'
import { TreeView, type TreeNode } from '../composites/TreeView'

interface DepartmentPickerProps {
  /** Accessible name of the field, e.g. "Phòng ban". */
  label: string
  departments: Department[]
  /** The chosen department; empty for none. */
  value: string
  onChange: (id: string) => void
  /** What "none" is called: "Không thuộc phòng nào", "Không có (phòng gốc)". */
  noneLabel: string
  /** Departments that cannot be chosen (a department's own subtree). */
  exclude?: ReadonlySet<string>
  disabled?: boolean
}

const asNode = (d: Department, index: Map<string, Department[]>): TreeNode => ({
  id: d.id,
  label: d.name,
  hasChildren: (index.get(d.id) ?? []).length > 0,
})

/**
 * Chooses one department by its name (DESIGN.md §6, Picker). The field shows the
 * department's path from the root; opening it lists the departments as a tree,
 * or, once something is typed, as a flat list of matches with their paths. There
 * is no way to type an id.
 *
 * The list opens in place rather than as a popover, so it works the same inside
 * a dialog or a side panel, where a floating layer would sit under its parent.
 * Esc closes the list and only it.
 */
export function DepartmentPicker({ label, departments, value, onChange, noneLabel, exclude, disabled }: DepartmentPickerProps) {
  const listId = useId()
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')

  const allowed = useMemo(() => departments.filter((d) => !exclude?.has(d.id)), [departments, exclude])
  const index = useMemo(() => childrenIndex(allowed), [allowed])
  const roots = useMemo(() => (index.get('') ?? []).map((d) => asNode(d, index)), [index])

  // Open on the way to the current choice, so it is visible without searching.
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(value ? deptAncestors(departments, value) : []))
  const toggle = useCallback((id: string) => setExpanded((cur) => {
    const next = new Set(cur)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    return next
  }), [])
  const useChildren = useCallback(
    (parentId: string) => ({ nodes: (index.get(parentId) ?? []).map((d) => asNode(d, index)), isLoading: false, isError: false }),
    [index],
  )

  const matches = useMemo(() => {
    const q = normalize(query)
    if (!q) return []
    return allowed.filter((d) => normalize(deptPathLabel(departments, d.id)).includes(q))
  }, [allowed, departments, query])

  const close = () => {
    setOpen(false)
    setQuery('')
  }
  const choose = (id: string) => {
    onChange(id)
    close()
    triggerRef.current?.focus()
  }

  // A pointer press outside the picker closes it.
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
    // Close the list without letting the dialog or panel around it close too.
    e.preventDefault()
    e.stopPropagation()
    e.nativeEvent.stopImmediatePropagation()
    close()
    triggerRef.current?.focus()
  }

  const shown = value ? deptPathLabel(departments, value) : ''

  return (
    <div ref={rootRef} className="grid gap-1.5 min-w-0" onKeyDown={onKeyDown}>
      <Pressable
        ref={triggerRef}
        aria-label={label}
        aria-expanded={open}
        aria-controls={listId}
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        className={`flex items-center gap-2 min-h-10 px-3 rounded-md bg-raised w-full min-w-0 text-base
          ${open ? 'field-focus' : 'field-line'}`}
      >
        <Building2 size={16} strokeWidth={1.75} className="text-ink-muted shrink-0" aria-hidden="true" />
        <span className={`flex-1 truncate ${shown ? 'text-ink' : 'text-ink-muted'}`}>{shown || noneLabel}</span>
        <ChevronDown size={16} strokeWidth={1.75} className="text-ink-muted shrink-0" aria-hidden="true" />
      </Pressable>

      {open && (
        <div id={listId} className="grid gap-1 p-1.5 rounded-surface bg-overlay shadow-overlay">
          <SearchField label="Tìm phòng ban" tone="sunk" value={query} onChange={(e) => setQuery(e.target.value)} autoFocus />
          <div className="max-h-60 overflow-y-auto grid gap-px content-start">
            {!query.trim() && (
              <Pressable
                aria-pressed={!value}
                onClick={() => choose('')}
                className={`flex items-center h-9 px-2.5 rounded-surface text-sm hover:bg-hover
                  ${!value ? 'bg-raised font-semibold' : 'text-ink-muted'}`}
              >
                {noneLabel}
              </Pressable>
            )}
            {query.trim() ? (
              matches.length === 0 ? (
                <p className="m-0 px-2.5 py-2 text-sm text-ink-muted">Không có phòng ban nào khớp.</p>
              ) : (
                matches.map((d) => (
                  <Pressable
                    key={d.id}
                    aria-pressed={d.id === value}
                    onClick={() => choose(d.id)}
                    className={`flex items-center gap-2 min-h-9 px-2.5 py-1.5 rounded-surface text-sm hover:bg-hover
                      ${d.id === value ? 'bg-raised font-semibold' : ''}`}
                  >
                    <Building2 size={16} strokeWidth={1.75} className="text-ink-muted shrink-0" aria-hidden="true" />
                    <span className="truncate">{deptPathLabel(departments, d.id)}</span>
                  </Pressable>
                ))
              )
            ) : roots.length === 0 ? (
              <p className="m-0 px-2.5 py-2 text-sm text-ink-muted">Chưa có phòng ban nào.</p>
            ) : (
              <TreeView
                label={`${label}: danh sách phòng ban`}
                roots={roots}
                useChildren={useChildren}
                expanded={expanded}
                onToggle={toggle}
                selectedId={value || null}
                onSelect={choose}
                icon={() => <Building2 size={16} strokeWidth={1.75} className="text-ink-muted shrink-0" aria-hidden="true" />}
                emptyLabel="Không có phòng ban con"
              />
            )}
          </div>
        </div>
      )}
    </div>
  )
}
