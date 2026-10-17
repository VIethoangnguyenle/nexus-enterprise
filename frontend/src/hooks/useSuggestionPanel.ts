import { useId, useMemo, useState, type KeyboardEvent, type ReactNode, type RefObject } from 'react'

/** One row a search box suggests before anything is typed. Titles and metas are names, never ids. */
export interface SuggestionItem {
  /** Unique within its group. */
  key: string
  title: string
  /** Second line: who and when ("Lê Quang Vinh · 09:12"). */
  meta?: string
  /** 32px (36 on the phone) leading mark; a recent search gets the clock when left out. */
  icon?: ReactNode
  /** Right side: a time or a pill. */
  trailing?: ReactNode
  onSelect: () => void
  /** Recent searches only: forget this one. */
  onRemove?: () => void
}

export interface SuggestionGroup {
  label: string
  items: SuggestionItem[]
  /** The module's Tìm gần đây; it may show more rows than the other groups. */
  recent?: boolean
  /** Still waiting for the screen's data: the group keeps its place with skeleton rows. */
  loading?: boolean
}

/** `popover` hangs under the box on a desktop; `screen` covers the list under the box on a phone. */
export type SuggestionVariant = 'popover' | 'screen'

/** Most rows a group shows, as [Tìm gần đây, any other group] (DESIGN.md "Gợi ý trong ô tìm"). */
const LIMITS: Record<SuggestionVariant, readonly [number, number]> = { popover: [5, 3], screen: [8, 5] }

/** A shown row, with a key unique across groups so a marked row is found again after the list changes. */
export type ShownItem = SuggestionItem & { uid: string }
export type ShownGroup = Omit<SuggestionGroup, 'items'> & { uid: string; items: ShownItem[] }

interface Options {
  /** What the box holds now: suggestions only show while it is empty. */
  value: string
  groups: SuggestionGroup[]
  variant: SuggestionVariant
  /** The search box; the popover lines up under the control around it. */
  anchorRef: RefObject<HTMLElement | null>
  /**
   * A screen opened only to search (Tìm trong chat) shows its suggestions at once instead of waiting for the box to
   * take the focus, which a browser may give late or, on some phones, not at all.
   */
  initiallyOpen?: boolean
}

export type SuggestionPanel = ReturnType<typeof useSuggestionPanel>

/**
 * State of the suggestions of one search box. Spread `inputProps` on the box and render the rows with
 * `<SearchSuggestions panel={…} />`.
 *
 * - `popover` (desktop): the box becomes a combobox that opens the list on focus while empty and closes it when the
 *   box is left; while open it owns ↑/↓, Enter (only with a row marked), Delete and Esc. Focus never leaves the box.
 * - `screen` (phone): the box stays a plain search box; the rows are buttons and stay up when the keyboard is put away,
 *   until something is typed or the screen calls `close()` (its Quay lại).
 *
 * A screen with its own key handling runs it after `inputProps.onKeyDown` and skips events marked `defaultPrevented`.
 */
export function useSuggestionPanel({ value, groups, variant, anchorRef, initiallyOpen = false }: Options) {
  const id = useId()
  const [engaged, setEngaged] = useState(initiallyOpen)
  const [dismissed, setDismissed] = useState(false)
  const [activeUid, setActiveUid] = useState<string | null>(null)

  // A new value (typing, or emptying the box again) undoes Esc and forgets the marked row.
  const [prevValue, setPrevValue] = useState(value)
  if (value !== prevValue) {
    setPrevValue(value)
    setDismissed(false)
    setActiveUid(null)
  }

  const shown = useMemo<ShownGroup[]>(() => {
    const [recentMax, otherMax] = LIMITS[variant]
    return groups
      .map((g, gi) => ({
        ...g,
        uid: `${gi}`,
        items: g.items.slice(0, g.recent ? recentMax : otherMax).map((it) => ({ ...it, uid: `${gi}:${it.key}` })),
      }))
      .filter((g) => g.items.length > 0 || g.loading)
  }, [groups, variant])
  const items = useMemo(() => shown.flatMap((g) => g.items), [shown])

  const popover = variant === 'popover'
  const open = engaged && !dismissed && value.trim() === ''
  const hasRows = shown.length > 0
  // The phone screen stays up with a hint when there is nothing to suggest; a popover would only be an empty box.
  const visible = open && (hasRows || !popover)
  // The marked row is followed by key: when the suggestions change it stays on the same thing, or on nothing.
  const active = activeUid === null ? -1 : items.findIndex((it) => it.uid === activeUid)
  const optionId = (i: number) => `${id}-${i}`

  const select = (item: SuggestionItem) => {
    setDismissed(true)
    setActiveUid(null)
    item.onSelect()
  }

  const close = () => {
    setEngaged(false)
    setActiveUid(null)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (!visible) return
    if (popover && e.key === 'ArrowDown' && items.length) {
      e.preventDefault()
      setActiveUid(items[(active + 1) % items.length]!.uid)
    } else if (popover && e.key === 'ArrowUp' && items.length) {
      e.preventDefault()
      setActiveUid(items[active <= 0 ? items.length - 1 : active - 1]!.uid)
    } else if (popover && e.key === 'Enter' && active >= 0) {
      e.preventDefault()
      select(items[active]!)
    } else if (popover && e.key === 'Delete' && active >= 0 && items[active]!.onRemove) {
      e.preventDefault()
      items[active]!.onRemove!()
    } else if (popover && e.key === 'Escape') {
      // Close the list without letting a dialog behind the box close too. The phone screen's Esc is its Quay lại.
      e.preventDefault()
      e.stopPropagation()
      e.nativeEvent.stopImmediatePropagation()
      setDismissed(true)
    }
  }

  const common = {
    autoComplete: 'off',
    onFocus: () => {
      setEngaged(true)
      setDismissed(false)
    },
    // Only the popover goes with the focus; the phone screen waits for its Quay lại.
    onBlur: () => {
      if (popover) setEngaged(false)
    },
    // A click into a box that kept the focus (after a choice or Esc) brings the suggestions back.
    onClick: () => {
      setEngaged(true)
      setDismissed(false)
    },
    onKeyDown,
  }

  return {
    id,
    variant,
    anchorRef,
    visible,
    groups: shown,
    active,
    optionId,
    mark: (item: ShownItem) => setActiveUid(item.uid),
    select,
    close,
    inputProps: popover
      ? {
          ...common,
          role: 'combobox' as const,
          'aria-expanded': visible && hasRows,
          'aria-controls': visible && hasRows ? id : undefined,
          'aria-autocomplete': 'list' as const,
          'aria-activedescendant': visible && active >= 0 ? optionId(active) : undefined,
        }
      : common,
  }
}
