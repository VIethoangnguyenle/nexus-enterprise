import { create } from 'zustand'

interface PwaState {
  /** A new service worker is installed and waiting to take over. */
  needRefresh: boolean
  /** Activate the waiting worker and reload the page onto it. */
  applyUpdate: () => void
}

export const usePwaStore = create<PwaState>()(() => ({
  needRefresh: false,
  applyUpdate: () => window.location.reload(),
}))
