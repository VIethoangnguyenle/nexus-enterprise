import { useState } from 'react'
import { addRecent, readRecent, removeRecent, type SearchModule } from '../lib/recent-searches'
import { useAuthStore } from '../stores/auth.store'
import type { SuggestionGroup } from './useSuggestionPanel'

/**
 * The signed-in person's recent searches in `module`, and the "Tìm gần đây" group of its search box. Keep a search
 * with `add` when the box is submitted or a result is opened; `group(pick)` runs a kept search again through `pick`.
 */
export function useRecentSearches(module: SearchModule) {
  const me = useAuthStore((s) => s.user?.id ?? '')
  const [state, setState] = useState(() => ({ me, list: readRecent(module, me) }))
  // Another account on the device has its own list.
  if (state.me !== me) setState({ me, list: readRecent(module, me) })
  const { list } = state

  const add = (query: string) => setState({ me, list: addRecent(module, me, query) })
  const remove = (query: string) => setState({ me, list: removeRecent(module, me, query) })
  const group = (pick: (query: string) => void): SuggestionGroup => ({
    label: 'Tìm gần đây',
    recent: true,
    items: list.map((q) => ({ key: q, title: q, onSelect: () => pick(q), onRemove: () => remove(q) })),
  })

  return { list, add, group }
}
