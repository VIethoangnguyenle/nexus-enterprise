import { create } from 'zustand'

type ViewMode = 'list' | 'grid'
type ContextTab = 'preview' | 'metadata' | 'permissions' | 'activity'

interface DriveState {
  // --- Selection ---
  selectedItemId: string | null
  selectedItemIds: Set<string>
  selectItem: (id: string | null) => void
  toggleSelectItem: (id: string) => void
  clearSelection: () => void

  // --- View ---
  viewMode: ViewMode
  setViewMode: (mode: ViewMode) => void

  // --- Context Panel ---
  contextPanelOpen: boolean
  contextPanelTab: ContextTab
  openContextPanel: (tab?: ContextTab) => void
  closeContextPanel: () => void
  setContextPanelTab: (tab: ContextTab) => void

  // --- Folder Tree ---
  expandedFolders: Set<string>
  activePath: string[]
  currentFolderId: string | null
  toggleFolder: (folderId: string) => void
  setActivePath: (path: string[]) => void
  navigateToFolder: (folderId: string | null) => void
}

export const useDriveStore = create<DriveState>()((set, get) => ({
  // Selection
  selectedItemId: null,
  selectedItemIds: new Set(),
  selectItem: (id) => {
    set({
      selectedItemId: id,
      selectedItemIds: id ? new Set([id]) : new Set(),
      contextPanelOpen: !!id,
      contextPanelTab: id ? get().contextPanelTab : 'preview',
    })
  },
  toggleSelectItem: (id) => {
    const selected = new Set(get().selectedItemIds)
    if (selected.has(id)) {
      selected.delete(id)
    } else {
      selected.add(id)
    }
    set({
      selectedItemIds: selected,
      selectedItemId: selected.size === 1 ? [...selected][0] : null,
    })
  },
  clearSelection: () => {
    set({ selectedItemId: null, selectedItemIds: new Set(), contextPanelOpen: false })
  },

  // View
  viewMode: 'list',
  setViewMode: (mode) => set({ viewMode: mode }),

  // Context Panel
  contextPanelOpen: false,
  contextPanelTab: 'preview',
  openContextPanel: (tab) => set({ contextPanelOpen: true, contextPanelTab: tab ?? 'preview' }),
  closeContextPanel: () => set({ contextPanelOpen: false }),
  setContextPanelTab: (tab) => set({ contextPanelTab: tab }),

  // Folder Tree
  expandedFolders: new Set(),
  activePath: [],
  currentFolderId: null,
  toggleFolder: (folderId) => {
    const expanded = new Set(get().expandedFolders)
    if (expanded.has(folderId)) {
      expanded.delete(folderId)
    } else {
      expanded.add(folderId)
    }
    set({ expandedFolders: expanded })
  },
  setActivePath: (path) => set({ activePath: path }),
  navigateToFolder: (folderId) => set({ currentFolderId: folderId, selectedItemId: null, selectedItemIds: new Set(), contextPanelOpen: false }),
}))
