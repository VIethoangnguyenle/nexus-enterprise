import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface NavCollapseState {
  /** Sections folded in the chat navigator, by `workspace:section`. Remembered on this device. */
  collapsed: Record<string, true>
  setCollapsed: (key: string, collapsed: boolean) => void
}

/** Client-only UI state: which navigator sections are folded (DESIGN.md "Chat: cột điều hướng và mục"). */
export const useNavCollapseStore = create<NavCollapseState>()(
  persist(
    (set) => ({
      collapsed: {},
      setCollapsed: (key, collapsed) =>
        set((s) => {
          const next = { ...s.collapsed }
          if (collapsed) next[key] = true
          else delete next[key]
          return { collapsed: next }
        }),
    }),
    { name: 'ngac-nav-collapse' },
  ),
)

export const collapseKey = (workspaceId: string, section: string) => `${workspaceId}:${section}`
