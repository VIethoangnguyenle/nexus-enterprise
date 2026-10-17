import { useEffect, useId, useMemo, useRef, useState, type FocusEvent, type KeyboardEvent, type ReactNode, type RefObject } from 'react'
import { Folder, Users } from 'lucide-react'
import { usePhone } from '../../hooks/usePhone'
import { normalize } from '../../lib/people'
import { Button, Pressable, SearchField } from '../primitives'
import { NavSheet } from '../spaces/NavSheet'

/** What a picker row stands for. The id never reaches the screen: it is only what the caller acts on. */
export interface ScopeOption {
  kind: 'role' | 'department'
  id: string
  name: string
  /** "14 người". */
  hint: string
  /** Levels below the root, for the department tree. */
  depth?: number
  /** Why this row cannot be chosen: "Đã ẩn", "Ẩn theo Kế toán", "Đã giới hạn". */
  why?: string
}

export interface ScopeGroup {
  label?: string
  options: ScopeOption[]
}

interface DirectoryScopePickerProps {
  /** The trigger: "Ẩn một phòng". */
  label: string
  icon: ReactNode
  /** Accessible name of the open picker: "Chọn phòng để ẩn". */
  dialogLabel: string
  /** Title of the phone's bottom sheet; the trigger's words. */
  sheetTitle: string
  searchLabel: string
  listLabel: string
  groups: ScopeGroup[]
  footer?: string
  onPick: (option: ScopeOption) => void
  disabled?: boolean
  /** The caller's handle on the trigger, to put focus back on it when the row it came from is gone. */
  triggerRef?: RefObject<HTMLButtonElement | null>
}

const Tile = ({ kind }: { kind: ScopeOption['kind'] }) => (
  <span
    aria-hidden="true"
    className={`grid place-items-center w-6 h-6 rounded-md shrink-0 ${kind === 'department' ? 'bg-accent-wash text-accent' : 'bg-sunk text-ink-muted'}`}
  >
    {kind === 'department' ? <Folder size={14} strokeWidth={1.75} /> : <Users size={14} strokeWidth={1.75} />}
  </span>
)

/**
 * Chooses a department, or a role or a department, by name for a Danh bạ rule (design/mockups/admin.html
 * 7b). On a desktop or tablet it opens in place under its button; on a phone it is a bottom sheet.
 * Rows that cannot be chosen say why on the right. Search ignores accents. Esc closes the picker
 * and only it; a choice closes it and hands focus back to the button, so the question that follows
 * returns focus there too.
 */
export function DirectoryScopePicker({
  label, icon, dialogLabel, sheetTitle, searchLabel, listLabel, groups, footer, onPick, disabled, triggerRef: outerRef,
}: DirectoryScopePickerProps) {
  const phone = usePhone()
  const listId = useId()
  const rootRef = useRef<HTMLDivElement>(null)
  const ownRef = useRef<HTMLButtonElement>(null)
  const triggerRef = outerRef ?? ownRef
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)

  const close = () => {
    setOpen(false)
    setQuery('')
    setActive(0)
  }

  useEffect(() => {
    if (!open || phone) return
    const onPointer = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) close()
    }
    document.addEventListener('pointerdown', onPointer)
    return () => document.removeEventListener('pointerdown', onPointer)
  }, [open, phone])

  const shown = useMemo(() => {
    const q = normalize(query)
    return groups
      .map((g) => ({ ...g, options: g.options.filter((o) => !q || normalize(o.name).includes(q)) }))
      .filter((g) => g.options.length > 0)
  }, [groups, query])

  // The arrows visit the rows that can be chosen, in the order they are listed.
  const choosable = useMemo(() => shown.flatMap((g) => g.options).filter((o) => !o.why), [shown])
  const activeOption = choosable[Math.min(active, choosable.length - 1)]
  const optionId = (o: ScopeOption) => `${listId}-${o.kind}-${o.id}`

  const choose = (o: ScopeOption) => {
    if (o.why) return
    close()
    triggerRef.current?.focus()
    onPick(o)
  }

  const onSearchKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown' && choosable.length) {
      e.preventDefault()
      setActive((i) => (Math.min(i, choosable.length - 1) + 1) % choosable.length)
    } else if (e.key === 'ArrowUp' && choosable.length) {
      e.preventDefault()
      setActive((i) => (Math.min(i, choosable.length - 1) - 1 + choosable.length) % choosable.length)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (activeOption) choose(activeOption)
    }
  }

  // Tabbing out of the open picker closes it, so nothing stays open behind the focus.
  const onBlur = (e: FocusEvent<HTMLDivElement>) => {
    if (open && !phone && !rootRef.current?.contains(e.relatedTarget as Node | null)) close()
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== 'Escape' || !open || phone) return
    e.preventDefault()
    e.stopPropagation()
    e.nativeEvent.stopImmediatePropagation()
    close()
    triggerRef.current?.focus()
  }

  const row = (o: ScopeOption) => (
    <Pressable
      key={`${o.kind}:${o.id}`}
      role="option"
      id={optionId(o)}
      aria-selected={o === activeOption}
      aria-disabled={o.why ? true : undefined}
      onClick={() => choose(o)}
      style={{ paddingLeft: 10 + 16 * (o.depth ?? 0) }}
      className={`grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-x-2.5 w-full pr-2.5 py-1.5 rounded-surface text-sm
        ${phone ? 'min-h-13' : 'min-h-10'} ${o.why ? 'cursor-default text-ink-muted' : 'hover:bg-hover'}
        ${o === activeOption ? 'bg-hover' : ''}`}
    >
      <span className="row-span-2"><Tile kind={o.kind} /></span>
      <span data-name className="truncate text-ink">{o.name}</span>
      {o.why && <span className="row-span-2 col-start-3 row-start-1 text-xs text-ink-muted whitespace-nowrap">{o.why}</span>}
      <small className="col-start-2 text-xs text-ink-muted">{o.hint}</small>
    </Pressable>
  )

  const body = (
    <>
      <div className="px-0.5 pb-1.5">
        <SearchField
          label={searchLabel}
          tone="sunk"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setActive(0)
          }}
          onKeyDown={onSearchKey}
          aria-controls={`${listId}-list`}
          aria-activedescendant={activeOption ? optionId(activeOption) : undefined}
          autoFocus
        />
      </div>
      {shown.length === 0 ? (
        <p className="m-0 px-2.5 py-2 text-sm text-ink-muted">Không có mục nào khớp.</p>
      ) : (
        <div id={`${listId}-list`} role="listbox" aria-label={listLabel} className="grid gap-px content-start">
          {shown.map((g, i) =>
            g.label ? (
              <div key={g.label} role="group" aria-label={g.label} className="grid gap-px">
                <div aria-hidden="true" className={`px-2.5 pb-1 text-label text-ink-muted ${i === 0 ? 'pt-1' : 'pt-3'}`}>{g.label}</div>
                {g.options.map(row)}
              </div>
            ) : (
              g.options.map(row)
            ),
          )}
        </div>
      )}
      {footer && <p className="m-0 px-2.5 pt-2 pb-1 text-xs text-ink-muted">{footer}</p>}
    </>
  )

  return (
    <div ref={rootRef} className="relative justify-self-start" onKeyDown={onKeyDown} onBlur={onBlur}>
      <Button
        ref={triggerRef}
        variant="soft"
        size="sm"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open && !phone ? listId : undefined}
        disabled={disabled}
        onClick={() => (open ? close() : setOpen(true))}
        className="inline-flex items-center gap-1.5 max-sm:h-11"
      >
        {icon}
        {label}
      </Button>
      {open && !phone && (
        <div
          id={listId}
          role="dialog"
          aria-label={dialogLabel}
          className="absolute left-0 top-full mt-1 z-dropdown w-[min(340px,calc(100vw-64px))] max-h-95 overflow-y-auto
            p-1.5 rounded-overlay bg-overlay shadow-overlay"
        >
          {body}
        </div>
      )}
      {open && phone && (
        <NavSheet label={dialogLabel} onClose={close}>
          <div className="px-3.5 pt-1 pb-2">
            <h2 className="m-0 text-lg font-semibold text-ink">{sheetTitle}</h2>
          </div>
          <div className="px-1 pb-2">{body}</div>
        </NavSheet>
      )}
    </div>
  )
}
