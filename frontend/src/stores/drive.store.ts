import { create } from 'zustand'

/**
 * Client-only state of Tài liệu. Where the user is (the open folder, the
 * workspace, the shared view) lives in the URL, not here: that is what makes
 * reload, Back and a pasted link land in the same place.
 */
interface DriveState {
  /** The item shown in the detail panel; null closes the panel. */
  selectedItemId: string | null
  selectItem: (id: string | null) => void

  /** Folders opened in the tree. Survives leaving Tài liệu and coming back. */
  expandedFolders: ReadonlySet<string>
  toggleFolder: (folderId: string) => void
  /** Open several at once, e.g. every ancestor of the folder the URL names. */
  expandFolders: (folderIds: string[]) => void
}

export const useDriveStore = create<DriveState>()((set) => ({
  selectedItemId: null,
  selectItem: (id) => set({ selectedItemId: id }),

  expandedFolders: new Set<string>(),
  toggleFolder: (folderId) =>
    set((s) => {
      const next = new Set(s.expandedFolders)
      if (next.has(folderId)) next.delete(folderId)
      else next.add(folderId)
      return { expandedFolders: next }
    }),
  expandFolders: (folderIds) =>
    set((s) => {
      if (folderIds.every((id) => s.expandedFolders.has(id))) return s
      return { expandedFolders: new Set([...s.expandedFolders, ...folderIds]) }
    }),
}))
