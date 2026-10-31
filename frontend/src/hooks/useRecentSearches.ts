import { useCallback, useState } from 'react'
import { toast } from '../components/primitives'
import { addRecent, clearRecent, readRecent, removeRecent, restoreRecent, type SearchModule } from '../lib/recent-searches'
import { useAuthStore } from '../stores/auth.store'
import type { SuggestionGroup } from './useSuggestionPanel'

/** Searches are cleared with a way back; this long to take it. */
const UNDO_MS = 5000

/**
 * Where a list of recent searches lives: a module's own search box, or a scope of the one search of every module
 * (DESIGN.md §6 "Tìm cho mọi tác vụ": Tìm gần đây is kept per scope, Tất cả and Lịch included).
 */
export type RecentScope = SearchModule | 'all' | 'calendar'

/**
 * The store keys a list by its name only (`ngac-recent-searches:<name>:<user>`) and sign-out clears every name by
 * that prefix, so a scope the store's own type does not list is kept, and cleared, exactly like a module.
 */
const storeName = (scope: RecentScope) => scope as SearchModule

/**
 * The signed-in person's recent searches in `scope`, and the "Tìm gần đây" group of its search box. Keep a search
 * with `add` (stable, so handlers that call it need not change) when the box is submitted or a result is opened;
 * `group(pick)` runs a kept search again through `pick`, forgets one with its ✕ and, from two on, all of them at
 * once with a Hoàn tác toast (nothing is lost for good: they are only this device's shortcuts). A change of scope
 * shows that scope's own list.
 */
export function useRecentSearches(scope: RecentScope) {
  const me = useAuthStore((s) => s.user?.id ?? '')
  const module = storeName(scope)
  const [state, setState] = useState(() => ({ me, module, list: readRecent(module, me) }))
  // Another account on the device, or another scope, has its own list.
  if (state.me !== me || state.module !== module) setState({ me, module, list: readRecent(module, me) })
  const list = state.me === me && state.module === module ? state.list : readRecent(module, me)

  const add = useCallback((query: string) => setState({ me, module, list: addRecent(module, me, query) }), [module, me])
  const remove = (query: string) => setState({ me, module, list: removeRecent(module, me, query) })
  const clearAll = () => {
    const held = clearRecent(module, me)
    setState({ me, module, list: [] })
    toast(`Đã xoá ${held.length} lần tìm gần đây`, {
      duration: UNDO_MS,
      action: { label: 'Hoàn tác', onClick: () => setState({ me, module, list: restoreRecent(module, me, held) }) },
    })
  }
  const group = (pick: (query: string) => void): SuggestionGroup => ({
    label: 'Tìm gần đây',
    recent: true,
    onClearAll: list.length >= 2 ? clearAll : undefined,
    items: list.map((q) => ({ key: q, title: q, onSelect: () => pick(q), onRemove: () => remove(q) })),
  })

  return { list, add, group }
}
