import { create } from 'zustand'

/** A notification that has just arrived while the list is open (DESIGN.md §7). */
export interface Fresh {
  /** The person who caused it, for the colour of the wash. */
  actorKey: string
  tag: string
}

export interface Burst {
  count: number
  actors: string[]
}

interface NotificationUiState {
  /** True while a surface showing the list (the desktop panel, or the phone sheet's list) is open. */
  listVisible: boolean
  setListVisible: (v: boolean) => void
  /** Bumped to ask whichever surface fits the screen to open the list. */
  openRequest: number
  requestOpen: () => void
  /** Rows that arrived in the last 2.4 s, by notification id. */
  fresh: Record<string, Fresh>
  /** Set when three or more arrive within 2 s: one summary line instead of per-row tags. */
  burst: Burst | null
  /** What the polite live region says next. */
  announcement: string
  patch: (p: Partial<Pick<NotificationUiState, 'fresh' | 'burst' | 'announcement'>>) => void
}

/** Client-only state of the notifications UI; the data itself is TanStack Query's. */
export const useNotificationUi = create<NotificationUiState>()((set) => ({
  listVisible: false,
  setListVisible: (v) => set({ listVisible: v }),
  openRequest: 0,
  requestOpen: () => set((s) => ({ openRequest: s.openRequest + 1 })),
  fresh: {},
  burst: null,
  announcement: '',
  patch: (p) => set(p),
}))
