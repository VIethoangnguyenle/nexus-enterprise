import { useCallback, useState } from 'react'
import { toast } from '../components/primitives'
import { addRecent, clearRecent, readRecent, removeRecent, restoreRecent, type SearchModule } from '../lib/recent-searches'
import { useAuthStore } from '../stores/auth.store'
import type { SuggestionGroup } from './useSuggestionPanel'

/** Searches are cleared with a way back; this long to take it. */
const UNDO_MS = 5000

/**
 * The signed-in person's recent searches in `module`, and the "Tìm gần đây" group of its search box. Keep a search
 * with `add` (stable, so handlers that call it need not change) when the box is submitted or a result is opened;
 * `group(pick)` runs a kept search again through `pick`, forgets one with its ✕ and, from two on, all of them at
 * once with a Hoàn tác toast (nothing is lost for good: they are only this device's shortcuts).
 */
export function useRecentSearches(module: SearchModule) {
  const me = useAuthStore((s) => s.user?.id ?? '')
  const [state, setState] = useState(() => ({ me, list: readRecent(module, me) }))
  // Another account on the device has its own list.
  if (state.me !== me) setState({ me, list: readRecent(module, me) })
  const { list } = state

  const add = useCallback((query: string) => setState({ me, list: addRecent(module, me, query) }), [module, me])
  const remove = (query: string) => setState({ me, list: removeRecent(module, me, query) })
  const clearAll = () => {
    const held = clearRecent(module, me)
    setState({ me, list: [] })
    toast(`Đã xoá ${held.length} lần tìm gần đây`, {
      duration: UNDO_MS,
      action: { label: 'Hoàn tác', onClick: () => setState({ me, list: restoreRecent(module, me, held) }) },
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
